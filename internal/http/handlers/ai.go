// Package handlers provides HTTP handlers for the BI query engine API.
package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/bytedance/sonic"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"github.com/biqly/biqly/internal/ai"
	"github.com/biqly/biqly/internal/ai/abtest"
	"github.com/biqly/biqly/internal/ai/prompt"
	"github.com/biqly/biqly/internal/ai/routing"
	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/core"
	"github.com/biqly/biqly/internal/dialect"
	bimw "github.com/biqly/biqly/internal/http/middleware"
	"github.com/biqly/biqly/internal/i18n"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/platform/observability"
	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/semantic"
	"github.com/google/uuid"
)

type aiQueryPhase int

const (
	aiPhaseGenerate aiQueryPhase = iota
	aiPhasePreview
	aiPhaseRun
)

type clarificationState struct {
	Question string
	Round    int
}

// AIHandler handles AI text-to-query operations.
type AIHandler struct {
	service              *ai.Service
	tableRouter          *routing.TableRouter
	deps                 *app.AIDeps
	authClient           *bimw.AuthClient
	metrics              AIMetricsRecorder
	activeClarifications sync.Map

	// Runtime config override caches (see ai_admin_config.go) memoize the
	// DB-managed domains so per-request reads stay off the database.
	ambiguityOverridesCache runtimeOverrides[ambiguityOverrides]
	memoryOverridesCache    runtimeOverrides[memoryOverrides]
	queueOverridesCache     runtimeOverrides[queueOverrides]
}

// SetAuthClient wires the auth service client for user model access checks.
func (h *AIHandler) SetAuthClient(c *bimw.AuthClient) {
	h.authClient = c
}

// SetAIMetricsRecorder wires process-level counters (e.g. Prometheus /metrics).
func (h *AIHandler) SetAIMetricsRecorder(m AIMetricsRecorder) {
	h.metrics = m
}

// NewAIHandler creates a new AI handler.
func NewAIHandler(deps *app.AIDeps) *AIHandler {
	provider := deps.AIQueryClient
	if provider == nil {
		provider = deps.AIClient
	}
	svc := ai.NewServiceWithProvider(new(deps.Config.AI.ResolvedQuery().Config), deps.Validator, provider).WithCache(deps.ResponseCache)
	metadataReader := routing.MetadataReader(deps.MetaRepo)
	var embeddingReader routing.EmbeddingReader = deps.MetaRepo
	if deps.CatalogClient != nil {
		metadataReader = deps.CatalogClient
	}
	router := routing.NewTableRouterWithEmbeddings(
		metadataReader,
		deps.Embedder,
		embeddingReader,
		deps.Config.AI.Embedding.Weight,
	)
	router.SetMetadataTranslator(deps.MetaRepo)
	router.SetTimeGrainStore(deps.TimeGrains)
	router.SetRoutingLimits(routing.LimitsFromConfig(
		deps.Config.AI.Routing.MaxDimensions,
		deps.Config.AI.Routing.MaxMetrics,
		deps.Config.AI.Routing.MaxColumnsPerTable,
		deps.Config.AI.Routing.MaxDateGrainExtras,
		deps.Config.AI.Routing.SlimNumericMetrics,
	))
	return &AIHandler{
		service:     svc,
		tableRouter: router,
		deps:        deps,
	}
}

func queryModelLabelFromUserPreference(
	ctx context.Context,
	store *ai.ProviderStore,
	authClient *bimw.AuthClient,
	userID string,
) string {
	models, prefs, _, err := userSelectableModels(ctx, store, authClient, userID)
	if err != nil {
		slog.WarnContext(ctx, "resolve user ai model label failed", "user_id", userID, "error", err)
		return ""
	}
	prefID := strings.TrimSpace(prefs[string(ai.PurposeQuery)])
	if prefID == "" {
		return ""
	}
	for _, m := range models {
		if m.ID != prefID {
			continue
		}
		if display := strings.TrimSpace(m.DisplayName); display != "" {
			return display
		}
		if modelID := strings.TrimSpace(m.ModelID); modelID != "" {
			return modelID
		}
	}
	return ""
}

func (h *AIHandler) queryModelUsedLabel(ctx context.Context) string {
	if h.deps.AIProviderStore != nil && h.authClient != nil {
		if userID := ai.UserIDFromContext(ctx); userID != "" {
			if label := queryModelLabelFromUserPreference(ctx, h.deps.AIProviderStore, h.authClient, userID); label != "" {
				return label
			}
		}
	}
	if h.deps.AIProviderStore != nil {
		return h.deps.AIProviderStore.ModelLabelForPurpose(ai.PurposeQuery)
	}
	return h.deps.Config.AI.ResolvedQuery().Config.Connection.Model
}

type aiQueryRequest struct {
	DatasourceID        string   `json:"datasource_id"`
	ModelID             string   `json:"model_id,omitempty"`
	CompositeID         string   `json:"composite_id,omitempty"`
	Question            string   `json:"question"`
	Tables              []string `json:"tables,omitempty"`
	ClarificationChoice string   `json:"clarification_choice,omitempty"`
	// ClarificationRound counts ambiguity clarification rounds already shown to
	// the user on this question thread. The client echoes the value from the
	// previous ambiguity response.
	ClarificationRound int `json:"clarification_round,omitempty"`
	// IncludeBaseTables / IncludeViews default to true when omitted (backward compatible).
	IncludeBaseTables *bool `json:"include_base_tables,omitempty"`
	IncludeViews      *bool `json:"include_views,omitempty"`
	// PriorTurns are recent (question, logical_query) pairs from the active
	// conversation, sent by the frontend so follow-ups can be resolved in
	// context ("filter that to last quarter", "now group by region").
	PriorTurns []priorTurnPayload `json:"prior_turns,omitempty"`
	// ExampleIDs lets the frontend specify which few-shot examples to use
	ExampleIDs []string `json:"example_ids,omitempty"`
	// IncludePastQueries when true, attaches recent conversation turns as
	// few-shot examples (frontend-side toggle).
	IncludePastQueries bool `json:"include_past_queries,omitempty"`
	// AutoFindSkills toggles the automatic embedding-RAG few-shot recall
	// (appendConfirmedFewShot). Nil or true preserves current behavior; false
	// skips auto-recall so only explicitly selected saved queries ground the
	// prompt. Optional — omitted means auto-find on.
	AutoFindSkills *bool `json:"auto_find_skills,omitempty"`
	// SavedQueryIDs are saved queries the user explicitly selected in the
	// composer ("/"-picker). They are injected as strong few-shot grounding for
	// this question, datasource-scoped server-side. May be empty.
	SavedQueryIDs []string `json:"saved_query_ids,omitempty"`
}

// priorTurnPayload is the wire shape for one entry in aiQueryRequest.PriorTurns.
// LogicalQuery is sent as raw JSON if available; an empty value is fine
// (e.g. when the prior turn never produced a valid query).
type priorTurnPayload struct {
	Question      string          `json:"question"`
	LogicalQuery  json.RawMessage `json:"logical_query,omitempty"`
	Note          string          `json:"note,omitempty"`
	ResultSummary string          `json:"result_summary,omitempty"`
}

// maxPriorTurns caps how many turns we forward to the LLM. Older turns drop
// off so the prompt stays bounded regardless of how long the conversation runs.
const maxPriorTurns = 5

// autoFindSkills reports whether the automatic embedding-RAG few-shot recall
// should run for this request. It defaults to true (current behavior) when the
// client omits auto_find_skills, so existing callers are unaffected.
func autoFindSkills(req aiQueryRequest) bool {
	return req.AutoFindSkills == nil || *req.AutoFindSkills
}

