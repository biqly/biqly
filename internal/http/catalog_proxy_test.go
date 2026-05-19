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

	cases := []struct {
		method string
		path   string
	}{
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

func TestRouter_DoesNotProxyNonCatalogPublicRoutes(t *testing.T) {
	t.Parallel()
	var calls int
	upstream := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, _ *stdhttp.Request) {
		calls++
		w.WriteHeader(stdhttp.StatusOK)
	}))
	t.Cleanup(upstream.Close)

	handler := Router(&app.Dependencies{
		Config: &config.Config{
			Services: config.ServicesConfig{CatalogURL: upstream.URL},
		},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), stdhttp.MethodPost, "/api/query/compile", bytes.NewBufferString(`{`))
	handler.ServeHTTP(rec, req)

	if rec.Code != stdhttp.StatusBadRequest {
		t.Fatalf("status: got %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
	if calls != 0 {
		t.Fatalf("query route should stay local, upstream calls=%d", calls)
	}
}

func TestRouter_CatalogProxyErrorUsesInternalEnvelope(t *testing.T) {
	t.Parallel()
	handler := Router(&app.Dependencies{
		Config: &config.Config{
			Services: config.ServicesConfig{CatalogURL: "http://127.0.0.1:1"},
		},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), stdhttp.MethodGet, "/api/metadata/tables/search?datasource_id=ds&q=orders", nil)
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
