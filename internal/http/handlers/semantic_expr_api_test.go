package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	internalsemantic "github.com/biqly/biqly/internal/semantic"
	pkgsemantic "github.com/biqly/biqly/pkg/semantic"
	"github.com/go-chi/chi/v5"
)

func TestDimensionFromRequestParsesExpressionString(t *testing.T) {
	restoreExpressionParser(t, func(expr string) (pkgsemantic.ExprNode, error) {
		if expr != "revenue - cost" {
			t.Fatalf("unexpected expression: %q", expr)
		}
		return pkgsemantic.BinaryExpr{
			Op:    pkgsemantic.OpSubtract,
			Left:  pkgsemantic.ColumnRefExpr{Column: "revenue"},
			Right: pkgsemantic.ColumnRefExpr{Column: "cost"},
		}, nil
	})

	req := createDimensionRequest{
		Name:                 "margin",
		ColumnRef:            "orders.revenue",
		Type:                 "number",
		CalculatedExpression: "revenue - cost",
	}

	dim, err := dimensionFromRequest("dim_1", "model_1", req)
	if err != nil {
		t.Fatalf("dimensionFromRequest() error = %v", err)
	}
	if dim.CalculatedExpression != "revenue - cost" {
		t.Fatalf("CalculatedExpression = %q", dim.CalculatedExpression)
	}
	assertBinarySubtractExpr(t, dim.CalculatedExpr)
}

func TestDimensionFromRequestAcceptsExpressionAST(t *testing.T) {
	req := createDimensionRequest{
		Name:                 "margin",
		ColumnRef:            "orders.revenue",
		Type:                 "number",
		CalculatedExpression: "legacy expression",
		CalculatedExpr: json.RawMessage(`{
			"type": "binary",
			"op": "subtract",
			"left": {"type": "column_ref", "column": "revenue"},
			"right": {"type": "column_ref", "column": "cost"}
		}`),
	}

	dim, err := dimensionFromRequest("dim_1", "model_1", req)
	if err != nil {
		t.Fatalf("dimensionFromRequest() error = %v", err)
	}
	if dim.CalculatedExpression != "legacy expression" {
		t.Fatalf("CalculatedExpression = %q", dim.CalculatedExpression)
	}
	assertBinarySubtractExpr(t, dim.CalculatedExpr)
}

func TestMetricFromRequestAcceptsExpressionAST(t *testing.T) {
	req := createMetricRequest{
		Name:        "net_revenue",
		Expression:  "legacy expression",
		Aggregation: "sum",
		Expr: json.RawMessage(`{
			"type": "binary",
			"op": "subtract",
			"left": {"type": "column_ref", "column": "revenue"},
			"right": {"type": "column_ref", "column": "cost"}
		}`),
	}

	metric, err := metricFromRequest("metric_1", "model_1", req)
	if err != nil {
		t.Fatalf("metricFromRequest() error = %v", err)
	}
	if metric.Expression != "legacy expression" {
		t.Fatalf("Expression = %q", metric.Expression)
	}
	assertBinarySubtractExpr(t, metric.Expr)
}

func TestExpressionAPIRejectsInvalidAST(t *testing.T) {
	_, dimErr := dimensionFromRequest("dim_1", "model_1", createDimensionRequest{
		Name:           "bad_dimension",
		ColumnRef:      "orders.revenue",
		Type:           "number",
		CalculatedExpr: json.RawMessage(`{"type":"unknown"}`),
	})
	if dimErr == nil || !strings.Contains(dimErr.Error(), "calculated_expr") {
		t.Fatalf("dimensionFromRequest() error = %v, want calculated_expr error", dimErr)
	}

	_, metricErr := metricFromRequest("metric_1", "model_1", createMetricRequest{
		Name:        "bad_metric",
		Expression:  "revenue",
		Aggregation: "sum",
		Expr:        json.RawMessage(`{"type":"unknown"}`),
	})
	if metricErr == nil || !strings.Contains(metricErr.Error(), "expr") {
		t.Fatalf("metricFromRequest() error = %v, want expr error", metricErr)
	}
}

func TestCreateDimensionRejectsInvalidExpressionASTBeforeRepoWrite(t *testing.T) {
	body := strings.NewReader(`{
		"name": "bad_dimension",
		"column_ref": "orders.revenue",
		"type": "number",
		"calculated_expr": {"type": "unknown"}
	}`)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/semantic/models/model_1/dimensions", body)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "model_1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	(&SemanticHandler{}).CreateDimension(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "calculated_expr") {
		t.Fatalf("body = %s, want calculated_expr error", rec.Body.String())
	}
}

func TestSemanticExpressionAPIResponseIncludesASTFields(t *testing.T) {
	dimJSON, err := json.Marshal(pkgsemantic.Dimension{
		ID:             "dim_1",
		ModelID:        "model_1",
		Name:           "margin",
		ColumnRef:      "orders.revenue",
		Type:           "number",
		CalculatedExpr: pkgsemantic.ColumnRefExpr{Column: "revenue"},
	})
	if err != nil {
		t.Fatalf("marshal dimension: %v", err)
	}
	if !strings.Contains(string(dimJSON), `"calculated_expr"`) {
		t.Fatalf("dimension response JSON = %s, want calculated_expr", dimJSON)
	}

	metricJSON, err := json.Marshal(pkgsemantic.Metric{
		ID:          "metric_1",
		ModelID:     "model_1",
		Name:        "net_revenue",
		Expression:  "revenue",
		Aggregation: "sum",
		Expr:        pkgsemantic.ColumnRefExpr{Column: "revenue"},
	})
	if err != nil {
		t.Fatalf("marshal metric: %v", err)
	}
	if !strings.Contains(string(metricJSON), `"expr"`) {
		t.Fatalf("metric response JSON = %s, want expr", metricJSON)
	}
}

func restoreExpressionParser(t *testing.T, parser func(string) (pkgsemantic.ExprNode, error)) {
	t.Helper()
	previous := internalsemantic.ExpressionParser
	internalsemantic.ExpressionParser = parser
	t.Cleanup(func() {
		internalsemantic.ExpressionParser = previous
	})
}

func assertBinarySubtractExpr(t *testing.T, expr pkgsemantic.ExprNode) {
	t.Helper()
	binary, ok := expr.(pkgsemantic.BinaryExpr)
	if !ok {
		t.Fatalf("expr = %#v, want BinaryExpr", expr)
	}
	if binary.Op != pkgsemantic.OpSubtract {
		t.Fatalf("binary.Op = %q, want %q", binary.Op, pkgsemantic.OpSubtract)
	}
}