// priorTurnsForPrompt converts wire-format turns into the AI service's
// ConversationTurn slice, taking the most recent maxPriorTurns entries.
func priorTurnsForPrompt(payload []priorTurnPayload) []prompt.ConversationTurn {
	if len(payload) == 0 {
		return nil
	}
	start := 0
	if len(payload) > maxPriorTurns {
		start = len(payload) - maxPriorTurns
	}
	out := make([]prompt.ConversationTurn, 0, len(payload)-start)
	for _, t := range payload[start:] {
		out = append(out, prompt.ConversationTurn{
			Question:      t.Question,
			LogicalQuery:  string(t.LogicalQuery),
			Note:          t.Note,
			ResultSummary: t.ResultSummary,
		})
	}
	return out
}

// parseAndRouteAIQuery decodes the request, validates required fields, loads the semantic
// model (and table routing). If it writes a response to w (bad request, model load error, or
// clarification-only response), ok is false.
func (h *AIHandler) parseAndRouteAIQuery(w http.ResponseWriter, r *http.Request) (aiQueryRequest, *ProcessContext, *semantic.SemanticModel, *routing.TableRoutingResult, bool) {
	req, ok := decodeJSON[aiQueryRequest](w, r)
	if !ok {
		return aiQueryRequest{}, nil, nil, nil, false
	}
	return h.routeAIQueryRequest(r.Context(), w, *req)
}

// routeAIQueryRequest validates required fields, loads the semantic model (and
// table routing) and resolves the process context for an already-decoded
// request. Shared by the HTTP query phases and history replay. If it writes a
// response to w (bad request, model load error, or clarification-only
// response), ok is false.
func (h *AIHandler) routeAIQueryRequest(ctx context.Context, w http.ResponseWriter, req aiQueryRequest) (aiQueryRequest, *ProcessContext, *semantic.SemanticModel, *routing.TableRoutingResult, bool) {
	if req.Question == "" {
		writeError(w, http.StatusBadRequest, "question is required")
		return req, nil, nil, nil, false
	}
	if req.DatasourceID == "" {
		writeError(w, http.StatusBadRequest, core.MsgDatasourceIDRequired)
		return req, nil, nil, nil, false
	}

	if req.ClarificationChoice == "" {
		h.checkAndRecordAbandon(ctx)
	}
	routeStart := time.Now()
	model, routeResult, err := h.loadQueryModel(ctx, req)
	routeMs := time.Since(routeStart).Milliseconds()
	if h.metrics != nil {
		h.metrics.RecordAIStep("table_route", routeMs)
	}
	if err != nil {
		h.writeModelLoadError(ctx, w, req, err)
		return req, nil, nil, nil, false
	}
	if resp, ok := h.tierZeroClarificationIfNeeded(ctx, req, model, routeResult); ok {
		writeJSON(w, http.StatusOK, resp)
		return req, nil, nil, nil, false
	}
	pc := buildProcessContext(req)
	pc.routeDurationMs = routeMs
	ctxStart := time.Now()
	if err := h.resolveProcessContext(ctx, pc, model); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return req, pc, nil, nil, false
	}
	pc.contextResolveMs = time.Since(ctxStart).Milliseconds()
	if h.metrics != nil {
		h.metrics.RecordAIStep("context_resolve", pc.contextResolveMs)
	}
	pc.ApplyToRequest(&req)
	return req, pc, model, routeResult, true
}

func (h *AIHandler) checkAndRecordAbandon(ctx context.Context) {
	userID := bimw.UserID(ctx)
	if userID == "" {
		return
	}
	if _, exists := h.activeClarifications.Load(userID); exists {
		if h.metrics != nil {
			h.metrics.RecordAmbiguityResolution("abandoned")
		}
		h.activeClarifications.Delete(userID)
	}
}

func (h *AIHandler) standardProcessOptions(ctx context.Context, pc *ProcessContext, req aiQueryRequest, model *semantic.SemanticModel) []ai.ProcessOption {
	question := req.Question
	if pc != nil && pc.Question != "" {
		question = pc.Question
	}
	promptCtxStart := time.Now()
	catalog, external := h.loadGlossaryEntries(ctx, model)
	opts := make([]ai.ProcessOption, 0, 8)
	fewShot, recallHits := h.loadFewShotExamples(ctx, model, question, autoFindSkills(req), req.SavedQueryIDs)
	if h.metrics != nil {
		h.metrics.RecordAIStep("prompt_context", time.Since(promptCtxStart).Milliseconds())
	}
	if pc != nil {
		pc.SetMemoryRecallHitCount(recallHits)
	}
	opts = append(opts,
		ai.WithTargetDialect(h.datasourceDialectName(ctx, req.DatasourceID)),
		ai.WithFewShotExamples(fewShot),
		ai.WithPriorTurns(priorTurnsForPrompt(req.PriorTurns)),
		ai.WithGlossary(prompt.SelectGlossaryForQuestion(question, prompt.MergeGlossaryEntries(catalog, external), model)),
		ai.WithAmbiguityGlossary(combineGlossaryEntries(catalog, external)),
	)
	if memories := h.loadMemoryFacts(ctx); len(memories) > 0 {
		opts = append(opts, ai.WithMemories(memories))
	}
	if model != nil {
		if instructions := h.loadInstructions(ctx, model.DatasourceID); len(instructions) > 0 {
			opts = append(opts, ai.WithInstructions(instructions))
		}
	}
	ambiguityCfg := h.effectiveAmbiguityConfig(ctx)
	if pc != nil && pc.ShouldUseInteractiveTier(ambiguityCfg) {
		slog.InfoContext(ctx, "ambiguity interactive tier engaged",
			"clarification_round", pc.clarificationRound,
		)
	} else if pc != nil && pc.AmbiguityCapReached(ambiguityCfg) {
		slog.WarnContext(ctx, "ambiguity round cap reached, bypassing check",
			"clarification_round", pc.clarificationRound,
			"max_rounds", maxClarificationRounds,
		)
		if h.metrics != nil {
			h.metrics.RecordAmbiguityRoundCapReached()
		}
	}
	var ambiguityObserver ai.AmbiguityAnalysisObserver
	var tierRecorder func(tier string)
	var stepObserver ai.AIStepObserver
	if h.metrics != nil {
		ambiguityObserver = h.metrics.RecordAmbiguityAnalysis
		tierRecorder = h.metrics.RecordAmbiguityTier
		stepObserver = h.metrics.RecordAIStep
	}
	if stepObserver != nil {
		opts = append(opts, ai.WithAIStepObserver(stepObserver))
	}
	opts = append(opts, ambiguityProcessOptions(ambiguityCfg, pc, ambiguityObserver, tierRecorder)...)
	return opts
}

func (h *AIHandler) processAIQuestion(
	ctx context.Context,
	pc *ProcessContext,
	req aiQueryRequest,
	model *semantic.SemanticModel,
	routeResult *routing.TableRoutingResult,
	extra ...ai.ProcessOption,
) (*ai.Response, error) {
	start := time.Now()
	if userID := bimw.UserID(ctx); userID != "" {
		ctx = ai.WithUserID(ctx, userID)
	}
	tracker := &abtest.ExperimentTracker{}
	ctx = abtest.WithExperimentTracker(ctx, tracker)

	question := req.Question
	if pc != nil && pc.Question != "" {
		question = pc.Question
	}
	rec := ai.NewRunRecorder()
	if pc != nil {
		rec.Record("table_route", ai.RunStepStatusOK, 0, pc.routeDurationMs, "")
		rec.Record("context_resolve", ai.RunStepStatusOK, 0, pc.contextResolveMs, "")
	}
	opts := h.standardProcessOptions(ctx, pc, req, model)
	opts = append(opts, ai.WithRunRecorder(rec))
	opts = append(opts, extra...)
	resp, err := h.service.ProcessQuestion(ctx, question, model, opts...)
	if resp != nil {
		if resp.Metadata == nil {
			resp.Metadata = &ai.AIMetadata{}
		}
		resp.Metadata.ModelUsed = h.queryModelUsedLabel(ctx)
		resp.Metadata.TableRouting = routeResult
		resp.Metadata.RunSteps = rec.Steps()

		// Populate resolved A/B experiment metadata if tracked
		if tracked := tracker.GetVariants(); len(tracked) > 0 {
			resp.Metadata.ABExperimentID = tracked[0].ExperimentID
			resp.Metadata.ABVariantID = tracked[0].ID
		}
	}
	if err != nil {
		failed := failedAIResponse(err)
		failed.Metadata = &ai.AIMetadata{RunSteps: rec.Steps()}
		h.observeAIRequest(ctx, req, model, routeResult, failed, time.Since(start).Milliseconds(), pc)
		return nil, err
	}
	if resp == nil {
		resp = failedAIResponse(errors.New("ai response missing"))
	}
	attachAmbiguityClarificationRound(pc, resp)
	return h.observeAIRequest(ctx, req, model, routeResult, resp, time.Since(start).Milliseconds(), pc), nil
}

