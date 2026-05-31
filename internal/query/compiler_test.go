package query

import (
	"context"
	"strings"
	"testing"

	"github.com/biqly/biqly/internal/dialect"
	"github.com/biqly/biqly/internal/semantic"
)

func TestCompiler_SimpleSelect(t *testing.T) {
	model := &semantic.SemanticModel{
		Name:       "orders",
		BaseSchema: "public",
		BaseTable:  "orders",
		Dimensions: []semantic.Dimension{
			{Name: "country", ColumnRef: "customers.country", Type: "text"},
			{Name: "created_at", ColumnRef: "orders.created_at", Type: "date"},
		},
		Metrics: []semantic.Metric{
			{Name: "order_count", Expression: "orders.id", Aggregation: "count"},
		},
		Joins: []semantic.Join{
			{
				Name:         "orders_customers",
				FromTable:    "orders",
				FromColumn:   "customer_id",
				ToTable:      "customers",
				ToColumn:     "id",
				JoinType:     "LEFT",
				Relationship: "many_to_one",
			},
		},
	}

	lq := LogicalQuery{
		DatasourceID: "ds1",
		ModelID:      "orders",
		Select: []SelectItem{
			{Type: "dimension", Name: "country"},
			{Type: "metric", Name: "order_count"},
		},
		Filters: []Filter{
			{Field: "created_at", Operator: OpGte, Value: "2026-01-01"},
		},
		GroupBy: []GroupBy{
			{Field: "country"},
		},
		OrderBy: []OrderBy{
			{Field: "order_count", Direction: "desc"},
		},
		Limit: 100,
	}

	compiler := NewCompiler(dialect.PostgresDialect{})
	cq, err := compiler.Compile(context.Background(), &lq, model)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedSQL := `SELECT "customers"."country" AS "country", COUNT("orders"."id") AS "order_count" FROM "public"."orders" LEFT JOIN "public"."customers" ON "public"."orders"."customer_id" = "public"."customers"."id" WHERE "orders"."created_at" >= $1 GROUP BY "customers"."country" ORDER BY "order_count" DESC LIMIT 100`

	if cq.SQL != expectedSQL {
		t.Errorf("SQL mismatch.\nGot:\n%s\n\nExpected:\n%s", cq.SQL, expectedSQL)
	}

	if len(cq.Args) != 1 {
		t.Errorf("expected 1 arg, got %d", len(cq.Args))
	}
	if cq.Args[0] != "2026-01-01" {
		t.Errorf("expected arg 0 to be '2026-01-01', got %v", cq.Args[0])
	}
}

func TestCompiler_RejectsUnknownGroupByAndOrderBy(t *testing.T) {
	model := &semantic.SemanticModel{
		Name:       "orders",
		BaseSchema: "public",
		BaseTable:  "orders",
		Dimensions: []semantic.Dimension{
			{Name: "created_at_year", ColumnRef: "orders.created_at", Type: "date", TimeGrain: "year"},
		},
		Metrics: []semantic.Metric{
			{Name: "order_count", Expression: "orders.id", Aggregation: "count"},
		},
	}
	compiler := NewCompiler(dialect.PostgresDialect{})

	_, err := compiler.Compile(context.Background(), &LogicalQuery{
		Select:  []SelectItem{{Type: SelectTypeMetric, Name: "order_count"}},
		GroupBy: []GroupBy{{Field: "year(orders.created_at)"}},
	}, model)
	if err == nil || !strings.Contains(err.Error(), "unknown dimension") {
		t.Fatalf("expected unknown group_by dimension error, got %v", err)
	}

	_, err = compiler.Compile(context.Background(), &LogicalQuery{
		Select:  []SelectItem{{Type: SelectTypeMetric, Name: "order_count"}},
		OrderBy: []OrderBy{{Field: "missing_metric", Direction: OrderDesc}},
	}, model)
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected unknown order_by field error, got %v", err)
	}
}

func TestCompiler_OmitsJoinsWhenQueryUsesOnlyBaseTable(t *testing.T) {
	// Two FKs from product to billofmaterials (component vs assembly). If the logical query
	// only references product columns, we must not JOIN billofmaterials twice (or at all).
	model := &semantic.SemanticModel{
		Name:       "auto:production.product,production.billofmaterials",
		BaseSchema: "production",
		BaseTable:  "product",
		Dimensions: []semantic.Dimension{
			{Name: "name", ColumnRef: "product.name", Type: "text"},
			{Name: "productid", ColumnRef: "product.productid", Type: "number"},
			{Name: "bomlevel", ColumnRef: "billofmaterials.bomlevel", Type: "number"},
		},
		Joins: []semantic.Join{
			{
				Name:       "product_component_fk",
				FromTable:  "product",
				FromColumn: "productid",
				ToTable:    "billofmaterials",
				ToColumn:   "componentid",
				JoinType:   "LEFT",
			},
			{
				Name:       "product_assembly_fk",
				FromTable:  "product",
				FromColumn: "productid",
				ToTable:    "billofmaterials",
				ToColumn:   "productassemblyid",
				JoinType:   "LEFT",
			},
		},
	}

	lq := LogicalQuery{
		DatasourceID: "ds1",
		ModelID:      model.Name,
		Select: []SelectItem{
			{Type: SelectTypeDimension, Name: "name"},
			{Type: SelectTypeDimension, Name: "productid"},
		},
		Limit: 100,
	}

	cq, err := NewCompiler(dialect.PostgresDialect{}).Compile(context.Background(), &lq, model)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if containsStr(cq.SQL, "JOIN") {
		t.Fatalf("expected no JOIN when only base table columns are used, got SQL:\n%s", cq.SQL)
	}
}

