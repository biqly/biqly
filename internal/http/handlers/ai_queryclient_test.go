package handlers

import (
	"context"
	"github.com/bytedance/sonic"
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
	resp := &ai.Response{Result: &ai.AIResult{LogicalQuery: &lq}}
	handler := &AIHandler{deps: (&app.Dependencies{QueryClient: queryclient.New(upstream.URL)}).AIDeps()}

	rec := httptest.NewRecorder()
	handler.finishAIPreview(context.Background(), rec, aiQueryRequest{DatasourceID: integrationDSID}, integrationSemanticModel(), resp)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, body %s", rec.Code, rec.Body.String())
	}
	if !dryRunCalled {
		t.Fatal("query dry-run upstream was not called")
	}
	if resp.Result == nil || resp.Result.SQL != "SELECT 1" || len(resp.Result.Args) != 1 || resp.Result.Args[0] != "arg" {
		t.Fatalf("response compile payload: sql=%q args=%#v", resp.Result.SQL, resp.Result.Args)
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
	resp := &ai.Response{Result: &ai.AIResult{LogicalQuery: &lq}}
	handler := &AIHandler{deps: (&app.Dependencies{QueryClient: queryclient.New(upstream.URL)}).AIDeps()}

	rec := httptest.NewRecorder()
	handler.finishAIRun(context.Background(), rec, integrationSemanticModel(), resp, nil, "which countries?")
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, body %s", rec.Code, rec.Body.String())
	}
	if !runCalled {
		t.Fatal("query run upstream was not called")
	}
	if resp.Result == nil || resp.Result.Result == nil || resp.Result.Result.Stats.RowCount != 1 || resp.Result.Result.Stats.DurationMs != 7 {
		t.Fatalf("run result stats: %#v", resp.Result)
	}
	if resp.Result == nil || resp.Result.SQL != `SELECT "TR"` {
		t.Fatalf("sql: got %q", resp.Result.SQL)
	}
}

func writeUpstreamJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := sonic.ConfigStd.NewEncoder(w).Encode(value); err != nil {
		t.Fatalf("encode upstream response: %v", err)
	}
}
