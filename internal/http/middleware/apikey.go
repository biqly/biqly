package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// APIKeyAuth returns a middleware that requires every request to present a
// shared secret in either `X-API-Key: <key>` or `Authorization: Bearer <key>`.
//
// The comparison is constant-time to prevent timing attacks. When `key` is
// empty the middleware is a no-op so local development without an API key
// still works — callers are expected to log a warning in that case.
//
// Optional `bypassPaths` lets callers exempt health/metrics endpoints by
// exact-prefix match (e.g. "/health"); they are checked against r.URL.Path.
func APIKeyAuth(key string, bypassPaths ...string) func(http.Handler) http.Handler {
	expected := []byte(strings.TrimSpace(key))
	return func(next http.Handler) http.Handler {
		if len(expected) == 0 {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for _, p := range bypassPaths {
				if p != "" && strings.HasPrefix(r.URL.Path, p) {
					next.ServeHTTP(w, r)
					return
				}
			}
			provided := extractAPIKey(r)
			if len(provided) == 0 || subtle.ConstantTimeCompare(provided, expected) != 1 {
				w.Header().Set("WWW-Authenticate", `Bearer realm="biqly"`)
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// extractAPIKey returns the raw key bytes from either header. The result has
// length zero when no credential is present.
func extractAPIKey(r *http.Request) []byte {
	if v := strings.TrimSpace(r.Header.Get("X-API-Key")); v != "" {
		return []byte(v)
	}
	if v := strings.TrimSpace(r.Header.Get("Authorization")); v != "" {
		const prefix = "Bearer "
		if len(v) > len(prefix) && strings.EqualFold(v[:len(prefix)], prefix) {
			return []byte(strings.TrimSpace(v[len(prefix):]))
		}
	}
	return nil
}