// processAndObserve is the shared entry for Query, Preview, and Run: parse/route,
// optional Run-phase datasource pool, LLM process + telemetry, then phase-specific compile/execute.
func (h *AIHandler) processAndObserve(w http.ResponseWriter, r *http.Request, phase aiQueryPhase) {
	req, pc, model, routeResult, ok := h.parseAndRouteAIQuery(w, r)
	if !ok {
		return
	}
	ctx := r.Context()

	workspaceID := bimw.WorkspaceID(ctx)
	if err := h.deps.SpendLimiter.Check(ctx, workspaceID); err != nil {
		writeError(w, http.StatusTooManyRequests, "workspace AI token budget exceeded for today")
		return
	}

	var resolved *app.ResolvedDatasource
	var processOpts []ai.ProcessOption
	if phase == aiPhaseRun {
		var ok bool
		resolved, processOpts, ok = h.applyRunPhaseForHTTP(ctx, w, req, model, pc)
		if !ok {
			return
		}
		if resolved != nil {
			defer closeResolvedDatasource(ctx, resolved)
		}
	}

	resp, err := h.processAIQuestion(ctx, pc, req, model, routeResult, processOpts...)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to process question", err,
			"question", req.Question,
			"model_id", req.ModelID,
			"datasource_id", req.DatasourceID,
		)
		return
	}
	if resp != nil && resp.Metadata != nil && resp.Metadata.TokenUsage != nil {
		h.deps.SpendLimiter.Record(ctx, workspaceID, resp.Metadata.TokenUsage.Total)
	}

	switch phase {
	case aiPhaseGenerate:
		writeJSON(w, http.StatusOK, resp)
	case aiPhasePreview:
		h.finishAIPreview(ctx, w, req, model, resp)
	case aiPhaseRun:
		question := req.Question
		if pc != nil && pc.Question != "" {
			question = pc.Question
		}
		if h.deps.QueryClient != nil {
			h.finishAIRunWithQueryClient(ctx, w, resp, model, question)
			return
		}
		runDatasource := resolved
		if runDatasource == nil {
			writeInternalError(ctx, w, http.StatusInternalServerError, "datasource not resolved", errors.New("datasource not resolved"))
			return
		}
		h.finishAIRun(ctx, w, model, resp, runDatasource, question)
	}
}

func (h *AIHandler) applyRunPhaseForHTTP(
	ctx context.Context,
	w http.ResponseWriter,
	req aiQueryRequest,
	model *semantic.SemanticModel,
	pc *ProcessContext,
) (*app.ResolvedDatasource, []ai.ProcessOption, bool) {
	resolved, processOpts, err := h.resolveRunPhaseProcessOptions(ctx, pc, req, model)
	if err == nil {
		return resolved, processOpts, true
	}
	if resolved != nil {
		closeResolvedDatasource(ctx, resolved)
		writeInternalError(ctx, w, http.StatusInternalServerError, "datasource not resolved", err)
		return nil, nil, false
	}
	writeCoreServiceError(ctx, w, err)
	return nil, nil, false
}

func (h *AIHandler) resolveRunPhaseProcessOptions(ctx context.Context, pc *ProcessContext, req aiQueryRequest, model *semantic.SemanticModel) (*app.ResolvedDatasource, []ai.ProcessOption, error) {
	if h.deps.QueryClient != nil {
		fewShot, recallHits := h.loadFewShotExamplesWithIDs(ctx, model, req.Question, req.ExampleIDs, req.IncludePastQueries, autoFindSkills(req), req.SavedQueryIDs)
		if pc != nil {
			pc.SetMemoryRecallHitCount(recallHits)
		}
		return nil, []ai.ProcessOption{
			ai.WithSQLValidator(newQueryClientDryRunValidator(h.deps.QueryClient, model)),
			ai.WithTargetDialect(h.datasourceDialectName(ctx, req.DatasourceID)),
			ai.WithFewShotExamples(fewShot),
		}, nil
	}
	resolved, err := h.deps.ResolveDatasourceDB(ctx, req.DatasourceID)
	if err != nil {
		return nil, nil, err
	}
	processOpts, err := h.localRunProcessOptions(ctx, pc, req, model, resolved)
	if err != nil {
		return resolved, nil, err
	}
	return resolved, processOpts, nil
}

func (h *AIHandler) resolveRunPhaseForJob(
	ctx context.Context,
	pc *ProcessContext,
	req aiQueryRequest,
	model *semantic.SemanticModel,
) (*app.ResolvedDatasource, []ai.ProcessOption, error) {
	resolved, processOpts, err := h.resolveRunPhaseProcessOptions(ctx, pc, req, model)
	if err == nil {
		return resolved, processOpts, nil
	}
	if resolved != nil {
		closeResolvedDatasource(ctx, resolved)
	}
	return nil, nil, err
}

func closeResolvedDatasource(ctx context.Context, resolved *app.ResolvedDatasource) {
	if resolved == nil || resolved.DB == nil {
		return
	}
	if closeErr := resolved.DB.Close(); closeErr != nil {
		slog.ErrorContext(ctx, "failed to close database connection", "error", closeErr)
	}
}

func (h *AIHandler) localRunProcessOptions(ctx context.Context, pc *ProcessContext, req aiQueryRequest, model *semantic.SemanticModel, resolved *app.ResolvedDatasource) ([]ai.ProcessOption, error) {
	if resolved == nil || resolved.DB == nil || resolved.Driver == nil {
		return nil, errors.New("datasource not resolved")
	}

	driver := resolved.Driver
	db := resolved.DB
	targetDialect := driver.Dialect()
	fewShot, recallHits := h.loadFewShotExamplesWithIDs(ctx, model, req.Question, req.ExampleIDs, req.IncludePastQueries, autoFindSkills(req), req.SavedQueryIDs)
	if pc != nil {
		pc.SetMemoryRecallHitCount(recallHits)
	}
	return []ai.ProcessOption{
		ai.WithSQLValidator(newSQLDryRunValidator(h.deps.QueryService, db, driver, model)),
		ai.WithTargetDialect(targetDialect.Name()),
		ai.WithFewShotExamples(fewShot),
		ai.WithSampleData(h.loadSampleData(ctx, db, targetDialect, model)),
	}, nil
}

