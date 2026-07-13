package query

import (
	"context"
	"testing"

	"github.com/biqly/biqly/internal/dialect"
	"github.com/biqly/biqly/internal/semantic"
)

// TestCompiler_CustomMetricExpressionReadOnlyGuard verifies that a raw
// custom-metric Expression is subjected to the same read-only guard as the AST
// expression path (S8): a metric whose Expression smuggles DML must be rejected
// at compile time, while a legitimate custom expression still compiles.
func TestCompiler_CustomMetricExpressionReadOnlyGuard(t *testing.T) {
	modelWith := func(expr string) *semantic.SemanticModel {
		return &semantic.SemanticModel{
			Name:       "orders",
			BaseSchema: "public",
			BaseTable:  "orders",
			Dimensions: []semantic.Dimension{
				{Name: "country", ColumnRef: "orders.country", Type: "text"},
			},
			Metrics: []semantic.Metric{
				{Name: "custom_measure", Expression: expr, Aggregation: "custom"},
			},
		}
	}
	query := func() LogicalQuery {
		return LogicalQuery{
			DatasourceID: "ds1",
			ModelID:      "orders",
			Select:       []SelectItem{{Type: "metric", Name: "custom_measure"}},
			Limit:        10,
		}
	}

	t.Run("rejects DML smuggled in a custom metric expression", func(t *testing.T) {
		model := modelWith("0); DROP TABLE users --")
		q := query()
		_, err := NewCompiler(dialect.PostgresDialect{}).Compile(context.Background(), &q, model)
		if err == nil {
			t.Fatal("expected the read-only guard to reject DML in a custom metric expression")
		}
	})

	t.Run("allows a safe custom metric expression", func(t *testing.T) {
		model := modelWith("orders.revenue")
		q := query()
		_, err := NewCompiler(dialect.PostgresDialect{}).Compile(context.Background(), &q, model)
		if err != nil {
			t.Fatalf("safe custom metric expression should compile, got: %v", err)
		}
	})
}
