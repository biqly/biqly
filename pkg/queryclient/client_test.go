package queryclient_test

import (
	"bytes"
	"context"
	"errors"
	"github.com/bytedance/sonic"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/biqly/biqly/pkg/common/httpclient"
	"github.com/biqly/biqly/pkg/common/requestid"
	"github.com/biqly/biqly/pkg/internalapi"
	"github.com/biqly/biqly/pkg/logicalquery"
	pkgquery "github.com/biqly/biqly/pkg/query"
	"github.com/biqly/biqly/pkg/queryclient"
	"github.com/biqly/biqly/pkg/semantic"
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

func sampleLQ() *logicalquery.LogicalQuery {
	return &logicalquery.LogicalQuery{
		DatasourceID: "ds_1",
		ModelID:      "m_1",
		Select:       []logicalquery.SelectItem{{Type: "metric", Name: "revenue"}},
		Limit:        100,
	}
}

func TestCompile_RoundTrip(t *testing.T) {
	c := fakeServer(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/internal/query/compile") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		var req internalapi.CompileRequest
		if err := sonic.ConfigStd.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if req.LogicalQuery.DatasourceID != "ds_1" {
			t.Errorf("unexpected LQ: %+v", req.LogicalQuery)
		}
		if err := sonic.ConfigStd.NewEncoder(w).Encode(internalapi.CompileResponse{
			SQL:         "SELECT SUM(revenue) FROM orders",
			Args:        []any{},
			Fingerprint: "abc123",
		}); err != nil {
			t.Fatalf("encode response: %v", err)
		}
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
		if err := sonic.ConfigStd.NewEncoder(w).Encode(internalapi.CompileResponse{SQL: "SELECT 1"}); err != nil {
			t.Fatalf("encode response: %v", err)
		}
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
		if err := sonic.ConfigStd.NewEncoder(w).Encode(internalapi.CompileResponse{SQL: "SELECT 1"}); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	})

	ctx := requestid.WithRequestID(context.Background(), "req-123")
	_, err := c.Compile(ctx, sampleLQ())
	if err != nil {
		t.Fatalf("Compile() error: %v", err)
	}
}

