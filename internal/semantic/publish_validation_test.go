package semantic_test

import (
	"context"
	"errors"
	"testing"

	"github.com/biqly/biqly/internal/semantic"
	pkgsemantic "github.com/biqly/biqly/pkg/semantic"
)

// TestValidateContextParseErrorSurfacing pins the P2 fix: a custom metric with
// a syntactically broken expression must surface a parse error rather than
// silently ignoring it. Before the fix, getOrParseExpr errors upstream of
// ValidateExprStrict were discarded. Now validateMetricExpressionAST calls
// addError with the parser error before returning.
func TestValidateContextParseErrorSurfacing(t *testing.T) {
	// Register a parser that returns an error for our deliberately invalid input.
	failOn := "BOOM!!!" // must match the expression below
	semantic.RegisterExpressionParser(func(expr string) (pkgsemantic.ExprNode, error) {
		if expr == failOn {
			return nil, errors.New("syntax error at position 4: unexpected '!'")
		}
		return nil, errors.New("unmatched expression")
	})

	model := validPublishModel()
	model.Metrics = append(model.Metrics, semantic.Metric{
		Name:        "broken_metric",
		Expression:  failOn,
		Aggregation: "custom",
		IsActive:    true,
	})

	result := semantic.ValidateContext(context.Background(), model, validPublishCatalog())
	if result.Valid {
		t.Fatal("ValidateContext() valid = true, want false (broken expression should fail)")
	}
	if !result.HasError(`metric "broken_metric" expression parse error: syntax error at position 4: unexpected '!'`) {
		t.Fatalf("expected parse error from broken_metric, got errors: %v", result.Errors)
	}
}
