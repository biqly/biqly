package handlers

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// AdminKeyMiddleware rejects requests when BI_ADMIN_API_KEY is unset or the
// key does not match. Comparison is constant-time. The Authorization header
// MUST use the Bearer scheme — an earlier version accepted raw tokens.
func AdminKeyMiddleware(adminKey string) func(http.Handler) http.Handler {
	expected := []byte(strings.TrimSpace(adminKey))
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if len(expected) == 0 {
				writeError(w, http.StatusForbidden, "admin endpoints require BI_ADMIN_API_KEY to be configured")
				return
			}
			token := adminKeyFromRequest(r)
			if len(token) == 0 || subtle.ConstantTimeCompare(token, expected) != 1 {
				writeError(w, http.StatusUnauthorized, "invalid or missing admin API key")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func adminKeyFromRequest(r *http.Request) []byte {
	if k := strings.TrimSpace(r.Header.Get("X-Admin-Key")); k != "" {
		return []byte(k)
	}
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	const prefix = "bearer "
	if len(authHeader) > len(prefix) && strings.EqualFold(authHeader[:len(prefix)], prefix) {
		return []byte(strings.TrimSpace(authHeader[len(prefix):]))
	}
	return nil
}
