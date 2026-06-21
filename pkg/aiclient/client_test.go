package aiclient_test

import (
	"context"
	"errors"
	"github.com/bytedance/sonic"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/biqly/biqly/pkg/aiclient"
	"github.com/biqly/biqly/pkg/common/httpclient"
	"github.com/biqly/biqly/pkg/common/requestid"
	"github.com/biqly/biqly/pkg/internalapi"
	"github.com/biqly/biqly/pkg/logicalquery"
	pkgquery "github.com/biqly/biqly/pkg/query"
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

func sampleQueryRequest() *aiclient.QueryRequest {
	return &aiclient.QueryRequest{
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
		_ = sonic.ConfigStd.NewEncoder(w).Encode(aiclient.SettingsResponse{Provider: "openai"})
	})

	ctx := requestid.WithRequestID(context.Background(), "req-123")
	_, err := c.Settings(ctx)
	if err != nil {
		t.Fatalf("Settings() error: %v", err)
	}
}

func TestQuery_RoundTrip(t *testing.T) {
	wantLQ := logicalquery.LogicalQuery{
		DatasourceID: "ds_1",
		ModelID:      "m_1",
		Select:       []logicalquery.SelectItem{{Type: "metric", Name: "revenue"}},
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
		if err := sonic.ConfigStd.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if req.DatasourceID != "ds_1" || req.Question != "total revenue" {
			t.Errorf("unexpected request: %+v", req)
		}
		_ = sonic.ConfigStd.NewEncoder(w).Encode(aiclient.Response{
			Result: &aiclient.AIResult{
				LogicalQuery: &wantLQ,
				Confidence:   0.92,
				Warnings:     []string{"ok"},
			},
			Metadata: &aiclient.AIMetadata{
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
		_ = sonic.ConfigStd.NewEncoder(w).Encode(aiclient.Response{
			Clarification: &aiclient.ClarificationResponse{
				NeedsClarification:    true,
				ClarificationQuestion: "Which table?",
			},
			Result: &aiclient.AIResult{
				Confidence: 0,
			},
		})
	})

	_, err := c.Query(context.Background(), sampleQueryRequest())
	if !errors.Is(err, aiclient.ErrNeedsClarification) {
		t.Fatalf("expected ErrNeedsClarification, got: %v", err)
	}
	clarErr, ok := errors.AsType[*aiclient.ClarificationError](err)
	if !ok {
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
		lq := logicalquery.LogicalQuery{DatasourceID: "ds_1", ModelID: "m_1", Limit: 10}
		_ = sonic.ConfigStd.NewEncoder(w).Encode(aiclient.Response{
			Result: &aiclient.AIResult{
				LogicalQuery: &lq,
				SQL:          "SELECT 1",
				Args:         []any{},
				Confidence:   0.88,
				Result: &pkgquery.Result{
					Columns: []pkgquery.ResultColumn{{Name: "revenue"}},
					Rows:    [][]any{{42.0}},
					Stats:   pkgquery.Stats{RowCount: 1, DurationMs: 3},
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
		lq := logicalquery.LogicalQuery{DatasourceID: "ds_1", Limit: 5}
		_ = sonic.ConfigStd.NewEncoder(w).Encode(aiclient.Response{
			Result: &aiclient.AIResult{
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
		_ = sonic.ConfigStd.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
	})
	_, err := c.Query(context.Background(), sampleQueryRequest())
	if !errors.Is(err, aiclient.ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got: %v", err)
	}
	apiErr, ok := errors.AsType[*aiclient.APIError](err)
	if !ok {
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
		_ = sonic.ConfigStd.NewEncoder(w).Encode(internalapi.Error{
			Code: internalapi.CodeInvalidRequest, Error: "question is required",
		})
	})
	_, err := c.Query(context.Background(), sampleQueryRequest())
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
		if err := sonic.ConfigStd.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if req.DatasourceID != "ds_1" || req.Table != "orders" {
			t.Errorf("unexpected request: %+v", req)
		}
		_ = sonic.ConfigStd.NewEncoder(w).Encode(aiclient.DescribeResponse{
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
		_ = sonic.ConfigStd.NewEncoder(w).Encode(aiclient.EmbedResponse{
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
		_ = sonic.ConfigStd.NewEncoder(w).Encode(aiclient.SettingsResponse{
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

func TestWithUserAgent_Applied(t *testing.T) {
	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		_ = sonic.ConfigStd.NewEncoder(w).Encode(aiclient.SettingsResponse{Provider: "openai"})
	}))
	t.Cleanup(srv.Close)

	c := aiclient.New(srv.URL, aiclient.WithUserAgent("custom-agent/1.0"))
	_, _ = c.Settings(context.Background())
	if gotUA != "custom-agent/1.0" {
		t.Fatalf("User-Agent = %q, want %q", gotUA, "custom-agent/1.0")
	}
}

func TestWithUserAgent_EmptyDoesNotOverride(t *testing.T) {
	c := aiclient.New("http://example.invalid", aiclient.WithUserAgent(""))
	if c.BaseURL() != "http://example.invalid" {
		t.Fatalf("unexpected base URL: %s", c.BaseURL())
	}
	// Empty WithUserAgent should not set it; verify via header in a real server.
	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		_ = sonic.ConfigStd.NewEncoder(w).Encode(aiclient.SettingsResponse{Provider: "openai"})
	}))
	t.Cleanup(srv.Close)
	c2 := aiclient.New(srv.URL, aiclient.WithUserAgent(""))
	_, _ = c2.Settings(context.Background())
	if gotUA != "biqly-aiclient/0.1" {
		t.Fatalf("User-Agent = %q, want default %q", gotUA, "biqly-aiclient/0.1")
	}
}

func TestWithRetryPolicy(t *testing.T) {
	custom := httpclient.RetryPolicy{MaxAttempts: 1, BaseBackoff: 1}
	c := aiclient.New("http://example.invalid", aiclient.WithRetryPolicy(custom))
	// The option should be applied; we can't easily inspect inside, but
	// constructing with it shouldn't panic or behave differently.
	if c.BaseURL() != "http://example.invalid" {
		t.Fatalf("unexpected base URL after WithRetryPolicy: %s", c.BaseURL())
	}
}

func TestWithCircuitBreaker_Nil(t *testing.T) {
	c := aiclient.New("http://example.invalid", aiclient.WithCircuitBreaker(nil))
	if c.BaseURL() != "http://example.invalid" {
		t.Fatalf("unexpected base URL after WithCircuitBreaker: %s", c.BaseURL())
	}
}

func TestBaseURL(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"http://localhost:8080", "http://localhost:8080"},
		{"https://ai.example.com/", "https://ai.example.com"},
		{"  https://ai.example.com  ", "https://ai.example.com"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			c := aiclient.New(tt.input, aiclient.WithAuthToken("x"))
			if got := c.BaseURL(); got != tt.want {
				t.Errorf("BaseURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAPIError_Error(t *testing.T) {
	tests := []struct {
		name   string
		status int
		code   string
		msg    string
		want   string
	}{
		{
			name:   "with code",
			status: http.StatusBadRequest,
			code:   "invalid_request",
			msg:    "question required",
			want:   "aiclient: 400 invalid_request: question required",
		},
		{
			name:   "without code",
			status: http.StatusNotFound,
			code:   "",
			msg:    "not found",
			want:   "aiclient: 404: not found",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := aiclient.NewAPIErrorFromResponseForTest(tt.status, internalapi.Error{Code: tt.code, Error: tt.msg})
			if got := err.Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestClarificationError_Error(t *testing.T) {
	tests := []struct {
		name     string
		response *aiclient.QueryResponse
		want     string
	}{
		{
			name: "with question",
			response: &aiclient.QueryResponse{
				Clarification: &aiclient.ClarificationResponse{
					NeedsClarification:    true,
					ClarificationQuestion: "Which table?",
				},
			},
			want: "aiclient: needs clarification: Which table?",
		},
		{
			name: "without question",
			response: &aiclient.QueryResponse{
				Clarification: &aiclient.ClarificationResponse{
					NeedsClarification: true,
				},
			},
			want: "aiclient: needs clarification",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := aiclient.NewClarificationError(tt.response)
			if got := err.Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSentinelForStatus(t *testing.T) {
	tests := []struct {
		name   string
		status int
		code   string
		want   error
	}{
		{"CodeNotFound", 0, internalapi.CodeNotFound, aiclient.ErrNotFound},
		{"CodeInvalidRequest", 0, internalapi.CodeInvalidRequest, aiclient.ErrInvalidRequest},
		{"StatusNotFound", http.StatusNotFound, "", aiclient.ErrNotFound},
		{"StatusBadRequest", http.StatusBadRequest, "", aiclient.ErrInvalidRequest},
		{"StatusUnauthorized", http.StatusUnauthorized, "", aiclient.ErrUnauthorized},
		{"StatusForbidden", http.StatusForbidden, "", aiclient.ErrUnauthorized},
		{"StatusUpstream", http.StatusInternalServerError, "", aiclient.ErrUpstream},
		{"StatusServiceUnavailable", http.StatusServiceUnavailable, "", aiclient.ErrUpstream},
		{"UnmappedStatus", http.StatusTeapot, "", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := aiclient.SentinelForStatusPublic(tt.status, tt.code)
			if tt.want == nil {
				if got != nil {
					t.Errorf("sentinelForStatus(%d, %q) = %v, want nil", tt.status, tt.code, got)
				}
				return
			}
			if !errors.Is(got, tt.want) {
				t.Errorf("sentinelForStatus(%d, %q) = %v, want %v", tt.status, tt.code, got, tt.want)
			}
		})
	}
}

func TestDecodeErrorResponse(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantMsg    string
		wantCode   string
	}{
		{
			name:       "legacy error field",
			statusCode: http.StatusBadRequest,
			body:       `{"error":"bad request"}`,
			wantMsg:    "bad request",
		},
		{
			name:       "legacy message field",
			statusCode: http.StatusNotFound,
			body:       `{"message":"table not found"}`,
			wantMsg:    "table not found",
		},
		{
			name:       "internalapi error",
			statusCode: http.StatusForbidden,
			body:       `{"code":"not_found","error":"resource missing"}`,
			wantMsg:    "resource missing",
		},
		{
			name:       "empty body falls back to status text",
			statusCode: http.StatusBadGateway,
			body:       "",
			wantMsg:    "Bad Gateway",
		},
		{
			name:       "non-JSON body",
			statusCode: http.StatusInternalServerError,
			body:       "upstream crashed",
			wantMsg:    "upstream crashed",
		},
		{
			name:       "invalid JSON",
			statusCode: http.StatusBadRequest,
			body:       `not json at all`,
			wantMsg:    "not json at all",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{
				StatusCode: tt.statusCode,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(tt.body)),
			}
			apiErr := aiclient.DecodeErrorResponsePublic(resp)
			if apiErr == nil {
				t.Fatal("expected non-nil error")
			}
			if !strings.Contains(apiErr.Error(), tt.wantMsg) {
				t.Errorf("error = %q, want containing %q", apiErr.Error(), tt.wantMsg)
			}
			if tt.wantCode != "" {
				var apiErr2 *aiclient.APIError
				if errors.As(apiErr, &apiErr2) {
					if apiErr2.Code != tt.wantCode {
						t.Errorf("Code = %q, want %q", apiErr2.Code, tt.wantCode)
					}
				}
			}
		})
	}
}

func TestDecodeErrorResponse_ReadError(t *testing.T) {
	// Simulate a read error from the response body.
	resp := &http.Response{
		StatusCode: http.StatusBadRequest,
		Body:       io.NopCloser(&errorReader{}),
	}
	err := aiclient.DecodeErrorResponsePublic(resp)
	if err == nil {
		t.Fatal("expected error from read failure")
	}
	if !strings.Contains(err.Error(), "read error response") {
		t.Fatalf("unexpected error: %v", err)
	}
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }

func TestDescribe_ErrorPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal error"}`))
	}))
	t.Cleanup(srv.Close)

	c := aiclient.New(srv.URL, aiclient.WithAuthToken("x"))
	_, err := c.Describe(context.Background(), aiclient.DescribeRequest{
		DatasourceID: "ds_1",
		Table:        "orders",
	})
	if err == nil {
		t.Fatal("expected error from Describe error path")
	}
}

func TestPreview_ErrorPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = sonic.ConfigStd.NewEncoder(w).Encode(internalapi.Error{
			Code: internalapi.CodeInvalidRequest, Error: "bad input",
		})
	}))
	t.Cleanup(srv.Close)

	c := aiclient.New(srv.URL, aiclient.WithAuthToken("x"))
	_, err := c.Preview(context.Background(), &aiclient.QueryRequest{DatasourceID: "ds_1", Question: "test"})
	if err == nil {
		t.Fatal("expected error from Preview error path")
	}
}

func TestRun_ErrorPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_ = sonic.ConfigStd.NewEncoder(w).Encode(internalapi.Error{
			Code: "rate_limited", Error: "rate limited",
		})
	}))
	t.Cleanup(srv.Close)

	c := aiclient.New(srv.URL, aiclient.WithAuthToken("x"))
	_, err := c.Run(context.Background(), &aiclient.QueryRequest{DatasourceID: "ds_1", Question: "test"})
	if err == nil {
		t.Fatal("expected error from Run error path")
	}
}
