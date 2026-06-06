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
