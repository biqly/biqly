package http

import (
	"context"
	"github.com/bytedance/sonic"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"

	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/config"
)

func TestReadinessHandlerChecksConfiguredUpstreams(t *testing.T) {
	t.Parallel()
	upstream := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.URL.Path != "/health" {
			t.Fatalf("path: got %q, want /health", r.URL.Path)
		}
		w.WriteHeader(stdhttp.StatusOK)
		_, _ = w.Write(healthCheckBody)
	}))
	t.Cleanup(upstream.Close)

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), stdhttp.MethodGet, "/ready", nil)
	ReadinessHandler(&app.Dependencies{}, map[string]string{"catalog": upstream.URL})(rec, req)

	if rec.Code != stdhttp.StatusOK {
		t.Fatalf("status: got %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var body readinessResponse
	if err := sonic.ConfigStd.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Status != "ok" || body.Checks["catalog"].Status != "ok" || body.Checks["metadata_db"].Status != "ok" {
		t.Fatalf("unexpected readiness body: %+v", body)
	}
}

func TestReadinessHandlerReportsUpstreamFailure(t *testing.T) {
	t.Parallel()
	upstream := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, _ *stdhttp.Request) {
		w.WriteHeader(stdhttp.StatusServiceUnavailable)
	}))
	t.Cleanup(upstream.Close)

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), stdhttp.MethodGet, "/ready", nil)
	ReadinessHandler(&app.Dependencies{}, map[string]string{"catalog": upstream.URL})(rec, req)

	if rec.Code != stdhttp.StatusServiceUnavailable {
		t.Fatalf("status: got %d, want 503; body=%s", rec.Code, rec.Body.String())
	}
	var body readinessResponse
	if err := sonic.ConfigStd.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Status != "degraded" || body.Checks["catalog"].Status != "error" {
		t.Fatalf("unexpected readiness body: %+v", body)
	}
}

func TestRoutersExposeReadyEndpoint(t *testing.T) {
	t.Parallel()
	deps := &app.Dependencies{Config: &config.Config{}}
	cases := []struct {
		name    string
		handler stdhttp.Handler
	}{
		{"api", Router(deps)},
		{"catalog", CatalogRouter(deps)},
		{"query", QueryRouter(deps)},
		{"ai", AIRouter(deps)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rec := httptest.NewRecorder()
			req := httptest.NewRequestWithContext(context.Background(), stdhttp.MethodGet, "/ready", nil)
			tc.handler.ServeHTTP(rec, req)
			if rec.Code != stdhttp.StatusOK {
				t.Fatalf("status: got %d, want 200; body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}
