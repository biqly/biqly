package ai

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/biqly/biqly/internal/dialect"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/semantic"
)

type fakeMetadataReader struct {
	tables    []metadata.Table
	columns   []metadata.Column
	relations []metadata.Relation
}

func (f fakeMetadataReader) ListTables(context.Context, string, string) ([]metadata.Table, error) {
	return f.tables, nil
}

func (f fakeMetadataReader) ListColumns(context.Context, string, string, string) ([]metadata.Column, error) {
	return f.columns, nil
}

func (f fakeMetadataReader) ListRelations(context.Context, string) ([]metadata.Relation, error) {
	return f.relations, nil
}

func TestTableRouter_RouteSelectsRelatedTables(t *testing.T) {
	router := NewTableRouter(testMetadataReader())

	model, routing, err := router.Route(context.Background(), "ds1", "show total sales by customer", nil)
	if err != nil {
		t.Fatalf("Route() error = %v, want nil", err)
	}
	if model == nil {
		t.Fatal("Route() model = nil, want semantic model")
	}
	if routing.NeedsClarification {
		t.Fatalf("Route() needs clarification = true, want false; routing = %+v", routing)
	}

	wantTables := []string{"public.orders", "public.customers"}
	if !sameStrings(routing.SelectedTables, wantTables) {
		t.Errorf("Route() selected tables = %v, want %v", routing.SelectedTables, wantTables)
	}
	if len(routing.JoinPaths) != 1 {
		t.Errorf("Route() join path count = %d, want 1", len(routing.JoinPaths))
	}
	if model.BaseTable != "orders" {
		t.Errorf("Route() base table = %q, want %q", model.BaseTable, "orders")
	}
	if !hasMetric(model.Metrics, "row_count", "*") {
		t.Errorf("Route() metrics = %+v, want row_count COUNT(*) metric", model.Metrics)
	}
	if !hasMetric(model.Metrics, "sum_total_amount", "orders.total_amount") {
		t.Errorf("Route() metrics = %+v, want sum_total_amount metric", model.Metrics)
	}

	lq := query.LogicalQuery{
		DatasourceID: "ds1",
		ModelID:      model.Name,
		Select: []query.SelectItem{
			{Type: query.SelectTypeDimension, Name: "name"},
			{Type: query.SelectTypeMetric, Name: "sum_total_amount"},
		},
		GroupBy: []query.GroupBy{{Field: "name"}},
		Limit:   100,
	}
	compiled, err := query.NewCompiler(dialect.PostgresDialect{}).Compile(context.Background(), lq, model)
	if err != nil {
		t.Fatalf("Compile() with routed model error = %v, want nil", err)
	}
	wantSQL := `SELECT "customers"."name" AS "name", SUM("orders"."total_amount") AS "sum_total_amount" FROM "public"."orders" LEFT JOIN "public"."customers" ON "public"."orders"."customer_id" = "public"."customers"."id" GROUP BY "customers"."name" LIMIT 100`
	if compiled.SQL != wantSQL {
		t.Errorf("Compile() with routed model SQL = %q, want %q", compiled.SQL, wantSQL)
	}
}

func TestTableRouter_BuildsMinMaxMetricsForDateColumns(t *testing.T) {
	reader := testMetadataReader()
	reader.columns = append(reader.columns, metadata.Column{
		DatasourceID: "ds1",
		SchemaName:   "public",
		TableName:    "orders",
		ColumnName:   "order_date",
		DataType:     "timestamp",
	})
	router := NewTableRouter(reader)

	model, _, err := router.Route(context.Background(), "ds1", "show total sales by customer", nil)
	if err != nil {
		t.Fatalf("Route() error = %v, want nil", err)
	}
	if !hasMetric(model.Metrics, "max_order_date", "orders.order_date") {
		t.Errorf("Route() metrics = %+v, want max_order_date metric", model.Metrics)
	}
	if !hasMetric(model.Metrics, "min_order_date", "orders.order_date") {
		t.Errorf("Route() metrics = %+v, want min_order_date metric", model.Metrics)
	}

	lq := query.LogicalQuery{
		DatasourceID: "ds1",
		ModelID:      model.Name,
		Select: []query.SelectItem{
			{Type: query.SelectTypeDimension, Name: "name"},
			{Type: query.SelectTypeMetric, Name: "row_count"},
			{Type: query.SelectTypeMetric, Name: "max_order_date"},
		},
		GroupBy: []query.GroupBy{{Field: "name"}},
		Limit:   100,
	}
	if _, err := query.NewCompiler(dialect.PostgresDialect{}).Compile(context.Background(), lq, model); err != nil {
		t.Fatalf("Compile() with date metric error = %v, want nil", err)
	}
}

