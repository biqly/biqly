package queryclient_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/pkg/common/httpclient"
	"github.com/biqly/biqly/pkg/common/requestid"
	"github.com/biqly/biqly/pkg/common/tracecontext"
	"github.com/biqly/biqly/pkg/internalapi"
	"github.com/biqly/biqly/pkg/queryclient"
)

// testToken is the bearer token every test server asserts. Centralised so
// the per-test handler can validate the header without parameterisation.
const testToken = "tok"

func fakeServer(t *testing.T, handler http.HandlerFunc) *queryclient.Client {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer "+testToken {
			t.Errorf("missing/wrong auth header: %q", got)
		}
		if got := r.Header.Get("X-Internal-Caller"); got != "test" {
			t.Errorf("missing/wrong caller header: got %q want test", got)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		handler(w, r)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return queryclient.New(srv.URL, queryclient.WithAuthToken(testToken), queryclient.WithCaller("test"))
}

func sampleLQ() query.LogicalQuery {
	return query.LogicalQuery{
		DatasourceID: "ds_1",
		ModelID:      "m_1",
		Select:       []query.SelectItem{{Type: "metric", Name: "revenue"}},
		Limit:        100,
	}
}

func TestCompile_RoundTrip(t *testing.T) {
	c := fakeServer(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/internal/query/compile") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		var req internalapi.CompileRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if req.LogicalQuery.DatasourceID != "ds_1" {
			t.Errorf("unexpected LQ: %+v", req.LogicalQuery)
		}
		_ = json.NewEncoder(w).Encode(internalapi.CompileResponse{
			SQL:         "SELECT SUM(revenue) FROM orders",
			Args:        []any{},
			Fingerprint: "abc123",
		})
	})

	out, err := c.Compile(context.Background(), sampleLQ())
	if err != nil {
		t.Fatalf("Compile() error: %v", err)
	}
	if out.SQL == "" || out.Fingerprint != "abc123" {
		t.Errorf("unexpected response: %+v", out)
	}
}

func TestCompile_RetriesTransientStatus(t *testing.T) {
	attempts := 0
	c := fakeServer(t, func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_ = json.NewEncoder(w).Encode(internalapi.CompileResponse{SQL: "SELECT 1"})
	})

	out, err := c.Compile(context.Background(), sampleLQ())
	if err != nil {
		t.Fatalf("Compile() error: %v", err)
	}
	if out.SQL != "SELECT 1" || attempts != 3 {
		t.Fatalf("response/attempts: got %+v/%d, want SQL SELECT 1 and 3 attempts", out, attempts)
	}
}

func TestCompile_CircuitBreakerOpens(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)

	c := queryclient.New(
		srv.URL,
		queryclient.WithAuthToken(testToken),
		queryclient.WithCaller("test"),
		queryclient.WithRetryPolicy(httpclient.RetryPolicy{MaxAttempts: 1}),
		queryclient.WithCircuitBreaker(httpclient.NewCircuitBreaker(httpclient.CircuitBreakerConfig{
			FailureThreshold: 1,
			OpenDuration:     time.Minute,
		})),
	)
	_, firstErr := c.Compile(context.Background(), sampleLQ())
	if firstErr == nil {
		t.Fatal("first Compile() should fail")
	}
	_, secondErr := c.Compile(context.Background(), sampleLQ())
	if !errors.Is(secondErr, httpclient.ErrCircuitOpen) {
		t.Fatalf("second Compile() error: got %v, want ErrCircuitOpen", secondErr)
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
		_ = json.NewEncoder(w).Encode(internalapi.CompileResponse{SQL: "SELECT 1"})
	})

	ctx := requestid.WithRequestID(context.Background(), "req-123")
	ctx = tracecontext.WithTraceparent(ctx, sampleTraceparent)
	_, err := c.Compile(ctx, sampleLQ())
	if err != nil {
		t.Fatalf("Compile() error: %v", err)
	}
}

const sampleTraceparent = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"

func TestRun_PassesOverrides(t *testing.T) {
	c := fakeServer(t, func(w http.ResponseWriter, r *http.Request) {
		var req internalapi.RunRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.MaxRows != 50 || req.TimeoutMs != 1000 {
			t.Errorf("overrides lost: %+v", req)
		}
		_ = json.NewEncoder(w).Encode(internalapi.RunResponse{
			Columns:    []query.ResultColumn{{Name: "revenue"}},
			Rows:       [][]any{{42.0}},
			RowCount:   1,
			DurationMs: 5,
			SQL:        "SELECT ...",
		})
	})

	out, err := c.Run(context.Background(), sampleLQ(), 50, 1000)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if out.RowCount != 1 || len(out.Rows) != 1 {
		t.Errorf("unexpected response: %+v", out)
	}
}

func TestDryRun_HitsDryRunPath(t *testing.T) {
	c := fakeServer(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/internal/query/dry-run") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(internalapi.DryRunResponse{
			SQL: "EXPLAIN SELECT 1", Fingerprint: "fp",
		})
	})

	out, err := c.DryRun(context.Background(), sampleLQ())
	if err != nil {
		t.Fatalf("DryRun() error: %v", err)
	}
	if out.SQL == "" || out.Fingerprint != "fp" {
		t.Errorf("unexpected response: %+v", out)
	}
}

func TestCompile_ErrorClass(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		code     string
		want     error
		wantHTTP int
	}{
		{"compile_error", http.StatusBadRequest, internalapi.CodeCompileError, queryclient.ErrCompile, http.StatusBadRequest},
		{"execution_error", http.StatusInternalServerError, internalapi.CodeExecutionError, queryclient.ErrExecution, http.StatusInternalServerError},
		{"permission_error", http.StatusForbidden, internalapi.CodePermissionError, queryclient.ErrPermission, http.StatusForbidden},
		{"read_only", http.StatusBadRequest, internalapi.CodeReadOnlyError, queryclient.ErrReadOnly, http.StatusBadRequest},
		{"not_found", http.StatusNotFound, internalapi.CodeNotFound, queryclient.ErrNotFound, http.StatusNotFound},
		{"invalid", http.StatusBadRequest, internalapi.CodeInvalidRequest, queryclient.ErrInvalidRequest, http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := fakeServer(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.status)
				_ = json.NewEncoder(w).Encode(internalapi.Error{
					Code: tt.code, Error: tt.code + " happened",
				})
			})
			_, err := c.Compile(context.Background(), sampleLQ())
			if !errors.Is(err, tt.want) {
				t.Fatalf("want %v sentinel, got %v", tt.want, err)
			}
			apiErr, ok := errors.AsType[*queryclient.APIError](err)
			if !ok {
				t.Fatalf("expected *APIError, got %T", err)
			}
			if apiErr.StatusCode != tt.wantHTTP {
				t.Errorf("unexpected status: %d", apiErr.StatusCode)
			}
		})
	}
}

func TestContextCancellation(t *testing.T) {
	c := fakeServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(internalapi.CompileResponse{SQL: "x"})
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.Compile(ctx, sampleLQ()); err == nil {
		t.Fatal("expected error from cancelled context")
	}
}
