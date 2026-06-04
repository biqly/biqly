package middleware

import (
	"net/http"
	"strings"
)

// RealIP is a middleware that sets r.RemoteAddr to the client's real IP
// from X-Forwarded-For or X-Real-IP headers.
// This is custom-implemented to replace the deprecated go-chi RealIP middleware
// and avoid deprecation warnings while maintaining compatibility with codebase RemoteAddr lookups.
func RealIP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if rip := r.Header.Get("X-Real-IP"); rip != "" {
			r.RemoteAddr = rip
		} else if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			if before, _, ok := strings.Cut(xff, ","); ok {
				r.RemoteAddr = strings.TrimSpace(before)
			} else {
				r.RemoteAddr = strings.TrimSpace(xff)
			}
		}
		next.ServeHTTP(w, r)
	})
}
