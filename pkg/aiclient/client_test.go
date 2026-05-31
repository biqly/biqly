package aiclient_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/biqly/biqly/internal/ai"
	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/pkg/aiclient"
	"github.com/biqly/biqly/pkg/common/requestid"
	"github.com/biqly/biqly/pkg/common/tracecontext"
	"github.com/biqly/biqly/pkg/internalapi"
)

const testToken = "tok"

func fakeServer(t *testing.T, handler http.HandlerFunc) *aiclient.Client {
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
	return aiclient.New(srv.URL, aiclient.WithAuthToken(testToken), aiclient.WithCaller("test"))
}

func sampleQueryRequest() aiclient.QueryRequest {
	return aiclient.QueryRequest{
		DatasourceID: "ds_1",
		ModelID:      "m_1",
		Question:     "total revenue",
		Tables:       []string{"orders"},
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
		_ = json.NewEncoder(w).Encode(aiclient.SettingsResponse{Provider: "openai"})
	})

	ctx := requestid.WithRequestID(context.Background(), "req-123")
	ctx = tracecontext.WithTraceparent(ctx, sampleTraceparent)
	_, err := c.Settings(ctx)
	if err != nil {
		t.Fatalf("Settings() error: %v", err)
	}
}

const sampleTraceparent = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"

func TestQuery_RoundTrip(t *testing.T) {
	wantLQ := query.LogicalQuery{
		DatasourceID: "ds_1",
		ModelID:      "m_1",
		Select:       []query.SelectItem{{Type: "metric", Name: "revenue"}},
		Limit:        100,
	}
	c := fakeServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/api/ai/query") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		var req aiclient.QueryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if req.DatasourceID != "ds_1" || req.Question != "total revenue" {
			t.Errorf("unexpected request: %+v", req)
		}
		_ = json.NewEncoder(w).Encode(ai.Response{
			Result: &ai.AIResult{
				LogicalQuery: &wantLQ,
				Confidence:   0.92,
				Warnings:     []string{"ok"},
			},
			Metadata: &ai.AIMetadata{
				ModelUsed: "gpt-4o",
			},
		})
	})

	out, err := c.Query(context.Background(), sampleQueryRequest())
	if err != nil {
		t.Fatalf("Query() error: %v", err)
	}
	if out.Metadata == nil || out.Metadata.ModelUsed != "gpt-4o" {
		t.Errorf("unexpected metadata: %+v", out)
	}
	if out.Result == nil || out.Result.Confidence != 0.92 {
		t.Errorf("unexpected confidence: %+v", out)
	}
	if out.Result == nil || out.Result.LogicalQuery == nil || out.Result.LogicalQuery.Select[0].Name != "revenue" {
		t.Errorf("unexpected logical query: %+v", out)
	}
}

func TestQuery_NeedsClarification(t *testing.T) {
	c := fakeServer(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/api/ai/query") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(ai.Response{
			Clarification: &ai.ClarificationResponse{
				NeedsClarification:    true,
				ClarificationQuestion: "Which table?",
			},
			Result: &ai.AIResult{
				Confidence: 0,
			},
		})
	})

	_, err := c.Query(context.Background(), sampleQueryRequest())
	if !errors.Is(err, aiclient.ErrNeedsClarification) {
		t.Fatalf("expected ErrNeedsClarification, got: %v", err)
	}
	var clarErr *aiclient.ClarificationError
	if !errors.As(err, &clarErr) {
		t.Fatalf("expected *ClarificationError, got %T", err)
	}
	if clarErr.Response == nil || clarErr.Response.Clarification == nil || clarErr.Response.Clarification.ClarificationQuestion != "Which table?" {
		t.Errorf("unexpected clarification payload: %+v", clarErr.Response)
	}
}

func TestRun_ReturnsResult(t *testing.T) {
	c := fakeServer(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/api/ai/query/run") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		lq := query.LogicalQuery{DatasourceID: "ds_1", ModelID: "m_1", Limit: 10}
		_ = json.NewEncoder(w).Encode(ai.Response{
			Result: &ai.AIResult{
				LogicalQuery: &lq,
				SQL:          "SELECT 1",
				Args:         []any{},
				Confidence:   0.88,
				Result: &query.QueryResult{
					Columns: []query.ResultColumn{{Name: "revenue"}},
					Rows:    [][]any{{42.0}},
					Stats:   query.Stats{RowCount: 1, DurationMs: 3},
				},
			},
		})
	})

	out, err := c.Run(context.Background(), sampleQueryRequest())
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if out.Result == nil || out.Result.Result == nil || out.Result.Result.Stats.RowCount != 1 {
		t.Errorf("unexpected result: %+v", out)
	}
	if out.Result == nil || out.Result.SQL != "SELECT 1" {
		t.Errorf("unexpected sql: %+v", out)
	}
}

