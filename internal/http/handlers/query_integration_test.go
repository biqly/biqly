package handlers

import (
	"bytes"
	"context"
	"github.com/bytedance/sonic"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestQueryHandlerIntegration_CompileAndRun(t *testing.T) {
	t.Parallel()

	handler := &QueryHandler{query: integrationQueryRunner{}}
	router := chi.NewRouter()
	router.Post("/api/query/compile", handler.Compile)
	router.Post("/api/query/run", handler.Run)

	body, err := sonic.ConfigStd.Marshal(integrationLogicalQuery())
	if err != nil {
		t.Fatalf("marshal logical query: %v", err)
	}

	compileRec := httptest.NewRecorder()
	compileReq := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/query/compile", bytes.NewReader(body))
	router.ServeHTTP(compileRec, compileReq)
	if compileRec.Code != http.StatusOK {
		t.Fatalf("compile status: got %d, body %s", compileRec.Code, compileRec.Body.String())
	}
	var compiled struct {
		SQL  string `json:"sql"`
		Args []any  `json:"args"`
	}
	if err := sonic.ConfigStd.Unmarshal(compileRec.Body.Bytes(), &compiled); err != nil {
		t.Fatalf("decode compile response: %v", err)
	}
	if !strings.Contains(compiled.SQL, `"public"."orders"`) {
		t.Fatalf("compile sql missing base table: %s", compiled.SQL)
	}
	if !strings.Contains(compiled.SQL, `LEFT JOIN "public"."customers"`) {
		t.Fatalf("compile sql missing customer join: %s", compiled.SQL)
	}
	if len(compiled.Args) != 1 || compiled.Args[0] != "2026-01-01" {
		t.Fatalf("compile args: got %#v, want [2026-01-01]", compiled.Args)
	}

	runRec := httptest.NewRecorder()
	runReq := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/query/run", bytes.NewReader(body))
	router.ServeHTTP(runRec, runReq)
	if runRec.Code != http.StatusOK {
		t.Fatalf("run status: got %d, body %s", runRec.Code, runRec.Body.String())
	}
	var result struct {
		Columns []struct {
			Name string `json:"name"`
		} `json:"columns"`
		Rows  [][]any `json:"rows"`
		Stats struct {
			RowCount int `json:"row_count"`
		} `json:"stats"`
	}
	if err := sonic.ConfigStd.Unmarshal(runRec.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode run response: %v", err)
	}
	if result.Stats.RowCount != 1 || len(result.Rows) != 1 || result.Rows[0][0] != "TR" {
		t.Fatalf("run result: got row_count=%d rows=%#v", result.Stats.RowCount, result.Rows)
	}
	if len(result.Columns) != 1 || result.Columns[0].Name != "country" {
		t.Fatalf("run columns: got %#v", result.Columns)
	}
}

func TestQueryHandlerIntegration_CompileInlineModelPayload(t *testing.T) {
	t.Parallel()

	handler := &QueryHandler{query: integrationQueryRunner{}}
	router := chi.NewRouter()
	router.Post("/api/query/compile", handler.Compile)

	model := integrationSemanticModel()
	model.ID = "auto:metadata"
	lq := integrationLogicalQuery()
	lq.ModelID = model.ID
	body, err := sonic.ConfigStd.Marshal(queryPayload{
		LogicalQuery: lq,
		Model:        model,
	})
	if err != nil {
		t.Fatalf("marshal inline query payload: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/query/compile", bytes.NewReader(body))
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("compile status: got %d, body %s", rec.Code, rec.Body.String())
	}
	var compiled struct {
		SQL string `json:"sql"`
	}
	if err := sonic.ConfigStd.Unmarshal(rec.Body.Bytes(), &compiled); err != nil {
		t.Fatalf("decode compile response: %v", err)
	}
	if !strings.Contains(compiled.SQL, `"public"."orders"`) {
		t.Fatalf("compile sql missing inline model base table: %s", compiled.SQL)
	}
}
