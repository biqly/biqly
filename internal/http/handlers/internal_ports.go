package handlers

import (
	"context"
	"time"

	ai "github.com/biqly/biqly/internal/ai/eval"
	"github.com/biqly/biqly/internal/core"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/semantic"
)

type internalMetaRepo interface {
	GetDatasource(ctx context.Context, id string) (*metadata.Datasource, error)
	ListDatasources(ctx context.Context) ([]metadata.Datasource, error)
	ListTables(ctx context.Context, datasourceID, schemaName string) ([]metadata.Table, error)
	ListColumns(ctx context.Context, datasourceID, schemaName, tableName string) ([]metadata.Column, error)
	ListRelations(ctx context.Context, datasourceID string) ([]metadata.Relation, error)
	ListFewShotCurated(ctx context.Context, datasourceID, modelID string) ([]metadata.FewShotCuratedRow, error)
	ListBusinessGlossary(ctx context.Context, datasourceID, modelID string) ([]metadata.BusinessGlossaryRow, error)
	CreateAIQueryHistory(ctx context.Context, entry *metadata.AIQueryHistoryEntry) error
	CreateQueryHistory(ctx context.Context, entry *query.HistoryEntry) error
}

type internalSemanticRepo interface {
	GetPublishedFullModel(ctx context.Context, id string) (*semantic.SemanticModel, error)
	ListModels(ctx context.Context, datasourceID string) ([]semantic.SemanticModel, error)
}

type internalEvalRepo interface {
	SaveRunResults(ctx context.Context, runID, provider, model string, contextVersion int, contextUpdatedAt time.Time, results []ai.ResultWithMetrics) error
}

type internalQueryRunner interface {
	Compile(ctx context.Context, lq *query.LogicalQuery) (*core.CompileResult, *core.ServiceError)
	Run(ctx context.Context, lq *query.LogicalQuery) (*core.RunResult, *core.ServiceError)
	// CompileWithModel and RunWithModel accept an optional inline semantic
	// model (nil loads the model referenced by the LogicalQuery from the
	// catalog) so callers can compile synthetic auto-routing models that are
	// not persisted.
	CompileWithModel(ctx context.Context, lq *query.LogicalQuery, inline *semantic.SemanticModel) (*core.CompileResult, *core.ServiceError)
	RunWithModel(ctx context.Context, lq *query.LogicalQuery, inline *semantic.SemanticModel) (*core.RunResult, *core.ServiceError)
}
