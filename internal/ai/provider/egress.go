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

var (
	airgappedMode atomic.Bool
	cloudMode     atomic.Bool
)

// SetAirgapped toggles the fail-closed egress policy. Set once at startup
// from BI_DEPLOYMENT_MODE=airgapped; when enabled, every provider HTTP call
// must target a private (in-cluster) host.
func SetAirgapped(enabled bool) {
	airgappedMode.Store(enabled)
}

// SetCloud marks the deployment as cloud (BI_DEPLOYMENT_MODE=cloud). In cloud
// mode admin-configured provider base URLs must be public, so CheckProviderBaseURL
// rejects private/loopback/link-local targets (SSRF guard). Private and
// airgapped deployments legitimately point providers at in-cluster hosts, so
// the guard does not apply there.
func SetCloud(enabled bool) {
	cloudMode.Store(enabled)
}

// CheckProviderBaseURL validates an admin-configured provider base URL against
// the deployment's egress posture. Airgapped: only private (in-cluster) hosts
// are allowed (same as CheckEgress). Cloud: private/loopback/link-local hosts
// are rejected so an operator cannot point a provider at the cloud metadata
// endpoint (169.254.169.254) or an in-cluster service — an authenticated-admin
// SSRF. Private mode: no restriction (self-hosted in-cluster LLMs are expected
// and the network is trusted). Note: this checks the literal host; it does not
// defend against a public hostname that later resolves to a private IP
// (DNS rebinding), which would require resolve-and-pin at request time.
func CheckProviderBaseURL(rawURL string) error {
	if airgappedMode.Load() {
		return CheckEgress(rawURL)
	}
	if !cloudMode.Load() {
		return nil
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("%w: unparseable URL", ErrEgressBlocked)
	}
	if isPrivateHost(u.Hostname()) {
		return fmt.Errorf("%w: private host not allowed for a provider in cloud mode: %s", ErrEgressBlocked, u.Hostname())
	}
	return nil
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
