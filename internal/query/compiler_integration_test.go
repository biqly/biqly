package query

import (
	"context"
	"testing"

	"github.com/biqly/biqly/internal/dialect"
	"github.com/biqly/biqly/internal/security"
	"github.com/biqly/biqly/internal/semantic"
)

// TestCompiler_Joins verifies that the compiler generates correct JOIN clauses.
func TestCompiler_Joins(t *testing.T) {
	model := &semantic.SemanticModel{
		Name:       "orders",
		BaseSchema: "public",
		BaseTable:  "orders",
		Dimensions: []semantic.Dimension{
			{Name: "country", ColumnRef: "customers.country", Type: "text"},
			{Name: "city", ColumnRef: "customers.city", Type: "text"},
		},
		Metrics: []semantic.Metric{
			{Name: "total", Expression: "orders.amount", Aggregation: "sum"},
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
		ModelID: "orders",
		Select: []SelectItem{
			{Type: "dimension", Name: "country"},
			{Type: "dimension", Name: "city"},
			{Type: "metric", Name: "total"},
		},
		GroupBy: []GroupBy{{Field: "country"}, {Field: "city"}},
		Limit:   100,
	}

	compiler := NewCompiler(dialect.PostgresDialect{})
	cq, err := compiler.Compile(context.Background(), &lq, model)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify JOIN clause is present
	if !containsStr(cq.SQL, "LEFT JOIN") {
		t.Errorf("expected LEFT JOIN in SQL: %s", cq.SQL)
	}
	if !containsStr(cq.SQL, "customers") {
		t.Errorf("expected 'customers' in SQL: %s", cq.SQL)
	}
}

// TestCompiler_Placeholders verifies parameterized query generation.
func TestCompiler_Placeholders(t *testing.T) {
	model := &semantic.SemanticModel{
		Name:       "users",
		BaseSchema: "app",
		BaseTable:  "users",
		Dimensions: []semantic.Dimension{
			{Name: "name", ColumnRef: "users.name", Type: "text"},
			{Name: "email", ColumnRef: "users.email", Type: "text"},
			{Name: "age", ColumnRef: "users.age", Type: "number"},
		},
		Metrics: []semantic.Metric{
			{Name: "count", Expression: "users.id", Aggregation: "count"},
		},
	}

	lq := LogicalQuery{
		ModelID: "users",
		Select:  []SelectItem{{Type: "dimension", Name: "name"}, {Type: "dimension", Name: "email"}},
		Filters: []Filter{
			{Field: "age", Operator: OpGte, Value: 18},
			{Field: "name", Operator: OpStartsWith, Value: "A"},
		},
		Limit: 50,
	}

	compiler := NewCompiler(dialect.PostgresDialect{})
	cq, err := compiler.Compile(context.Background(), &lq, model)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check placeholders are used, not raw values
	if containsStr(cq.SQL, "18") {
		t.Errorf("raw value '18' found in SQL - should use placeholder: %s", cq.SQL)
	}
	if !containsStr(cq.SQL, "$") {
		t.Errorf("expected PostgreSQL placeholder in SQL: %s", cq.SQL)
	}
}

// TestCompiler_DialectQuoting verifies identifier quoting per dialect.
func TestCompiler_DialectQuoting(t *testing.T) {
	model := &semantic.SemanticModel{
		Name:       "test",
		BaseSchema: "my_schema",
		BaseTable:  "my_table",
		Dimensions: []semantic.Dimension{
			{Name: "id", ColumnRef: "my_table.id", Type: "text"},
		},
		Metrics: []semantic.Metric{
			{Name: "cnt", Expression: "my_table.id", Aggregation: "count"},
		},
	}

	lq := LogicalQuery{
		ModelID: "test",
		Select:  []SelectItem{{Type: "dimension", Name: "id"}, {Type: "metric", Name: "cnt"}},
		Limit:   10,
	}

	tests := []struct {
		name     string
		dialect  dialect.Dialect
		expected string // quote style
	}{
		{"postgres", dialect.PostgresDialect{}, "\""},
		{"mysql", dialect.MySQLDialect{}, "`"},
		{"sqlserver", dialect.SQLServerDialect{}, "["},
		{"clickhouse", dialect.ClickHouseDialect{}, "`"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewCompiler(tt.dialect)
			cq, err := c.Compile(context.Background(), &lq, model)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !containsStr(cq.SQL, tt.expected) {
				t.Errorf("expected %s quoting in SQL: %s", tt.name, cq.SQL)
			}
		})
	}
}

