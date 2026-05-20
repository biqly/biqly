package ai

import (
	"context"
	"errors"
	"slices"
	"strconv"
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
		return
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
	if routing.ContextSource != "auto" {
		t.Errorf("Route() context source = %q, want auto", routing.ContextSource)
	}
	if routing.ContextKey == "" {
		t.Errorf("Route() context key is empty, want generated context key")
	}
	if !slices.Contains(routing.SelectedDimensions, "name") {
		t.Errorf("Route() selected dimensions = %v, want name", routing.SelectedDimensions)
	}
	if !slices.Contains(routing.SelectedMetrics, "sum_total_amount") {
		t.Errorf("Route() selected metrics = %v, want sum_total_amount", routing.SelectedMetrics)
	}
	ordersCandidate, ok := findRoutingCandidate(routing.Candidates, "public.orders")
	if !ok {
		t.Fatalf("Route() candidates = %+v, want public.orders candidate", routing.Candidates)
	}
	if !ordersCandidate.Selected {
		t.Errorf("Route() public.orders selected = false, want true")
	}
	if ordersCandidate.KeywordScore <= 0 {
		t.Errorf("Route() public.orders keyword score = %v, want > 0", ordersCandidate.KeywordScore)
	}
	if ordersCandidate.TotalScore <= 0 {
		t.Errorf("Route() public.orders total score = %v, want > 0", ordersCandidate.TotalScore)
	}
	if routing.Debug == nil {
		t.Fatalf("Route() debug = nil, want routing debug details")
		return
	}
	if !slices.Contains(routing.Debug.RelationExpansion, "public.orders.customer_id = public.customers.id") {
		t.Errorf("Route() relation expansion = %v, want orders/customers relation", routing.Debug.RelationExpansion)
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
		return
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

// TestTableRouter_DateGrainHourDimensionForTimestamp covers the hourly /
// saatlik grain that auto-context emits for timestamp/datetime columns. Pure
// DATE columns should not get a `_hour` variant (hour bucketing on a clockless
// value is meaningless).
func TestTableRouter_DateGrainHourDimensionForTimestamp(t *testing.T) {
	reader := testMetadataReader()
	reader.columns = append(reader.columns,
		metadata.Column{
			DatasourceID: "ds1", SchemaName: "public", TableName: "orders",
			ColumnName: "orderdate", DataType: "timestamp",
		},
		metadata.Column{
			DatasourceID: "ds1", SchemaName: "public", TableName: "orders",
			ColumnName: "ship_date", DataType: "date",
		},
	)
	router := NewTableRouter(reader)

	model, _, err := router.Route(context.Background(), "ds1", "saatlik sipariş sayısı", nil, true, true)
	if err != nil {
		t.Fatalf("Route() error = %v, want nil", err)
	}

	var hourDim *semantic.Dimension
	for i := range model.Dimensions {
		if model.Dimensions[i].Name == "orderdate_hour" {
			hourDim = &model.Dimensions[i]
			break
		}
	}
	if hourDim == nil {
		t.Fatalf("expected orderdate_hour dimension for timestamp column, got names: %v", dimNames(model.Dimensions))
		return
	}
	if hourDim.TimeGrain != "hour" {
		t.Errorf("orderdate_hour TimeGrain = %q, want hour", hourDim.TimeGrain)
	}
	for _, s := range []string{"hourly", "saatlik", "by hour"} {
		if !slices.Contains(hourDim.Synonyms, s) {
			t.Errorf("orderdate_hour synonyms missing %q; got %v", s, hourDim.Synonyms)
		}
	}

	// ship_date (pure DATE) must NOT receive an _hour variant.
	for _, d := range model.Dimensions {
		if d.Name == "ship_date_hour" {
			t.Errorf("ship_date is pure DATE; should not produce ship_date_hour, but found: %+v", d)
		}
	}
}

// TestTableRouter_DateGrainDayDimensionAdded covers the daily/günlük grain that
// was missing from auto-context: prior to this change date columns only got
// year/quarter/month variants so AI fell back to monthly for "günlük" questions.
func TestTableRouter_DateGrainDayDimensionAdded(t *testing.T) {
	reader := testMetadataReader()
	reader.columns = append(reader.columns, metadata.Column{
		DatasourceID: "ds1",
		SchemaName:   "public",
		TableName:    "orders",
		ColumnName:   "orderdate",
		DataType:     "timestamp",
	})
	router := NewTableRouter(reader)

	model, _, err := router.Route(context.Background(), "ds1", "günlük sipariş sayısı", nil, true, true)
	if err != nil {
		t.Fatalf("Route() error = %v, want nil", err)
	}

	var dayDim *semantic.Dimension
	for i := range model.Dimensions {
		if model.Dimensions[i].Name == "orderdate_day" {
			dayDim = &model.Dimensions[i]
			break
		}
	}
	if dayDim == nil {
		t.Fatalf("expected orderdate_day dimension, got names: %v", dimNames(model.Dimensions))
		return
	}
	if dayDim.TimeGrain != "day" {
		t.Errorf("orderdate_day TimeGrain = %q, want day", dayDim.TimeGrain)
	}
	if dayDim.ColumnRef != "orders.orderdate" {
		t.Errorf("orderdate_day ColumnRef = %q, want orders.orderdate", dayDim.ColumnRef)
	}
	wantSyns := []string{"daily", "günlük", "by day"}
	for _, s := range wantSyns {
		if !slices.Contains(dayDim.Synonyms, s) {
			t.Errorf("orderdate_day synonyms missing %q; got %v", s, dayDim.Synonyms)
		}
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
		return
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

// Turkish "top products by quantity" must not fall through to zero-score routing:
// normalized "ürünü" is urunu, which must expand to product/urun tokens.
func TestTableRouter_TurkishTopProductsByQuantity(t *testing.T) {
	reader := fakeMetadataReader{
		tables: []metadata.Table{
			{DatasourceID: "ds1", SchemaName: "humanresources", TableName: "employee", TableType: "BASE TABLE"},
			{DatasourceID: "ds1", SchemaName: "sales", TableName: "salesorderdetail", TableType: "BASE TABLE"},
			{DatasourceID: "ds1", SchemaName: "production", TableName: "product", TableType: "BASE TABLE"},
		},
		columns: []metadata.Column{
			{DatasourceID: "ds1", SchemaName: "humanresources", TableName: "employee", ColumnName: "businessentityid", DataType: "int", IsPrimaryKey: true},
			{DatasourceID: "ds1", SchemaName: "sales", TableName: "salesorderdetail", ColumnName: "salesorderdetailid", DataType: "int", IsPrimaryKey: true},
			{DatasourceID: "ds1", SchemaName: "sales", TableName: "salesorderdetail", ColumnName: "productid", DataType: "int", IsForeignKey: true},
			{DatasourceID: "ds1", SchemaName: "sales", TableName: "salesorderdetail", ColumnName: "orderqty", DataType: "int"},
			{DatasourceID: "ds1", SchemaName: "production", TableName: "product", ColumnName: "productid", DataType: "int", IsPrimaryKey: true},
			{DatasourceID: "ds1", SchemaName: "production", TableName: "product", ColumnName: "name", DataType: "text"},
		},
		relations: []metadata.Relation{
			{
				DatasourceID: "ds1", ConstraintName: "sod_product",
				FromSchema: "sales", FromTable: "salesorderdetail", FromColumn: "productid",
				ToSchema: "production", ToTable: "product", ToColumn: "productid",
				RelationshipType: "many_to_one",
			},
		},
	}
	router := NewTableRouter(reader)

	q := "En çok satan 5 ürünü adet bazında göster."
	model, routing, err := router.Route(context.Background(), "ds1", q, nil, true, true)
	if err != nil {
		t.Fatalf("Route() error = %v", err)
	}
	if routing.NeedsClarification {
		t.Fatalf("Route() needs clarification, routing=%+v candidates=%+v", routing, routing.Candidates)
	}
	if model == nil {
		t.Fatal("Route() model = nil")
		return
	}
	got := strings.Join(routing.SelectedTables, ",")
	for _, need := range []string{"sales.salesorderdetail", "production.product"} {
		if !strings.Contains(got, need) {
			t.Errorf("selected %q want substring %q", got, need)
		}
	}
	if routing.Confidence < minRouteConfidence {
		t.Errorf("confidence %v < minRouteConfidence", routing.Confidence)
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
	tables := make([]metadata.Table, 0, 4)
	tables = append(tables,
		metadata.Table{DatasourceID: "ds1", SchemaName: "sales", TableName: "header", TableType: "BASE TABLE"},
		metadata.Table{DatasourceID: "ds1", SchemaName: "sales", TableName: "detail", TableType: "BASE TABLE"},
		metadata.Table{DatasourceID: "ds1", SchemaName: "prod", TableName: "productcategory", TableType: "BASE TABLE"},
	)
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

func TestTableRouter_ColumnEmbeddingsNarrowWideTableButKeepRequiredColumns(t *testing.T) {
	reader := fakeMetadataReader{
		tables: []metadata.Table{
			{DatasourceID: "ds1", SchemaName: "sales", TableName: "salesorderheader", TableType: "BASE TABLE"},
		},
	}
	reader.columns = append(reader.columns,
		metadata.Column{DatasourceID: "ds1", SchemaName: "sales", TableName: "salesorderheader", ColumnName: "salesorderid", DataType: "int", IsPrimaryKey: true},
		metadata.Column{DatasourceID: "ds1", SchemaName: "sales", TableName: "salesorderheader", ColumnName: "customerid", DataType: "int", IsForeignKey: true},
		metadata.Column{DatasourceID: "ds1", SchemaName: "sales", TableName: "salesorderheader", ColumnName: "orderdate", DataType: "timestamp"},
		metadata.Column{DatasourceID: "ds1", SchemaName: "sales", TableName: "salesorderheader", ColumnName: "totaldue", DataType: "numeric"},
	)
	for i := 0; i < 40; i++ {
		reader.columns = append(reader.columns, metadata.Column{
			DatasourceID: "ds1",
			SchemaName:   "sales",
			TableName:    "salesorderheader",
			ColumnName:   "noise_" + strconv.Itoa(i),
			DataType:     "text",
		})
	}

	columnEmbeddings := make([]metadata.ColumnEmbedding, 0, len(reader.columns))
	for _, col := range reader.columns {
		vec := []float32{-1, 0}
		if col.ColumnName == "totaldue" {
			vec = []float32{1, 0}
		}
		columnEmbeddings = append(columnEmbeddings, metadata.ColumnEmbedding{
			SchemaName: col.SchemaName,
			TableName:  col.TableName,
			ColumnName: col.ColumnName,
			Model:      "fake",
			Embedding:  vec,
		})
	}
	router := NewTableRouterWithEmbeddings(
		reader,
		&fakeEmbedder{model: "fake", vectors: map[string][]float32{"Yıllara göre toplam satış tutarını göster.": {1, 0}}, fallback: []float32{1, 0}},
		&fakeEmbeddingReader{columnEmbeddings: columnEmbeddings},
		30.0,
	)

	model, routing, err := router.Route(context.Background(), "ds1", "Yıllara göre toplam satış tutarını göster.", []string{"sales.salesorderheader"}, true, true)
	if err != nil {
		t.Fatalf("Route() error = %v, want nil", err)
	}
	if routing.NeedsClarification {
		t.Fatalf("Route() needs clarification = true, routing=%+v", routing)
	}
	if !hasMetric(model.Metrics, "sum_totaldue", "salesorderheader.totaldue") {
		t.Fatalf("metrics missing sum_totaldue: %+v", model.Metrics)
	}
	if !hasDimension(model.Dimensions, "orderdate_year", "salesorderheader.orderdate") {
		t.Fatalf("dimensions missing orderdate_year; got %v", dimNames(model.Dimensions))
	}
	if len(model.Dimensions) >= len(reader.columns) {
		t.Fatalf("column embeddings should narrow wide table columns; dims=%d columns=%d", len(model.Dimensions), len(reader.columns))
	}
	if hasDimension(model.Dimensions, "noise_39", "salesorderheader.noise_39") {
		t.Fatalf("column embeddings should have filtered tail low-similarity noise column; dims=%v", dimNames(model.Dimensions))
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

func TestSoftDeleteColumnSynonyms(t *testing.T) {
	tests := []struct {
		col, typ string
		substr   string
		empty    bool
	}{
		{"deleted_at", "timestamp with time zone", "silinen", false},
		{"timeline_tweets_deleted_at", "timestamptz", "silinen", false},
		{"archived_at", "timestamp with time zone", "arsiv", false},
		{"is_deleted", "boolean", "silinen", false},
		{"is_deleted", "bool", "deleted", false},
		{"delete_flag", "integer", "silinen", false},
		{"created_at", "timestamp with time zone", "", true},
		{"email", "text", "", true},
	}
	for _, tt := range tests {
		got := softDeleteColumnSynonyms(tt.col, tt.typ)
		if tt.empty {
			if len(got) != 0 {
				t.Errorf("%s %s: want no synonyms, got %v", tt.col, tt.typ, got)
			}
			continue
		}
		if !slices.Contains(got, tt.substr) {
			t.Errorf("%s %s: want synonyms to contain %q, got %v", tt.col, tt.typ, tt.substr, got)
		}
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

func hasDimension(dimensions []semantic.Dimension, name, columnRef string) bool {
	for _, dimension := range dimensions {
		if dimension.Name == name && dimension.ColumnRef == columnRef {
			return true
		}
	}
	return false
}

func findRoutingCandidate(candidates []TableCandidate, table string) (TableCandidate, bool) {
	for _, candidate := range candidates {
		if candidate.Table == table {
			return candidate, true
		}
	}
	return TableCandidate{}, false
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
