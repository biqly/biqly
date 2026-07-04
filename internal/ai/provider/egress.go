package provider

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"sync/atomic"
)

// ErrEgressBlocked is returned when an outbound LLM/embedding call targets a
// non-private host while the deployment runs in airgapped mode.
var ErrEgressBlocked = errors.New("external egress blocked by airgapped deployment mode")

var airgappedMode atomic.Bool

// SetAirgapped toggles the fail-closed egress policy. Set once at startup
// from BI_DEPLOYMENT_MODE=airgapped; when enabled, every provider HTTP call
// must target a private (in-cluster) host.
func SetAirgapped(enabled bool) {
	airgappedMode.Store(enabled)
}

// CheckEgress validates a provider endpoint URL against the egress policy.
// It is a no-op unless airgapped mode is enabled.
func CheckEgress(rawURL string) error {
	if !airgappedMode.Load() {
		return nil
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("%w: unparseable URL", ErrEgressBlocked)
	}
	host := u.Hostname()
	if isPrivateHost(host) {
		return nil
	}
	return fmt.Errorf("%w: %s", ErrEgressBlocked, host)
}

// isPrivateHost accepts loopback/private/link-local IPs, localhost,
// single-label hostnames (Kubernetes service names), and common in-cluster
// DNS suffixes. Everything else is treated as external.
func isPrivateHost(host string) bool {
	if host == "" {
		return false
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast()
	}
	h := strings.ToLower(strings.TrimSuffix(host, "."))
	if h == "localhost" {
		return true
	}
	if !strings.Contains(h, ".") {
		return true
	}
	for _, suffix := range []string{".svc", ".svc.cluster.local", ".cluster.local", ".internal", ".local", ".localhost"} {
		if strings.HasSuffix(h, suffix) {
			return true
		}
	}
	return false
}
