package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"testing"
	"time"

	ai "github.com/biqly/biqly/internal/ai/eval"
	"github.com/biqly/biqly/internal/audit"
	"github.com/biqly/biqly/internal/core"
	"github.com/biqly/biqly/internal/datasource"
	"github.com/biqly/biqly/internal/dialect"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/semantic"
	"github.com/go-chi/chi/v5"
)

const (
	integrationToken = "integration-secret"
	integrationDSID  = "ds_1"
	integrationModel = "orders"
	//nolint:gosec // test fixture DSN, not a real credential
	plaintextProbeDSN = "postgres://user:supersecret@localhost:5432/db"
)

type integrationCatalog struct {
	datasource metadata.Datasource
	model      *semantic.SemanticModel
	models     []semantic.SemanticModel
	tables     []metadata.Table
	columns    []metadata.Column
	relations  []metadata.Relation
	fewShot    []metadata.FewShotCuratedRow
	glossary   []metadata.BusinessGlossaryRow
}

func (c integrationCatalog) GetDatasource(_ context.Context, id string) (*metadata.Datasource, error) {
	if id != c.datasource.ID {
		return nil, errors.New("datasource not found")
	}
	return &c.datasource, nil
}

func (c integrationCatalog) ListDatasources(context.Context) ([]metadata.Datasource, error) {
	return []metadata.Datasource{c.datasource}, nil
}

func (c integrationCatalog) GetPublishedFullModel(_ context.Context, id string) (*semantic.SemanticModel, error) {
	if id != c.model.ID {
		return nil, errors.New("model not found")
	}
	return c.model, nil
}

func (c integrationCatalog) ListModels(_ context.Context, datasourceID string) ([]semantic.SemanticModel, error) {
	if datasourceID != integrationDSID {
		return nil, nil
	}
	return c.models, nil
}

func (c integrationCatalog) ListTables(_ context.Context, datasourceID, _ string) ([]metadata.Table, error) {
	if datasourceID != integrationDSID {
		return nil, nil
	}
	return c.tables, nil
}

func (c integrationCatalog) ListColumns(_ context.Context, datasourceID, _, _ string) ([]metadata.Column, error) {
	if datasourceID != integrationDSID {
		return nil, nil
	}
	return c.columns, nil
}

func (c integrationCatalog) ListRelations(_ context.Context, datasourceID string) ([]metadata.Relation, error) {
	if datasourceID != integrationDSID {
		return nil, nil
	}
	return c.relations, nil
}

func (c integrationCatalog) ListFewShotCurated(_ context.Context, datasourceID, _ string) ([]metadata.FewShotCuratedRow, error) {
	if datasourceID != integrationDSID {
		return nil, nil
	}
	return c.fewShot, nil
}

func (c integrationCatalog) ListBusinessGlossary(_ context.Context, datasourceID, _ string) ([]metadata.BusinessGlossaryRow, error) {
	if datasourceID != integrationDSID {
		return nil, nil
	}
	return c.glossary, nil
}

func (integrationCatalog) CreateAIQueryHistory(_ context.Context, entry *metadata.AIQueryHistoryEntry) error {
	if entry.ID == "" {
		entry.ID = "ai_hist_1"
	}
	return nil
}

func (integrationCatalog) CreateQueryHistory(_ context.Context, entry *query.HistoryEntry) error {
	if entry.ID == "" {
		entry.ID = "query_hist_1"
	}
	return nil
}

type integrationEvalRepo struct{}

func (integrationEvalRepo) SaveRunResults(context.Context, string, string, string, int, time.Time, []ai.EvalResultWithMetrics) error {
	return nil
}

type integrationQueryRunner struct {
	compile *core.CompileResult
}

func (r integrationQueryRunner) Compile(ctx context.Context, lq *query.LogicalQuery) (*core.CompileResult, *core.ServiceError) {
	if r.compile != nil {
		return r.compile, nil
	}
	return core.NewQueryService(core.QueryServiceDeps{
		Models:      fakeModelLoader{model: integrationSemanticModel()},
		Datasources: fakeDatasourceLoader{datasource: metadata.Datasource{ID: integrationDSID, Type: "postgres"}},
		Drivers:     integrationDriverRegistry(),
		Validator:   query.NewValidator(1000),
		Executor:    query.NewExecutor(1000, 0),
	}).Compile(ctx, lq)
}

func (r integrationQueryRunner) Run(ctx context.Context, lq *query.LogicalQuery) (*core.RunResult, *core.ServiceError) {
	compiled, se := r.Compile(ctx, lq)
	if se != nil {
		return nil, se
	}
	return &core.RunResult{
		CompileResult: *compiled,
		Result: &query.Result{
			Columns: []query.ResultColumn{{Name: "country", Type: "text"}},
			Rows:    [][]any{{"TR"}},
			Stats:   query.Stats{RowCount: 1, DurationMs: 3},
		},
	}, nil
}

type fakeModelLoader struct {
	model *semantic.SemanticModel
}

func (f fakeModelLoader) GetPublishedFullModel(context.Context, string) (*semantic.SemanticModel, error) {
	return f.model, nil
}

type fakeDatasourceLoader struct {
	datasource metadata.Datasource
}

func (f fakeDatasourceLoader) GetDatasource(context.Context, string) (*metadata.Datasource, error) {
	return &f.datasource, nil
}

type integrationDriver struct {
	dialect dialect.Dialect
}