func TestCompiler_SingleJoinWhenRelatedTableColumnUsed(t *testing.T) {
	model := &semantic.SemanticModel{
		Name:       "auto:production.product,production.billofmaterials",
		BaseSchema: "production",
		BaseTable:  "product",
		Dimensions: []semantic.Dimension{
			{Name: "name", ColumnRef: "product.name", Type: "text"},
			{Name: "bomlevel", ColumnRef: "billofmaterials.bomlevel", Type: "number"},
		},
		Joins: []semantic.Join{
			{
				Name:       "product_component_fk",
				FromTable:  "product",
				FromColumn: "productid",
				ToTable:    "billofmaterials",
				ToColumn:   "componentid",
				JoinType:   "LEFT",
			},
			{
				Name:       "product_assembly_fk",
				FromTable:  "product",
				FromColumn: "productid",
				ToTable:    "billofmaterials",
				ToColumn:   "productassemblyid",
				JoinType:   "LEFT",
			},
		},
	}

	lq := LogicalQuery{
		DatasourceID: "ds1",
		ModelID:      model.Name,
		Select: []SelectItem{
			{Type: SelectTypeDimension, Name: "name"},
			{Type: SelectTypeDimension, Name: "bomlevel"},
		},
		Limit: 50,
	}

	cq, err := NewCompiler(dialect.PostgresDialect{}).Compile(context.Background(), &lq, model)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	n := strings.Count(cq.SQL, "LEFT JOIN")
	if n != 1 {
		t.Fatalf("expected exactly 1 LEFT JOIN, got %d in SQL:\n%s", n, cq.SQL)
	}
}

func TestCompiler_TimeGrainYearGroupBy(t *testing.T) {
	model := &semantic.SemanticModel{
		Name:       "sales_orders",
		BaseSchema: "sales",
		BaseTable:  "salesorderheader",
		Dimensions: []semantic.Dimension{
			{Name: "orderdate", ColumnRef: "salesorderheader.orderdate", Type: "date"},
			{
				Name:      "orderdate_year",
				ColumnRef: "salesorderheader.orderdate",
				Type:      string(semantic.DimensionTypeDate),
				TimeGrain: "year",
			},
		},
		Metrics: []semantic.Metric{
			{Name: "sum_totaldue", Expression: "salesorderheader.totaldue", Aggregation: "sum"},
		},
	}

	lq := LogicalQuery{
		DatasourceID: "ds1",
		ModelID:      model.Name,
		Select: []SelectItem{
			{Type: SelectTypeDimension, Name: "orderdate_year"},
			{Type: SelectTypeMetric, Name: "sum_totaldue"},
		},
		GroupBy: []GroupBy{{Field: "orderdate_year"}},
		OrderBy: []OrderBy{{Field: "orderdate_year", Direction: "asc"}},
		Limit:   100,
	}

	cq, err := NewCompiler(dialect.PostgresDialect{}).Compile(context.Background(), &lq, model)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	want := `SELECT CAST(EXTRACT(YEAR FROM "salesorderheader"."orderdate") AS INTEGER) AS "orderdate_year", SUM("salesorderheader"."totaldue") AS "sum_totaldue" FROM "sales"."salesorderheader" GROUP BY CAST(EXTRACT(YEAR FROM "salesorderheader"."orderdate") AS INTEGER) ORDER BY CAST(EXTRACT(YEAR FROM "salesorderheader"."orderdate") AS INTEGER) ASC LIMIT 100`
	if cq.SQL != want {
		t.Errorf("SQL mismatch.\nGot:\n%s\n\nWant:\n%s", cq.SQL, want)
	}
}

func TestCompiler_ProjectsGroupByDimensionWhenSelectOnlyHasMetrics(t *testing.T) {
	model := &semantic.SemanticModel{
		Name:       "tweets",
		BaseSchema: "public",
		BaseTable:  "timeline_tweets",
		Dimensions: []semantic.Dimension{
			{Name: "created_at_ts_day", ColumnRef: "timeline_tweets.created_at_ts", Type: "timestamp", TimeGrain: TimeGrainDay},
			{Name: "created_at_ts_year", ColumnRef: "timeline_tweets.created_at_ts", Type: "timestamp", TimeGrain: TimeGrainYear},
		},
		Metrics: []semantic.Metric{
			{Name: "row_count", Expression: "*", Aggregation: "count"},
			{Name: "sum_retweets", Expression: "timeline_tweets.retweets", Aggregation: "sum"},
		},
	}
	lq := LogicalQuery{
		Select: []SelectItem{
			{Type: SelectTypeMetric, Name: "row_count"},
			{Type: SelectTypeMetric, Name: "sum_retweets"},
		},
		Filters: []Filter{{Field: "created_at_ts_year", Operator: OpEq, Value: 2026}},
		GroupBy: []GroupBy{{Field: "created_at_ts_day"}},
		OrderBy: []OrderBy{{Field: "created_at_ts_day", Direction: OrderAsc}},
		Limit:   100,
	}

	cq, err := NewCompiler(dialect.PostgresDialect{}).Compile(context.Background(), &lq, model)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	want := `SELECT DATE_TRUNC('day', "timeline_tweets"."created_at_ts") AS "created_at_ts_day", COUNT(*) AS "row_count", SUM("timeline_tweets"."retweets") AS "sum_retweets" FROM "public"."timeline_tweets" WHERE CAST(EXTRACT(YEAR FROM "timeline_tweets"."created_at_ts") AS INTEGER) = $1 GROUP BY DATE_TRUNC('day', "timeline_tweets"."created_at_ts") ORDER BY DATE_TRUNC('day', "timeline_tweets"."created_at_ts") ASC LIMIT 100`
	if cq.SQL != want {
		t.Errorf("SQL mismatch.\nGot:\n%s\n\nWant:\n%s", cq.SQL, want)
	}
}

