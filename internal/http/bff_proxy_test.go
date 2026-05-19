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
)

func TestRouter_BFFRoutesFrontendTrafficToAllServices(t *testing.T) {
	t.Parallel()

	catalog := newTraceUpstream(t, "catalog")
	query := newTraceUpstream(t, "query")
	ai := newTraceUpstream(t, "ai")
	t.Cleanup(catalog.Close)
	t.Cleanup(query.Close)
	t.Cleanup(ai.Close)

	handler := Router(&app.Dependencies{
		Config: &config.Config{
			Services: config.ServicesConfig{
				CatalogURL: catalog.URL,
				QueryURL:   query.URL,
				AIURL:      ai.URL,
			},
		},
	})

	cases := []struct {
		method string
		path   string
		want   string
	}{
		{stdhttp.MethodGet, "/api/datasources", "catalog"},
		{stdhttp.MethodGet, "/api/metadata/tables/search?datasource_id=ds_1&q=orders", "catalog"},
		{stdhttp.MethodGet, "/api/semantic/models?datasource_id=ds_1", "catalog"},
		{stdhttp.MethodPost, "/api/query/compile", "query"},
		{stdhttp.MethodGet, "/api/query/history", "query"},
		{stdhttp.MethodPost, "/api/ai/query", "ai"},
		{stdhttp.MethodGet, "/api/ai/settings", "ai"},
	}

	for _, tc := range cases {
		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(context.Background(), tc.method, tc.path, bytes.NewBufferString(`{}`))
		req.Header.Set("traceparent", sampleTraceparent)
		handler.ServeHTTP(rec, req)
		if rec.Code != stdhttp.StatusOK {
			t.Fatalf("%s %s status: got %d, want 200; body=%s", tc.method, tc.path, rec.Code, rec.Body.String())
		}
		var body struct {
			Service string `json:"service"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
			t.Fatalf("decode %s %s: %v", tc.method, tc.path, err)
		}
		if body.Service != tc.want {
			t.Fatalf("%s %s service: got %q, want %q", tc.method, tc.path, body.Service, tc.want)
		}
	}

	catalog.assertRequests(t, []string{
		"GET /api/datasources",
		"GET /api/metadata/tables/search?datasource_id=ds_1&q=orders",
		"GET /api/semantic/models?datasource_id=ds_1",
	})
	query.assertRequests(t, []string{
		"POST /api/query/compile",
		"GET /api/query/history",
	})
	ai.assertRequests(t, []string{
		"POST /api/ai/query",
		"GET /api/ai/settings",
	})
}

type traceUpstream struct {
	*httptest.Server
	service  string
	mu       sync.Mutex
	requests []string
}

func newTraceUpstream(t *testing.T, service string) *traceUpstream {
	t.Helper()
	up := &traceUpstream{service: service}
	up.Server = httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		up.mu.Lock()
		up.requests = append(up.requests, r.Method+" "+r.URL.RequestURI())
		up.mu.Unlock()
		if r.Header.Get("X-Forwarded-Host") == "" {
			t.Errorf("%s: X-Forwarded-Host should be set", service)
		}
		if r.Header.Get("X-Request-ID") == "" {
			t.Errorf("%s: X-Request-ID should be propagated", service)
		}
		if got := r.Header.Get("traceparent"); got != sampleTraceparent {
			t.Errorf("%s: traceparent got %q, want %q", service, got, sampleTraceparent)
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]string{"service": service}); err != nil {
			t.Fatalf("%s: encode response: %v", service, err)
		}
	}))
	return up
}

const sampleTraceparent = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"

func (u *traceUpstream) assertRequests(t *testing.T, want []string) {
	t.Helper()
	u.mu.Lock()
	defer u.mu.Unlock()
	if len(u.requests) != len(want) {
		t.Fatalf("%s requests: got %v, want %v", u.service, u.requests, want)
	}
	for i := range want {
		if u.requests[i] != want[i] {
			t.Fatalf("%s request %d: got %q, want %q", u.service, i, u.requests[i], want[i])
		}
	}
}
