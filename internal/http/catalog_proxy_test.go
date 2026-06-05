package http

import (
	stdhttp "net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/config"
)

func TestRouter_ProxiesCatalogOwnedPublicRoutes(t *testing.T) {
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
			Services: config.ServicesConfig{CatalogURL: upstream.URL},
		},
	})

	cases := []proxyRouteCase{
		{stdhttp.MethodGet, "/api/datasources"},
		{stdhttp.MethodPost, "/api/datasources"},
		{stdhttp.MethodPost, "/api/datasources/test-connection"},
		{stdhttp.MethodPost, "/api/datasources/ds_1/sync-metadata"},
		{stdhttp.MethodGet, "/api/datasources/ds_1/tables?schema=public"},
		{stdhttp.MethodGet, "/api/datasources/ds_1/columns?schema=public&table=orders"},
		{stdhttp.MethodGet, "/api/metadata/tables/search?datasource_id=ds_1&q=orders"},
		{stdhttp.MethodPatch, "/api/metadata/tables/table_1"},
		{stdhttp.MethodPut, "/api/metadata/tables/table_1/translations"},
		{stdhttp.MethodGet, "/api/semantic/models?datasource_id=ds_1"},
		{stdhttp.MethodPost, "/api/semantic/models/generate"},
		{stdhttp.MethodGet, "/api/semantic/models/model_1?include_inactive=true"},
		{stdhttp.MethodPost, "/api/semantic/models/model_1/publish"},
		{stdhttp.MethodGet, "/api/semantic/models/model_1/suggested-joins"},
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

func TestRouter_DoesNotProxyNonCatalogPublicRoutes(t *testing.T) {
	t.Parallel()
	handler, calls := routerWithCountingUpstream(t, func(s *config.ServicesConfig, url string) { s.CatalogURL = url })
	assertRouteStaysLocal(t, handler, stdhttp.MethodPost, "/api/query/compile", "{", stdhttp.StatusBadRequest, calls, "query")
}

func TestRouter_CatalogProxyErrorUsesInternalEnvelope(t *testing.T) {
	t.Parallel()
	handler := Router(&app.Dependencies{
		Config: &config.Config{
			Services: config.ServicesConfig{CatalogURL: "http://127.0.0.1:1"},
		},
	})
	assertProxyErrorEnvelope(t, handler, stdhttp.MethodGet, "/api/metadata/tables/search?datasource_id=ds&q=orders", "")
}
