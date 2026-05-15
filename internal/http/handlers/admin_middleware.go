package handlers

import (
	"net/http"
	"strings"
)

// AdminKeyMiddleware rejects requests when BI_ADMIN_API_KEY is unset or the key does not match.
func AdminKeyMiddleware(adminKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if adminKey == "" {
				writeError(w, http.StatusForbidden, "eval endpoints require BI_ADMIN_API_KEY to be configured")
				return
			}
			token := adminKeyFromRequest(r)
			if token == "" || token != adminKey {
				writeError(w, http.StatusUnauthorized, "invalid or missing admin API key")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func adminKeyFromRequest(r *http.Request) string {
	if k := strings.TrimSpace(r.Header.Get("X-Admin-Key")); k != "" {
		return k
	}
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		return strings.TrimSpace(authHeader[7:])
	}
	return authHeader
}
