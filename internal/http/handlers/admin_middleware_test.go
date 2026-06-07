package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	bimw "github.com/biqly/biqly/internal/http/middleware"
)

func adminTestHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func TestAdminKeyMiddleware_UnconfiguredFailsClosed(t *testing.T) {
	h := AdminKeyMiddleware("")(adminTestHandler())
	r := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/ai/jobs/admin/stale", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when admin key unset, got %d", w.Code)
	}
}

func TestAdminKeyMiddleware_MissingCredentialRejected(t *testing.T) {
	h := AdminKeyMiddleware("s3cret")(adminTestHandler())
	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/ai/jobs/admin/stale", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestAdminKeyMiddleware_RawAuthHeaderRejected(t *testing.T) {
	// Earlier middleware silently accepted the raw value when no "Bearer "
	// prefix was present. Now the Bearer scheme is required.
	h := AdminKeyMiddleware("s3cret")(adminTestHandler())
	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/ai/jobs/admin/stale", nil)
	r.Header.Set("Authorization", "s3cret")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on raw (non-Bearer) Authorization header, got %d", w.Code)
	}
}

func TestAdminKeyMiddleware_AcceptsBearer(t *testing.T) {
	h := AdminKeyMiddleware("s3cret")(adminTestHandler())
	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/ai/jobs/admin/stale", nil)
	r.Header.Set("Authorization", "Bearer s3cret")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with Bearer token, got %d", w.Code)
	}
}

func TestAdminKeyMiddleware_AcceptsXAdminKey(t *testing.T) {
	h := AdminKeyMiddleware("s3cret")(adminTestHandler())
	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/ai/jobs/admin/stale", nil)
	r.Header.Set("X-Admin-Key", "s3cret")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with X-Admin-Key, got %d", w.Code)
	}
}

func TestAdminKeyMiddleware_StripsXAdminKey(t *testing.T) {
	h := AdminKeyMiddleware("s3cret")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Admin-Key"); got != "" {
			t.Fatalf("downstream X-Admin-Key = %q, want stripped", got)
		}
		w.WriteHeader(http.StatusOK)
	}))
	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/ai/jobs/admin/stale", nil)
	r.Header.Set("X-Admin-Key", "s3cret")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with X-Admin-Key, got %d", w.Code)
	}
}

func TestAdminKeyMiddleware_SuperAdminJWTPasses(t *testing.T) {
	// A verified super_admin identity from the outer JWT middleware is a
	// sufficient credential — no shared admin key needed, even when one is
	// configured and the Bearer token is the user's session JWT.
	h := AdminKeyMiddleware("s3cret")(adminTestHandler())
	ctx := context.WithValue(context.Background(), bimw.UserRolesKey, []string{bimw.RoleSuperAdmin})
	r := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/ai/providers", nil)
	r.Header.Set("Authorization", "Bearer some.session.jwt")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for super_admin JWT identity, got %d", w.Code)
	}
}

func TestAdminKeyMiddleware_NonAdminJWTRejected(t *testing.T) {
	h := AdminKeyMiddleware("s3cret")(adminTestHandler())
	ctx := context.WithValue(context.Background(), bimw.UserRolesKey, []string{"analyst"})
	r := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/ai/providers", nil)
	r.Header.Set("Authorization", "Bearer some.session.jwt")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for non-admin JWT identity, got %d", w.Code)
	}
}

func TestAdminKeyMiddleware_RejectsWrongKey(t *testing.T) {
	h := AdminKeyMiddleware("s3cret")(adminTestHandler())
	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/ai/jobs/admin/stale", nil)
	r.Header.Set("X-Admin-Key", "wrong")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with wrong key, got %d", w.Code)
	}
}
