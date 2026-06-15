package security

import (
	"net"
	"os"
	"strings"
	"sync"
)

var (
	trustedProxies []net.IPNet
	defaultCIDRs   = []string{
		"127.0.0.1/8",    // IPv4 loopback
		"::1/128",        // IPv6 loopback
		"10.0.0.0/8",     // RFC 1918 private
		"172.16.0.0/12",  // RFC 1918 private
		"192.168.0.0/16", // RFC 1918 private
		"fc00::/7",       // IPv6 Unique Local Address
	}
	parseOnce sync.Once
)

func initTrustedProxies() {
	env := os.Getenv("BI_TRUSTED_PROXIES")
	var cidrStrings []string
	if env != "" {
		cidrStrings = strings.Split(env, ",")
	} else {
		cidrStrings = defaultCIDRs
	}

	for _, s := range cidrStrings {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		// Try parsing as CIDR
		_, ipNet, err := net.ParseCIDR(s)
		if err == nil {
			trustedProxies = append(trustedProxies, *ipNet)
			continue
		}
		// Try parsing as raw IP
		ip := net.ParseIP(s)
		if ip != nil {
			var mask net.IPMask
			if ip.To4() != nil {
				mask = net.CIDRMask(32, 32)
			} else {
				mask = net.CIDRMask(128, 128)
			}
			trustedProxies = append(trustedProxies, net.IPNet{IP: ip, Mask: mask})
		}
	}
}

// IsTrustedProxy returns true if the given IP address is a trusted proxy.
func IsTrustedProxy(ipStr string) bool {
	parseOnce.Do(initTrustedProxies)
	ip := net.ParseIP(strings.TrimSpace(ipStr))
	if ip == nil {
		return false
	}
	for _, ipNet := range trustedProxies {
		if ipNet.Contains(ip) {
			return true
		}
	}
	return false
}

// ResetTrustedProxies clears the cached trusted proxy list so the next call
// re-parses BI_TRUSTED_PROXIES (or defaults). Used in tests to isolate cases.
func ResetTrustedProxies() {
	parseOnce = sync.Once{}
	trustedProxies = nil
}

// ClientIPFromXFF parses an X-Forwarded-For header value and returns the
// first untrusted client IP by walking the chain from right to left.
// Trusted proxy IPs at the end of the chain (closest to the server) are
// skipped. If every IP in the chain is trusted, the leftmost (client-
// originating) IP is returned as a best-effort fallback.
func ClientIPFromXFF(xff string) string {
	parts := strings.Split(xff, ",")
	// Walk right-to-left: the rightmost entry is the most recent hop,
	// which should be the immediate proxy we trust. Skip trusted proxies
	// and return the first untrusted IP (the actual client).
	for i := len(parts) - 1; i >= 0; i-- {
		ip := strings.TrimSpace(parts[i])
		if !IsTrustedProxy(ip) {
			return ip
		}
	}
	// All IPs are trusted proxies — return the leftmost (closest to client).
	if len(parts) > 0 {
		return strings.TrimSpace(parts[0])
	}
	return ""
}