// TestCompiler_GroupByTimeGrainOverridesDimensionDefault verifies that
// LogicalQuery.GroupBy.TimeGrain is propagated to the dimension projection in
// both SELECT and GROUP BY without callers having to declare a separate
// pre-bucketed dimension. This is the BI-facing knob for per-query
// daily/weekly/monthly rollups.
func TestCompiler_GroupByTimeGrainOverridesDimensionDefault(t *testing.T) {
	model := &semantic.SemanticModel{
		Name:       "orders",
		BaseSchema: "public",
		BaseTable:  "orders",
		Dimensions: []semantic.Dimension{
			{Name: "order_date", ColumnRef: "orders.created_at", Type: "date"},
		},
		Metrics: []semantic.Metric{
			{Name: "order_count", Expression: "orders.id", Aggregation: "count"},
		},
	}
	lq := LogicalQuery{
		DatasourceID: "ds1",
		ModelID:      model.Name,
		Select: []SelectItem{
			{Type: SelectTypeDimension, Name: "order_date"},
			{Type: SelectTypeMetric, Name: "order_count"},
		},
		GroupBy: []GroupBy{{Field: "order_date", TimeGrain: TimeGrainMonth}},
		Limit:   100,
	}

	tests := []struct {
		name    string
		dialect dialect.Dialect
		wantSQL string
	}{
		{
			name:    "postgres",
			dialect: dialect.PostgresDialect{},
			wantSQL: `SELECT CAST(EXTRACT(MONTH FROM "orders"."created_at") AS INTEGER) AS "order_date", COUNT("orders"."id") AS "order_count" FROM "public"."orders" GROUP BY CAST(EXTRACT(MONTH FROM "orders"."created_at") AS INTEGER) LIMIT 100`,
		},
		{
			name:    "mysql",
			dialect: dialect.MySQLDialect{},
			wantSQL: "SELECT MONTH(`orders`.`created_at`) AS `order_date`, COUNT(`orders`.`id`) AS `order_count` FROM `public`.`orders` GROUP BY MONTH(`orders`.`created_at`) LIMIT 100",
		},
		{
			name:    "clickhouse",
			dialect: dialect.ClickHouseDialect{},
			wantSQL: "SELECT toMonth(`orders`.`created_at`) AS `order_date`, count(`orders`.`id`) AS `order_count` FROM `public`.`orders` GROUP BY toMonth(`orders`.`created_at`) LIMIT 100",
		},
		{
			name:    "sqlserver",
			dialect: dialect.SQLServerDialect{},
			wantSQL: `SELECT MONTH([orders].[created_at]) AS [order_date], COUNT([orders].[id]) AS [order_count] FROM [public].[orders] GROUP BY MONTH([orders].[created_at]) ORDER BY (SELECT NULL) OFFSET 0 ROWS FETCH NEXT 100 ROWS ONLY`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cq, err := NewCompiler(tt.dialect).Compile(context.Background(), &lq, model)
			if err != nil {
				t.Fatalf("Compile() error = %v", err)
			}
			if cq.SQL != tt.wantSQL {
				t.Errorf("SQL mismatch.\nGot:\n%s\n\nWant:\n%s", cq.SQL, tt.wantSQL)
			}
		})
	}
}

// TestCompiler_GroupByTimeGrainDayUsesDateTrunc covers grains that fall through
// to dialect DateTrunc rather than the CalendarPart integer shortcut.
func TestCompiler_GroupByTimeGrainDayUsesDateTrunc(t *testing.T) {
	model := &semantic.SemanticModel{
		Name:       "orders",
		BaseSchema: "public",
		BaseTable:  "orders",
		Dimensions: []semantic.Dimension{
			{Name: "order_date", ColumnRef: "orders.created_at", Type: "date"},
		},
		Metrics: []semantic.Metric{
			{Name: "order_count", Expression: "orders.id", Aggregation: "count"},
		},
	}
	lq := LogicalQuery{
		DatasourceID: "ds1",
		ModelID:      model.Name,
		Select: []SelectItem{
			{Type: SelectTypeDimension, Name: "order_date"},
			{Type: SelectTypeMetric, Name: "order_count"},
		},
		GroupBy: []GroupBy{{Field: "order_date", TimeGrain: TimeGrainWeek}},
		Limit:   50,
	}

	cq, err := NewCompiler(dialect.PostgresDialect{}).Compile(context.Background(), &lq, model)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if !strings.Contains(cq.SQL, `DATE_TRUNC('week', "orders"."created_at")`) {
		t.Errorf("expected DATE_TRUNC('week', ...) wrapping in SQL, got:\n%s", cq.SQL)
	}
}

func TestCompiler_MultipleFilters(t *testing.T) {
	model := &semantic.SemanticModel{
		Name:       "users",
		BaseSchema: "public",
		BaseTable:  "users",
		Dimensions: []semantic.Dimension{
			{Name: "name", ColumnRef: "users.name", Type: "text"},
			{Name: "age", ColumnRef: "users.age", Type: "number"},
			{Name: "email", ColumnRef: "users.email", Type: "text"},
		},
		Metrics: []semantic.Metric{
			{Name: "user_count", Expression: "users.id", Aggregation: "count"},
		},
	}

	lq := LogicalQuery{
		DatasourceID: "ds1",
		ModelID:      "users",
		Select: []SelectItem{
			{Type: "dimension", Name: "name"},
			{Type: "dimension", Name: "email"},
		},
		Filters: []Filter{
			{Field: "age", Operator: OpGte, Value: 18},
			{Field: "name", Operator: OpContains, Value: "john"},
		},
		Limit: 50,
	}

	compiler := NewCompiler(dialect.PostgresDialect{})
	cq, err := compiler.Compile(context.Background(), &lq, model)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check that both filters are present in SQL
	if len(cq.Args) != 2 {
		t.Errorf("expected 2 args, got %d", len(cq.Args))
	}

	// Check ILIKE is used for contains
	if !containsStr(cq.SQL, "ILIKE") {
		t.Errorf("expected ILIKE in SQL for contains operator, got: %s", cq.SQL)
	}
}

