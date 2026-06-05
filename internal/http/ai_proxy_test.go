package http

import (
	stdhttp "net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/config"
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

	cases := []proxyRouteCase{
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
	assertProxiedRoutes(t, handler, cases)

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
	handler, calls := routerWithCountingUpstream(t, func(s *config.ServicesConfig, url string) { s.AIURL = url })
	assertRouteStaysLocal(t, handler, stdhttp.MethodPost, "/api/query/compile", "", stdhttp.StatusBadRequest, calls, "query")
}

func TestRouter_AIProxyErrorUsesInternalEnvelope(t *testing.T) {
	t.Parallel()
	handler := Router(&app.Dependencies{
		Config: &config.Config{
			Services: config.ServicesConfig{AIURL: "http://127.0.0.1:1"},
		},
	})
	assertProxyErrorEnvelope(t, handler, stdhttp.MethodPost, "/api/ai/query", `{}`)
}
