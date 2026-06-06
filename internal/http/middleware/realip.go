package middleware

import (
	"net"
	"net/http"
	"strings"

	"github.com/biqly/biqly/internal/security"
)

func getRealIP(r *http.Request) string {
	if rip := r.Header.Get("X-Real-IP"); rip != "" {
		return rip
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if before, _, ok := strings.Cut(xff, ","); ok {
			return strings.TrimSpace(before)
		}
		return strings.TrimSpace(xff)
	}
	return ""
}

// RealIP is a middleware that sets r.RemoteAddr to the client's real IP
// from X-Forwarded-For or X-Real-IP headers if request comes from a trusted proxy.
func RealIP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			host = r.RemoteAddr
		}
		if security.IsTrustedProxy(host) {
			if rip := getRealIP(r); rip != "" {
				r.RemoteAddr = rip
			}
		}
		next.ServeHTTP(w, r)
	})
}