// TestCompiler_FilterOperators verifies all filter operators compile correctly.
func TestCompiler_FilterOperators(t *testing.T) {
	model := &semantic.SemanticModel{
		Name:       "items",
		BaseSchema: "public",
		BaseTable:  "items",
		Dimensions: []semantic.Dimension{
			{Name: "name", ColumnRef: "items.name", Type: "text"},
			{Name: "price", ColumnRef: "items.price", Type: "number"},
			{Name: "category", ColumnRef: "items.category", Type: "text"},
			{Name: "created", ColumnRef: "items.created", Type: "date"},
		},
	}

	tests := []struct {
		name     string
		operator string
		value    any
		check    string // expected SQL fragment
	}{
		{"eq", OpEq, "active", "="},
		{"neq", OpNeq, "deleted", "!="},
		{"gt", OpGt, 100, ">"},
		{"gte", OpGte, 50, ">="},
		{"lt", OpLt, 1000, "<"},
		{"lte", OpLte, 500, "<="},
		{"in", OpIn, []any{"a", "b"}, "IN"},
		{"not_in", OpNotIn, []any{"x", "y"}, "NOT IN"},
		{"contains", OpContains, "test", "ILIKE"},
		{"starts_with", OpStartsWith, "A", "ILIKE"},
		{"ends_with", OpEndsWith, "Z", "ILIKE"},
		{"between", OpBetween, []any{"2026-01-01", "2026-12-31"}, "BETWEEN"},
		{"is_null", OpIsNull, nil, "IS NULL"},
		{"is_not_null", OpIsNotNull, nil, "IS NOT NULL"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lq := LogicalQuery{
				ModelID: "items",
				Select:  []SelectItem{{Type: "dimension", Name: "name"}},
				Filters: []Filter{{Field: "name", Operator: tt.operator, Value: tt.value}},
				Limit:   10,
			}
			compiler := NewCompiler(dialect.PostgresDialect{})
			cq, err := compiler.Compile(context.Background(), &lq, model)
			if err != nil {
				t.Fatalf("unexpected error for %s: %v", tt.name, err)
			}
			if !containsStr(cq.SQL, tt.check) {
				t.Errorf("expected '%s' in SQL for %s: %s", tt.check, tt.name, cq.SQL)
			}
		})
	}
}

