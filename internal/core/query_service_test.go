package core_test

import (
	"context"
	"database/sql"
	"errors"
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

	service := core.NewQueryService(&core.QueryServiceDeps{
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

// TestQueryServiceCompileWithModelSkipsCatalog proves an inline model (auto
// table routing produces models that only exist in the caller's memory)
// compiles without a catalog lookup: the model loader always errors, so a
// catalog hit would fail the compile.
func TestQueryServiceCompileWithModelSkipsCatalog(t *testing.T) {
	ctx := context.Background()
	model := coreTestModel()
	model.ID = "auto:public.orders,public.customers"
	lq := coreTestLogicalQuery()
	lq.ModelID = model.ID
	registry := datasource.NewRegistry()
	registry.Register(fakeDriver{dialect: dialect.PostgresDialect{}})

	service := core.NewQueryService(&core.QueryServiceDeps{
		Models:      failingModelLoader{},
		Datasources: fakeDatasourceLoader{datasource: metadata.Datasource{ID: "ds1", Type: "postgres"}},
		Drivers:     registry,
		Validator:   query.NewValidator(1000),
		Executor:    query.NewExecutor(1000, 0),
	})

	if _, se := service.Compile(ctx, &lq); se == nil {
		t.Fatal("Compile without inline model: error = nil, want catalog lookup failure")
	}

	got, se := service.CompileWithModel(ctx, &lq, model)
	if se != nil {
		t.Fatalf("CompileWithModel(%+v) error = %v, want nil", lq, se)
	}
	if got.Compiled.SQL == "" {
		t.Error("CompileWithModel returned empty SQL")
	}
	if got.Model != model {
		t.Error("CompileWithModel did not use the inline model")
	}
}

func TestQueryServiceDryRunWithModelSkipsConnectionWithoutExplainSupport(t *testing.T) {
	ctx := context.Background()
	model := coreTestModel()
	lq := coreTestLogicalQuery()
	opens := 0
	registry := datasource.NewRegistry()
	registry.Register(countingOpenDriver{dialect: dialect.SQLServerDialect{}, opens: &opens})
	service := core.NewQueryService(&core.QueryServiceDeps{
		Models:      fakeModelLoader{model: model},
		Datasources: fakeDatasourceLoader{datasource: metadata.Datasource{ID: "ds1", Type: "postgres"}},
		Drivers:     registry,
		Validator:   query.NewValidator(1000),
		Executor:    query.NewExecutor(1000, 0),
	})

	got, se := service.DryRunWithModel(ctx, &lq, nil)
	if se != nil {
		t.Fatalf("DryRunWithModel() error = %v, want nil", se)
	}
	if got.Compiled.SQL == "" {
		t.Fatal("DryRunWithModel returned empty SQL")
	}
	if opens != 0 {
		t.Fatalf("driver Open calls = %d, want 0 when dialect has no EXPLAIN support", opens)
	}
}

func TestQueryServiceCompileBlocksDatasourceDeniedFunction(t *testing.T) {
	ctx := context.Background()
	model := coreTestModel()
	model.Dimensions = append(model.Dimensions, semantic.Dimension{
		Name:                 "restricted_value",
		ColumnRef:            "orders.id",
		Type:                 "text",
		CalculatedExpression: "custom_reader(orders.id)",
	})
	lq := coreTestLogicalQuery()
	lq.Select = []query.SelectItem{{Type: query.SelectTypeDimension, Name: "restricted_value"}}
	lq.GroupBy = nil
	lq.OrderBy = nil
	registry := datasource.NewRegistry()
	registry.Register(fakeDriver{dialect: dialect.PostgresDialect{}})
	service := core.NewQueryService(&core.QueryServiceDeps{
		Models: fakeModelLoader{model: model},
		Datasources: fakeDatasourceLoader{datasource: metadata.Datasource{
			ID: "ds1", Type: "postgres", Config: `{"function_blocklist":["custom_reader"]}`,
		}},
		Drivers:   registry,
		Validator: query.NewValidator(1000),
		Executor:  query.NewExecutor(1000, 0),
	})

	if _, se := service.Compile(ctx, &lq); se == nil {
		t.Fatal("Compile() error = nil, want datasource function blocklist rejection")
	}
}

func TestQueryServiceDryRunBlocksDatasourceDeniedFunctionBeforeExplain(t *testing.T) {
	ctx := context.Background()
	model := coreTestModel()
	model.Dimensions = append(model.Dimensions, semantic.Dimension{
		Name:                 "restricted_value",
		ColumnRef:            "orders.id",
		Type:                 "text",
		CalculatedExpression: "custom_reader(orders.id)",
	})
	lq := coreTestLogicalQuery()
	lq.Select = []query.SelectItem{{Type: query.SelectTypeDimension, Name: "restricted_value"}}
	lq.GroupBy = nil
	lq.OrderBy = nil
	opens := 0
	registry := datasource.NewRegistry()
	registry.Register(countingOpenDriver{dialect: dialect.PostgresDialect{}, opens: &opens})
	service := core.NewQueryService(&core.QueryServiceDeps{
		Models: fakeModelLoader{model: model},
		Datasources: fakeDatasourceLoader{datasource: metadata.Datasource{
			ID: "ds1", Type: "postgres", Config: `{"function_blocklist":["custom_reader"]}`,
		}},
		Drivers:   registry,
		Validator: query.NewValidator(1000),
		Executor:  query.NewExecutor(1000, 0),
	})

	if _, se := service.DryRunWithModel(ctx, &lq, nil); se == nil {
		t.Fatal("DryRunWithModel() error = nil, want datasource function blocklist rejection")
	}
	if opens != 0 {
		t.Fatalf("driver Open calls = %d, want 0 before rejected dry-run", opens)
	}
}

type failingModelLoader struct{}

func (failingModelLoader) GetPublishedFullModel(context.Context, string) (*semantic.SemanticModel, error) {
	return nil, errors.New("model not found")
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

func (fakeDriver) Type() string                       { return "postgres" }
func (fakeDriver) Ping(context.Context, string) error { return nil }
func (fakeDriver) Open(context.Context, string) (*sql.DB, error) {
	return nil, nil //nolint:nilnil // test stub driver is never opened
}
func (fakeDriver) Introspect(context.Context, *sql.DB) (*datasource.IntrospectionResult, error) {
	return nil, nil //nolint:nilnil // optional result
}
func (f fakeDriver) Dialect() dialect.Dialect { return f.dialect }
func (fakeDriver) SupportsReadOnlyTx() bool   { return false }

type countingOpenDriver struct {
	dialect dialect.Dialect
	opens   *int
}

func (countingOpenDriver) Type() string                       { return "postgres" }
func (countingOpenDriver) Ping(context.Context, string) error { return nil }
func (d countingOpenDriver) Open(context.Context, string) (*sql.DB, error) {
	*d.opens++
	return nil, errors.New("driver Open must not be called")
}
func (countingOpenDriver) Introspect(context.Context, *sql.DB) (*datasource.IntrospectionResult, error) {
	return nil, nil //nolint:nilnil // test stub never introspects
}
func (d countingOpenDriver) Dialect() dialect.Dialect { return d.dialect }
func (countingOpenDriver) SupportsReadOnlyTx() bool   { return false }