// TestValidator_RejectsIntegerFilterOnRawTimestamp catches the AI mistake of
// comparing a raw timestamp column to an integer year/month value, which would
// otherwise blow up at bind time with a confusing pgx encode error. The
// validator must point the caller at the matching *_year/*_month grain
// dimension whose value space is actually integers.
func TestValidator_RejectsIntegerFilterOnRawTimestamp(t *testing.T) {
	model := &semantic.SemanticModel{
		Name:       "orders",
		BaseSchema: "public",
		BaseTable:  "orders",
		Dimensions: []semantic.Dimension{
			{Name: "created_at", ColumnRef: "orders.created_at", Type: "timestamp"},
			{Name: "created_at_year", ColumnRef: "orders.created_at", Type: "date", TimeGrain: "year"},
		},
		Metrics: []semantic.Metric{
			{Name: "order_count", Expression: "orders.id", Aggregation: "count"},
		},
	}

	validator := NewValidator(10000)

	// Wrong: raw timestamp = 2026
	bad := LogicalQuery{
		ModelID: "orders",
		Select:  []SelectItem{{Type: "metric", Name: "order_count"}},
		Filters: []Filter{{Field: "created_at", Operator: OpEq, Value: 2026}},
		Limit:   100,
	}
	if err := validator.Validate(&bad, model); err == nil {
		t.Fatal("expected validation error for integer filter on raw timestamp")
	}

	// OK: grain dim accepts integer
	good := LogicalQuery{
		ModelID: "orders",
		Select:  []SelectItem{{Type: "metric", Name: "order_count"}},
		Filters: []Filter{{Field: "created_at_year", Operator: OpEq, Value: 2026}},
		Limit:   100,
	}
	if err := validator.Validate(&good, model); err != nil {
		t.Errorf("expected no error for integer filter on grain dim, got %v", err)
	}

	// OK: raw timestamp with ISO date string
	goodIso := LogicalQuery{
		ModelID: "orders",
		Select:  []SelectItem{{Type: "metric", Name: "order_count"}},
		Filters: []Filter{{Field: "created_at", Operator: OpGte, Value: "2026-01-01"}},
		Limit:   100,
	}
	if err := validator.Validate(&goodIso, model); err != nil {
		t.Errorf("expected no error for ISO string filter on raw timestamp, got %v", err)
	}
}

func TestValidator_InvalidQuery(t *testing.T) {
	model := &semantic.SemanticModel{
		Name:       "orders",
		BaseSchema: "public",
		BaseTable:  "orders",
		Dimensions: []semantic.Dimension{
			{Name: "country", ColumnRef: "customers.country", Type: "text"},
		},
		Metrics: []semantic.Metric{
			{Name: "order_count", Expression: "orders.id", Aggregation: "count"},
		},
	}

	validator := NewValidator(10000)

	// Test unknown dimension
	lq := LogicalQuery{
		ModelID: "orders",
		Select:  []SelectItem{{Type: "dimension", Name: "unknown_field"}},
		Limit:   100,
	}

	err := validator.Validate(&lq, model)
	if err == nil {
		t.Fatal("expected validation error for unknown dimension")
	}

	// Test limit exceeds max
	lq2 := LogicalQuery{
		ModelID: "orders",
		Select:  []SelectItem{{Type: "dimension", Name: "country"}},
		Limit:   99999,
	}

	err = validator.Validate(&lq2, model)
	if err == nil {
		t.Fatal("expected validation error for exceeding max rows")
	}
}

func TestValidator_CalendarMonthNumericRequiresYearFilter(t *testing.T) {
	model := &semantic.SemanticModel{
		Name:       "tweets",
		BaseSchema: "public",
		BaseTable:  "tweets",
		Dimensions: []semantic.Dimension{
			{Name: "created_at_ts_month", ColumnRef: "public.tweets.created_at_ts", Type: "timestamp", TimeGrain: TimeGrainMonth},
			{Name: "created_at_ts_year", ColumnRef: "public.tweets.created_at_ts", Type: "timestamp", TimeGrain: TimeGrainYear},
		},
		Metrics: []semantic.Metric{
			{Name: "row_count", Expression: "*", Aggregation: "count"},
		},
	}
	v := NewValidator(10000)
	bad := LogicalQuery{
		ModelID: "tweets",
		Select:  []SelectItem{{Type: SelectTypeMetric, Name: "row_count"}},
		Filters: []Filter{{Field: "created_at_ts_month", Operator: OpEq, Value: 4}},
		Limit:   100,
	}
	if err := v.Validate(&bad, model); err == nil {
		t.Fatal("expected validation error for month int without year")
	}

	good := LogicalQuery{
		ModelID: "tweets",
		Select:  []SelectItem{{Type: SelectTypeMetric, Name: "row_count"}},
		Filters: []Filter{
			{Field: "created_at_ts_year", Operator: OpEq, Value: 2026},
			{Field: "created_at_ts_month", Operator: OpEq, Value: 4},
		},
		Limit: 100,
	}
	if err := v.Validate(&good, model); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	iso := LogicalQuery{
		ModelID: "tweets",
		Select:  []SelectItem{{Type: SelectTypeMetric, Name: "row_count"}},
		Filters: []Filter{{Field: "created_at_ts_month", Operator: OpEq, Value: "2026-04-01"}},
		Limit:   100,
	}
	if err := v.Validate(&iso, model); err != nil {
		t.Fatalf("unexpected error for ISO month anchor: %v", err)
	}
}