func (integrationDriver) Type() string                                  { return "postgres" }
func (integrationDriver) Ping(context.Context, string) error            { return nil }
func (integrationDriver) Open(context.Context, string) (*sql.DB, error) {
	return nil, nil //nolint:nilnil // integration test stub never opens a DB
}
func (integrationDriver) Introspect(context.Context, *sql.DB) (*datasource.IntrospectionResult, error) {
	return nil, nil //nolint:nilnil // integration test stub never introspects
}
func (d integrationDriver) Dialect() dialect.Dialect { return d.dialect }

func integrationDriverRegistry() *datasource.Registry {
	reg := datasource.NewRegistry()
	reg.Register(integrationDriver{dialect: dialect.PostgresDialect{}})
	return reg
}

func integrationSemanticModel() *semantic.SemanticModel {
	return &semantic.SemanticModel{
		ID:           integrationModel,
		Name:         integrationModel,
		DatasourceID: integrationDSID,
		BaseSchema:   "public",
		BaseTable:    "orders",
		Status:       semantic.ModelStatusPublished,
		Version:      1,
		Dimensions: []semantic.Dimension{
			{Name: "country", ColumnRef: "customers.country", Type: "text"},
			{Name: "created_at", ColumnRef: "orders.created_at", Type: "date"},
		},
		Metrics: []semantic.Metric{
			{Name: "order_count", Expression: "orders.id", Aggregation: string(semantic.AggCount)},
		},
		Joins: []semantic.Join{
			{Name: "orders_customers", FromTable: "orders", FromColumn: "customer_id", ToTable: "customers", ToColumn: "id", JoinType: "LEFT", Relationship: "many_to_one"},
		},
	}
}

func integrationLogicalQuery() query.LogicalQuery {
	return query.LogicalQuery{
		DatasourceID: integrationDSID,
		ModelID:      integrationModel,
		Select: []query.SelectItem{
			{Type: query.SelectTypeDimension, Name: "country"},
			{Type: query.SelectTypeMetric, Name: "order_count"},
		},
		Filters: []query.Filter{{Field: "created_at", Operator: query.OpGte, Value: "2026-01-01"}},
		GroupBy: []query.GroupBy{{Field: "country"}},
		OrderBy: []query.OrderBy{{Field: "order_count", Direction: query.OrderDesc}},
		Limit:   100,
	}
}

func integrationCatalogFixture(encryptedDSN string) integrationCatalog {
	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	return integrationCatalog{
		datasource: metadata.Datasource{
			ID:           integrationDSID,
			Name:         "primary",
			Type:         "postgres",
			DSNEncrypted: encryptedDSN,
			IsActive:     true,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		model:  integrationSemanticModel(),
		models: []semantic.SemanticModel{{ID: integrationModel, Name: integrationModel, DatasourceID: integrationDSID, Status: semantic.ModelStatusPublished, Version: 1}},
		tables: []metadata.Table{{ID: "t1", DatasourceID: integrationDSID, SchemaName: "public", TableName: "orders"}},
		columns: []metadata.Column{
			{ID: "c1", DatasourceID: integrationDSID, SchemaName: "public", TableName: "orders", ColumnName: "id", DataType: "integer"},
		},
		relations: []metadata.Relation{{ID: "r1", DatasourceID: integrationDSID, FromSchema: "public", FromTable: "orders", FromColumn: "customer_id", ToSchema: "public", ToTable: "customers", ToColumn: "id"}},
		fewShot:   []metadata.FewShotCuratedRow{{ID: "fs1", DatasourceID: integrationDSID, ModelID: integrationModel, Question: "orders by country", Name: "orders by country", IsFewShot: true}},
		glossary:  []metadata.BusinessGlossaryRow{{ID: "g1", DatasourceID: integrationDSID, Term: "revenue", MapsToType: "metric", MapsToName: "order_count"}},
	}
}

type internalIntegrationEnv struct {
	handler http.Handler
	audit   *bytes.Buffer
}

func newInternalIntegrationEnv(t *testing.T, catalog integrationCatalog, queryRunner internalQueryRunner) *internalIntegrationEnv {
	t.Helper()
	var auditBuf bytes.Buffer
	auditLogger := audit.NewLogger(slog.New(slog.NewJSONHandler(&auditBuf, nil)))
	internalHandler := &InternalHandler{
		meta:     catalog,
		semantic: catalog,
		eval:     integrationEvalRepo{},
	}
	queryHandler := &InternalQueryHandler{query: queryRunner}

	r := chi.NewRouter()
	r.Route("/internal", func(r chi.Router) {
		r.Use(InternalAuditMiddleware(auditLogger))
		r.Use(InternalTokenMiddleware(integrationToken))
		r.Get("/health", internalHandler.Health)
		r.Get("/datasources/{id}", internalHandler.GetDatasource)
		r.Get("/models", internalHandler.ListModels)
		r.Get("/models/{id}", internalHandler.GetFullModel)
		r.Get("/datasources/{id}/tables", internalHandler.ListTables)
		r.Get("/datasources/{id}/columns", internalHandler.ListColumns)
		r.Get("/datasources/{id}/relations", internalHandler.ListRelations)
		r.Get("/few-shot", internalHandler.ListFewShot)
		r.Get("/glossary", internalHandler.ListGlossary)
		r.Post("/history/ai", internalHandler.CreateAIHistory)
		r.Post("/history/query", internalHandler.CreateQueryHistory)
		r.Post("/eval-results", internalHandler.CreateEvalResults)
		r.Post("/query/compile", queryHandler.Compile)
		r.Post("/query/run", queryHandler.Run)
		r.Post("/query/dry-run", queryHandler.DryRun)
	})
	return &internalIntegrationEnv{handler: r, audit: &auditBuf}
}