func (h *AIHandler) finishAIPreview(ctx context.Context, w http.ResponseWriter, req aiQueryRequest, model *semantic.SemanticModel, resp *ai.Response) {
	var logicalQuery *query.LogicalQuery
	if resp != nil && resp.Result != nil {
		logicalQuery = resp.Result.LogicalQuery
	}
	if logicalQuery == nil {
		writeJSON(w, http.StatusOK, resp)
		return
	}
	if resp.Result == nil {
		resp.Result = &ai.AIResult{}
	}

	if h.deps.QueryClient != nil {
		compileStart := time.Now()
		compiled, err := h.deps.QueryClient.DryRunWithModel(ctx, logicalQuery, inlineAutoModel(model))
		if h.metrics != nil {
			h.metrics.RecordAIStep("query_compile", time.Since(compileStart).Milliseconds())
		}
		if err != nil {
			slog.ErrorContext(ctx, "AI preview query service dry-run failed", "error", err)
			resp.Result.Warnings = append(resp.Result.Warnings, "compilation failed")
		} else {
			resp.Result.SQL = compiled.SQL
			resp.Result.Args = compiled.Args
		}
		writeJSON(w, http.StatusOK, resp)
		return
	}

	resolved, err := h.deps.ResolveDatasourceDB(ctx, req.DatasourceID)
	if err != nil {
		writeCoreServiceError(ctx, w, err)
		return
	}
	defer closeResolvedDatasource(ctx, resolved)

	compileStart := time.Now()
	cq, se := h.deps.QueryService.CompileWithContext(ctx, logicalQuery, model, resolved.Driver)
	if h.metrics != nil {
		h.metrics.RecordAIStep("query_compile", time.Since(compileStart).Milliseconds())
	}
	if se != nil {
		slog.ErrorContext(ctx, "AI preview compilation failed", "error", core.LogCause(se),
			"model_id", model.ID,
			"datasource_id", model.DatasourceID,
		)
		resp.Result.Warnings = append(resp.Result.Warnings, "compilation failed")
	} else {
		resp.Result.SQL = cq.SQL
		resp.Result.Args = cq.Args
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *AIHandler) finishAIRun(ctx context.Context, w http.ResponseWriter, model *semantic.SemanticModel, resp *ai.Response, resolved *app.ResolvedDatasource, question string) {
	var logicalQuery *query.LogicalQuery
	if resp != nil && resp.Result != nil {
		logicalQuery = resp.Result.LogicalQuery
	}
	if logicalQuery == nil {
		writeJSON(w, http.StatusOK, resp)
		return
	}
	if resp.Result == nil {
		resp.Result = &ai.AIResult{}
	}

	if h.deps.QueryClient != nil {
		h.finishAIRunWithQueryClient(ctx, w, resp, model, question)
		return
	}
	if resolved == nil || resolved.DB == nil || resolved.Driver == nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "datasource not resolved", errors.New("datasource not resolved"))
		return
	}

	driver := resolved.Driver
	db := resolved.DB

	compileStart := time.Now()
	cq, se := h.deps.QueryService.CompileWithContext(ctx, logicalQuery, model, driver)
	if h.metrics != nil {
		h.metrics.RecordAIStep("query_compile", time.Since(compileStart).Milliseconds())
	}
	if se != nil {
		persistQueryHistory(ctx, h.deps.MetaRepo, logicalQuery, model, nil, nil, queryStatusFailed, core.ErrAsError(se))
		writeServiceError(ctx, w, se,
			"model_id", model.ID,
			"datasource_id", model.DatasourceID,
		)
		return
	}

	resp.Result.SQL = cq.SQL
	resp.Result.Args = cq.Args

	if fp, fpErr := query.LogicalQueryFingerprint(logicalQuery, model); fpErr == nil {
		ctx = observability.WithQueryFingerprint(ctx, fp)
	}
	execStart := time.Now()
	result, err := h.deps.Executor.Execute(ctx, db, cq, driver != nil && driver.SupportsReadOnlyTx())
	if h.metrics != nil {
		h.metrics.RecordAIStep("query_execute", time.Since(execStart).Milliseconds())
	}
	if err != nil {
		persistQueryHistory(ctx, h.deps.MetaRepo, logicalQuery, model, cq, nil, queryStatusFailed, err)
		writeInternalError(ctx, w, http.StatusInternalServerError, "execution failed", err,
			"sql", cq.SQL,
			"args", fmt.Sprintf("%v", cq.Args),
			"model_id", model.ID,
			"datasource_id", model.DatasourceID,
		)
		return
	}

	query.EnrichResult(result, logicalQuery, model)
	chartType, reason := query.VisualizationHintFromResult(result)
	resp.Result.VisualizationHint = &ai.VisualizationHint{ChartType: chartType, Reason: reason}
	if anomalyWarnings := query.AnomalyWarningMessages(result); len(anomalyWarnings) > 0 {
		resp.Result.Warnings = append(resp.Result.Warnings, anomalyWarnings...)
	}
	resp.Result.Result = result
	h.attachAINaturalLanguageAnswer(ctx, resp, question)
	persistQueryHistory(ctx, h.deps.MetaRepo, logicalQuery, model, cq, result, queryStatusSuccess, nil)
	writeJSON(w, http.StatusOK, resp)
}

func (h *AIHandler) finishAIRunWithQueryClient(ctx context.Context, w http.ResponseWriter, resp *ai.Response, model *semantic.SemanticModel, question string) {
	var logicalQuery *query.LogicalQuery
	if resp != nil && resp.Result != nil {
		logicalQuery = resp.Result.LogicalQuery
	}
	if logicalQuery == nil {
		writeJSON(w, http.StatusOK, resp)
		return
	}
	if resp.Result == nil {
		resp.Result = &ai.AIResult{}
	}

	execStart := time.Now()
	run, err := h.deps.QueryClient.RunWithModel(ctx, logicalQuery, inlineAutoModel(model), 0, 0)
	if h.metrics != nil {
		h.metrics.RecordAIStep("query_execute", time.Since(execStart).Milliseconds())
	}
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "query service execution failed", err)
		return
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
	query.EnrichResult(result, logicalQuery, model)
	chartType, reason := query.VisualizationHintFromResult(result)
	resp.Result.VisualizationHint = &ai.VisualizationHint{ChartType: chartType, Reason: reason}
	if anomalyWarnings := query.AnomalyWarningMessages(result); len(anomalyWarnings) > 0 {
		resp.Result.Warnings = append(resp.Result.Warnings, anomalyWarnings...)
	}
	resp.Result.Result = result
	h.attachAINaturalLanguageAnswer(ctx, resp, question)
	writeJSON(w, http.StatusOK, resp)
}

// attachAINaturalLanguageAnswer synthesizes a short natural-language answer from
// the executed result and attaches it to resp.Result.Answer. Best-effort and
// gated: it is skipped when answer synthesis is disabled (BI_AI_ANSWER_ENABLED),
// when there is no executed result, or when the workspace has hit its daily
// token budget. It never fails the request. Call it only after the executed
// result (resp.Result.Result) has been populated.
func (h *AIHandler) attachAINaturalLanguageAnswer(ctx context.Context, resp *ai.Response, question string) {
	if resp == nil || resp.Result == nil || resp.Result.Result == nil {
		return
	}
	if h.service == nil || !h.service.AnswerEnabled() {
		return
	}
	if h.deps.SpendLimiter != nil {
		if err := h.deps.SpendLimiter.Check(ctx, bimw.WorkspaceID(ctx)); err != nil {
			return
		}
	}
	locale := string(prompt.LocaleForQuestion(question, i18n.FromContext(ctx)))
	resp.Result.Answer = h.service.SynthesizeAnswer(ctx, question, locale, resp.Result.Result)
}

// Query handles AI-powered natural language queries.
func (h *AIHandler) Query(w http.ResponseWriter, r *http.Request) {
	h.processAndObserve(w, r, aiPhaseGenerate)
}

// Preview handles AI query preview (compiles but does not execute).
func (h *AIHandler) Preview(w http.ResponseWriter, r *http.Request) {
	h.processAndObserve(w, r, aiPhasePreview)
}