func TestPreview_ReturnsSQL(t *testing.T) {
	c := fakeServer(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/api/ai/query/preview") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		lq := query.LogicalQuery{DatasourceID: "ds_1", Limit: 5}
		_ = json.NewEncoder(w).Encode(ai.Response{
			Result: &ai.AIResult{
				LogicalQuery: &lq,
				SQL:          "SELECT SUM(amount) FROM orders",
				Args:         []any{int64(100)},
				Confidence:   0.9,
			},
		})
	})

	out, err := c.Preview(context.Background(), sampleQueryRequest())
	if err != nil {
		t.Fatalf("Preview() error: %v", err)
	}
	if out.Result == nil || out.Result.SQL == "" {
		t.Fatalf("expected compiled SQL, got empty")
	}
	if !strings.Contains(out.Result.SQL, "SUM") {
		t.Errorf("unexpected sql: %q", out.Result.SQL)
	}
}

func TestQuery_Unauthorized(t *testing.T) {
	c := fakeServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
	})
	_, err := c.Query(context.Background(), sampleQueryRequest())
	if !errors.Is(err, aiclient.ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got: %v", err)
	}
	var apiErr *aiclient.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.StatusCode != http.StatusUnauthorized {
		t.Errorf("unexpected status: %d", apiErr.StatusCode)
	}
}

func TestQuery_InternalAPIErrorEnvelope(t *testing.T) {
	c := fakeServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(internalapi.Error{
			Code: internalapi.CodeInvalidRequest, Error: "question is required",
		})
	})
	_, err := c.Query(context.Background(), aiclient.QueryRequest{})
	if !errors.Is(err, aiclient.ErrInvalidRequest) {
		t.Fatalf("expected ErrInvalidRequest, got: %v", err)
	}
}

func TestDescribe_RoundTrip(t *testing.T) {
	c := fakeServer(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/api/ai/metadata/describe") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		var req aiclient.DescribeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if req.DatasourceID != "ds_1" || req.Table != "orders" {
			t.Errorf("unexpected request: %+v", req)
		}
		_ = json.NewEncoder(w).Encode(ai.DescribeResult{
			Schema:      "public",
			Table:       "orders",
			Description: "Customer orders",
			Model:       "gpt-4o",
		})
	})

	out, err := c.Describe(context.Background(), aiclient.DescribeRequest{
		DatasourceID: "ds_1",
		Table:        "orders",
	})
	if err != nil {
		t.Fatalf("Describe() error: %v", err)
	}
	if out.Description != "Customer orders" {
		t.Errorf("unexpected describe result: %+v", out)
	}
}

func TestEmbed_RoundTrip(t *testing.T) {
	c := fakeServer(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/api/ai/metadata/embed") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(aiclient.EmbedResponse{
			DatasourceID: "ds_1",
			Model:        "text-embedding-3-small",
			Embedded:     2,
			Skipped:      1,
		})
	})

	out, err := c.Embed(context.Background(), aiclient.EmbedRequest{DatasourceID: "ds_1"})
	if err != nil {
		t.Fatalf("Embed() error: %v", err)
	}
	if out.Embedded != 2 || out.Model == "" {
		t.Errorf("unexpected embed response: %+v", out)
	}
}

func TestSettings_GET(t *testing.T) {
	c := fakeServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/api/ai/settings") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(aiclient.SettingsResponse{
			Provider: "openai", LLMModel: "gpt-4o", APIKeyConfigured: true,
		})
	})

	out, err := c.Settings(context.Background())
	if err != nil {
		t.Fatalf("Settings() error: %v", err)
	}
	if out.Provider != "openai" || !out.APIKeyConfigured {
		t.Errorf("unexpected settings: %+v", out)
	}
}

type fakeDoer struct{ called bool }

func (f *fakeDoer) Do(*http.Request) (*http.Response, error) {
	f.called = true
	return nil, errors.New("transport blew up")
}

func TestWithHTTPClient_Transport(t *testing.T) {
	d := &fakeDoer{}
	c := aiclient.New("http://example.invalid", aiclient.WithHTTPClient(d))
	if _, err := c.Settings(context.Background()); err == nil {
		t.Fatal("expected error from fake doer")
	}
	if !d.called {
		t.Fatal("custom doer was not used")
	}
}
