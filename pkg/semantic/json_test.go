package semantic

import (
	"testing"

	"github.com/bytedance/sonic"
)

// TestSemanticModelJSONRoundTripWithExprs proves a model carrying expression
// ASTs survives a plain marshal/unmarshal cycle — the wire path used by the
// catalog /internal/models endpoint and inline models on /internal/query
// requests. Before the custom unmarshalers, decoding into the ExprNode
// interface fields failed outright.
func TestSemanticModelJSONRoundTripWithExprs(t *testing.T) {
	model := SemanticModel{
		ID:   "m1",
		Name: "orders",
		Dimensions: []Dimension{{
			Name:                 "full_name",
			CalculatedExpression: "CONCAT(first_name, last_name)",
			CalculatedExpr: &FunctionCallExpr{
				Name: "CONCAT",
				Args: []ExprNode{
					&ColumnRefExpr{Column: "first_name"},
					&ColumnRefExpr{Column: "last_name"},
				},
			},
		}},
		Metrics: []Metric{{
			Name:       "revenue",
			Expression: "orders.amount",
			Expr:       &ColumnRefExpr{Table: "orders", Column: "amount"},
		}},
	}

	raw, err := sonic.ConfigStd.Marshal(model)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got SemanticModel
	if err := sonic.ConfigStd.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	dimExpr, ok := got.Dimensions[0].CalculatedExpr.(*FunctionCallExpr)
	if !ok || dimExpr.Name != "CONCAT" || len(dimExpr.Args) != 2 {
		t.Errorf("dimension expr lost in round trip: %#v", got.Dimensions[0].CalculatedExpr)
	}
	metricExpr, ok := got.Metrics[0].Expr.(*ColumnRefExpr)
	if !ok || metricExpr.Column != "amount" {
		t.Errorf("metric expr lost in round trip: %#v", got.Metrics[0].Expr)
	}
	if got.Metrics[0].Expression != "orders.amount" {
		t.Errorf("metric expression string lost: %q", got.Metrics[0].Expression)
	}
}

// TestSemanticModelJSONDropsInvalidExprs proves malformed or legacy AST
// payloads degrade to a nil AST (the Expression string remains the source of
// truth) instead of failing the whole model decode.
func TestSemanticModelJSONDropsInvalidExprs(t *testing.T) {
	raw := []byte(`{
		"id": "m1",
		"dimensions": [{"name": "d1", "calculated_expr": {"type": "bogus"}}],
		"metrics": [{"name": "m1", "expression": "orders.amount", "expr": ""}]
	}`)
	var got SemanticModel
	if err := sonic.ConfigStd.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Dimensions[0].CalculatedExpr != nil {
		t.Errorf("invalid dimension expr should decode to nil, got %#v", got.Dimensions[0].CalculatedExpr)
	}
	if got.Metrics[0].Expr != nil {
		t.Errorf("invalid metric expr should decode to nil, got %#v", got.Metrics[0].Expr)
	}
	if got.Metrics[0].Expression != "orders.amount" {
		t.Errorf("expression string lost: %q", got.Metrics[0].Expression)
	}
}

func TestSemanticModelJSONDropsEmptyExprNode(t *testing.T) {
	// exprNodeFromRaw with nil/empty JSON: returns nil
	raw := []byte(`{
		"id": "m2",
		"dimensions": [{"name": "d1", "calculated_expr": null}],
		"metrics": [{"name": "m1", "expression": "total", "expr": null}]
	}`)
	var got SemanticModel
	if err := sonic.ConfigStd.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Dimensions[0].CalculatedExpr != nil {
		t.Errorf("null dimension expr should decode to nil, got %#v", got.Dimensions[0].CalculatedExpr)
	}
	if got.Metrics[0].Expr != nil {
		t.Errorf("null metric expr should decode to nil, got %#v", got.Metrics[0].Expr)
	}
}

func TestDimensionUnmarshalJSONBadFieldType(t *testing.T) {
	// sonic.ConfigStd.Unmarshal fails inside Dimension.UnmarshalJSON
	// because "id" is a number, not a string
	raw := []byte(`{"id":123,"name":"d1","column_ref":"col","type":"text"}`)
	var d Dimension
	err := sonic.ConfigStd.Unmarshal(raw, &d)
	if err == nil {
		t.Fatal("expected unmarshal error for wrong id type")
	}
}

func TestMetricUnmarshalJSONBadFieldType(t *testing.T) {
	// sonic.ConfigStd.Unmarshal fails inside Metric.UnmarshalJSON
	// because "name" is a number, not a string
	raw := []byte(`{"name":123,"expression":"count","aggregation":"sum"}`)
	var m Metric
	err := sonic.ConfigStd.Unmarshal(raw, &m)
	if err == nil {
		t.Fatal("expected unmarshal error for wrong name type")
	}
}