// Run handles AI query execution (compiles and executes).
func (h *AIHandler) Run(w http.ResponseWriter, r *http.Request) {
	h.processAndObserve(w, r, aiPhaseRun)
}

// Describe runs the AI metadata describer over a single table and (optionally) writes
// the suggested table/column descriptions back into the metadata DB.
// Describe handles AI-powered table/column description generation.
func (h *AIHandler) Describe(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[ai.DescribeRequest](w, r)
	if !ok {
		return
	}
	if req.DatasourceID == "" || req.Table == "" {
		writeError(w, http.StatusBadRequest, "datasource_id and table are required")
		return
	}

	ctx := r.Context()
	if loc := strings.TrimSpace(req.Locale); loc != "" {
		ctx = i18n.WithLocale(ctx, i18n.ParseLocale(loc))
	}
	result, err := h.executeMetadataDescribe(ctx, *req)
	if err != nil {
		writeCoreServiceError(r.Context(), w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *AIHandler) executeMetadataDescribe(ctx context.Context, req ai.DescribeRequest) (*ai.DescribeResult, error) {
	return h.deps.AIDescriber.Describe(ctx, req)
}

type embedMetadataRequest struct {
	DatasourceID    string `json:"datasource_id"`
	ModelID         string `json:"model_id,omitempty"`
	ClientSessionID string `json:"client_session_id,omitempty"`
}

type embedMetadataResponse struct {
	DatasourceID string                `json:"datasource_id"`
	ModelID      string                `json:"model_id,omitempty"`
	Model        string                `json:"model"`
	Embedded     int                   `json:"embedded"`
	Skipped      int                   `json:"skipped"`
	Results      []ai.EmbedTableResult `json:"results,omitempty"`
}

// EmbedMetadata computes vector embeddings for every table and column in a
// datasource and stores them so the AI router can use hybrid retrieval.
// Idempotent — re-runs simply overwrite the prior vectors.
func (h *AIHandler) EmbedMetadata(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[embedMetadataRequest](w, r)
	if !ok {
		return
	}
	if req.DatasourceID == "" {
		writeError(w, http.StatusBadRequest, core.MsgDatasourceIDRequired)
		return
	}
	if h.deps.AIEmbedMeta == nil || h.deps.Embedder == nil {
		writeError(w, http.StatusServiceUnavailable, "embeddings are not configured (set BI_AI_EMBEDDING_MODEL and API access: BI_AI_EMBEDDING_API_KEY or BI_AI_API_KEY; optional BI_AI_EMBEDDING_BASE_URL / BI_AI_BASE_URL)")
		return
	}
	ctx := r.Context()

	if req.ClientSessionID != "" {
		h.enqueueEmbedMetadataJob(ctx, w, req)
		return
	}

	var (
		results []ai.EmbedTableResult
		err     error
	)
	if req.ModelID != "" {
		model, ferr := h.deps.SemanticRepo.GetFullModel(ctx, req.ModelID)
		if ferr != nil {
			writeEntityNotFound(w, "semantic model")
			return
		}
		allowed := tablesForModel(model)
		results, err = h.deps.AIEmbedMeta.EmbedForTables(ctx, req.DatasourceID, allowed)
	} else {
		results, err = h.deps.AIEmbedMeta.EmbedAllForDatasource(ctx, req.DatasourceID)
	}
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "embedding failed", err)
		return
	}
	embedded, skipped := 0, 0
	for _, r := range results {
		if r.Skipped {
			skipped++
		} else {
			embedded++
		}
	}
	writeJSON(w, http.StatusOK, embedMetadataResponse{
		DatasourceID: req.DatasourceID,
		ModelID:      req.ModelID,
		Model:        h.deps.Embedder.Model(),
		Embedded:     embedded,
		Skipped:      skipped,
		Results:      results,
	})
}

func (h *AIHandler) enqueueEmbedMetadataJob(ctx context.Context, w http.ResponseWriter, req *embedMetadataRequest) {
	b, err := sonic.ConfigStd.Marshal(req)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to marshal job request", err)
		return
	}
	job := &metadata.AIJob{
		ID:              uuid.NewString(),
		ClientSessionID: req.ClientSessionID,
		Kind:            "embed_metadata",
		Status:          metadata.AIJobStatusQueued,
		Phase:           "queued",
		PhaseMessage:    "waiting in queue",
		ProgressPct:     0,
		DatasourceID:    &req.DatasourceID,
		ScopeSchemas:    []string{},
		RequestJSON:     b,
	}
	if err := h.deps.MetaRepo.CreateAIJob(ctx, job); err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to create job", err)
		return
	}
	if h.deps.AIJobQueue != nil {
		if err := h.deps.AIJobQueue.Publish(ctx, job.ID); err != nil {
			if failErr := h.deps.MetaRepo.FailAIJob(ctx, job.ID, err.Error()); failErr != nil {
				slog.WarnContext(ctx, "mark async AI job failed", "job_id", job.ID, "err", failErr)
			}
			writeInternalError(ctx, w, http.StatusInternalServerError, "failed to publish job to queue", err)
			return
		}
	}
	writeJSON(w, http.StatusAccepted, job)
}

func tablesForModel(model *semantic.SemanticModel) map[string]bool {
	allowed := map[string]bool{}
	if model == nil {
		return allowed
	}
	add := func(schema, table string) {
		if table == "" {
			return
		}
		if schema == "" {
			schema = model.BaseSchema
		}
		allowed[schema+"."+table] = true
	}
	add(model.BaseSchema, model.BaseTable)
	for _, j := range model.Joins {
		if j.IsActive {
			add(j.FromSchema, j.FromTable)
			add(j.ToSchema, j.ToTable)
		}
	}
	for _, d := range model.Dimensions {
		if !d.IsActive {
			continue
		}
		if s, t := splitColumnRef(d.ColumnRef, model.BaseSchema); t != "" {
			add(s, t)
		}
	}
	for _, m := range model.Metrics {
		if !m.IsActive {
			continue
		}
		if s, t := splitColumnRef(m.Expression, model.BaseSchema); t != "" {
			add(s, t)
		}
	}
	return allowed
}

func splitColumnRef(ref, baseSchema string) (schema, table string) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", ""
	}
	parts := strings.Split(ref, ".")
	switch len(parts) {
	case 2:
		return baseSchema, parts[0]
	case 3:
		return parts[0], parts[1]
	default:
		return "", ""
	}
}

func (h *AIHandler) loadQueryModel(
	ctx context.Context,
	req aiQueryRequest,
) (*semantic.SemanticModel, *routing.TableRoutingResult, error) {
	if req.CompositeID != "" {
		return h.loadCompositeModel(ctx, req.CompositeID)
	}
	if req.ModelID != "" {
		model, err := h.loadModel(ctx, req.DatasourceID, req.ModelID)
		if err != nil {
			return nil, nil, err
		}
		return model, routingForSemanticModel(model, 1), nil
	}
	if len(nonEmptyStringSlice(req.Tables)) == 0 {
		if model, routeResult, ok := h.loadPreferredSemanticModel(ctx, req.DatasourceID, req.Question); ok {
			return model, routeResult, nil
		}
	}
	base, views, err := typeScopeFromAIQueryRequest(req)
	if err != nil {
		return nil, nil, err
	}
	return h.tableRouter.Route(ctx, req.DatasourceID, req.Question, req.Tables, base, views)
}

func (h *AIHandler) loadCompositeModel(
	ctx context.Context,
	compositeID string,
) (*semantic.SemanticModel, *routing.TableRoutingResult, error) {
	if h.deps.CompositeRepo == nil {
		return nil, nil, errors.New("composite models are not configured")
	}
	model, err := h.deps.CompositeRepo.GetPublishedResolvedComposite(ctx, compositeID)
	if err != nil {
		return nil, nil, err
	}
	routingResult := routingForSemanticModel(model, 1)
	if routingResult != nil {
		routingResult.ContextSource = "composite_model"
		routingResult.ContextKey = compositeID
	}
	return model, routingResult, nil
}

