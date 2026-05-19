package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/biqly/biqly/internal/ai"
	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/pkg/internalapi"
	"github.com/biqly/biqly/pkg/queryclient"
)

func TestAIHandlerPreviewUsesQueryClientDryRun(t *testing.T) {
	t.Parallel()

	var dryRunCalled bool
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/query/dry-run" {
			t.Fatalf("path: got %s, want /internal/query/dry-run", r.URL.Path)
		}
		dryRunCalled = true
		writeUpstreamJSON(t, w, internalapi.DryRunResponse{
			SQL:  `SELECT 1`,
			Args: []any{"arg"},
		})
	}))
	defer upstream.Close()

	lq := integrationLogicalQuery()
	resp := &ai.Response{LogicalQuery: &lq}
	handler := &AIHandler{deps: &app.Dependencies{QueryClient: queryclient.New(upstream.URL)}}

	rec := httptest.NewRecorder()
	handler.finishAIPreview(context.Background(), rec, aiQueryRequest{DatasourceID: integrationDSID}, integrationSemanticModel(), resp)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, body %s", rec.Code, rec.Body.String())
	}
	if !dryRunCalled {
		t.Fatal("query dry-run upstream was not called")
	}
	if resp.SQL != "SELECT 1" || len(resp.Args) != 1 || resp.Args[0] != "arg" {
		t.Fatalf("response compile payload: sql=%q args=%#v", resp.SQL, resp.Args)
	}
}

func TestAIHandlerRunUsesQueryClientRun(t *testing.T) {
	t.Parallel()

	var runCalled bool
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/query/run" {
			t.Fatalf("path: got %s, want /internal/query/run", r.URL.Path)
		}
		runCalled = true
		writeUpstreamJSON(t, w, internalapi.RunResponse{
			Columns:    []query.ResultColumn{{Name: "country", Type: "text"}},
			Rows:       [][]any{{"TR"}},
			RowCount:   1,
			DurationMs: 7,
			SQL:        `SELECT "TR"`,
		})
	}))
	defer upstream.Close()

	lq := integrationLogicalQuery()
	resp := &ai.Response{LogicalQuery: &lq}
	handler := &AIHandler{deps: &app.Dependencies{QueryClient: queryclient.New(upstream.URL)}}

	rec := httptest.NewRecorder()
	handler.finishAIRun(context.Background(), rec, integrationSemanticModel(), resp, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, body %s", rec.Code, rec.Body.String())
	}
	if !runCalled {
		t.Fatal("query run upstream was not called")
	}
	if resp.Result == nil || resp.Result.Stats.RowCount != 1 || resp.Result.Stats.DurationMs != 7 {
		t.Fatalf("run result stats: %#v", resp.Result)
	}
	if resp.SQL != `SELECT "TR"` {
		t.Fatalf("sql: got %q", resp.SQL)
	}
}

func writeUpstreamJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatalf("encode upstream response: %v", err)
	}
}