func TestCompiler_MonthGrainISOUsesDateTruncInWhere(t *testing.T) {
	model := &semantic.SemanticModel{
		Name:       "tweets",
		BaseSchema: "public",
		BaseTable:  "tweets",
		Dimensions: []semantic.Dimension{
			{Name: "created_at_ts_month", ColumnRef: "public.tweets.created_at_ts", Type: "timestamp", TimeGrain: TimeGrainMonth},
			{Name: "created_at_ts_year", ColumnRef: "public.tweets.created_at_ts", Type: "timestamp", TimeGrain: TimeGrainYear},
		},
		Metrics: []semantic.Metric{
			{Name: "row_count", Expression: "*", Aggregation: "count"},
		},
	}
	lq := LogicalQuery{
		DatasourceID: "ds",
		ModelID:      "tweets",
		Select:       []SelectItem{{Type: SelectTypeMetric, Name: "row_count"}},
		Filters:      []Filter{{Field: "created_at_ts_month", Operator: OpEq, Value: "2026-04-01T00:00:00Z"}},
		Limit:        100,
	}
	cq, err := NewCompiler(dialect.PostgresDialect{}).Compile(context.Background(), &lq, model)
	if err != nil {
		t.Fatal(err)
	}
	low := strings.ToLower(cq.SQL)
	if !strings.Contains(low, "date_trunc('month'") {
		t.Fatalf("expected DATE_TRUNC('month' in WHERE, got:\n%s", cq.SQL)
	}
	if strings.Contains(low, "extract(month") {
		t.Fatalf("EXTRACT(MONTH) should not be used for ISO month-grain equality, got:\n%s", cq.SQL)
	}
}

// TestCompiler_JoinDirectionWhenBaseIsFKTarget verifies the compiler swaps
// join orientation when the join's ToTable is the base table (or anything
// already in the FROM set), so the SQL never lists the same table twice.
// Regression for "table name X specified more than once" when the natural
// FK direction points at the base table (e.g. salesorderdetail → salesorderheader
// while salesorderheader is the base).
func TestCompiler_JoinDirectionWhenBaseIsFKTarget(t *testing.T) {
	model := &semantic.SemanticModel{
		Name:       "orders",
		BaseSchema: "public",
		BaseTable:  "salesorderheader",
		Dimensions: []semantic.Dimension{
			{Name: "qty", ColumnRef: "salesorderdetail.orderqty", Type: "number"},
		},
		Metrics: []semantic.Metric{
			{Name: "total", Expression: "salesorderheader.totaldue", Aggregation: "sum"},
		},
		Joins: []semantic.Join{
			{
				// FK direction: detail → header. base = header.
				Name:         "fk_detail_header",
				FromTable:    "salesorderdetail",
				FromColumn:   "salesorderid",
				ToTable:      "salesorderheader",
				ToColumn:     "salesorderid",
				JoinType:     "LEFT",
				Relationship: "many_to_one",
			},
		},
	}
	lq := LogicalQuery{
		DatasourceID: "ds1",
		ModelID:      "orders",
		Select: []SelectItem{
			{Type: SelectTypeDimension, Name: "qty"},
			{Type: SelectTypeMetric, Name: "total"},
		},
		GroupBy: []GroupBy{{Field: "qty"}},
		Limit:   100,
	}
	cq, err := NewCompiler(dialect.PostgresDialect{}).Compile(context.Background(), &lq, model)
	if err != nil {
		t.Fatalf("Compile() error = %v, want nil", err)
	}
	occurrences := strings.Count(cq.SQL, `"salesorderheader"`)
	// header appears in: FROM + the metric expression alias. Should NOT appear in JOIN target.
	if strings.Contains(cq.SQL, `JOIN "public"."salesorderheader"`) {
		t.Fatalf("Compile() emitted duplicate base-table join, SQL=%q (header refs=%d)", cq.SQL, occurrences)
	}
	if !strings.Contains(cq.SQL, `JOIN "public"."salesorderdetail"`) {
		t.Fatalf("Compile() did not introduce salesorderdetail via join, SQL=%q", cq.SQL)
	}
}

// TestCompiler_Having verifies post-aggregation HAVING clauses are emitted
// with the metric's aggregation expression substituted (e.g. SUM(orders.total)).
func TestCompiler_Having(t *testing.T) {
	model := &semantic.SemanticModel{
		Name:       "orders",
		BaseSchema: "public",
		BaseTable:  "orders",
		Dimensions: []semantic.Dimension{
			{Name: "country", ColumnRef: "customers.country", Type: "text"},
		},
		Metrics: []semantic.Metric{
			{Name: "total_revenue", Expression: "orders.total_amount", Aggregation: "sum"},
			{Name: "order_count", Expression: "*", Aggregation: "count"},
		},
		Joins: []semantic.Join{
			{
				Name: "orders_customers", FromTable: "orders", FromColumn: "customer_id",
				ToTable: "customers", ToColumn: "id", JoinType: "LEFT", Relationship: "many_to_one",
			},
		},
	}
	lq := LogicalQuery{
		Select: []SelectItem{
			{Type: SelectTypeDimension, Name: "country"},
			{Type: SelectTypeMetric, Name: "total_revenue"},
		},
		Filters: []Filter{{Field: "country", Operator: OpEq, Value: "US"}},
		GroupBy: []GroupBy{{Field: "country"}},
		Having:  []Filter{{Field: "total_revenue", Operator: OpGt, Value: 1000}},
		Limit:   100,
	}
	cq, err := NewCompiler(dialect.PostgresDialect{}).Compile(context.Background(), &lq, model)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if !strings.Contains(cq.SQL, ` HAVING SUM("orders"."total_amount") > $2`) {
		t.Errorf("expected HAVING with substituted aggregate; got SQL=%q", cq.SQL)
	}
	if len(cq.Args) != 2 || cq.Args[0] != "US" || cq.Args[1] != 1000 {
		t.Errorf("unexpected args=%v", cq.Args)
	}
}

