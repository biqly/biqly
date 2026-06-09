package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/bytedance/sonic"
	"strings"

	"github.com/biqly/biqly/internal/ai"
	"github.com/biqly/biqly/internal/ai/routing"
	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/core"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/platform/observability"
	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/semantic"
)

type AIJobProgress struct {
	Phase    string
	Message  string
	Progress int
	Status   string
	Detail   json.RawMessage
}

type AIJobProgressFunc func(p AIJobProgress)

func (h *AIHandler) resolveAIQuery(ctx context.Context, req aiQueryRequest) (*semantic.SemanticModel, *routing.TableRoutingResult, *ai.Response, error) {
	if req.Question == "" {
		return nil, nil, nil, errors.New("question is required")
	}
	if req.DatasourceID == "" {
		return nil, nil, nil, errors.New(core.MsgDatasourceIDRequired)
	}
	model, routeResult, err := h.loadQueryModel(ctx, req)
	if err != nil {
		return nil, nil, nil, err
	}
	if routeResult != nil && routeResult.NeedsClarification {
		if h.metrics != nil {
			h.metrics.RecordAmbiguityTier("0")
		}
		return nil, routeResult, clarificationResponse(routeResult), nil
	}
	return model, routeResult, nil, nil
}

func (h *AIHandler) executeAIQueryPhase(
	ctx context.Context,
	req aiQueryRequest,
	phase aiQueryPhase,
	report AIJobProgressFunc,
) (*ai.Response, error) {
	if report != nil {
		report(AIJobProgress{Phase: "routing", Message: "routing tables", Progress: 10, Status: metadata.AIJobStatusRunning})
	}
	model, routeResult, clarify, err := h.resolveAIQuery(ctx, req)
	if err != nil {
		return nil, err
	}
	if clarify != nil {
		return clarify, nil
	}
	pc := buildProcessContext(req)
	if err := h.resolveProcessContext(ctx, pc, model); err != nil {
		return nil, err
	}
	pc.ApplyToRequest(&req)

	if report != nil {
		report(AIJobProgress{Phase: "generating", Message: "generating logical query", Progress: 35, Status: metadata.AIJobStatusRunning})
	}

	var resolved *app.ResolvedDatasource
	var processOpts []ai.ProcessOption
	if phase == aiPhaseRun {
		var resolveErr error
		resolved, processOpts, resolveErr = h.resolveRunPhaseForJob(ctx, pc, req, model)
		if resolveErr != nil {
			return nil, resolveErr
		}
		if resolved != nil {
			defer closeResolvedDatasource(ctx, resolved)
		}
	} else {
		processOpts = h.standardProcessOptions(ctx, pc, req, model)
	}

	if report != nil {
		report(AIJobProgress{Phase: "validating", Message: "validating response", Progress: 55, Status: metadata.AIJobStatusRunning})
	}

	resp, err := h.processAIQuestion(ctx, pc, req, model, routeResult, processOpts...)
	if err != nil {
		return nil, err
	}
	attachGenerationTrace(routeResult, model, resp)

	switch phase {
	case aiPhaseGenerate:
		return resp, nil
	case aiPhasePreview:
		if report != nil {
			report(AIJobProgress{Phase: "compiling", Message: "compiling sql", Progress: 75, Status: metadata.AIJobStatusRunning})
		}
		return h.finishAIPreviewResult(ctx, req, model, resp)
	case aiPhaseRun:
		if report != nil {
			report(AIJobProgress{Phase: "executing", Message: "executing query", Progress: 85, Status: metadata.AIJobStatusRunning})
		}
		return h.finishAIRunResult(ctx, req, model, resp, resolved)
	default:
		return resp, nil
	}
}

func (h *AIHandler) finishAIPreviewResult(ctx context.Context, req aiQueryRequest, model *semantic.SemanticModel, resp *ai.Response) (*ai.Response, error) {
	var logicalQuery *query.LogicalQuery
	if resp != nil && resp.Result != nil {
		logicalQuery = resp.Result.LogicalQuery
	}
	if logicalQuery == nil {
		return resp, nil
	}
	if resp.Result == nil {
		resp.Result = &ai.AIResult{}
	}

	if h.deps.QueryClient != nil {
		compiled, err := h.deps.QueryClient.DryRun(ctx, logicalQuery)
		if err != nil {
			resp.Result.Warnings = append(resp.Result.Warnings, "compilation failed")
		} else {
			resp.Result.SQL = compiled.SQL
			resp.Result.Args = compiled.Args
		}
		return resp, nil
	}
	resolved, err := h.deps.ResolveDatasourceDB(ctx, req.DatasourceID)
	if err != nil {
		return nil, err
	}
	defer closeResolvedDatasource(ctx, resolved)
	cq, se := h.deps.QueryService.CompileWithContext(ctx, logicalQuery, model, resolved.Driver)
	if se != nil {
		resp.Result.Warnings = append(resp.Result.Warnings, "compilation failed")
	} else {
		resp.Result.SQL = cq.SQL
		resp.Result.Args = cq.Args
	}
	return resp, nil
}

