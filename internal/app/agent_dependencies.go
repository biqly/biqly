package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/biqly/biqly/internal/agent"
	"github.com/biqly/biqly/internal/ai"
	"github.com/biqly/biqly/internal/config"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/platform/observability"
	"github.com/biqly/biqly/internal/queue"
	"github.com/biqly/biqly/pkg/aiclient"
	"github.com/biqly/biqly/pkg/catalogclient"
	"github.com/biqly/biqly/pkg/logicalquery"
	"github.com/biqly/biqly/pkg/queryclient"
	"github.com/bytedance/sonic"
)

// NewAgentDependencies wires the agentic query runner service's dependency
// graph: a lean subset of NewAIDependencies' — only what the runtime loop
// and its internal HTTP server need (see agent.AgentDependencies) — built
// from real HTTP clients (WithCaller("agent")) rather than in-process calls,
// since cmd/agent is its own deployable service.
func NewAgentDependencies(ctx context.Context, cfg *config.Config) (*agent.AgentDependencies, error) {
	if cfg.NATS.URL == "" {
		return nil, errors.New("BI_NATS_URL is required for the agent service")
	}

	db, err := openMetadataDB(ctx, cfg)
	if err != nil {
		return nil, err
	}
	metaRepo, _ := provideRepositories(db)

	//nolint:contextcheck // ConnectNATS's signature takes no context (see
	// internal/queue/nats.go); it already uses its own bounded internal
	// timeout for the initial stream setup. Same call shape as the existing
	// NewAIJobQueue(cfg) call site.
	nq, err := queue.ConnectNATS(queue.NATSConfig{
		URL:     cfg.NATS.URL,
		Stream:  cfg.NATS.Stream,
		Subject: cfg.Agent.JobSubject,
	})
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("connect nats: %w", err)
	}

	catalogHTTP := catalogclient.New(cfg.Services.CatalogURL,
		catalogclient.WithAuthToken(cfg.Security.InternalAPIToken), catalogclient.WithCaller("agent"))
	aiHTTP := aiclient.New(cfg.Services.AIURL,
		aiclient.WithAuthToken(cfg.Security.InternalAPIToken), aiclient.WithCaller("agent"))
	queryHTTP := queryclient.New(cfg.Services.QueryURL,
		queryclient.WithAuthToken(cfg.Security.InternalAPIToken), queryclient.WithCaller("agent"))

	encryptor := provideEncryptor(ctx, db, false)
	providerStore := provideProviderStore(ctx, cfg, db, encryptor)
	// The planner reuses the NL→LogicalQuery purpose: both are "turn a
	// natural-language question into a structured decision" tasks. Adding a
	// dedicated purpose would touch ProviderStore's purpose enum and admin UI
	// across several files — out of scope for standing up this service.
	planner := agent.NewProviderPlanner(ai.NewPurposeProvider(providerStore, ai.PurposeQuery, nil, nil))

	policy := &agent.PolicyEngine{}
	registry := agent.NewRegistry(policy,
		agent.NewCatalogTool(&catalogResolverAdapter{client: catalogHTTP}),
		agent.NewSemanticTool(&semanticGeneratorAdapter{client: aiHTTP}),
		agent.NewQueryCompileTool(&queryCompilerAdapter{client: queryHTTP}),
		agent.NewQueryExecuteTool(&queryCompilerAdapter{client: queryHTTP}, &queryExecutorAdapter{client: queryHTTP}),
		agent.NewMemoryTool(&memoryRecallerAdapter{repo: metaRepo}, agentMemoryRecallLimit(cfg)),
	)

	runtime := agent.NewRuntime(planner, registry, &metadataStateStore{repo: metaRepo})
	shadow := agent.NewShadowEvaluator(&metadataShadowStore{repo: metaRepo})
	metrics := observability.Default()
	runtime.SetMetrics(metrics)
	shadow.SetMetrics(metrics)
	planner.SetMetrics(metrics)

	return &agent.AgentDependencies{
		Config:   cfg,
		MetaRepo: metaRepo,
		Queue:    nq.SubjectQueue(),
		Planner:  planner,
		Policy:   policy,
		Tools:    registry,
		Runtime:  runtime,
		Shadow:   shadow,
		Metrics:  metrics,
		Close: func() error {
			_ = nq.Close()
			return db.Close()
		},
		Ready: &atomic.Bool{},
		Runs:  agent.NewRunRegistry(),
	}, nil
}

