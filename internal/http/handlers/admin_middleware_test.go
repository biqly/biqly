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

// permStubServer fakes the auth service's permission check endpoint.
func permStubServer(t *testing.T, allowed bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/auth/check-permission" {
			t.Fatalf("unexpected auth call: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		if allowed {
			_, _ = w.Write([]byte(`{"allowed":true}`))
		} else {
			_, _ = w.Write([]byte(`{"allowed":false}`))
		}
	}))
}

func rbacUserContext() context.Context {
	ctx := context.WithValue(context.Background(), bimw.UserIDKey, "u-1")
	return context.WithValue(ctx, bimw.UserRolesKey, []string{"admin"})
}

func TestAdminAccessMiddleware_RBACPermissionGrantsAccess(t *testing.T) {
	srv := permStubServer(t, true)
	defer srv.Close()
	client := bimw.NewAuthClient(srv.URL, "tok")

	h := AdminAccessMiddleware("s3cret", client, "ai:settings")(adminTestHandler())
	r := httptest.NewRequestWithContext(rbacUserContext(), http.MethodGet, "/api/ai/admin/config", nil)
	r.Header.Set("Authorization", "Bearer some.session.jwt")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for JWT with ai:settings permission, got %d (%s)", w.Code, w.Body.String())
	}
}

func TestAdminAccessMiddleware_RBACPermissionDenied(t *testing.T) {
	srv := permStubServer(t, false)
	defer srv.Close()
	client := bimw.NewAuthClient(srv.URL, "tok")

	h := AdminAccessMiddleware("s3cret", client, "ai:settings")(adminTestHandler())
	r := httptest.NewRequestWithContext(rbacUserContext(), http.MethodGet, "/api/ai/admin/config", nil)
	r.Header.Set("Authorization", "Bearer some.session.jwt")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for JWT without ai:settings permission, got %d", w.Code)
	}
}

func TestAdminAccessMiddleware_AdminKeyStillPassesBeforeRBAC(t *testing.T) {
	// A valid shared key must not require an auth-service round trip.
	h := AdminAccessMiddleware("s3cret", bimw.NewAuthClient("http://127.0.0.1:1", "tok"), "ai:settings")(adminTestHandler())
	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/ai/admin/config", nil)
	r.Header.Set("X-Admin-Key", "s3cret")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with valid admin key, got %d", w.Code)
	}
}

func TestAdminAccessMiddleware_AnonymousStillRejected(t *testing.T) {
	srv := permStubServer(t, true)
	defer srv.Close()
	client := bimw.NewAuthClient(srv.URL, "tok")

	h := AdminAccessMiddleware("s3cret", client, "ai:settings")(adminTestHandler())
	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/ai/admin/config", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for anonymous caller, got %d", w.Code)
	}
}
