package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/biqly/biqly/internal/auth"
)

// TestHandleCSRF pins the public CSRF bootstrap contract: the endpoint requires
// no authentication, returns 204 No Content with an empty body, and the CSRF
// middleware populates the X-CSRF-Token response header. The SPA relies on this
// to obtain a token without triggering a 401 on the auth-protected /me route.
func TestHandleCSRF(t *testing.T) {
	h := &AuthHandler{}
	r := chi.NewRouter()
	r.Use(auth.CSRF(false))
	r.Get("/csrf", h.handleCSRF)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/csrf", http.NoBody)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	require.Equal(t, http.StatusNoContent, rr.Code)
	assert.Empty(t, rr.Body.String())
	assert.NotEmpty(t, rr.Header().Get("X-CSRF-Token"))

	var csrfCookie *http.Cookie
	for _, c := range rr.Result().Cookies() {
		if c.Name == "csrf_token" {
			csrfCookie = c
		}
	}
	require.NotNil(t, csrfCookie)
	assert.Equal(t, csrfCookie.Value, rr.Header().Get("X-CSRF-Token"))
}