// defaultAgentMemoryRecallLimit mirrors internal/http/handlers' fewShotLimit
// default for the legacy pipeline's confirmed-query recall.
const defaultAgentMemoryRecallLimit = 5

// agentMemoryRecallLimit reuses the AI memory config's recall limit so the
// agent path and the legacy pipeline stay consistent unless explicitly
// tuned apart.
func agentMemoryRecallLimit(cfg *config.Config) int {
	if cfg.AI.Memory.RecallLimit > 0 {
		return cfg.AI.Memory.RecallLimit
	}
	return defaultAgentMemoryRecallLimit
}

// catalogResolverAdapter adapts pkg/catalogclient.Client to agent.CatalogResolver.
type catalogResolverAdapter struct {
	client *catalogclient.Client
}

func (a *catalogResolverAdapter) ResolveEntities(ctx context.Context, datasourceID, _ string) ([]agent.CatalogEntity, error) {
	tables, err := a.client.ListTables(ctx, datasourceID, "")
	if err != nil {
		return nil, fmt.Errorf("list tables: %w", err)
	}
	entities := make([]agent.CatalogEntity, 0, len(tables))
	for _, table := range tables {
		columns, err := a.client.ListColumns(ctx, datasourceID, table.SchemaName, table.TableName)
		if err != nil {
			return nil, fmt.Errorf("list columns for %s.%s: %w", table.SchemaName, table.TableName, err)
		}
		columnNames := make([]string, 0, len(columns))
		for _, col := range columns {
			columnNames = append(columnNames, col.ColumnName)
		}
		entities = append(entities, agent.CatalogEntity{Table: table.TableName, Columns: columnNames})
	}
	return entities, nil
}

// semanticGeneratorAdapter adapts pkg/aiclient.Client to agent.SemanticGenerator.
type semanticGeneratorAdapter struct {
	client *aiclient.Client
}

func (a *semanticGeneratorAdapter) GeneratePlan(ctx context.Context, datasourceID, modelID, question string) (agent.SemanticPlan, error) {
	resp, err := a.client.Query(ctx, &aiclient.QueryRequest{
		DatasourceID: datasourceID,
		ModelID:      modelID,
		Question:     question,
	})
	if err != nil {
		return agent.SemanticPlan{}, fmt.Errorf("ai query: %w", err)
	}
	if resp.Result == nil {
		return agent.SemanticPlan{}, errors.New("ai query returned no result (a clarification may be required)")
	}
	raw, err := sonic.Marshal(resp.Result.LogicalQuery)
	if err != nil {
		return agent.SemanticPlan{}, fmt.Errorf("encode logical query: %w", err)
	}
	return agent.SemanticPlan{LogicalQuery: raw, Confidence: resp.Result.Confidence}, nil
}

// queryCompilerAdapter adapts pkg/queryclient.Client to agent.QueryCompiler.
type queryCompilerAdapter struct {
	client *queryclient.Client
}

func (a *queryCompilerAdapter) Compile(ctx context.Context, datasourceID string, logicalQueryJSON json.RawMessage) (agent.CompileResult, error) {
	lq, err := decodeLogicalQuery(datasourceID, logicalQueryJSON)
	if err != nil {
		return agent.CompileResult{}, err
	}
	resp, err := a.client.Compile(ctx, lq)
	if err != nil {
		return agent.CompileResult{}, fmt.Errorf("query compile: %w", err)
	}
	return agent.CompileResult{Fingerprint: resp.Fingerprint, SQL: resp.SQL}, nil
}

// queryExecutorAdapter adapts pkg/queryclient.Client to agent.QueryExecutor.
type queryExecutorAdapter struct {
	client *queryclient.Client
}

