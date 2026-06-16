package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/biqly/biqly/internal/security"
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

// TestRealIPFromTrustedProxy verifies that when the immediate peer is trusted,
// the middleware walks X-Forwarded-For right-to-left skipping trusted proxies.
func TestRealIPFromTrustedProxy(t *testing.T) {
	// Reset trusted proxies to defaults for this test.
	security.ResetTrustedProxies()

	cases := []struct {
		name       string
		remoteAddr string
		xff        string
		xrip       string
		want       string
	}{
		{
			name:       "xff rightmost untrusted wins",
			remoteAddr: "10.0.0.1:443",
			xff:        "203.0.113.5, 10.0.0.1",
			want:       "203.0.113.5",
		},
		{
			name:       "xff client behind two trusted proxies",
			remoteAddr: "192.168.1.1:443",
			xff:        "203.0.113.5, 10.0.0.1, 192.168.1.1",
			want:       "203.0.113.5",
		},
		{
			name:       "xff all trusted returns leftmost",
			remoteAddr: "10.0.0.1:443",
			xff:        "192.168.1.1, 10.0.0.1",
			want:       "192.168.1.1",
		},
		{
			name:       "xff single entry",
			remoteAddr: "10.0.0.1:443",
			xff:        "203.0.113.5",
			want:       "203.0.113.5",
		},
		{
			name:       "xff empty string",
			remoteAddr: "10.0.0.1:443",
			xff:        "",
			want:       "10.0.0.1",
		},
		{
			name:       "xff spoof attempt ignored",
			remoteAddr: "10.0.0.1:443",
			xff:        "1.2.3.4, 203.0.113.5, 10.0.0.1",
			want:       "203.0.113.5", // 1.2.3.4 is spoofed; 203.0.113.5 is the first untrusted
		},
		{
			name:       "xff absent, x-real-ip falls back",
			remoteAddr: "10.0.0.1:443",
			xrip:       "203.0.113.5",
			want:       "203.0.113.5",
		},
		{
			name:       "xff takes priority over x-real-ip",
			remoteAddr: "10.0.0.1:443",
			xff:        "198.51.100.1, 10.0.0.1",
			xrip:       "203.0.113.5", // should be ignored — xff wins
			want:       "198.51.100.1",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got string
			h := RealIP(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				got = r.RemoteAddr
			}))
			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody)
			req.RemoteAddr = tc.remoteAddr
			if tc.xff != "" {
				req.Header.Set("X-Forwarded-For", tc.xff)
			}
			if tc.xrip != "" {
				req.Header.Set("X-Real-IP", tc.xrip)
			}
			h.ServeHTTP(httptest.NewRecorder(), req)
			if got != tc.want {
				t.Fatalf("RemoteAddr = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestRealIPIgnoresForwardedHeadersFromUntrustedPeer verifies clients cannot
// spoof their IP by sending X-Forwarded-For or X-Real-IP when the immediate peer
// is not a trusted proxy.
func TestRealIPIgnoresForwardedHeadersFromUntrustedPeer(t *testing.T) {
	security.ResetTrustedProxies()

	cases := []struct {
		name       string
		remoteAddr string
		xff        string
		xrip       string
		want       string
	}{
		{
			name:       "xff ignored from public peer",
			remoteAddr: "203.0.113.9:443",
			xff:        "198.51.100.1, 10.0.0.1",
			want:       "203.0.113.9",
		},
		{
			name:       "x-real-ip ignored from public peer",
			remoteAddr: "203.0.113.9:443",
			xrip:       "198.51.100.1",
			want:       "203.0.113.9",
		},
		{
			name:       "both headers ignored from public peer",
			remoteAddr: "198.51.100.2:52253",
			xff:        "1.2.3.4",
			xrip:       "5.6.7.8",
			want:       "198.51.100.2",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got string
			h := RealIP(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				got = r.RemoteAddr
			}))
			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody)
			req.RemoteAddr = tc.remoteAddr
			if tc.xff != "" {
				req.Header.Set("X-Forwarded-For", tc.xff)
			}
			if tc.xrip != "" {
				req.Header.Set("X-Real-IP", tc.xrip)
			}
			h.ServeHTTP(httptest.NewRecorder(), req)
			if got != tc.want {
				t.Fatalf("RemoteAddr = %q, want %q", got, tc.want)
			}
		})
	}
}