func TestCompiler_HavingRejectsNonMetric(t *testing.T) {
	model := &semantic.SemanticModel{
		Name: "orders", BaseSchema: "public", BaseTable: "orders",
		Dimensions: []semantic.Dimension{{Name: "country", ColumnRef: "orders.country", Type: "text"}},
	}
	lq := LogicalQuery{
		Select:  []SelectItem{{Type: SelectTypeDimension, Name: "country"}},
		GroupBy: []GroupBy{{Field: "country"}},
		Having:  []Filter{{Field: "country", Operator: OpEq, Value: "US"}},
	}
	if _, err := NewCompiler(dialect.PostgresDialect{}).Compile(context.Background(), &lq, model); err == nil {
		t.Fatal("expected having on non-metric to fail compilation")
	}
}

// TestCompiler_WindowFunction verifies window/analytic select items render
// as <AGG>(<expr>) OVER (PARTITION BY ... ORDER BY ...).
func TestCompiler_WindowFunction(t *testing.T) {
	model := &semantic.SemanticModel{
		Name: "orders", BaseSchema: "public", BaseTable: "orders",
		Dimensions: []semantic.Dimension{
			{Name: "country", ColumnRef: "orders.country", Type: "text"},
			{Name: "created_at", ColumnRef: "orders.created_at", Type: "date"},
		},
		Metrics: []semantic.Metric{
			{Name: "total_revenue", Expression: "orders.total_amount", Aggregation: "sum"},
		},
	}
	lq := LogicalQuery{
		Select: []SelectItem{
			{Type: SelectTypeDimension, Name: "country"},
			{Type: SelectTypeDimension, Name: "created_at"},
			{
				Type:  SelectTypeWindow,
				Name:  "running_total",
				Alias: "running_total",
				Window: &WindowSpec{
					Metric:      "total_revenue",
					PartitionBy: []string{"country"},
					OrderBy:     []OrderBy{{Field: "created_at", Direction: "asc"}},
					Frame:       "ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW",
				},
			},
		},
		Limit: 100,
	}
	cq, err := NewCompiler(dialect.PostgresDialect{}).Compile(context.Background(), &lq, model)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	want := `SUM("orders"."total_amount") OVER (PARTITION BY "orders"."country" ORDER BY "orders"."created_at" ASC ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW) AS "running_total"`
	if !strings.Contains(cq.SQL, want) {
		t.Errorf("expected window expr %q in SQL, got: %s", want, cq.SQL)
	}
}

func TestCompiler_WindowRanking(t *testing.T) {
	model := &semantic.SemanticModel{
		Name: "orders", BaseSchema: "public", BaseTable: "orders",
		Dimensions: []semantic.Dimension{
			{Name: "country", ColumnRef: "orders.country", Type: "text"},
		},
		Metrics: []semantic.Metric{
			{Name: "total_revenue", Expression: "orders.total_amount", Aggregation: "sum"},
		},
	}
	lq := LogicalQuery{
		Select: []SelectItem{
			{Type: SelectTypeDimension, Name: "country"},
			{
				Type:  SelectTypeWindow,
				Name:  "rnk",
				Alias: "rnk",
				Window: &WindowSpec{
					Aggregation: "rank",
					PartitionBy: []string{"country"},
					OrderBy:     []OrderBy{{Field: "total_revenue", Direction: "desc"}},
				},
			},
		},
	}
	cq, err := NewCompiler(dialect.PostgresDialect{}).Compile(context.Background(), &lq, model)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if !strings.Contains(cq.SQL, `RANK() OVER (PARTITION BY "orders"."country" ORDER BY SUM("orders"."total_amount") DESC) AS "rnk"`) {
		t.Errorf("unexpected SQL: %s", cq.SQL)
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStrHelper(s, substr))
}

func containsStrHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestCompiler_CalculatedDimension verifies dimensions with calculated
// expressions emit the expression directly instead of quoting a column ref.
func TestCompiler_CalculatedDimension(t *testing.T) {
	model := &semantic.SemanticModel{
		Name: "orders", BaseSchema: "public", BaseTable: "orders",
		Dimensions: []semantic.Dimension{
			{Name: "full_name", CalculatedExpression: `COALESCE(orders.first_name, '') || ' ' || COALESCE(orders.last_name, '')`, Type: "text"},
			{Name: "total_with_tax", CalculatedExpression: `orders.total_amount * 1.18`, Type: "number"},
		},
		Metrics: []semantic.Metric{
			{Name: "order_count", Expression: "orders.id", Aggregation: "count"},
		},
	}
	lq := LogicalQuery{
		Select: []SelectItem{
			{Type: SelectTypeDimension, Name: "full_name"},
			{Type: SelectTypeMetric, Name: "order_count"},
		},
		GroupBy: []GroupBy{{Field: "full_name"}},
		Limit:   50,
	}
	cq, err := NewCompiler(dialect.PostgresDialect{}).Compile(context.Background(), &lq, model)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	// Calculated expression should appear directly in SELECT and GROUP BY
	if !strings.Contains(cq.SQL, `COALESCE(orders.first_name, '') || ' ' || COALESCE(orders.last_name, '')`) {
		t.Errorf("expected calculated expression in SQL, got: %s", cq.SQL)
	}
	if !strings.Contains(cq.SQL, `COUNT("orders"."id")`) {
		t.Errorf("expected metric in SQL, got: %s", cq.SQL)
	}
}