func (h *AIHandler) loadPreferredSemanticModel(ctx context.Context, datasourceID, question string) (*semantic.SemanticModel, *routing.TableRoutingResult, bool) {
	models, err := h.listSemanticModels(ctx, datasourceID)
	if err != nil {
		slog.WarnContext(ctx, "list semantic models failed; falling back to auto context", "datasource_id", datasourceID, "error", err)
		return nil, nil, false
	}
	model, ok := chooseSemanticModelForQuestion(models, question)
	if !ok {
		return nil, nil, false
	}
	full, err := h.loadModel(ctx, datasourceID, model.Name)
	if err != nil {
		slog.WarnContext(ctx, "load preferred semantic model failed; falling back to auto context", "datasource_id", datasourceID, "model", model.Name, "error", err)
		return nil, nil, false
	}
	return full, routingForSemanticModel(full, semanticModelConfidence(models, model, question)), true
}

func (h *AIHandler) listSemanticModels(ctx context.Context, datasourceID string) ([]semantic.SemanticModel, error) {
	if h.deps.CatalogClient != nil {
		return h.deps.CatalogClient.ListModels(ctx, datasourceID)
	}
	return h.deps.SemanticRepo.ListModels(ctx, datasourceID)
}

func chooseSemanticModelForQuestion(models []semantic.SemanticModel, question string) (semantic.SemanticModel, bool) {
	active := make([]semantic.SemanticModel, 0, len(models))
	for _, model := range models {
		status := model.Status
		if status == "" {
			status = semantic.ModelStatusPublished
		}
		if !model.IsActive || status != semantic.ModelStatusPublished || strings.HasPrefix(model.Name, "auto:") {
			continue
		}
		active = append(active, model)
	}
	if len(active) == 0 {
		return semantic.SemanticModel{}, false
	}
	if len(active) == 1 {
		return active[0], true
	}

	tokens := routing.TokenSet(question)
	var (
		best      semantic.SemanticModel
		bestScore float64
	)
	for _, model := range active {
		score := scoreSemanticModelForQuestion(model, tokens)
		if score > bestScore {
			best = model
			bestScore = score
		}
	}
	if bestScore == 0 {
		return semantic.SemanticModel{}, false
	}
	return best, true
}

func semanticModelConfidence(models []semantic.SemanticModel, selected semantic.SemanticModel, question string) float64 {
	activeCount := 0
	for _, model := range models {
		status := model.Status
		if status == "" {
			status = semantic.ModelStatusPublished
		}
		if model.IsActive && status == semantic.ModelStatusPublished && !strings.HasPrefix(model.Name, "auto:") {
			activeCount++
		}
	}
	if activeCount <= 1 {
		return 1
	}
	score := scoreSemanticModelForQuestion(selected, routing.TokenSet(question))
	if score <= 0 {
		return 0.65
	}
	if score > 10 {
		return 0.95
	}
	return 0.75
}

func scoreSemanticModelForQuestion(model semantic.SemanticModel, tokens map[string]struct{}) float64 {
	score := routing.WeightedTokenScore(tokens, model.Name, 4)
	score += routing.WeightedTokenScore(tokens, model.BaseTable, 3)
	if model.Label != nil {
		score += routing.WeightedTokenScore(tokens, *model.Label, 2)
	}
	if model.Description != nil {
		score += routing.WeightedTokenScore(tokens, *model.Description, 1.5)
	}
	for _, synonym := range model.Synonyms {
		score += routing.WeightedTokenScore(tokens, synonym, 3)
	}
	return score
}

func routingForSemanticModel(model *semantic.SemanticModel, confidence float64) *routing.TableRoutingResult {
	if model == nil {
		return nil
	}
	selectedTables := selectedTablesForSemanticModel(model)
	routeResult := &routing.TableRoutingResult{
		SelectedModels:     []string{model.Name},
		SelectedTables:     selectedTables,
		SelectedDimensions: semanticDimensionNames(model.Dimensions),
		SelectedMetrics:    semanticMetricNames(model.Metrics),
		JoinPaths:          semanticJoinPaths(model),
		Confidence:         confidence,
		ContextSource:      "semantic_model",
		ContextKey:         model.ID,
		RankingMethod:      "semantic",
		Candidates: []routing.TableCandidate{{
			Table:      qualifySemanticTable(model, model.BaseTable),
			Score:      confidence,
			TotalScore: confidence,
			Selected:   true,
		}},
		Debug: &routing.TableRoutingDebug{
			RelationExpansion: semanticJoinPaths(model),
		},
	}
	if routeResult.ContextKey == "" {
		routeResult.ContextKey = model.Name
	}
	if !model.UpdatedAt.IsZero() {
		routeResult.ContextUpdatedAt = new(model.UpdatedAt)
	}
	return routeResult
}

func selectedTablesForSemanticModel(model *semantic.SemanticModel) []string {
	seen := map[string]bool{}
	var out []string
	add := func(table string) {
		table = qualifySemanticTable(model, table)
		if table == "" || seen[table] {
			return
		}
		seen[table] = true
		out = append(out, table)
	}
	add(model.BaseTable)
	for _, join := range model.Joins {
		add(join.FromTable)
		add(join.ToTable)
	}
	return out
}

func semanticJoinPaths(model *semantic.SemanticModel) []string {
	out := make([]string, 0, len(model.Joins))
	for _, join := range model.Joins {
		out = append(out, fmt.Sprintf(
			"%s.%s = %s.%s",
			qualifySemanticTable(model, join.FromTable),
			join.FromColumn,
			qualifySemanticTable(model, join.ToTable),
			join.ToColumn,
		))
	}
	return out
}

func qualifySemanticTable(model *semantic.SemanticModel, table string) string {
	table = strings.TrimSpace(table)
	if table == "" {
		return ""
	}
	if strings.Contains(table, ".") || model.BaseSchema == "" {
		return table
	}
	return model.BaseSchema + "." + table
}

func semanticDimensionNames(dimensions []semantic.Dimension) []string {
	out := make([]string, 0, len(dimensions))
	for _, dimension := range dimensions {
		out = append(out, dimension.Name)
	}
	return out
}

func semanticMetricNames(metrics []semantic.Metric) []string {
	out := make([]string, 0, len(metrics))
	for _, metric := range metrics {
		out = append(out, metric.Name)
	}
	return out
}

func nonEmptyStringSlice(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			out = append(out, value)
		}
	}
	return out
}

func typeScopeFromAIQueryRequest(req aiQueryRequest) (includeBase, includeViews bool, err error) {
	includeBase = true
	includeViews = true
	if req.IncludeBaseTables != nil {
		includeBase = *req.IncludeBaseTables
	}
	if req.IncludeViews != nil {
		includeViews = *req.IncludeViews
	}
	if !includeBase && !includeViews {
		return false, false, routing.ErrTypeScopeEmpty
	}
	return includeBase, includeViews, nil
}

func (*AIHandler) writeModelLoadError(ctx context.Context, w http.ResponseWriter, req aiQueryRequest, err error) {
	switch {
	case errors.Is(err, routing.ErrTypeScopeEmpty):
		writeError(w, http.StatusBadRequest, "at least one of include_base_tables or include_views must be true")
	case errors.Is(err, routing.ErrTableScopeInvalid):
		writeError(w, http.StatusBadRequest, "table scope invalid")
	case errors.Is(err, core.ErrLoadSemanticModel):
		writeEntityNotFound(w, "semantic model")
	case req.ModelID == "":
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to route table scope", err)
	default:
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to load query model", err)
	}
}