func TestTableRouter_DisplayNameDimensionInheritsTableSynonyms(t *testing.T) {
	router := NewTableRouter(testMetadataReader())

	model, _, err := router.Route(context.Background(), "ds1", "show total sales by customer", nil)
	if err != nil {
		t.Fatalf("Route() error = %v, want nil", err)
	}

	var nameDim *semantic.Dimension
	for i := range model.Dimensions {
		if model.Dimensions[i].ColumnRef == "customers.name" {
			nameDim = &model.Dimensions[i]
			break
		}
	}
	if nameDim == nil {
		t.Fatalf("expected customers.name dimension, got %+v", model.Dimensions)
	}

	want := []string{"customer", "musteri"}
	for _, syn := range want {
		if !slices.Contains(nameDim.Synonyms, syn) {
			t.Errorf("customers.name synonyms = %v, want to contain %q", nameDim.Synonyms, syn)
		}
	}
}

func TestTableRouter_RouteUsesManualScope(t *testing.T) {
	router := NewTableRouter(testMetadataReader())

	model, routing, err := router.Route(context.Background(), "ds1", "anything", []string{"public.customers"})
	if err != nil {
		t.Fatalf("Route() error = %v, want nil", err)
	}
	if model.BaseTable != "customers" {
		t.Errorf("Route() base table = %q, want %q", model.BaseTable, "customers")
	}
	if !routing.Manual {
		t.Errorf("Route() manual = false, want true")
	}
	if routing.Confidence != 1 {
		t.Errorf("Route() confidence = %v, want 1", routing.Confidence)
	}
}

func TestTableRouter_RouteRejectsInvalidManualScope(t *testing.T) {
	router := NewTableRouter(testMetadataReader())

	_, _, err := router.Route(context.Background(), "ds1", "anything", []string{"public.missing"})
	if !errors.Is(err, ErrTableScopeInvalid) {
		t.Fatalf("Route() error = %v, want ErrTableScopeInvalid", err)
	}
}

func TestTableRouter_RouteNeedsClarificationForNoMatch(t *testing.T) {
	router := NewTableRouter(testMetadataReader())

	model, routing, err := router.Route(context.Background(), "ds1", "completely unrelated question", nil)
	if err != nil {
		t.Fatalf("Route() error = %v, want nil", err)
	}
	if model != nil {
		t.Fatalf("Route() model = %+v, want nil", model)
	}
	if !routing.NeedsClarification {
		t.Errorf("Route() needs clarification = false, want true")
	}
}

func testMetadataReader() fakeMetadataReader {
	return fakeMetadataReader{
		tables: []metadata.Table{
			{DatasourceID: "ds1", SchemaName: "public", TableName: "orders"},
			{DatasourceID: "ds1", SchemaName: "public", TableName: "customers"},
			{DatasourceID: "ds1", SchemaName: "public", TableName: "products"},
		},
		columns: []metadata.Column{
			{DatasourceID: "ds1", SchemaName: "public", TableName: "orders", ColumnName: "id", DataType: "uuid", IsPrimaryKey: true},
			{DatasourceID: "ds1", SchemaName: "public", TableName: "orders", ColumnName: "customer_id", DataType: "uuid", IsForeignKey: true},
			{DatasourceID: "ds1", SchemaName: "public", TableName: "orders", ColumnName: "total_amount", DataType: "numeric"},
			{DatasourceID: "ds1", SchemaName: "public", TableName: "customers", ColumnName: "id", DataType: "uuid", IsPrimaryKey: true},
			{DatasourceID: "ds1", SchemaName: "public", TableName: "customers", ColumnName: "name", DataType: "text"},
			{DatasourceID: "ds1", SchemaName: "public", TableName: "products", ColumnName: "id", DataType: "uuid", IsPrimaryKey: true},
			{DatasourceID: "ds1", SchemaName: "public", TableName: "products", ColumnName: "name", DataType: "text"},
		},
		relations: []metadata.Relation{
			{
				DatasourceID:     "ds1",
				ConstraintName:   "orders_customer_id_fkey",
				FromSchema:       "public",
				FromTable:        "orders",
				FromColumn:       "customer_id",
				ToSchema:         "public",
				ToTable:          "customers",
				ToColumn:         "id",
				RelationshipType: "many_to_one",
			},
		},
	}
}

func hasMetric(metrics []semantic.Metric, name, expression string) bool {
	for _, metric := range metrics {
		if metric.Name == name && metric.Expression == expression {
			return true
		}
	}
	return false
}

func sameStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
