package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestRealIPStripsPort verifies RealIP always exposes a bare IP downstream so
// handlers can persist r.RemoteAddr into Postgres inet columns. These cases use
// non-trusted peer addresses and send no forwarded headers, so the result is
// purely the port-stripped peer address (independent of trusted-proxy config).
func TestRealIPStripsPort(t *testing.T) {
	cases := []struct {
		name       string
		remoteAddr string
		want       string
	}{
		{"ipv6 loopback with port", "[::1]:52253", "::1"},
		{"ipv4 with port", "203.0.113.5:443", "203.0.113.5"},
		{"bare ipv4 unchanged", "203.0.113.5", "203.0.113.5"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got string
			h := RealIP(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				got = r.RemoteAddr
			}))
			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody)
			req.RemoteAddr = tc.remoteAddr
			h.ServeHTTP(httptest.NewRecorder(), req)
			if got != tc.want {
				t.Fatalf("RemoteAddr = %q, want %q", got, tc.want)
			}
		})
	}
}
