package query

import (
	"context"
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
	cq, err := compiler.Compile(context.Background(), lq, model)
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
	cq, err := compiler.Compile(context.Background(), lq, model)
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

	err := validator.Validate(lq, model)
	if err == nil {
		t.Fatal("expected validation error for unknown dimension")
	}

	// Test limit exceeds max
	lq2 := LogicalQuery{
		ModelID: "orders",
		Select:  []SelectItem{{Type: "dimension", Name: "country"}},
		Limit:   99999,
	}

	err = validator.Validate(lq2, model)
	if err == nil {
		t.Fatal("expected validation error for exceeding max rows")
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