func (h *AIHandler) finishAIRunResult(ctx context.Context, req aiQueryRequest, model *semantic.SemanticModel, resp *ai.Response, resolved *app.ResolvedDatasource) (*ai.Response, error) {
	var logicalQuery *query.LogicalQuery
	if resp != nil && resp.Result != nil {
		logicalQuery = resp.Result.LogicalQuery
	}
	if logicalQuery == nil {
		return resp, nil
	}
	if resp.Result == nil {
		resp.Result = &ai.AIResult{}
	}

	if h.deps.QueryClient != nil {
		return h.finishAIRunResultWithQueryClient(ctx, resp, model)
	}
	if resolved == nil {
		var err error
		resolved, err = h.deps.ResolveDatasourceDB(ctx, req.DatasourceID)
		if err != nil {
			return nil, err
		}
		defer closeResolvedDatasource(ctx, resolved)
	}
	driver := resolved.Driver
	db := resolved.DB
	cq, se := h.deps.QueryService.CompileWithContext(ctx, logicalQuery, model, driver)
	if se != nil {
		persistQueryHistory(ctx, h.deps.MetaRepo, logicalQuery, model, nil, nil, queryStatusFailed, core.ErrAsError(se))
		return nil, core.ErrAsError(se)
	}
	resp.Result.SQL = cq.SQL
	resp.Result.Args = cq.Args
	if fp, fpErr := query.LogicalQueryFingerprint(logicalQuery, model); fpErr == nil {
		ctx = observability.WithQueryFingerprint(ctx, fp)
	}
	result, err := h.deps.Executor.Execute(ctx, db, cq)
	if err != nil {
		persistQueryHistory(ctx, h.deps.MetaRepo, logicalQuery, model, cq, nil, queryStatusFailed, err)
		return nil, err
	}
	enrichAIRunResponse(resp, result, logicalQuery, model)
	persistQueryHistory(ctx, h.deps.MetaRepo, logicalQuery, model, cq, result, queryStatusSuccess, nil)
	return resp, nil
}

func (h *AIHandler) finishAIRunResultWithQueryClient(ctx context.Context, resp *ai.Response, model *semantic.SemanticModel) (*ai.Response, error) {
	var logicalQuery *query.LogicalQuery
	if resp != nil && resp.Result != nil {
		logicalQuery = resp.Result.LogicalQuery
	}
	if logicalQuery == nil {
		return resp, nil
	}
	if resp.Result == nil {
		resp.Result = &ai.AIResult{}
	}

	run, err := h.deps.QueryClient.Run(ctx, logicalQuery, 0, 0)
	if err != nil {
		return nil, err
	}
	resp.Result.SQL = run.SQL
	result := &query.Result{
		Columns: run.Columns,
		Rows:    run.Rows,
		Stats: query.Stats{
			RowCount:   run.RowCount,
			DurationMs: run.DurationMs,
		},
	}
	enrichAIRunResponse(resp, result, logicalQuery, model)
	return resp, nil
}

func enrichAIRunResponse(resp *ai.Response, result *query.Result, logicalQuery *query.LogicalQuery, model *semantic.SemanticModel) {
	if resp == nil || result == nil || logicalQuery == nil {
		return
	}
	if resp.Result == nil {
		resp.Result = &ai.AIResult{}
	}
	query.EnrichResult(result, logicalQuery, model)
	chartType, reason := query.VisualizationHintFromResult(result)
	resp.Result.VisualizationHint = &ai.VisualizationHint{ChartType: chartType, Reason: reason}
	if anomalyWarnings := query.AnomalyWarningMessages(result); len(anomalyWarnings) > 0 {
		resp.Result.Warnings = append(resp.Result.Warnings, anomalyWarnings...)
	}
	resp.Result.Result = result
}

func aiJobPhaseFromKind(kind string) (aiQueryPhase, error) {
	switch kind {
	case "query":
		return aiPhaseGenerate, nil
	case "preview":
		return aiPhasePreview, nil
	case "run":
		return aiPhaseRun, nil
	default:
		return 0, fmt.Errorf("unknown job kind %q", kind)
	}
}