// fewShotLimit caps how many prior successful queries we attach to the prompt.
// Higher values dilute attention and inflate tokens; lower values miss context.
const fewShotLimit = 5

// sampleRowLimit and sampleCellRunes shape the prompt-side sample-data block:
// few rows, short cells. The goal is to show value formats, not full data.
const (
	sampleRowLimit   = 3
	sampleCellRunes  = 80
	sampleColumnsMax = 30 // skip super-wide tables to keep tokens bounded
)

// loadSampleData fetches a small sample of rows from the model's base table to
// embed in the prompt. Errors are non-fatal — sampling is purely advisory.
func (h *AIHandler) loadSampleData(ctx context.Context, db *sql.DB, d dialect.Dialect, model *semantic.SemanticModel) []prompt.TableSample {
	if model == nil || db == nil || model.BaseTable == "" {
		return nil
	}
	cols, err := h.deps.MetaRepo.ListColumns(ctx, model.DatasourceID, model.BaseSchema, model.BaseTable)
	if err != nil {
		slog.WarnContext(ctx, "load columns for sample failed", "error", err)
		return nil
	}
	if len(cols) == 0 || len(cols) > sampleColumnsMax {
		return nil
	}
	// Withhold PII column values from the NL→SQL prompt sent to the external LLM
	// (same rule as the describe path). Names/types still reach the model via the
	// schema context; only raw PII values are excluded.
	cols = ai.ExcludePIIColumns(cols)
	if len(cols) == 0 {
		return nil
	}
	rows, err := ai.FetchTableSample(ctx, db, d, cols, model.BaseSchema, model.BaseTable, sampleRowLimit, sampleCellRunes)
	if err != nil {
		slog.WarnContext(ctx, "fetch sample rows failed", "error", err)
		return nil
	}
	if len(rows) == 0 {
		return nil
	}
	return []prompt.TableSample{{Schema: model.BaseSchema, Table: model.BaseTable, Rows: rows}}
}

// fewShotBuilder assembles few-shot examples under the fewShotLimit cap,
// deduping by question hash so higher-priority sources (added first) win over
// lower-priority ones (e.g. explicit selections over auto-recall).
type fewShotBuilder struct {
	out  []prompt.FewShotExample
	seen map[string]bool
}

func newFewShotBuilder() *fewShotBuilder {
	return &fewShotBuilder{
		out:  make([]prompt.FewShotExample, 0, fewShotLimit),
		seen: make(map[string]bool, fewShotLimit),
	}
}

// add appends ex unless it is blank, a duplicate, or the cap is reached. It
// returns false once the cap is full so callers can stop iterating.
func (b *fewShotBuilder) add(ex prompt.FewShotExample) bool {
	if len(b.out) >= fewShotLimit {
		return false
	}
	q := strings.TrimSpace(ex.Question)
	if q == "" || strings.TrimSpace(ex.LogicalQuery) == "" {
		return true
	}
	key := metadata.QuestionHash(q)
	if b.seen[key] {
		return true
	}
	b.seen[key] = true
	b.out = append(b.out, ex)
	return len(b.out) < fewShotLimit
}

func (b *fewShotBuilder) addAll(examples []prompt.FewShotExample) {
	for _, ex := range examples {
		if !b.add(ex) {
			return
		}
	}
}

// reset re-seats the builder around examples (used after recall, which appends
// to and dedupes the accumulated slice), rebuilding the dedup index.
func (b *fewShotBuilder) reset(examples []prompt.FewShotExample) {
	b.out = examples
	clear(b.seen)
	for _, ex := range examples {
		b.seen[metadata.QuestionHash(strings.TrimSpace(ex.Question))] = true
	}
}

func (b *fewShotBuilder) remaining() int { return fewShotLimit - len(b.out) }

func optionalModelID(model *semantic.SemanticModel) *string {
	if model == nil || model.ID == "" {
		return nil
	}
	return new(model.ID)
}

// listCuratedFewShot loads curated few-shot rows for the model, preferring the
// catalog service when configured. Errors are non-fatal — logged and skipped.
func (h *AIHandler) listCuratedFewShot(ctx context.Context, model *semantic.SemanticModel, modelID *string) []metadata.FewShotCuratedRow {
	var curated []metadata.FewShotCuratedRow
	var err error
	if h.deps.CatalogClient != nil {
		curated, err = h.deps.CatalogClient.ListFewShot(ctx, model.DatasourceID, stringValue(modelID))
	} else {
		curated, err = h.deps.MetaRepo.ListFewShotCurated(ctx, model.DatasourceID, stringValue(modelID))
	}
	if err != nil {
		slog.WarnContext(ctx, "load curated few-shot examples failed", "error", err)
	}
	return curated
}

// addRecall runs the embedding-RAG few-shot recall when autoFind is set and
// merges the deduped result into b, returning the recall hit count.
func (h *AIHandler) addRecall(ctx context.Context, b *fewShotBuilder, model *semantic.SemanticModel, question string, autoFind bool) int {
	if !autoFind {
		return 0
	}
	recalled, hits := h.appendConfirmedFewShot(ctx, model, question, b.out)
	b.reset(dedupeFewShot(recalled))
	return hits
}

// addHistoryFewShot fills any remaining cap with recent successful queries.
func (h *AIHandler) addHistoryFewShot(ctx context.Context, b *fewShotBuilder, model *semantic.SemanticModel, modelID *string) {
	if b.remaining() <= 0 {
		return
	}
	historyRows, err := h.deps.MetaRepo.ListSuccessfulAIQueries(ctx, model.DatasourceID, modelID, b.remaining())
	if err != nil {
		slog.WarnContext(ctx, "load history few-shot examples failed", "error", err)
		return
	}
	for _, r := range historyRows {
		if !b.add(prompt.FewShotExample{Question: r.Question, LogicalQuery: string(r.LogicalQuery)}) {
			return
		}
	}
}

// loadFewShotExamplesWithIDs returns few-shot examples with optional explicit
// example_ids and an opt-in to include recent successful queries. Used by the
// Run phase so the frontend can override which exemplars hit the prompt. It
// honors the same query-time grounding controls as loadFewShotExamples:
// explicitly selected saved queries (savedQueryIDs) are injected first and
// autoFind=false skips the embedding-RAG recall.
func (h *AIHandler) loadFewShotExamplesWithIDs(ctx context.Context, model *semantic.SemanticModel, question string, exampleIDs []string, includePastQueries, autoFind bool, savedQueryIDs []string) ([]prompt.FewShotExample, int) {
	ctx, span := otel.Tracer("biqly/ai").Start(ctx, "ai.LoadFewShot")
	defer span.End()

	if model == nil {
		return nil, 0
	}
	span.SetAttributes(attribute.String("model.id", model.ID))
	modelID := optionalModelID(model)
	curated := h.listCuratedFewShot(ctx, model, modelID)

	idMap := make(map[string]bool, len(exampleIDs))
	for _, id := range exampleIDs {
		idMap[id] = true
	}

	b := newFewShotBuilder()
	// Explicit "/"-selected saved queries take priority within the cap.
	b.addAll(h.loadSavedQueryGrounding(ctx, model, savedQueryIDs))
	for _, r := range curated {
		matches := r.IsFewShot
		if len(exampleIDs) > 0 {
			matches = idMap[r.ID]
		}
		if matches && !b.add(prompt.FewShotExample{
			Question:     r.Question,
			LogicalQuery: string(r.LogicalQuery),
			Locale:       r.Locale,
		}) {
			break
		}
	}

	recallHits := h.addRecall(ctx, b, model, question, autoFind)
	if includePastQueries {
		h.addHistoryFewShot(ctx, b, model, modelID)
	}

	span.SetAttributes(attribute.Int("ai.few_shot.count", len(b.out)))
	return b.out, recallHits
}