// TestCompiler_CalculatedDimensionWithFilter verifies filters work with
// calculated dimensions.
func TestCompiler_CalculatedDimensionWithFilter(t *testing.T) {
	model := &semantic.SemanticModel{
		Name: "orders", BaseSchema: "public", BaseTable: "orders",
		Dimensions: []semantic.Dimension{
			{Name: "total_with_tax", CalculatedExpression: `orders.total_amount * 1.18`, Type: "number"},
		},
	}
	lq := LogicalQuery{
		Select:  []SelectItem{{Type: SelectTypeDimension, Name: "total_with_tax"}},
		Filters: []Filter{{Field: "total_with_tax", Operator: OpGt, Value: 100}},
		Limit:   50,
	}
	cq, err := NewCompiler(dialect.PostgresDialect{}).Compile(context.Background(), &lq, model)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if !strings.Contains(cq.SQL, `orders.total_amount * 1.18`) {
		t.Errorf("expected calculated expression in WHERE, got: %s", cq.SQL)
	}
}

func TestCompiler_CaseSelect(t *testing.T) {
	model := &semantic.SemanticModel{
		Name: "orders", BaseSchema: "public", BaseTable: "orders",
		Dimensions: []semantic.Dimension{
			{Name: "country", ColumnRef: "orders.country", Type: "text"},
			{Name: "amount", ColumnRef: "orders.amount", Type: "number"},
		},
	}
	lq := LogicalQuery{
		Select: []SelectItem{{
			Type: "case",
			Name: "size_band",
			Case: &CaseExpr{
				Branches: []CaseBranch{{
					When: []Filter{{Field: "amount", Operator: OpGt, Value: 1000}},
					Then: CaseThen{Type: CaseThenTypeLiteral, Literal: "Large"},
				}},
				Else: &CaseThen{Type: CaseThenTypeLiteral, Literal: "Small"},
			},
		}},
		Limit: 10,
	}
	cq, err := NewCompiler(dialect.PostgresDialect{}).Compile(context.Background(), &lq, model)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if !strings.Contains(cq.SQL, "CASE") || !strings.Contains(cq.SQL, "WHEN") {
		t.Errorf("expected CASE expression, got: %s", cq.SQL)
	}
	if !strings.Contains(cq.SQL, `"size_band"`) {
		t.Errorf("expected case alias, got: %s", cq.SQL)
	}
}

func TestCompiler_InSubqueryFilter(t *testing.T) {
	model := &semantic.SemanticModel{
		Name: "orders", BaseSchema: "public", BaseTable: "orders",
		Dimensions: []semantic.Dimension{
			{Name: "customer_id", ColumnRef: "orders.customer_id", Type: "number"},
			{Name: "order_date", ColumnRef: "orders.order_date", Type: "date"},
		},
		Metrics: []semantic.Metric{
			{Name: "order_count", Expression: "orders.id", Aggregation: "count"},
		},
	}
	lq := LogicalQuery{
		Select: []SelectItem{
			{Type: SelectTypeDimension, Name: "customer_id"},
			{Type: SelectTypeMetric, Name: "order_count"},
		},
		Filters: []Filter{{
			Field:    "customer_id",
			Operator: OpIn,
			Subquery: &SubqueryFilter{
				ResultField: "customer_id",
				Body: SubqueryBody{
					Select: []SelectItem{{Type: SelectTypeDimension, Name: "customer_id"}},
					Filters: []Filter{
						{Field: "order_date", Operator: OpGte, Value: "2026-01-01"},
					},
					GroupBy: []GroupBy{{Field: "customer_id"}},
				},
			},
		}},
		GroupBy: []GroupBy{{Field: "customer_id"}},
		Limit:   50,
	}
	cq, err := NewCompiler(dialect.PostgresDialect{}).Compile(context.Background(), &lq, model)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if !strings.Contains(cq.SQL, " IN (SELECT") {
		t.Errorf("expected IN (subquery), got: %s", cq.SQL)
	}
}

func TestCompiler_CTEWithFromCTE(t *testing.T) {
	model := &semantic.SemanticModel{
		Name: "orders", BaseSchema: "public", BaseTable: "orders",
		Dimensions: []semantic.Dimension{
			{Name: "country", ColumnRef: "orders.country", Type: "text"},
		},
		Metrics: []semantic.Metric{
			{Name: "order_count", Expression: "orders.id", Aggregation: "count"},
		},
	}
	lq := LogicalQuery{
		CTEs: []CTE{{
			Name: "by_country",
			Select: []SelectItem{
				{Type: SelectTypeDimension, Name: "country"},
				{Type: SelectTypeMetric, Name: "order_count"},
			},
			GroupBy: []GroupBy{{Field: "country"}},
		}},
		FromCTE: "by_country",
		Select: []SelectItem{
			{Type: SelectTypeDimension, Name: "country"},
			{Type: SelectTypeMetric, Name: "order_count"},
		},
		OrderBy: []OrderBy{{Field: "order_count", Direction: OrderDesc}},
		Limit:   5,
	}
	cq, err := NewCompiler(dialect.PostgresDialect{}).Compile(context.Background(), &lq, model)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if !strings.HasPrefix(cq.SQL, "WITH ") {
		t.Errorf("expected WITH clause, got: %s", cq.SQL)
	}
	if !strings.Contains(cq.SQL, `FROM "by_country"`) {
		t.Errorf("expected FROM cte name, got: %s", cq.SQL)
	}
}

