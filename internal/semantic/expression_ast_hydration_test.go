package semantic

import (
	"errors"
	"testing"

	pkgsemantic "github.com/biqly/biqly/pkg/semantic"
)

func TestHydrateExpressionASTsParsesDimensionAndMetricExpressions(t *testing.T) {
	previous := CurrentExpressionParser()
	t.Cleanup(func() { RegisterExpressionParser(previous) })

	RegisterExpressionParser(func(expr string) (pkgsemantic.ExprNode, error) {
		switch expr {
		case "orders.total_amount - 10":
			return pkgsemantic.BinaryExpr{
				Op:    pkgsemantic.OpSubtract,
				Left:  pkgsemantic.ColumnRefExpr{Table: "orders", Column: "total_amount"},
				Right: pkgsemantic.LiteralExpr{Value: int64(10)},
			}, nil
		case "orders.revenue - orders.cost":
			return pkgsemantic.BinaryExpr{
				Op:    pkgsemantic.OpSubtract,
				Left:  pkgsemantic.ColumnRefExpr{Table: "orders", Column: "revenue"},
				Right: pkgsemantic.ColumnRefExpr{Table: "orders", Column: "cost"},
			}, nil
		default:
			t.Fatalf("ExpressionParser(%q) called unexpectedly", expr)
			return nil, nil //nolint:nilnil // unreachable after Fatalf
		}
	})

	model := &SemanticModel{
		Dimensions: []Dimension{{Name: "net_amount", CalculatedExpression: "orders.total_amount - 10"}},
		Metrics:    []Metric{{Name: "gross_margin", Expression: "orders.revenue - orders.cost"}},
	}

	hydrateExpressionASTs(model)

	if model.Dimensions[0].CalculatedExpr == nil {
		t.Fatal("hydrateExpressionASTs() did not populate Dimension.CalculatedExpr")
	}
	if model.Metrics[0].Expr == nil {
		t.Fatal("hydrateExpressionASTs() did not populate Metric.Expr")
	}
}

func TestHydrateExpressionASTsLeavesNilOnParseError(t *testing.T) {
	previous := CurrentExpressionParser()
	t.Cleanup(func() { RegisterExpressionParser(previous) })

	RegisterExpressionParser(func(string) (pkgsemantic.ExprNode, error) {
		return nil, errors.New("parse failed")
	})
	model := &SemanticModel{
		Dimensions: []Dimension{{Name: "bad_dim", CalculatedExpression: "not valid"}},
		Metrics:    []Metric{{Name: "bad_metric", Expression: "not valid"}},
	}

	hydrateExpressionASTs(model)

	if model.Dimensions[0].CalculatedExpr != nil {
		t.Fatalf("Dimension.CalculatedExpr = %#v, want nil on parse error", model.Dimensions[0].CalculatedExpr)
	}
	if model.Metrics[0].Expr != nil {
		t.Fatalf("Metric.Expr = %#v, want nil on parse error", model.Metrics[0].Expr)
	}
}
