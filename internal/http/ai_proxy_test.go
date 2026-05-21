package http

import (
	"bytes"
	"context"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/config"
	"github.com/biqly/biqly/pkg/internalapi"
)

func TestRouter_ProxiesAIOwnedPublicRoutes(t *testing.T) {
	t.Parallel()
	var (
		mu       sync.Mutex
		requests []string
	)
	upstream := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		mu.Lock()
		requests = append(requests, r.Method+" "+r.URL.RequestURI())
		mu.Unlock()
		if r.Header.Get("X-Forwarded-Host") == "" {
			t.Errorf("X-Forwarded-Host should be set")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"proxied":true}`))
	}))
	t.Cleanup(upstream.Close)

	handler := Router(&app.Dependencies{
		Config: &config.Config{
			Services: config.ServicesConfig{AIURL: upstream.URL},
		},
	})

	cases := []struct {
		method string
		path   string
	}{
		{stdhttp.MethodPost, "/api/ai/query"},
		{stdhttp.MethodPost, "/api/ai/query/preview"},
		{stdhttp.MethodPost, "/api/ai/query/run"},
		{stdhttp.MethodPost, "/api/ai/metadata/describe"},
		{stdhttp.MethodGet, "/api/ai/settings"},
		{stdhttp.MethodGet, "/api/ai/examples"},
		{stdhttp.MethodPost, "/api/ai/feedback"},
		{stdhttp.MethodGet, "/api/ai/glossary"},
		{stdhttp.MethodGet, "/api/ai/prompt-templates"},
		{stdhttp.MethodPut, "/api/ai/prompt-templates/system_rules/en"},
	}

	for _, tc := range cases {
		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(context.Background(), tc.method, tc.path, bytes.NewBufferString(`{}`))
		handler.ServeHTTP(rec, req)

		if rec.Code != stdhttp.StatusOK {
			t.Fatalf("%s %s status: got %d, want 200; body=%s", tc.method, tc.path, rec.Code, rec.Body.String())
		}
		var body map[string]bool
		if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
			t.Fatalf("decode %s %s: %v", tc.method, tc.path, err)
		}
		if !body["proxied"] {
			t.Fatalf("%s %s expected proxied response, got %+v", tc.method, tc.path, body)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if len(requests) != len(cases) {
		t.Fatalf("upstream request count: got %d, want %d: %v", len(requests), len(cases), requests)
	}
	for i, tc := range cases {
		want := tc.method + " " + tc.path
		if requests[i] != want {
			t.Fatalf("upstream request %d: got %q, want %q", i, requests[i], want)
		}
	}
}

func TestRouter_DoesNotProxyNonAIPublicRoutes(t *testing.T) {
	t.Parallel()
	var calls int
	upstream := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, _ *stdhttp.Request) {
		calls++
		w.WriteHeader(stdhttp.StatusOK)
	}))
	t.Cleanup(upstream.Close)

	handler := Router(&app.Dependencies{
		Config: &config.Config{
			Services: config.ServicesConfig{AIURL: upstream.URL},
		},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), stdhttp.MethodPost, "/api/query/compile", nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != stdhttp.StatusBadRequest {
		t.Fatalf("status: got %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
	if calls != 0 {
		t.Fatalf("query route should stay local, upstream calls=%d", calls)
	}
}

func TestRouter_AIProxyErrorUsesInternalEnvelope(t *testing.T) {
	t.Parallel()
	handler := Router(&app.Dependencies{
		Config: &config.Config{
			Services: config.ServicesConfig{AIURL: "http://127.0.0.1:1"},
		},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), stdhttp.MethodPost, "/api/ai/query", bytes.NewBufferString(`{}`))
	handler.ServeHTTP(rec, req)

	if rec.Code != stdhttp.StatusBadGateway {
		t.Fatalf("status: got %d, want 502; body=%s", rec.Code, rec.Body.String())
	}
	var env internalapi.Error
	if err := json.NewDecoder(rec.Body).Decode(&env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Code != internalapi.CodeUpstream {
		t.Fatalf("code: got %q, want %q", env.Code, internalapi.CodeUpstream)
	}
}
