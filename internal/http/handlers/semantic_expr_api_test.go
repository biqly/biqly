package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/bytedance/sonic"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/config"
	"github.com/biqly/biqly/internal/datasource"
	"github.com/biqly/biqly/internal/datasource/postgres"
	"github.com/biqly/biqly/internal/metadata"
	internalsemantic "github.com/biqly/biqly/internal/semantic"
	pkgsemantic "github.com/biqly/biqly/pkg/semantic"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func TestDimensionFromRequestParsesExpressionString(t *testing.T) {
	restoreExpressionParser(t, func(expr string) (pkgsemantic.ExprNode, error) {
		if expr != "revenue - cost" {
			t.Fatalf("unexpected expression: %q", expr)
		}
		return &pkgsemantic.BinaryExpr{
			Op:    pkgsemantic.OpSubtract,
			Left:  &pkgsemantic.ColumnRefExpr{Column: "revenue"},
			Right: &pkgsemantic.ColumnRefExpr{Column: "cost"},
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
	dimJSON, err := sonic.ConfigStd.Marshal(pkgsemantic.Dimension{
		ID:             "dim_1",
		ModelID:        "model_1",
		Name:           "margin",
		ColumnRef:      "orders.revenue",
		Type:           "number",
		CalculatedExpr: &pkgsemantic.ColumnRefExpr{Column: "revenue"},
	})
	if err != nil {
		t.Fatalf("marshal dimension: %v", err)
	}
	if !strings.Contains(string(dimJSON), `"calculated_expr"`) {
		t.Fatalf("dimension response JSON = %s, want calculated_expr", dimJSON)
	}

	metricJSON, err := sonic.ConfigStd.Marshal(pkgsemantic.Metric{
		ID:          "metric_1",
		ModelID:     "model_1",
		Name:        "net_revenue",
		Expression:  "revenue",
		Aggregation: "sum",
		Expr:        &pkgsemantic.ColumnRefExpr{Column: "revenue"},
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
	previous := internalsemantic.CurrentExpressionParser()
	internalsemantic.RegisterExpressionParser(parser)
	t.Cleanup(func() {
		internalsemantic.RegisterExpressionParser(previous)
	})
}

func assertBinarySubtractExpr(t *testing.T, expr pkgsemantic.ExprNode) {
	t.Helper()
	binary, ok := expr.(*pkgsemantic.BinaryExpr)
	if !ok {
		t.Fatalf("expr = %#v, want BinaryExpr", expr)
	}
	if binary.Op != pkgsemantic.OpSubtract {
		t.Fatalf("binary.Op = %q, want %q", binary.Op, pkgsemantic.OpSubtract)
	}
}

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("BI_METADATA_DB_DSN")
	if dsn == "" {
		//nolint:gosec // local test DSN only
		dsn = "postgres://bi_user:bi_password@localhost:5432/bi_metadata?sslmode=disable"
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Skip("skipping integration; DB not available:", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Skip("skipping integration; ping failed:", err)
	}
	return db
}

func TestGetModelLineage(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	// Seed datasource & model
	datasourceID := uuid.NewString()
	modelID := uuid.NewString()
	dimID := uuid.NewString()
	metID := uuid.NewString()

	_, err := db.ExecContext(ctx,
		`INSERT INTO datasources (id, name, type, dsn_encrypted) VALUES ($1, $2, 'postgres', 'enc')`,
		datasourceID, "lineage-test-ds")
	if err != nil {
		t.Fatalf("failed to seed datasource: %v", err)
	}
	defer func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM datasources WHERE id = $1`, datasourceID)
	}()

	_, err = db.ExecContext(ctx,
		`INSERT INTO semantic_models (id, datasource_id, name, base_schema, base_table) VALUES ($1, $2, $3, 'public', $4)`,
		modelID, datasourceID, "lineage_test_model", "orders")
	if err != nil {
		t.Fatalf("failed to seed model: %v", err)
	}

	// Insert dimension margin = revenue - cost
	_, err = db.ExecContext(ctx, `
		INSERT INTO semantic_dimensions (id, model_id, name, label, column_ref, type, is_active, calculated_expression, calculated_expr_json)
		VALUES ($1, $2, $3, $4, $5, $6, true, $7, $8)
	`, dimID, modelID, "margin", "Margin", "revenue", "number", "orders.revenue - orders.cost",
		`{"type":"binary","op":"subtract","left":{"type":"column_ref","column":"revenue"},"right":{"type":"column_ref","column":"cost"}}`)
	if err != nil {
		t.Fatalf("failed to seed dimension: %v", err)
	}

	// Insert metric profit_ratio = sum([margin]) / count(orders.id)
	_, err = db.ExecContext(ctx, `
		INSERT INTO semantic_metrics (id, model_id, name, label, expression, aggregation, is_active, expr_json)
		VALUES ($1, $2, $3, $4, $5, $6, true, $7)
	`, metID, modelID, "profit_ratio", "Profit Ratio", "sum([margin]) / count(orders.id)", "custom",
		`{"type":"binary","op":"divide","left":{"type":"function_call","name":"sum","args":[{"type":"dimension_ref","name":"margin"}]},"right":{"type":"function_call","name":"count","args":[{"type":"column_ref","column":"id"}]}}`)
	if err != nil {
		t.Fatalf("failed to seed metric: %v", err)
	}

	// Build handler
	cfg := &config.Config{}
	deps := &app.CatalogDeps{
		Config:       cfg,
		SemanticRepo: internalsemantic.NewRepository(db),
	}
	handler := NewSemanticHandler(deps)

	// Make request
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/semantic/models/"+modelID+"/lineage", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", modelID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	handler.GetModelLineage(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp LineageResponse
	if err := sonic.ConfigStd.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Validate nodes
	if len(resp.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d: %+v", len(resp.Nodes), resp.Nodes)
	}
	// Validate edges: profit_ratio depends on dimension margin (so From: profit_ratio, To: margin)
	if len(resp.Edges) != 1 {
		t.Fatalf("expected 1 edge, got %d: %+v", len(resp.Edges), resp.Edges)
	}
	edge := resp.Edges[0]
	if strings.ToLower(edge.From) != "profit_ratio" || strings.ToLower(edge.To) != "margin" {
		t.Fatalf("unexpected edge: %+v", edge)
	}
}

type compileExpressionFixture struct {
	ctx     context.Context
	handler *SemanticHandler
	modelID string
}

func setupCompileExpressionFixture(t *testing.T) compileExpressionFixture {
	t.Helper()
	db := openTestDB(t)
	ctx := context.Background()

	datasourceID := uuid.NewString()
	modelID := uuid.NewString()

	_, err := db.ExecContext(ctx,
		`INSERT INTO datasources (id, name, type, dsn_encrypted) VALUES ($1, $2, 'postgres', 'enc')`,
		datasourceID, "compile-test-ds")
	if err != nil {
		t.Fatalf("failed to seed datasource: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM datasources WHERE id = $1`, datasourceID)
	})

	_, err = db.ExecContext(ctx,
		`INSERT INTO semantic_models (id, datasource_id, name, base_schema, base_table) VALUES ($1, $2, $3, 'public', $4)`,
		modelID, datasourceID, "compile_test_model", "orders")
	if err != nil {
		t.Fatalf("failed to seed model: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM semantic_models WHERE id = $1`, modelID)
	})

	reg := datasource.NewRegistry()
	reg.Register(postgres.NewDriver())

	deps := &app.CatalogDeps{
		Config:       &config.Config{},
		DriverReg:    reg,
		SemanticRepo: internalsemantic.NewRepository(db),
		MetaRepo:     metadata.NewRepository(db),
	}

	return compileExpressionFixture{
		ctx:     ctx,
		handler: NewSemanticHandler(deps),
		modelID: modelID,
	}
}

func postCompileExpression(t *testing.T, fx compileExpressionFixture, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(fx.ctx, http.MethodPost, "/api/semantic/models/"+fx.modelID+"/compile-expression", strings.NewReader(body))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", fx.modelID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()
	fx.handler.CompileExpression(rec, req)
	return rec
}

func assertCompileExpressionSQL(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int, wantSQL string) {
	t.Helper()
	if rec.Code != wantStatus {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, wantStatus, rec.Body.String())
	}
	if wantSQL == "" {
		return
	}
	var resp struct {
		SQL string `json:"sql"`
	}
	if err := sonic.ConfigStd.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.SQL != wantSQL {
		t.Fatalf("expected SQL %q, got %q", wantSQL, resp.SQL)
	}
}

func TestCompileExpression(t *testing.T) {
	fx := setupCompileExpressionFixture(t)
	expectedSQL := `("revenue" - "cost")`

	t.Run("compile JSON AST successfully", func(t *testing.T) {
		rec := postCompileExpression(t, fx, `{
			"expr": {
				"type": "binary",
				"op": "subtract",
				"left": {"type": "column_ref", "column": "revenue"},
				"right": {"type": "column_ref", "column": "cost"}
			}
		}`)
		assertCompileExpressionSQL(t, rec, http.StatusOK, expectedSQL)
	})

	t.Run("compile expression string successfully", func(t *testing.T) {
		restoreExpressionParser(t, func(expr string) (pkgsemantic.ExprNode, error) {
			if expr != "revenue - cost" {
				t.Fatalf("unexpected expression: %q", expr)
			}
			return &pkgsemantic.BinaryExpr{
				Op:    pkgsemantic.OpSubtract,
				Left:  &pkgsemantic.ColumnRefExpr{Column: "revenue"},
				Right: &pkgsemantic.ColumnRefExpr{Column: "cost"},
			}, nil
		})

		rec := postCompileExpression(t, fx, `{"expression": "revenue - cost"}`)
		assertCompileExpressionSQL(t, rec, http.StatusOK, expectedSQL)
	})

	t.Run("returns bad request for invalid expression", func(t *testing.T) {
		rec := postCompileExpression(t, fx, `{"expr": {"type": "unknown"}}`)
		assertCompileExpressionSQL(t, rec, http.StatusBadRequest, "")
	})

	t.Run("rejects aggregate without allow_aggregates", func(t *testing.T) {
		rec := postCompileExpression(t, fx, `{
			"expr": {
				"type": "function_call",
				"name": "sum",
				"args": [{"type": "column_ref", "column": "revenue"}]
			}
		}`)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
		}
		if !strings.Contains(strings.ToLower(rec.Body.String()), "sum") {
			t.Fatalf("body = %s, want sum disallowed error", rec.Body.String())
		}
	})

	t.Run("compiles aggregate with allow_aggregates", func(t *testing.T) {
		rec := postCompileExpression(t, fx, `{
			"allow_aggregates": true,
			"expr": {
				"type": "function_call",
				"name": "sum",
				"args": [{"type": "column_ref", "column": "revenue"}]
			}
		}`)
		assertCompileExpressionSQL(t, rec, http.StatusOK, `SUM("revenue")`)
	})
}