func encodeAIJobResult(resp *ai.Response) (json.RawMessage, error) {
	if resp == nil {
		return nil, nil
	}
	b, err := sonic.ConfigStd.Marshal(resp)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (h *AIHandler) executeMetadataDescribeJob(
	ctx context.Context,
	req ai.DescribeRequest,
	report AIJobProgressFunc,
) (*ai.DescribeResult, error) {
	if report != nil {
		report(AIJobProgress{Phase: "sampling", Message: "sampling table metadata", Progress: 15, Status: metadata.AIJobStatusRunning})
		report(AIJobProgress{Phase: "generating", Message: "generating metadata descriptions", Progress: 35, Status: metadata.AIJobStatusRunning})
	}
	result, err := h.executeMetadataDescribe(ctx, req)
	if err != nil {
		return nil, err
	}
	if report != nil {
		report(AIJobProgress{Phase: "applying", Message: "saving generated descriptions", Progress: 90, Status: metadata.AIJobStatusRunning})
	}
	return result, nil
}

func encodeDescribeJobResult(result *ai.DescribeResult) (json.RawMessage, error) {
	if result == nil {
		return nil, nil
	}
	b, err := sonic.ConfigStd.Marshal(result)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func encodeDescribeBatchJobResult(result *ai.DescribeBatchResult) (json.RawMessage, error) {
	if result == nil {
		return nil, nil
	}
	b, err := sonic.ConfigStd.Marshal(result)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (h *AIHandler) isAIJobCancelled(ctx context.Context, jobID string) bool {
	if jobID == "" || h.deps.MetaRepo == nil {
		return false
	}
	job, err := h.deps.MetaRepo.GetAIJob(ctx, jobID)
	if err != nil {
		return false
	}
	return job.Status == metadata.AIJobStatusCancelled
}

func (h *AIHandler) executeMetadataDescribeBatchJob(
	ctx context.Context,
	jobID string,
	req ai.DescribeBatchRequest,
	report AIJobProgressFunc,
) (*ai.DescribeBatchResult, error) {
	if req.DatasourceID == "" || len(req.Tables) == 0 {
		return nil, errors.New("datasource_id and tables are required")
	}

	existingDesc, err := h.loadDescribedTableKeys(ctx, req)
	if err != nil {
		return nil, err
	}

	out := &ai.DescribeBatchResult{Entries: make([]ai.DescribeBatchEntryResult, 0, len(req.Tables))}
	total := len(req.Tables)
	completedKeys := make([]string, 0, total)
	for i, target := range req.Tables {
		if h.isAIJobCancelled(ctx, jobID) {
			return out, errors.New("job cancelled")
		}
		schema := strings.TrimSpace(target.Schema)
		table := strings.TrimSpace(target.Table)
		if schema == "" || table == "" {
			out.Entries = append(out.Entries, ai.DescribeBatchEntryResult{
				Schema: schema, Table: table, Status: "error", Message: "schema and table are required",
			})
			out.Error++
			completedKeys = append(completedKeys, ai.DescribeBatchTableKey(schema, table))
			continue
		}
		key := schema + "." + table
		if req.SkipExisting && existingDesc[key] {
			out.Entries = append(out.Entries, ai.DescribeBatchEntryResult{
				Schema: schema, Table: table, Status: "skipped", Message: "already has description",
			})
			out.Skipped++
			completedKeys = append(completedKeys, key)
			continue
		}

		reportDescribeBatchProgress(report, req.Tables, total, i, schema, table, key, completedKeys)
		entry := h.runDescribeBatchTable(ctx, req, schema, table)
		out.Entries = append(out.Entries, entry)
		switch entry.Status {
		case "error":
			out.Error++
		case "ok":
			out.OK++
		}
		completedKeys = append(completedKeys, key)
	}

	if report != nil {
		report(AIJobProgress{Phase: "applying", Message: "batch complete", Progress: 100, Status: metadata.AIJobStatusRunning})
	}
	return out, nil
}

func (h *AIHandler) loadDescribedTableKeys(ctx context.Context, req ai.DescribeBatchRequest) (map[string]bool, error) {
	existingDesc := map[string]bool{}
	if !req.SkipExisting || h.deps.MetaRepo == nil {
		return existingDesc, nil
	}
	tables, err := h.deps.MetaRepo.ListTables(ctx, req.DatasourceID, "")
	if err != nil {
		return nil, err
	}
	for _, t := range tables {
		if t.Description != nil && strings.TrimSpace(*t.Description) != "" {
			existingDesc[t.SchemaName+"."+t.TableName] = true
		}
	}
	return existingDesc, nil
}

func (h *AIHandler) runDescribeBatchTable(ctx context.Context, req ai.DescribeBatchRequest, schema, table string) ai.DescribeBatchEntryResult {
	single := ai.DescribeRequest{
		DatasourceID: req.DatasourceID,
		Schema:       schema,
		Table:        table,
		SampleSize:   req.SampleSize,
		AutoApply:    req.AutoApply,
	}
	result, err := h.executeMetadataDescribeJob(ctx, single, nil)
	if err != nil {
		return ai.DescribeBatchEntryResult{
			Schema: schema, Table: table, Status: "error", Message: err.Error(),
		}
	}
	cols := 0
	if result != nil {
		cols = len(result.Columns)
	}
	return ai.DescribeBatchEntryResult{
		Schema: schema, Table: table, Status: "ok",
		Message: fmt.Sprintf("%d columns described", cols),
		Result:  result,
	}
}

func reportDescribeBatchProgress(report AIJobProgressFunc, tables []ai.DescribeBatchTable, total, index int, schema, table, key string, completedKeys []string) {
	if report == nil {
		return
	}
	denom := max(total, 1)
	pct := 5 + (index * 90 / denom)
	detail, err := sonic.ConfigStd.Marshal(ai.DescribeBatchJobProgress{
		Total:          total,
		Index:          index,
		CurrentSchema:  schema,
		CurrentTable:   table,
		Completed:      append([]string(nil), completedKeys...),
		PendingPreview: describeBatchPendingPreview(tables, index),
	})
	if err != nil {
		detail = nil
	}
	report(AIJobProgress{
		Phase:    "generating",
		Message:  "describing " + key,
		Progress: pct,
		Status:   metadata.AIJobStatusRunning,
		Detail:   detail,
	})
}

func describeBatchPendingPreview(tables []ai.DescribeBatchTable, fromIndex int) []string {
	nextPreview := make([]string, 0, 5)
	for j := fromIndex + 1; j < len(tables) && len(nextPreview) < 5; j++ {
		ns := strings.TrimSpace(tables[j].Schema)
		nt := strings.TrimSpace(tables[j].Table)
		if ns == "" || nt == "" {
			continue
		}
		nextPreview = append(nextPreview, ai.DescribeBatchTableKey(ns, nt))
	}
	return nextPreview
}

func (h *AIHandler) executeEmbedMetadataJob(
	ctx context.Context,
	req embedMetadataRequest,
	report AIJobProgressFunc,
) (*embedMetadataResponse, error) {
	if report != nil {
		report(AIJobProgress{Phase: "fetching", Message: "fetching tables for embedding", Progress: 10, Status: metadata.AIJobStatusRunning})
	}

	if h.deps.AIEmbedMeta == nil || h.deps.Embedder == nil {
		return nil, errors.New("embeddings are not configured")
	}

	var (
		results []ai.EmbedTableResult
		err     error
	)
	if req.ModelID != "" {
		model, ferr := h.deps.SemanticRepo.GetFullModel(ctx, req.ModelID)
		if ferr != nil {
			return nil, fmt.Errorf("semantic model not found: %w", ferr)
		}
		allowed := tablesForModel(model)
		if report != nil {
			report(AIJobProgress{Phase: "embedding", Message: fmt.Sprintf("computing embeddings for %d tables", len(allowed)), Progress: 30, Status: metadata.AIJobStatusRunning})
		}
		results, err = h.deps.AIEmbedMeta.EmbedForTables(ctx, req.DatasourceID, allowed)
	} else {
		if report != nil {
			report(AIJobProgress{Phase: "embedding", Message: "computing embeddings for all tables in datasource", Progress: 30, Status: metadata.AIJobStatusRunning})
		}
		results, err = h.deps.AIEmbedMeta.EmbedAllForDatasource(ctx, req.DatasourceID)
	}

	if err != nil {
		return nil, err
	}

	if report != nil {
		report(AIJobProgress{Phase: "completing", Message: "saving vector embeddings", Progress: 90, Status: metadata.AIJobStatusRunning})
	}

	embedded, skipped := 0, 0
	for _, r := range results {
		if r.Skipped {
			skipped++
		} else {
			embedded++
		}
	}

	return &embedMetadataResponse{
		DatasourceID: req.DatasourceID,
		ModelID:      req.ModelID,
		Model:        h.deps.Embedder.Model(),
		Embedded:     embedded,
		Skipped:      skipped,
		Results:      results,
	}, nil
}