func TestCompiler_CustomExpression(t *testing.T) {
	model := &semantic.SemanticModel{
		Name:       "orders",
		BaseSchema: "public",
		BaseTable:  "orders",
		Dimensions: []semantic.Dimension{
			{Name: "country", ColumnRef: "customers.country", Type: "text"},
			{Name: "tax", ColumnRef: "orders.tax_amount", Type: "number"},
		},
		Metrics: []semantic.Metric{
			{Name: "order_count", Expression: "orders.id", Aggregation: "count"},
			{Name: "sum_tax", Expression: "orders.tax_amount", Aggregation: "sum"},
			{Name: "custom_ratio", Expression: "sum([orders.tax_amount]) / [order_count]", Aggregation: "custom"},
			{Name: "conditional_sum", Expression: "sum(case when [country] = 'US' then [tax] else 0 end)", Aggregation: "custom"},
		},
		Joins: []semantic.Join{
			{
				Name:         "orders_customers",
				FromTable:    "orders",
				FromColumn:   "customer_id",
				ToTable:      "customers",
				ToColumn:     "id",
				JoinType:     "LEFT",
				Relationship: "many_to_one",
			},
		},
	}

	lq := LogicalQuery{
		DatasourceID: "ds1",
		ModelID:      "orders",
		Select: []SelectItem{
			{Type: "dimension", Name: "country"},
			{Type: "metric", Name: "custom_ratio"},
			{Type: "metric", Name: "conditional_sum"},
		},
		GroupBy: []GroupBy{
			{Field: "country"},
		},
		Limit: 100,
	}

	compiler := NewCompiler(dialect.PostgresDialect{})
	cq, err := compiler.Compile(context.Background(), &lq, model)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedSQL := `SELECT "customers"."country" AS "country", sum("orders"."tax_amount") / COUNT("orders"."id") AS "custom_ratio", sum(case when "customers"."country" = 'US' then "orders"."tax_amount" else 0 end) AS "conditional_sum" FROM "public"."orders" LEFT JOIN "public"."customers" ON "public"."orders"."customer_id" = "public"."customers"."id" GROUP BY "customers"."country" LIMIT 100`

	if cq.SQL != expectedSQL {
		t.Errorf("SQL mismatch.\nGot:\n%s\n\nExpected:\n%s", cq.SQL, expectedSQL)
	}
}

func TestCompiler_MetabaseTableSearch(t *testing.T) {
	model := &semantic.SemanticModel{
		Name:       "products",
		BaseSchema: "public",
		BaseTable:  "products",
		Dimensions: []semantic.Dimension{
			{Name: "title", ColumnRef: "products.title", Type: "text"},
		},
	}

	tests := []struct {
		name          string
		dialect       dialect.Dialect
		filter        Filter
		wantSQLPart   string
		wantArgs      []any
	}{
		{
			name:    "postgres contains case-insensitive single",
			dialect: dialect.PostgresDialect{},
			filter:  Filter{Field: "title", Operator: OpContains, Value: "Marble"},
			wantSQLPart: `"products"."title" ILIKE $1`,
			wantArgs:    []any{"%Marble%"},
		},
		{
			name:    "postgres contains case-sensitive single",
			dialect: dialect.PostgresDialect{},
			filter:  Filter{Field: "title", Operator: OpContains, Value: "Marble", CaseSensitive: true},
			wantSQLPart: `"products"."title" LIKE $1`,
			wantArgs:    []any{"%Marble%"},
		},
		{
			name:    "postgres contains case-insensitive multi",
			dialect: dialect.PostgresDialect{},
			filter:  Filter{Field: "title", Operator: OpContains, Value: []any{"Marble", "Watch"}},
			wantSQLPart: `("products"."title" ILIKE $1 OR "products"."title" ILIKE $2)`,
			wantArgs:    []any{"%Marble%", "%Watch%"},
		},
		{
			name:    "mysql contains case-sensitive multi",
			dialect: dialect.MySQLDialect{},
			filter:  Filter{Field: "title", Operator: OpContains, Value: []any{"Marble", "Watch"}, CaseSensitive: true},
			wantSQLPart: "(`products`.`title` LIKE BINARY ? OR `products`.`title` LIKE BINARY ?)",
			wantArgs:    []any{"%Marble%", "%Watch%"},
		},
		{
			name:    "sqlserver contains case-sensitive single",
			dialect: dialect.SQLServerDialect{},
			filter:  Filter{Field: "title", Operator: OpContains, Value: "Marble", CaseSensitive: true},
			wantSQLPart: `[products].[title] LIKE @p1 COLLATE Latin1_General_CS_AS`,
			wantArgs:    []any{"%Marble%"},
		},
		{
			name:    "postgres neq multi",
			dialect: dialect.PostgresDialect{},
			filter:  Filter{Field: "title", Operator: OpNeq, Value: []any{"Marble", "Watch"}},
			wantSQLPart: `("products"."title" != $1 AND "products"."title" != $2)`,
			wantArgs:    []any{"Marble", "Watch"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lq := LogicalQuery{
				Select:  []SelectItem{{Type: SelectTypeDimension, Name: "title"}},
				Filters: []Filter{tt.filter},
				Limit:   100,
			}
			cq, err := NewCompiler(tt.dialect).Compile(context.Background(), &lq, model)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.Contains(cq.SQL, tt.wantSQLPart) {
				t.Errorf("expected SQL part %q, got: %s", tt.wantSQLPart, cq.SQL)
			}
			if len(cq.Args) != len(tt.wantArgs) {
				t.Fatalf("args len mismatch: got %d, want %d", len(cq.Args), len(tt.wantArgs))
			}
			for i, arg := range cq.Args {
				if arg != tt.wantArgs[i] {
					t.Errorf("arg at %d mismatch: got %v, want %v", i, arg, tt.wantArgs[i])
				}
			}
		})
	}
}