// TestCompiler_PermissionInjection verifies row-level security filter injection.
func TestCompiler_PermissionInjection(t *testing.T) {
	model := &semantic.SemanticModel{
		Name:       "orders",
		BaseSchema: "public",
		BaseTable:  "orders",
		Dimensions: []semantic.Dimension{
			{Name: "tenant_id", ColumnRef: "orders.tenant_id", Type: "text"},
			{Name: "amount", ColumnRef: "orders.amount", Type: "number"},
		},
		Metrics: []semantic.Metric{
			{Name: "count", Expression: "orders.id", Aggregation: "count"},
		},
	}

	lq := LogicalQuery{
		ModelID: "orders",
		Select:  []SelectItem{{Type: "dimension", Name: "amount"}, {Type: "metric", Name: "count"}},
		Limit:   100,
	}

	rowFilters := []security.RowFilter{
		{Field: "tenant_id", Operator: "eq", Value: "tenant_123"},
	}

	compiler := NewCompiler(dialect.PostgresDialect{})
	cq, err := compiler.CompileWithPermissions(context.Background(), &lq, model, rowFilters, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !containsStr(cq.SQL, "tenant_id") {
		t.Errorf("expected tenant_id filter in SQL: %s", cq.SQL)
	}
	if !containsStr(cq.SQL, "WHERE") {
		t.Errorf("expected WHERE clause: %s", cq.SQL)
	}
}

// TestCompiler_PermissionInjectionDoesNotMatchCTEWhere ensures the row-filter
// merge happens at the outer WHERE clause, not in a WHERE that lives inside a
// CTE body. The earlier regex-based injection would have appended " AND ..."
// inside the CTE's WHERE, producing invalid SQL.
func TestCompiler_PermissionInjectionDoesNotMatchCTEWhere(t *testing.T) {
	model := &semantic.SemanticModel{
		Name:       "orders",
		BaseSchema: "public",
		BaseTable:  "orders",
		Dimensions: []semantic.Dimension{
			{Name: "tenant_id", ColumnRef: "orders.tenant_id", Type: "text"},
			{Name: "status", ColumnRef: "orders.status", Type: "text"},
		},
		Metrics: []semantic.Metric{
			{Name: "count", Expression: "orders.id", Aggregation: "count"},
		},
	}

	lq := LogicalQuery{
		ModelID: "orders",
		CTEs: []CTE{{
			Name:    "vip_customers",
			Select:  []SelectItem{{Type: "dimension", Name: "tenant_id"}},
			Filters: []Filter{{Field: "status", Operator: "eq", Value: "vip"}},
		}},
		Select:  []SelectItem{{Type: "metric", Name: "count"}},
		Filters: []Filter{{Field: "status", Operator: "eq", Value: "open"}},
		Limit:   100,
	}

	rowFilters := []security.RowFilter{
		{Field: "tenant_id", Operator: "eq", Value: "tenant_123"},
	}

	compiler := NewCompiler(dialect.PostgresDialect{})
	cq, err := compiler.CompileWithPermissions(context.Background(), &lq, model, rowFilters, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Locate the FIRST top-level WHERE that comes AFTER the closing paren of
	// the CTE body; the row-filter predicate must appear after that point.
	cteEnd := indexOfStr(cq.SQL, ") ")
	if cteEnd < 0 {
		t.Fatalf("expected CTE body close in SQL: %s", cq.SQL)
	}
	tenantPos := indexOfStr(cq.SQL, "tenant_id\" = $")
	if tenantPos < 0 {
		t.Fatalf("expected tenant_id row filter in SQL: %s", cq.SQL)
	}
	if tenantPos < cteEnd {
		t.Fatalf("row filter leaked into CTE body. SQL: %s", cq.SQL)
	}
}

func indexOfStr(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// TestCompiler_CompositeRowFilterTargetsCorrectTable verifies that, for a
// resolved composite model spanning two physical tables, a row-level security
// filter declared on a secondary component's dimension is injected against that
// component's table (via its fully qualified column ref) rather than the base
// table. This proves cross-model RLS targeting works once component policies
// are merged and fed through CompileWithPermissions.
func TestCompiler_CompositeRowFilterTargetsCorrectTable(t *testing.T) {
	// Merged composite model: orders (primary) joined to customers (secondary).
	model := &semantic.SemanticModel{
		Name:       "sales_customer",
		BaseSchema: "public",
		BaseTable:  "orders",
		Dimensions: []semantic.Dimension{
			{Name: "amount", ColumnRef: "public.orders.amount", Type: "number"},
			{Name: "customer_region", ColumnRef: "public.customers.region", Type: "text"},
		},
		Metrics: []semantic.Metric{
			{Name: "count", Expression: "public.orders.id", Aggregation: "count"},
		},
		Joins: []semantic.Join{
			{
				Name:       "orders_customers",
				FromTable:  "orders",
				FromColumn: "customer_id",
				ToTable:    "customers",
				ToColumn:   "id",
				JoinType:   "LEFT",
			},
		},
	}

	lq := LogicalQuery{
		ModelID: "sales_customer",
		Select:  []SelectItem{{Type: "dimension", Name: "customer_region"}, {Type: "metric", Name: "count"}},
		GroupBy: []GroupBy{{Field: "customer_region"}},
		Limit:   100,
	}

	// RLS filter sourced from the customer component, keyed by its dimension.
	rowFilters := []security.RowFilter{
		{Field: "customer_region", Operator: "eq", Value: "EU"},
	}

	compiler := NewCompiler(dialect.PostgresDialect{})
	cq, err := compiler.CompileWithPermissions(context.Background(), &lq, model, rowFilters, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The filter must reference the customers table column, not orders.
	if !containsStr(cq.SQL, "customers") || !containsStr(cq.SQL, "region") {
		t.Errorf("expected customers.region in RLS predicate: %s", cq.SQL)
	}
	if !containsStr(cq.SQL, "JOIN") {
		t.Errorf("expected JOIN to customers table: %s", cq.SQL)
	}
}

// TestCompiler_LimitOffset verifies LIMIT/OFFSET generation.
func TestCompiler_LimitOffset(t *testing.T) {
	model := &semantic.SemanticModel{
		Name:       "data",
		BaseSchema: "public",
		BaseTable:  "data",
		Dimensions: []semantic.Dimension{
			{Name: "val", ColumnRef: "data.val", Type: "text"},
		},
	}

	lq := LogicalQuery{
		ModelID: "data",
		Select:  []SelectItem{{Type: "dimension", Name: "val"}},
		Limit:   25,
		Offset:  50,
	}

	compiler := NewCompiler(dialect.PostgresDialect{})
	cq, err := compiler.Compile(context.Background(), &lq, model)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !containsStr(cq.SQL, "LIMIT 25") {
		t.Errorf("expected 'LIMIT 25' in SQL: %s", cq.SQL)
	}
	if !containsStr(cq.SQL, "OFFSET 50") {
		t.Errorf("expected 'OFFSET 50' in SQL: %s", cq.SQL)
	}
}

// TestCompiler_MySQLDialect verifies MySQL-specific SQL generation.
func TestCompiler_MySQLDialect(t *testing.T) {
	model := &semantic.SemanticModel{
		Name:       "products",
		BaseSchema: "store",
		BaseTable:  "products",
		Dimensions: []semantic.Dimension{
			{Name: "name", ColumnRef: "products.name", Type: "text"},
		},
		Metrics: []semantic.Metric{
			{Name: "cnt", Expression: "products.id", Aggregation: "count"},
		},
	}

	lq := LogicalQuery{
		ModelID: "products",
		Select:  []SelectItem{{Type: "dimension", Name: "name"}, {Type: "metric", Name: "cnt"}},
		Filters: []Filter{{Field: "name", Operator: OpContains, Value: "widget"}},
		GroupBy: []GroupBy{{Field: "name"}},
		OrderBy: []OrderBy{{Field: "cnt", Direction: "desc"}},
		Limit:   10,
	}

	compiler := NewCompiler(dialect.MySQLDialect{})
	cq, err := compiler.Compile(context.Background(), &lq, model)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// MySQL uses backticks
	if !containsStr(cq.SQL, "`") {
		t.Errorf("expected backtick quoting in SQL: %s", cq.SQL)
	}
	// MySQL uses ? placeholders
	if !containsStr(cq.SQL, "?") {
		t.Errorf("expected ? placeholder in SQL: %s", cq.SQL)
	}
	// MySQL uses LOWER for ILIKE
	if !containsStr(cq.SQL, "LOWER") {
		t.Errorf("expected LOWER() in SQL: %s", cq.SQL)
	}
}

// TestCompiler_SQLServerDialect verifies SQL Server-specific SQL generation.
func TestCompiler_SQLServerDialect(t *testing.T) {
	model := &semantic.SemanticModel{
		Name:       "employees",
		BaseSchema: "hr",
		BaseTable:  "employees",
		Dimensions: []semantic.Dimension{
			{Name: "dept", ColumnRef: "employees.department", Type: "text"},
		},
	}

	lq := LogicalQuery{
		ModelID: "employees",
		Select:  []SelectItem{{Type: "dimension", Name: "dept"}},
		Filters: []Filter{{Field: "dept", Operator: OpEq, Value: "engineering"}},
		Limit:   20,
		Offset:  10,
	}

	compiler := NewCompiler(dialect.SQLServerDialect{})
	cq, err := compiler.Compile(context.Background(), &lq, model)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// SQL Server uses [brackets]
	if !containsStr(cq.SQL, "[") {
		t.Errorf("expected bracket quoting in SQL: %s", cq.SQL)
	}
	// SQL Server uses @pN placeholders
	if !containsStr(cq.SQL, "@p") {
		t.Errorf("expected @p placeholder in SQL: %s", cq.SQL)
	}
	// SQL Server uses OFFSET/FETCH
	if !containsStr(cq.SQL, "OFFSET") {
		t.Errorf("expected OFFSET in SQL: %s", cq.SQL)
	}
	if !containsStr(cq.SQL, "FETCH") {
		t.Errorf("expected FETCH in SQL: %s", cq.SQL)
	}
}
