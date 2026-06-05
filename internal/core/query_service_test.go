package core_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/biqly/biqly/internal/core"
	"github.com/biqly/biqly/internal/datasource"
	"github.com/biqly/biqly/internal/dialect"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/semantic"
)

func TestQueryServiceCompileUsesSameSQLAsCompiler(t *testing.T) {
	ctx := context.Background()
	model := coreTestModel()
	lq := coreTestLogicalQuery()
	registry := datasource.NewRegistry()
	registry.Register(fakeDriver{dialect: dialect.PostgresDialect{}})

	service := core.NewQueryService(core.QueryServiceDeps{
		Models:      fakeModelLoader{model: model},
		Datasources: fakeDatasourceLoader{datasource: metadata.Datasource{ID: "ds1", Type: "postgres"}},
		Drivers:     registry,
		Validator:   query.NewValidator(1000),
		Executor:    query.NewExecutor(1000, 0),
		Encryptor:   nil,
	})

	got, se := service.Compile(ctx, &lq)
	if se != nil {
		t.Fatalf("QueryService.Compile(%+v) error = %v, want nil", lq, se)
	}
	want, err := query.NewCompiler(dialect.PostgresDialect{}).Compile(ctx, &lq, model)
	if err != nil {
		t.Fatalf("Compiler.Compile(%+v) error = %v, want nil", lq, err)
	}
	if got.Compiled.SQL != want.SQL {
		t.Errorf("QueryService.Compile(%+v) SQL = %q, want %q", lq, got.Compiled.SQL, want.SQL)
	}
}

func coreTestLogicalQuery() query.LogicalQuery {
	return query.LogicalQuery{
		DatasourceID: "ds1",
		ModelID:      "orders",
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

func coreTestModel() *semantic.SemanticModel {
	return &semantic.SemanticModel{
		ID:           "orders",
		Name:         "orders",
		DatasourceID: "ds1",
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

type fakeDriver struct {
	dialect dialect.Dialect
}

func (fakeDriver) Type() string                                  { return "postgres" }
func (fakeDriver) Ping(context.Context, string) error            { return nil }
func (fakeDriver) Open(context.Context, string) (*sql.DB, error) {
	return nil, nil //nolint:nilnil // test stub driver is never opened
}
func (fakeDriver) Introspect(context.Context, *sql.DB) (*datasource.IntrospectionResult, error) {
	return nil, nil //nolint:nilnil // optional result
}
func (f fakeDriver) Dialect() dialect.Dialect { return f.dialect }
