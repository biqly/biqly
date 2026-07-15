package http

import (
	"context"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"

	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/config"
	"github.com/biqly/biqly/internal/dashboard"
	"github.com/biqly/biqly/internal/testutil"
)

// publicRouteTestDeps builds a minimal *app.Dependencies whose only live
// component is a real PublicResolver, plus an API key so /api/* auth is
// exercised. All service URLs are empty, so the catalog + public routes are
// served in-process (not proxied).
func publicRouteTestDeps(t *testing.T) *app.Dependencies {
	t.Helper()
	db := testutil.OpenMetadataDB(t)
	return &app.Dependencies{
		Config: &config.Config{
			// Auth disabled → APIKeyAuth path; a non-empty key means a request
			// with no key is rejected with 401 (proves auth is still enforced).
			Security:    config.SecurityConfig{APIKey: "test-api-key"},
			PublicShare: config.PublicShareConfig{RateLimitPerMinute: 60},
		},
		PublicResolver: dashboard.NewPublicResolver(db),
	}
}

// TestCatalogRouter_PublicRouteIsAnonymousAuthedRoutesStillGuarded boots the
// standalone catalog router and proves the /api/public route is reachable
// without auth while an existing /api route still requires it.
func TestCatalogRouter_PublicRouteIsAnonymousAuthedRoutesStillGuarded(t *testing.T) {
	deps := publicRouteTestDeps(t)
	handler := CatalogRouter(deps)

	t.Run("public dashboard route needs no auth (reaches handler, 404 for bad token)", func(t *testing.T) {
		req := httptest.NewRequestWithContext(context.Background(), stdhttp.MethodGet, "/api/public/dashboards/bogus-token", stdhttp.NoBody)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		// 404 (not 401) proves the request passed the router without auth and
		// reached the handler, which resolved the bad token to ErrShareNotFound.
		if rec.Code != stdhttp.StatusNotFound {
			t.Fatalf("public route status = %d, want %d (body: %s)", rec.Code, stdhttp.StatusNotFound, rec.Body.String())
		}
		if got := rec.Header().Get("Content-Security-Policy"); got != "frame-ancestors *" {
			t.Fatalf("public embed CSP = %q, want %q", got, "frame-ancestors *")
		}
		if got := rec.Header().Get("X-Frame-Options"); got != "" {
			t.Fatalf("public embed X-Frame-Options = %q, want empty", got)
		}
	})

	t.Run("existing authed dashboards route still returns 401 without auth", func(t *testing.T) {
		req := httptest.NewRequestWithContext(context.Background(), stdhttp.MethodGet, "/api/dashboards", stdhttp.NoBody)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != stdhttp.StatusUnauthorized {
			t.Fatalf("authed route status = %d, want %d — auth may have been stripped!", rec.Code, stdhttp.StatusUnauthorized)
		}
	})
}

// TestMonolithRouter_PublicRouteIsAnonymousAuthedRoutesStillGuarded boots the
// monolith router (in-process, no service URLs) and proves the same two
// properties there.
func TestMonolithRouter_PublicRouteIsAnonymousAuthedRoutesStillGuarded(t *testing.T) {
	deps := publicRouteTestDeps(t)
	handler := Router(deps)

	t.Run("public dashboard route needs no auth (reaches handler, 404 for bad token)", func(t *testing.T) {
		req := httptest.NewRequestWithContext(context.Background(), stdhttp.MethodGet, "/api/public/dashboards/bogus-token", stdhttp.NoBody)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != stdhttp.StatusNotFound {
			t.Fatalf("public route status = %d, want %d (body: %s)", rec.Code, stdhttp.StatusNotFound, rec.Body.String())
		}
		if got := rec.Header().Get("Content-Security-Policy"); got != "frame-ancestors *" {
			t.Fatalf("public embed CSP = %q, want %q", got, "frame-ancestors *")
		}
	})

	t.Run("public widget-query route needs no auth (reaches handler, 404 for bad token)", func(t *testing.T) {
		req := httptest.NewRequestWithContext(context.Background(), stdhttp.MethodPost, "/api/public/widget-query/bogus-token/w1", stdhttp.NoBody)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != stdhttp.StatusNotFound {
			t.Fatalf("public widget-query status = %d, want %d (body: %s)", rec.Code, stdhttp.StatusNotFound, rec.Body.String())
		}
		if got := rec.Header().Get("Content-Security-Policy"); got != "frame-ancestors *" {
			t.Fatalf("public embed CSP = %q, want %q", got, "frame-ancestors *")
		}
	})

	t.Run("existing authed dashboards route still returns 401 without auth", func(t *testing.T) {
		req := httptest.NewRequestWithContext(context.Background(), stdhttp.MethodGet, "/api/dashboards", stdhttp.NoBody)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != stdhttp.StatusUnauthorized {
			t.Fatalf("authed route status = %d, want %d — auth may have been stripped!", rec.Code, stdhttp.StatusUnauthorized)
		}
	})

	t.Run("existing authed query run route still returns 401 without auth", func(t *testing.T) {
		req := httptest.NewRequestWithContext(context.Background(), stdhttp.MethodPost, "/api/query/run", stdhttp.NoBody)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != stdhttp.StatusUnauthorized {
			t.Fatalf("authed query route status = %d, want %d — auth may have been stripped!", rec.Code, stdhttp.StatusUnauthorized)
		}
	})
}

// TestQueryRouter_PublicRouteIsAnonymousAuthedRoutesStillGuarded boots the
// standalone query router and proves the /api/public/widget-query route is
// reachable without auth while an existing /api/query route still requires it.
func TestQueryRouter_PublicRouteIsAnonymousAuthedRoutesStillGuarded(t *testing.T) {
	deps := publicRouteTestDeps(t)
	handler := QueryRouter(deps)

	t.Run("public widget-query route needs no auth (reaches handler, 404 for bad token)", func(t *testing.T) {
		req := httptest.NewRequestWithContext(context.Background(), stdhttp.MethodPost, "/api/public/widget-query/bogus-token/w1", stdhttp.NoBody)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		// 404 (not 401) proves the request passed the router without auth and
		// reached the handler, which resolved the bad token to ErrShareNotFound.
		if rec.Code != stdhttp.StatusNotFound {
			t.Fatalf("public widget-query status = %d, want %d (body: %s)", rec.Code, stdhttp.StatusNotFound, rec.Body.String())
		}
		if got := rec.Header().Get("Content-Security-Policy"); got != "frame-ancestors *" {
			t.Fatalf("public embed CSP = %q, want %q", got, "frame-ancestors *")
		}
		if got := rec.Header().Get("X-Frame-Options"); got != "" {
			t.Fatalf("public embed X-Frame-Options = %q, want empty", got)
		}
	})

	t.Run("existing authed query run route still returns 401 without auth", func(t *testing.T) {
		req := httptest.NewRequestWithContext(context.Background(), stdhttp.MethodPost, "/api/query/run", stdhttp.NoBody)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != stdhttp.StatusUnauthorized {
			t.Fatalf("authed query route status = %d, want %d — auth may have been stripped!", rec.Code, stdhttp.StatusUnauthorized)
		}
	})
}
