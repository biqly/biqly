package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func TestAPIKeyAuth_NoKeyMeansNoOp(t *testing.T) {
	h := APIKeyAuth("")(okHandler())
	req := httptest.NewRequest(http.MethodGet, "/api/x", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 when no key configured, got %d", w.Code)
	}
}

func TestAPIKeyAuth_MissingCredentialRejected(t *testing.T) {
	h := APIKeyAuth("s3cret")(okHandler())
	req := httptest.NewRequest(http.MethodGet, "/api/x", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestAPIKeyAuth_AcceptsXAPIKey(t *testing.T) {
	h := APIKeyAuth("s3cret")(okHandler())
	req := httptest.NewRequest(http.MethodGet, "/api/x", nil)
	req.Header.Set("X-API-Key", "s3cret")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with valid X-API-Key, got %d", w.Code)
	}
}

func TestAPIKeyAuth_AcceptsBearer(t *testing.T) {
	h := APIKeyAuth("s3cret")(okHandler())
	req := httptest.NewRequest(http.MethodGet, "/api/x", nil)
	req.Header.Set("Authorization", "Bearer s3cret")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with valid Bearer token, got %d", w.Code)
	}
}

func TestAPIKeyAuth_RejectsWrongKey(t *testing.T) {
	h := APIKeyAuth("s3cret")(okHandler())
	req := httptest.NewRequest(http.MethodGet, "/api/x", nil)
	req.Header.Set("X-API-Key", "nope")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with wrong key, got %d", w.Code)
	}
}

func TestAPIKeyAuth_BypassPathSkipsAuth(t *testing.T) {
	h := APIKeyAuth("s3cret", "/health", "/ready")(okHandler())
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 on bypassed path, got %d", w.Code)
	}
}