func TestRun_PassesOverrides(t *testing.T) {
	c := fakeServer(t, func(w http.ResponseWriter, r *http.Request) {
		var req internalapi.RunRequest
		if err := sonic.ConfigStd.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.MaxRows != 50 || req.TimeoutMs != 1000 {
			t.Errorf("overrides lost: %+v", req)
		}
		_ = sonic.ConfigStd.NewEncoder(w).Encode(internalapi.RunResponse{
			Columns:    []pkgquery.ResultColumn{{Name: "revenue"}},
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
		_ = sonic.ConfigStd.NewEncoder(w).Encode(internalapi.DryRunResponse{
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

func TestDryRunWithModel_SendsInlineModel(t *testing.T) {
	model := &semantic.SemanticModel{ID: "auto:public.orders", Name: "auto:public.orders"}
	c := fakeServer(t, func(w http.ResponseWriter, r *http.Request) {
		var req internalapi.DryRunRequest
		if err := sonic.ConfigStd.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Model == nil || req.Model.ID != model.ID {
			t.Errorf("inline model lost: %+v", req.Model)
		}
		_ = sonic.ConfigStd.NewEncoder(w).Encode(internalapi.DryRunResponse{
			SQL: "EXPLAIN SELECT 1", Fingerprint: "fp",
		})
	})

	if _, err := c.DryRunWithModel(context.Background(), sampleLQ(), model); err != nil {
		t.Fatalf("DryRunWithModel() error: %v", err)
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
				_ = sonic.ConfigStd.NewEncoder(w).Encode(internalapi.Error{
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
		_ = sonic.ConfigStd.NewEncoder(w).Encode(internalapi.CompileResponse{SQL: "x"})
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.Compile(ctx, sampleLQ()); err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

func TestBaseURL(t *testing.T) {
	c := queryclient.New("http://example.com")
	if got := c.BaseURL(); got != "http://example.com" {
		t.Fatalf("BaseURL() = %q, want %q", got, "http://example.com")
	}

	// Ensure trailing slash is trimmed
	c2 := queryclient.New("http://example.com/")
	if got := c2.BaseURL(); got != "http://example.com" {
		t.Fatalf("BaseURL() = %q, want %q", got, "http://example.com")
	}
}

func TestWithHTTPClient_NilDefaultsToServiceClient(t *testing.T) {
	c := queryclient.New("http://example.com", queryclient.WithHTTPClient(nil))
	if c.BaseURL() != "http://example.com" {
		t.Fatalf("unexpected base URL: %s", c.BaseURL())
	}
}

// roundTripper is a minimal HTTPDoer for testing WithHTTPClient.
type roundTripper struct {
	fn func(req *http.Request) (*http.Response, error)
}

func (rt *roundTripper) Do(req *http.Request) (*http.Response, error) {
	return rt.fn(req)
}

func TestWithHTTPClient_CustomClient(t *testing.T) {
	var gotMethod, gotPath string
	rt := &roundTripper{fn: func(req *http.Request) (*http.Response, error) {
		gotMethod = req.Method
		gotPath = req.URL.Path
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader([]byte(`{"sql":"SELECT 1"}`))),
		}, nil
	}}
	c := queryclient.New("http://example.com", queryclient.WithHTTPClient(rt))
	if _, err := c.Compile(context.Background(), sampleLQ()); err != nil {
		t.Fatalf("Compile() error: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if !strings.HasSuffix(gotPath, "/internal/query/compile") {
		t.Errorf("path = %q, want /internal/query/compile", gotPath)
	}
}

func TestWithUserAgent(t *testing.T) {
	var gotUA string
	rt := &roundTripper{fn: func(req *http.Request) (*http.Response, error) {
		gotUA = req.Header.Get("User-Agent")
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader([]byte(`{"sql":"SELECT 1"}`))),
		}, nil
	}}
	c := queryclient.New("http://example.com",
		queryclient.WithHTTPClient(rt),
		queryclient.WithUserAgent("custom-agent/1.0"),
	)
	if _, err := c.Compile(context.Background(), sampleLQ()); err != nil {
		t.Fatalf("Compile() error: %v", err)
	}
	if gotUA != "custom-agent/1.0" {
		t.Errorf("User-Agent = %q, want %q", gotUA, "custom-agent/1.0")
	}

	// Empty string should keep default
	gotUA = ""
	c2 := queryclient.New("http://example.com",
		queryclient.WithHTTPClient(rt),
		queryclient.WithUserAgent(""),
	)
	if _, err := c2.Compile(context.Background(), sampleLQ()); err != nil {
		t.Fatalf("Compile() error: %v", err)
	}
	if gotUA != "biqly-queryclient/0.1" {
		t.Errorf("User-Agent = %q, want %q (default)", gotUA, "biqly-queryclient/0.1")
	}
}

func TestDo_EmptyBaseURL(t *testing.T) {
	c := queryclient.New("")
	if _, err := c.Compile(context.Background(), sampleLQ()); err == nil {
		t.Fatal("expected error for empty baseURL")
	}
}

func TestDo_NoContentResponse(t *testing.T) {
	rt := &roundTripper{fn: func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusNoContent,
			Body:       io.NopCloser(bytes.NewReader(nil)),
		}, nil
	}}
	c := queryclient.New("http://example.com", queryclient.WithHTTPClient(rt))
	out, err := c.Compile(context.Background(), sampleLQ())
	if err != nil {
		t.Fatalf("Compile() error: %v", err)
	}
	if out.SQL != "" {
		t.Errorf("expected empty response, got SQL=%q", out.SQL)
	}
}

func TestSentinelForStatus(t *testing.T) {
	tests := []struct {
		name   string
		status int
		code   string
		want   error
	}{
		// Code-based mapping (highest priority)
		{"code_not_found", http.StatusBadRequest, internalapi.CodeNotFound, queryclient.ErrNotFound},
		{"code_invalid_request", http.StatusBadRequest, internalapi.CodeInvalidRequest, queryclient.ErrInvalidRequest},
		{"code_compile_error", http.StatusBadRequest, internalapi.CodeCompileError, queryclient.ErrCompile},
		{"code_execution_error", http.StatusInternalServerError, internalapi.CodeExecutionError, queryclient.ErrExecution},
		{"code_permission_error", http.StatusForbidden, internalapi.CodePermissionError, queryclient.ErrPermission},
		{"code_read_only", http.StatusBadRequest, internalapi.CodeReadOnlyError, queryclient.ErrReadOnly},
		// Status-based mapping (fallback)
		{"status_404", http.StatusNotFound, "", queryclient.ErrNotFound},
		{"status_400", http.StatusBadRequest, "", queryclient.ErrInvalidRequest},
		{"status_401", http.StatusUnauthorized, "", queryclient.ErrUnauthorized},
		{"status_403", http.StatusForbidden, "", queryclient.ErrUnauthorized},
		{"status_500", http.StatusInternalServerError, "", queryclient.ErrUpstream},
		{"status_503", http.StatusServiceUnavailable, "", queryclient.ErrUpstream},
		// Unknown status returns nil sentinel (but still returns an APIError)
		{"status_429", http.StatusTooManyRequests, "", nil},
		// Code takes priority over status
		{"code_compile_overrides_500", http.StatusInternalServerError, internalapi.CodeCompileError, queryclient.ErrCompile},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test through the error response path
			rt := &roundTripper{fn: func(_ *http.Request) (*http.Response, error) {
				body := `{"code":"` + tt.code + `","error":"test error"}`
				return &http.Response{
					StatusCode: tt.status,
					Body:       io.NopCloser(bytes.NewReader([]byte(body))),
				}, nil
			}}
			c := queryclient.New("http://example.com", queryclient.WithHTTPClient(rt))
			_, err := c.Compile(context.Background(), sampleLQ())
			if tt.want == nil {
				// For unknown status codes, we still get an APIError but without a sentinel
				if err == nil {
					t.Fatal("expected error")
				}
				// Verify no sentinel matches
				if errors.Is(err, queryclient.ErrNotFound) {
					t.Fatal("should not match ErrNotFound")
				}
				return
			}
			if !errors.Is(err, tt.want) {
				t.Fatalf("status=%d code=%q: got %v, want sentinel %v", tt.status, tt.code, err, tt.want)
			}
		})
	}
}

func TestAPIError_Error(t *testing.T) {
	// Test with Code set
	err := &queryclient.APIError{
		StatusCode: http.StatusBadRequest,
		Code:       "COMPILE_ERR",
		Message:    "syntax error at line 1",
	}
	msg := err.Error()
	want := "queryclient: 400 COMPILE_ERR: syntax error at line 1"
	if msg != want {
		t.Errorf("Error() = %q, want %q", msg, want)
	}

	// Test without Code
	err2 := &queryclient.APIError{
		StatusCode: http.StatusNotFound,
		Message:    "not found",
	}
	msg2 := err2.Error()
	want2 := "queryclient: 404: not found"
	if msg2 != want2 {
		t.Errorf("Error() = %q, want %q", msg2, want2)
	}
}

func TestDo_Non2XXErrorResponse(t *testing.T) {
	rt := &roundTripper{fn: func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusBadRequest,
			Body:       io.NopCloser(bytes.NewReader([]byte(`{"code":"compile_error","error":"bad query"}`))),
		}, nil
	}}
	c := queryclient.New("http://example.com", queryclient.WithHTTPClient(rt))
	_, err := c.Compile(context.Background(), sampleLQ())
	if err == nil {
		t.Fatal("expected error from 400 response")
	}
	if !errors.Is(err, queryclient.ErrCompile) {
		t.Fatalf("expected ErrCompile sentinel, got %v", err)
	}
}

func TestDo_NonJSONErrorResponse(t *testing.T) {
	rt := &roundTripper{fn: func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusBadRequest,
			Body:       io.NopCloser(bytes.NewReader([]byte(`plain text error`))),
		}, nil
	}}
	c := queryclient.New("http://example.com", queryclient.WithHTTPClient(rt))
	_, err := c.Compile(context.Background(), sampleLQ())
	if err == nil {
		t.Fatal("expected error from 400 response")
	}
	var apiErr *queryclient.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.Message != "plain text error" {
		t.Errorf("Message = %q, want %q", apiErr.Message, "plain text error")
	}
}

func TestDo_EmptyErrorResponse(t *testing.T) {
	rt := &roundTripper{fn: func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusBadRequest,
			Body:       io.NopCloser(bytes.NewReader(nil)),
		}, nil
	}}
	c := queryclient.New("http://example.com", queryclient.WithHTTPClient(rt))
	_, err := c.Compile(context.Background(), sampleLQ())
	if err == nil {
		t.Fatal("expected error from 400 response")
	}
	var apiErr *queryclient.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.Message != "Bad Request" {
		t.Errorf("Message = %q, want %q (http.StatusText)", apiErr.Message, "Bad Request")
	}
}
