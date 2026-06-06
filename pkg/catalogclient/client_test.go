package catalogclient_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/biqly/biqly/pkg/catalogclient"
	"github.com/biqly/biqly/pkg/common/requestid"
	"github.com/biqly/biqly/pkg/common/tracecontext"
	"github.com/biqly/biqly/pkg/internalapi"
	"github.com/biqly/biqly/pkg/logicalquery"
	"github.com/biqly/biqly/pkg/metadata"
	"github.com/biqly/biqly/pkg/semantic"
)

// testToken is the bearer token every test server asserts. Centralised so
// each handler can validate the header without parameterisation.
const testToken = "tok"

// fakeServer wires a test HTTPS server to handler and returns a *Client
// pointed at it. The auth + Accept headers are validated centrally to keep
// the per-test handlers focused on response shape.
func fakeServer(t *testing.T, handler http.HandlerFunc) *catalogclient.Client {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer "+testToken {
			t.Errorf("missing/wrong auth header: got %q want Bearer %q", got, testToken)
		}
		if got := r.Header.Get("X-Internal-Caller"); got != "test" {
			t.Errorf("missing/wrong caller header: got %q want test", got)
		}
		if got := r.Header.Get("Accept"); got != "application/json" {
			t.Errorf("missing Accept header: %q", got)
		}
		handler(w, r)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return catalogclient.New(srv.URL, catalogclient.WithAuthToken(testToken), catalogclient.WithCaller("test"))
}

func encodeTestJSON(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Fatalf("encode response: %v", err)
	}
}

func writeTestString(t *testing.T, w http.ResponseWriter, s string) {
	t.Helper()
	if _, err := w.Write([]byte(s)); err != nil {
		t.Fatalf("write response: %v", err)
	}
}

func TestHealth_OK(t *testing.T) {
	c := fakeServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/health" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		encodeTestJSON(t, w, internalapi.HealthResponse{
			Status: "ok", Service: "catalog",
		})
	})
	out, err := c.Health(context.Background())
	if err != nil {
		t.Fatalf("Health() error: %v", err)
	}
	if out.Status != "ok" || out.Service != "catalog" {
		t.Errorf("unexpected response: %+v", out)
	}
}

func TestRequestIDPropagation(t *testing.T) {
	c := fakeServer(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Request-ID"); got != "req-123" {
			t.Fatalf("X-Request-ID: got %q, want req-123", got)
		}
		if got := r.Header.Get("traceparent"); got != sampleTraceparent {
			t.Fatalf("traceparent: got %q, want %q", got, sampleTraceparent)
		}
		encodeTestJSON(t, w, internalapi.HealthResponse{Status: "ok"})
	})

	ctx := requestid.WithRequestID(context.Background(), "req-123")
	ctx = tracecontext.WithTraceparent(ctx, sampleTraceparent)
	_, err := c.Health(ctx)
	if err != nil {
		t.Fatalf("Health() error: %v", err)
	}
}

const sampleTraceparent = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"

func TestGetDatasource_NotFound(t *testing.T) {
	c := fakeServer(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/internal/datasources/missing") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		encodeTestJSON(t, w, internalapi.Error{
			Code: internalapi.CodeNotFound, Error: "datasource not found",
		})
	})
	_, err := c.GetDatasource(context.Background(), "missing")
	if !errors.Is(err, catalogclient.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got: %v", err)
	}
	apiErr, ok := errors.AsType[*catalogclient.APIError](err)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.StatusCode != http.StatusNotFound || apiErr.Code != internalapi.CodeNotFound {
		t.Errorf("APIError mismatch: %+v", apiErr)
	}
}

func TestGetDatasource_OK(t *testing.T) {
	want := metadata.Datasource{ID: "ds_1", Name: "primary", Type: "postgres"}
	c := fakeServer(t, func(w http.ResponseWriter, _ *http.Request) {
		encodeTestJSON(t, w, want)
	})
	got, err := c.GetDatasource(context.Background(), "ds_1")
	if err != nil {
		t.Fatalf("GetDatasource() error: %v", err)
	}
	if got.ID != want.ID || got.Name != want.Name || got.Type != want.Type {
		t.Errorf("unexpected datasource: %+v", got)
	}
}

func TestListTables_QueryParams(t *testing.T) {
	c := fakeServer(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("datasource_id") != "ds_1" {
			t.Errorf("missing datasource_id: %v", q)
		}
		if q.Get("schema_name") != "public" {
			t.Errorf("missing schema_name: %v", q)
		}
		encodeTestJSON(t, w, []metadata.Table{{ID: "t1", TableName: "orders"}})
	})
	out, err := c.ListTables(context.Background(), "ds_1", "public")
	if err != nil {
		t.Fatalf("ListTables() error: %v", err)
	}
	if len(out) != 1 || out[0].TableName != "orders" {
		t.Errorf("unexpected response: %+v", out)
	}
}

