package middleware

import (
	"net"
	"net/http"

	"github.com/biqly/biqly/internal/security"
)

func getRealIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if ip := security.ClientIPFromXFF(xff); ip != "" {
			return ip
		}
	}
	if rip := r.Header.Get("X-Real-IP"); rip != "" {
		return rip
	}
	return ""
}

// RealIP normalizes r.RemoteAddr to a bare client IP for all downstream
// handlers. Behind a trusted proxy it uses the X-Forwarded-For / X-Real-IP
// client IP; otherwise it strips the port from the peer address. Always emitting
// a port-less IP matters because handlers persist r.RemoteAddr into Postgres
// inet columns (sessions, audit_events, account_state), which reject "ip:port"
// (e.g. a direct connection's "[::1]:52253").
func RealIP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			host = r.RemoteAddr
		}
		if security.IsTrustedProxy(host) {
			if rip := getRealIP(r); rip != "" {
				host = rip
			}
		}
		r.RemoteAddr = host
		next.ServeHTTP(w, r)
	})
}
