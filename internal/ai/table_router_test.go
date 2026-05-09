package ai

import (
	"context"
	"errors"
	"slices"
	"strings"
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

	model, routing, err := router.Route(context.Background(), "ds1", "show total sales by customer", nil, true, true)
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

func TestTableRouter_DateGrainDimensionsOnDateColumns(t *testing.T) {
	reader := testMetadataReader()
	reader.columns = append(reader.columns, metadata.Column{
		DatasourceID: "ds1",
		SchemaName:   "public",
		TableName:    "orders",
		ColumnName:   "orderdate",
		DataType:     "timestamp",
	})
	router := NewTableRouter(reader)

	model, _, err := router.Route(context.Background(), "ds1", "show total sales by customer", nil, true, true)
	if err != nil {
		t.Fatalf("Route() error = %v, want nil", err)
	}
	var yearDim *semantic.Dimension
	for i := range model.Dimensions {
		if model.Dimensions[i].Name == "orderdate_year" {
			yearDim = &model.Dimensions[i]
			break
		}
	}
	if yearDim == nil {
		t.Fatalf("expected orderdate_year dimension, got names: %v", dimNames(model.Dimensions))
	}
	if yearDim.TimeGrain != "year" {
		t.Errorf("orderdate_year TimeGrain = %q, want year", yearDim.TimeGrain)
	}
	if yearDim.ColumnRef != "orders.orderdate" {
		t.Errorf("orderdate_year ColumnRef = %q, want orders.orderdate", yearDim.ColumnRef)
	}
	if !slices.Contains(yearDim.Synonyms, "by year") {
		t.Errorf("orderdate_year synonyms = %v, want by year", yearDim.Synonyms)
	}
}

func dimNames(dims []semantic.Dimension) []string {
	out := make([]string, len(dims))
	for i := range dims {
		out[i] = dims[i].Name
	}
	return out
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

	model, _, err := router.Route(context.Background(), "ds1", "show total sales by customer", nil, true, true)
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

	model, _, err := router.Route(context.Background(), "ds1", "show total sales by customer", nil, true, true)
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

// TestTableRouter_NameResolverPullsInFKChain verifies that a question asking
// for an entity's name pulls in the entity table AND a downstream display-name
// table reached over an FK chain, even when neither was a top-scoring pick.
// AdventureWorks shape: salesorderheader → customer (no name) → person (firstname/lastname).
func TestTableRouter_NameResolverPullsInFKChain(t *testing.T) {
	reader := fakeMetadataReader{
		tables: []metadata.Table{
			{DatasourceID: "ds1", SchemaName: "sales", TableName: "salesorderheader", TableType: "BASE TABLE"},
			{DatasourceID: "ds1", SchemaName: "sales", TableName: "customer", TableType: "BASE TABLE"},
			{DatasourceID: "ds1", SchemaName: "person", TableName: "person", TableType: "BASE TABLE"},
		},
		columns: []metadata.Column{
			{DatasourceID: "ds1", SchemaName: "sales", TableName: "salesorderheader", ColumnName: "salesorderid", DataType: "int", IsPrimaryKey: true},
			{DatasourceID: "ds1", SchemaName: "sales", TableName: "salesorderheader", ColumnName: "customerid", DataType: "int", IsForeignKey: true},
			{DatasourceID: "ds1", SchemaName: "sales", TableName: "salesorderheader", ColumnName: "orderdate", DataType: "timestamp"},
			{DatasourceID: "ds1", SchemaName: "sales", TableName: "customer", ColumnName: "customerid", DataType: "int", IsPrimaryKey: true},
			{DatasourceID: "ds1", SchemaName: "sales", TableName: "customer", ColumnName: "personid", DataType: "int", IsForeignKey: true},
			{DatasourceID: "ds1", SchemaName: "person", TableName: "person", ColumnName: "businessentityid", DataType: "int", IsPrimaryKey: true},
			{DatasourceID: "ds1", SchemaName: "person", TableName: "person", ColumnName: "firstname", DataType: "text"},
			{DatasourceID: "ds1", SchemaName: "person", TableName: "person", ColumnName: "lastname", DataType: "text"},
		},
		relations: []metadata.Relation{
			{DatasourceID: "ds1", ConstraintName: "soh_customer", FromSchema: "sales", FromTable: "salesorderheader", FromColumn: "customerid", ToSchema: "sales", ToTable: "customer", ToColumn: "customerid", RelationshipType: "many_to_one"},
			{DatasourceID: "ds1", ConstraintName: "customer_person", FromSchema: "sales", FromTable: "customer", FromColumn: "personid", ToSchema: "person", ToTable: "person", ToColumn: "businessentityid", RelationshipType: "many_to_one"},
		},
	}
	router := NewTableRouter(reader)

	model, routing, err := router.Route(context.Background(), "ds1", "show each sales order with the customer name and order date", nil, true, true)
	if err != nil {
		t.Fatalf("Route() error = %v, want nil", err)
	}
	if routing.NeedsClarification {
		t.Fatalf("Route() needs clarification = true, want false; routing = %+v", routing)
	}
	got := strings.Join(routing.SelectedTables, ",")
	for _, want := range []string{"sales.salesorderheader", "sales.customer", "person.person"} {
		if !strings.Contains(got, want) {
			t.Errorf("Route() selected = %v, want to contain %q", routing.SelectedTables, want)
		}
	}
	// Display-name dim must be reachable.
	hasFirstname := false
	for _, d := range model.Dimensions {
		if d.ColumnRef == "person.firstname" {
			hasFirstname = true
			break
		}
	}
	if !hasFirstname {
		t.Fatalf("Route() model missing person.firstname dim; dims=%v", dimNames(model.Dimensions))
	}
}

func TestTableRouter_ManualScopeTransitiveJoins(t *testing.T) {
	reader := fakeMetadataReader{
		tables: []metadata.Table{
			{DatasourceID: "ds1", SchemaName: "public", TableName: "emp", TableType: "BASE TABLE"},
			{DatasourceID: "ds1", SchemaName: "public", TableName: "edh", TableType: "BASE TABLE"},
			{DatasourceID: "ds1", SchemaName: "public", TableName: "dept", TableType: "BASE TABLE"},
		},
		columns: []metadata.Column{
			{DatasourceID: "ds1", SchemaName: "public", TableName: "emp", ColumnName: "id", DataType: "int", IsPrimaryKey: true},
			{DatasourceID: "ds1", SchemaName: "public", TableName: "edh", ColumnName: "emp_id", DataType: "int", IsForeignKey: true},
			{DatasourceID: "ds1", SchemaName: "public", TableName: "edh", ColumnName: "dept_id", DataType: "int", IsForeignKey: true},
			{DatasourceID: "ds1", SchemaName: "public", TableName: "dept", ColumnName: "id", DataType: "int", IsPrimaryKey: true},
			{DatasourceID: "ds1", SchemaName: "public", TableName: "dept", ColumnName: "name", DataType: "text"},
		},
		relations: []metadata.Relation{
			{
				DatasourceID: "ds1", ConstraintName: "edh_emp",
				FromSchema: "public", FromTable: "emp", FromColumn: "id",
				ToSchema: "public", ToTable: "edh", ToColumn: "emp_id",
			},
			{
				DatasourceID: "ds1", ConstraintName: "edh_dept",
				FromSchema: "public", FromTable: "edh", FromColumn: "dept_id",
				ToSchema: "public", ToTable: "dept", ToColumn: "id",
			},
		},
	}
	router := NewTableRouter(reader)
	model, routing, err := router.Route(context.Background(), "ds1", "list", []string{"public.emp", "public.edh", "public.dept"}, true, true)
	if err != nil {
		t.Fatalf("Route() error = %v", err)
	}
	if routing.NeedsClarification {
		t.Fatalf("unexpected clarification: %+v", routing)
	}
	if len(model.Joins) != 2 {
		t.Fatalf("Joins() = %d, want 2: %+v", len(model.Joins), model.Joins)
	}
	var hasDeptName bool
	for _, d := range model.Dimensions {
		if d.ColumnRef == "dept.name" {
			hasDeptName = true
			break
		}
	}
	if !hasDeptName {
		t.Fatalf("expected dept.name dimension, got %+v", model.Dimensions)
	}
}

func TestTableRouter_RouteUsesManualScope(t *testing.T) {
	router := NewTableRouter(testMetadataReader())

	model, routing, err := router.Route(context.Background(), "ds1", "anything", []string{"public.customers"}, true, true)
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

	_, _, err := router.Route(context.Background(), "ds1", "anything", []string{"public.missing"}, true, true)
	if !errors.Is(err, ErrTableScopeInvalid) {
		t.Fatalf("Route() error = %v, want ErrTableScopeInvalid", err)
	}
}

func TestTableRouter_RouteNeedsClarificationForNoMatch(t *testing.T) {
	router := NewTableRouter(testMetadataReader())

	model, routing, err := router.Route(context.Background(), "ds1", "completely unrelated question", nil, true, true)
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

func TestExpandSelectedWithJoinBridges_AddsIntermediateTables(t *testing.T) {
	tables := []metadata.Table{
		{DatasourceID: "ds1", SchemaName: "sales", TableName: "header", TableType: "BASE TABLE"},
		{DatasourceID: "ds1", SchemaName: "sales", TableName: "detail", TableType: "BASE TABLE"},
		{DatasourceID: "ds1", SchemaName: "prod", TableName: "productcategory", TableType: "BASE TABLE"},
	}
	relations := []metadata.Relation{
		{DatasourceID: "ds1", FromSchema: "sales", FromTable: "header", FromColumn: "id", ToSchema: "sales", ToTable: "detail", ToColumn: "order_id"},
		{DatasourceID: "ds1", FromSchema: "sales", FromTable: "detail", FromColumn: "product_id", ToSchema: "prod", ToTable: "product", ToColumn: "id"},
		{DatasourceID: "ds1", FromSchema: "prod", FromTable: "product", FromColumn: "cat_id", ToSchema: "prod", ToTable: "productcategory", ToColumn: "id"},
	}
	tables = append(tables, metadata.Table{DatasourceID: "ds1", SchemaName: "prod", TableName: "product", TableType: "BASE TABLE"})
	idx := indexTables(tables)
	selected := []tableBundle{
		{table: tables[0], score: 10},
		{table: tables[2], score: 8},
	}
	out := expandSelectedWithJoinBridges(selected, relations, idx, 10)
	keys := make(map[string]bool)
	for _, b := range out {
		keys[tableKey(b.table.SchemaName, b.table.TableName)] = true
	}
	for _, need := range []string{"sales.header", "sales.detail", "prod.product", "prod.productcategory"} {
		if !keys[need] {
			t.Fatalf("missing %s, got %v", need, keys)
		}
	}
}

func TestTableRouter_RouteTypeScopeEmpty(t *testing.T) {
	router := NewTableRouter(testMetadataReader())
	_, _, err := router.Route(context.Background(), "ds1", "anything", nil, false, false)
	if !errors.Is(err, ErrTypeScopeEmpty) {
		t.Fatalf("Route() error = %v, want ErrTypeScopeEmpty", err)
	}
}

func TestTableRouter_RouteRejectsManualBaseTableWhenViewsOnly(t *testing.T) {
	router := NewTableRouter(testMetadataReader())
	_, _, err := router.Route(context.Background(), "ds1", "anything", []string{"public.orders"}, false, true)
	if !errors.Is(err, ErrTableScopeInvalid) {
		t.Fatalf("Route() error = %v, want ErrTableScopeInvalid", err)
	}
}

func TestTableRouter_RouteViewsOnlyAutoSelectsView(t *testing.T) {
	reader := testMetadataReader()
	reader.tables = append(reader.tables, metadata.Table{
		DatasourceID: "ds1",
		SchemaName:   "public",
		TableName:    "v_sales",
		TableType:    "VIEW",
	})
	reader.columns = append(reader.columns,
		metadata.Column{DatasourceID: "ds1", SchemaName: "public", TableName: "v_sales", ColumnName: "total_amount", DataType: "numeric"},
		metadata.Column{DatasourceID: "ds1", SchemaName: "public", TableName: "v_sales", ColumnName: "customer_name", DataType: "text"},
	)
	router := NewTableRouter(reader)

	model, routing, err := router.Route(context.Background(), "ds1", "show total sales by customer", nil, false, true)
	if err != nil {
		t.Fatalf("Route() error = %v, want nil", err)
	}
	if routing.NeedsClarification {
		t.Fatalf("Route() needs clarification = true, want false; routing = %+v", routing)
	}
	if model == nil || model.BaseTable != "v_sales" {
		t.Fatalf("Route() base table = %q, want v_sales", model.BaseTable)
	}
}

func testMetadataReader() fakeMetadataReader {
	return fakeMetadataReader{
		tables: []metadata.Table{
			{DatasourceID: "ds1", SchemaName: "public", TableName: "orders", TableType: "BASE TABLE"},
			{DatasourceID: "ds1", SchemaName: "public", TableName: "customers", TableType: "BASE TABLE"},
			{DatasourceID: "ds1", SchemaName: "public", TableName: "products", TableType: "BASE TABLE"},
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