func TestCreateAIHistory_ReturnsID(t *testing.T) {
	c := fakeServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		var req internalapi.AIHistoryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if req.Entry.DatasourceID != "ds_1" {
			t.Errorf("unexpected entry: %+v", req.Entry)
		}
		w.WriteHeader(http.StatusCreated)
		encodeTestJSON(t, w, internalapi.AIHistoryResponse{ID: "hist_42"})
	})
	id, err := c.CreateAIHistory(context.Background(), &metadata.AIQueryHistoryEntry{
		DatasourceID: "ds_1",
		Question:     "hello",
	})
	if err != nil {
		t.Fatalf("CreateAIHistory() error: %v", err)
	}
	if id != "hist_42" {
		t.Errorf("unexpected id: %s", id)
	}
}

func TestCreateEvalResults_PostsBatch(t *testing.T) {
	now := time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC)
	c := fakeServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/internal/eval-results" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		var req internalapi.EvalResultsRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if req.RunID != "run_1" || req.Provider != "openai" || req.Model != "gpt-4o" {
			t.Errorf("unexpected request metadata: %+v", req)
		}
		if len(req.Results) != 1 || req.Results[0].Case.ID != "case_1" {
			t.Errorf("unexpected results: %+v", req.Results)
		}
		encodeTestJSON(t, w, internalapi.EvalResultsResponse{
			RunID:      req.RunID,
			TotalCases: len(req.Results),
		})
	})

	resp, err := c.CreateEvalResults(context.Background(), &catalogclient.EvalResultsInput{
		RunID:            "run_1",
		Provider:         "openai",
		Model:            "gpt-4o",
		ContextVersion:   3,
		ContextUpdatedAt: now,
		Results: []internalapi.EvalResultMetrics{{
			Case: internalapi.EvalGoldenCase{
				ID:       "case_1",
				Question: "total revenue",
				Expected: logicalquery.LogicalQuery{Select: []logicalquery.SelectItem{{Type: "metric", Name: "revenue"}}},
			},
			Got:        &logicalquery.LogicalQuery{Select: []logicalquery.SelectItem{{Type: "metric", Name: "revenue"}}},
			Match:      true,
			Confidence: 0.98,
			LatencyMs:  1200,
			TokenCount: 321,
		}},
	})
	if err != nil {
		t.Fatalf("CreateEvalResults() error: %v", err)
	}
	if resp.RunID != "run_1" || resp.TotalCases != 1 {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestGetModel_PassesIDInPath(t *testing.T) {
	c := fakeServer(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/internal/models/m_1") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		encodeTestJSON(t, w, semantic.SemanticModel{ID: "m_1", Name: "orders"})
	})
	m, err := c.GetModel(context.Background(), "m_1")
	if err != nil {
		t.Fatalf("GetModel() error: %v", err)
	}
	if m.ID != "m_1" {
		t.Errorf("unexpected model id: %s", m.ID)
	}
}

func TestUpstreamError_5xx(t *testing.T) {
	c := fakeServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		encodeTestJSON(t, w, internalapi.Error{
			Code: internalapi.CodeUpstream, Error: "downstream catalog dead",
		})
	})
	_, err := c.ListDatasources(context.Background())
	if !errors.Is(err, catalogclient.ErrUpstream) {
		t.Fatalf("expected ErrUpstream, got: %v", err)
	}
}

func TestHTMLProxyError_IsParsed(t *testing.T) {
	// Misconfigured ingress returns HTML — client must still surface a usable
	// APIError instead of a JSON decode panic.
	c := fakeServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusBadGateway)
		writeTestString(t, w, "<html><body>502 Bad Gateway</body></html>")
	})
	_, err := c.ListDatasources(context.Background())
	if !errors.Is(err, catalogclient.ErrUpstream) {
		t.Fatalf("expected ErrUpstream, got: %v", err)
	}
	apiErr, ok := errors.AsType[*catalogclient.APIError](err)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.StatusCode != http.StatusBadGateway {
		t.Errorf("unexpected status: %d", apiErr.StatusCode)
	}
}

// fakeDoer lets the next test exercise the HTTPDoer seam.
type fakeDoer struct{ called bool }

func (f *fakeDoer) Do(*http.Request) (*http.Response, error) {
	f.called = true
	return nil, errors.New("transport blew up")
}

func TestWithHTTPClient_Transport(t *testing.T) {
	d := &fakeDoer{}
	c := catalogclient.New("http://example.invalid", catalogclient.WithHTTPClient(d))
	if _, err := c.Health(context.Background()); err == nil {
		t.Fatal("expected error from fake doer")
	}
	if !d.called {
		t.Fatal("custom doer was not used")
	}
}