func (a *queryExecutorAdapter) Execute(
	ctx context.Context, datasourceID string, logicalQueryJSON json.RawMessage, rowLimit, timeoutSeconds int,
) (agent.QueryResult, error) {
	lq, err := decodeLogicalQuery(datasourceID, logicalQueryJSON)
	if err != nil {
		return agent.QueryResult{}, err
	}
	resp, err := a.client.Run(ctx, lq, rowLimit, timeoutSeconds*1000)
	if err != nil {
		return agent.QueryResult{}, fmt.Errorf("query run: %w", err)
	}
	rowsJSON, err := sonic.Marshal(resp.Rows)
	if err != nil {
		return agent.QueryResult{}, fmt.Errorf("encode rows: %w", err)
	}
	return agent.QueryResult{Rows: rowsJSON, RowCount: resp.RowCount}, nil
}

func decodeLogicalQuery(datasourceID string, raw json.RawMessage) (*logicalquery.LogicalQuery, error) {
	var lq logicalquery.LogicalQuery
	if err := sonic.Unmarshal(raw, &lq); err != nil {
		return nil, fmt.Errorf("decode logical query: %w", err)
	}
	if lq.DatasourceID == "" {
		lq.DatasourceID = datasourceID
	}
	return &lq, nil
}

// memoryRecallerAdapter adapts *metadata.Repository's confirmed-query
// few-shot store to agent.MemoryRecaller.
type memoryRecallerAdapter struct {
	repo *metadata.Repository
}

func (a *memoryRecallerAdapter) Recall(ctx context.Context, datasourceID, modelID, question string, limit int) ([]agent.RecalledExample, error) {
	rows, err := a.repo.ListActiveSavedQueryExamples(ctx, datasourceID, modelID, "", limit)
	if err != nil {
		return nil, fmt.Errorf("list active saved query examples: %w", err)
	}
	_ = question // the recall pool is scoped by datasource/model, not re-ranked by question here
	examples := make([]agent.RecalledExample, 0, len(rows))
	for _, row := range rows {
		examples = append(examples, agent.RecalledExample{Question: row.NLQuery})
	}
	return examples, nil
}

// metadataStateStore adapts *metadata.Repository to agent.StateStore.
type metadataStateStore struct {
	repo *metadata.Repository
}

func (s *metadataStateStore) Save(ctx context.Context, runID string, state agent.RuntimeState) error {
	raw, err := agent.MarshalState(state)
	if err != nil {
		return fmt.Errorf("marshal runtime state: %w", err)
	}
	if state.Terminal != nil {
		status := metadata.AgentRunStatusCompleted
		var confidence float64
		var answer string
		switch {
		case state.Terminal.Final != nil:
			confidence = state.Terminal.Final.Confidence
			answer = state.Terminal.Final.Answer
		case state.Terminal.Failure != nil:
			status = metadata.AgentRunStatusFailed
			answer = state.Terminal.Failure.Message
		}
		return s.repo.CompleteAgentRunTerminal(ctx, runID, status, confidence, answer, raw)
	}
	if state.QueryExecuteStarted {
		if err := s.repo.MarkAgentRunQueryExecuteStarted(ctx, runID); err != nil {
			return err
		}
	}
	return s.repo.SaveAgentRuntimeState(ctx, runID, raw)
}

func (s *metadataStateStore) Load(ctx context.Context, runID string) (agent.RuntimeState, bool, error) {
	raw, err := s.repo.LoadAgentRuntimeState(ctx, runID)
	if err != nil {
		if errors.Is(err, metadata.ErrAgentRunNotFound) {
			return agent.RuntimeState{}, false, nil
		}
		return agent.RuntimeState{}, false, err
	}
	state, err := agent.UnmarshalState(raw)
	if err != nil {
		return agent.RuntimeState{}, false, err
	}
	return state, true, nil
}

// metadataShadowStore adapts *metadata.Repository to agent.ShadowComparisonStore.
type metadataShadowStore struct {
	repo *metadata.Repository
}

func (s *metadataShadowStore) RecordShadowComparison(
	ctx context.Context, jobID, legacyRunID, agentRunID string, category agent.ShadowCategory, detail []byte,
) error {
	return s.repo.RecordShadowComparison(ctx, jobID, legacyRunID, agentRunID, string(category), detail)
}