// datasourceDialectName returns the driver type for prompt dialect examples.
// Failures are non-fatal — empty string defaults to postgres in the prompt builder.
func (h *AIHandler) datasourceDialectName(ctx context.Context, datasourceID string) string {
	if datasourceID == "" {
		return ""
	}
	ds, err := h.loadDatasource(ctx, datasourceID)
	if err != nil {
		slog.WarnContext(ctx, "load datasource for dialect hint failed", "error", err)
		return ""
	}
	return ds.Type
}

func (h *AIHandler) loadDatasource(ctx context.Context, datasourceID string) (*metadata.Datasource, error) {
	if h.deps.CatalogClient != nil {
		return h.deps.CatalogClient.GetDatasource(ctx, datasourceID)
	}
	return h.deps.MetaRepo.GetDatasource(ctx, datasourceID)
}

func (h *AIHandler) loadGlossaryForAmbiguity(ctx context.Context, model *semantic.SemanticModel) []prompt.GlossaryEntry {
	catalog, external := h.loadGlossaryEntries(ctx, model)
	return combineGlossaryEntries(catalog, external)
}

func (h *AIHandler) loadGlossaryEntries(ctx context.Context, model *semantic.SemanticModel) ([]prompt.GlossaryEntry, []prompt.GlossaryEntry) {
	if model == nil {
		return nil, nil
	}
	ctx, span := otel.Tracer("biqly/ai").Start(ctx, "ai.GlossaryLoad")
	defer span.End()
	catalog := prompt.GlossaryFromSemanticModel(model)
	var ext []prompt.ExternalGlossaryInput
	rows, err := h.listBusinessGlossary(ctx, model.DatasourceID, model.ID)
	if err != nil {
		span.RecordError(err)
		slog.WarnContext(ctx, "load business glossary failed", "error", err)
	} else {
		for _, r := range rows {
			ext = append(ext, prompt.ExternalGlossaryInput{
				Term:       r.Term,
				Definition: r.Definition,
				MapsToType: r.MapsToType,
				MapsToName: r.MapsToName,
				Aliases:    r.Aliases,
				AIContext:  r.AIContext,
			})
		}
	}
	span.SetAttributes(
		attribute.Int("ai.glossary.catalog", len(catalog)),
		attribute.Int("ai.glossary.external", len(ext)),
	)
	return catalog, prompt.GlossaryFromExternal(ext)
}

func combineGlossaryEntries(catalog, external []prompt.GlossaryEntry) []prompt.GlossaryEntry {
	entries := make([]prompt.GlossaryEntry, 0, len(catalog)+len(external))
	entries = append(entries, catalog...)
	return append(entries, external...)
}

func (h *AIHandler) listBusinessGlossary(ctx context.Context, datasourceID, modelID string) ([]metadata.BusinessGlossaryRow, error) {
	if h.deps.CatalogClient != nil {
		return h.deps.CatalogClient.ListGlossary(ctx, datasourceID, modelID)
	}
	return h.deps.MetaRepo.ListBusinessGlossary(ctx, datasourceID, modelID)
}

// loadFewShotExamples returns recent high-confidence (question, logical_query)
// pairs for this datasource+model. Errors are non-fatal — we just log and skip.
//
// Grounding sources are layered under the fewShotLimit cap: explicitly selected
// saved queries (savedQueryIDs) first so they take priority, then curated
// examples, then — when autoFind is true — embedding-RAG recall, then recent
// successful history. Passing autoFind=false and no ids reproduces a
// recall-free prompt; autoFind=true with no ids reproduces the historical
// default behavior.
func (h *AIHandler) loadFewShotExamples(ctx context.Context, model *semantic.SemanticModel, question string, autoFind bool, savedQueryIDs []string) ([]prompt.FewShotExample, int) {
	if model == nil {
		return nil, 0
	}
	modelID := optionalModelID(model)
	curated := h.listCuratedFewShot(ctx, model, modelID)

	b := newFewShotBuilder()
	// Explicit "/"-selected saved queries take priority within the cap.
	b.addAll(h.loadSavedQueryGrounding(ctx, model, savedQueryIDs))
	for _, r := range curated {
		if r.IsFewShot && !b.add(prompt.FewShotExample{
			Question:     r.Question,
			LogicalQuery: string(r.LogicalQuery),
			Locale:       r.Locale,
		}) {
			break
		}
	}

	recallHits := h.addRecall(ctx, b, model, question, autoFind)
	h.addHistoryFewShot(ctx, b, model, modelID)

	return b.out, recallHits
}

// loadSavedQueryGrounding loads the explicitly selected saved queries and maps
// them to few-shot examples. Datasource-scoped so a request cannot inject
// another datasource's queries. Errors are non-fatal.
func (h *AIHandler) loadSavedQueryGrounding(ctx context.Context, model *semantic.SemanticModel, ids []string) []prompt.FewShotExample {
	if model == nil || len(ids) == 0 || h.deps == nil || h.deps.MetaRepo == nil {
		return nil
	}
	rows, err := h.deps.MetaRepo.GetSavedQueriesByIDs(ctx, model.DatasourceID, ids)
	if err != nil {
		slog.WarnContext(ctx, "load saved query grounding failed", "error", err)
		return nil
	}
	out := make([]prompt.FewShotExample, 0, len(rows))
	for _, row := range rows {
		lq := strings.TrimSpace(row.SQLQuery)
		if lq == "" {
			lq = strings.TrimSpace(string(row.LogicalQuery))
		}
		if lq == "" || strings.TrimSpace(row.Question) == "" {
			continue
		}
		out = append(out, prompt.FewShotExample{Question: row.Question, LogicalQuery: lq})
	}
	return out
}

// dedupeFewShot removes duplicate examples by question hash, keeping the first
// occurrence so higher-priority (explicit, curated) rows win over recalled ones.
func dedupeFewShot(examples []prompt.FewShotExample) []prompt.FewShotExample {
	seen := make(map[string]bool, len(examples))
	out := examples[:0]
	for _, ex := range examples {
		key := metadata.QuestionHash(strings.TrimSpace(ex.Question))
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, ex)
	}
	return out
}

func stringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func clarificationResponse(routeResult *routing.TableRoutingResult) *ai.Response {
	question := "Which table or topic do you want to query? Please pick one or more from the candidates."
	return &ai.Response{
		Result: &ai.AIResult{
			Warnings: []string{
				"could not confidently choose the relevant table scope; select one or more tables and try again",
			},
			Confidence: 0,
		},
		Metadata: &ai.AIMetadata{
			TableRouting: routeResult,
		},
		Clarification: &ai.ClarificationResponse{
			NeedsClarification:    true,
			ClarificationQuestion: question,
			Clarification:         ai.ClarificationFromRouting(routeResult, question),
		},
	}
}

func failedAIResponse(err error) *ai.Response {
	message := "ai request failed"
	if err != nil {
		message = err.Error()
	}
	return &ai.Response{
		Result: &ai.AIResult{
			Warnings:   []string{message},
			Confidence: 0,
		},
	}
}

func (h *AIHandler) loadModel(ctx context.Context, datasourceID, modelRef string) (*semantic.SemanticModel, error) {
	if h.deps.CatalogClient != nil {
		if model, err := h.deps.CatalogClient.GetModel(ctx, modelRef); err == nil {
			return model, nil
		}
	}
	if model, err := h.deps.SemanticRepo.GetPublishedFullModel(ctx, modelRef); err == nil {
		return model, nil
	}
	model, err := h.deps.SemanticRepo.GetPublishedModelByName(ctx, datasourceID, modelRef)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", core.ErrLoadSemanticModel, err)
	}
	return model, nil
}
