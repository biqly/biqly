package security

import (
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResetTrustedProxies(t *testing.T) {
	// Set custom env, init proxies
	t.Setenv("BI_TRUSTED_PROXIES", "8.8.8.8")
	parseOnce = sync.Once{}
	trustedProxies = nil

	// Trigger init
	assert.True(t, IsTrustedProxy("8.8.8.8"))
	assert.False(t, IsTrustedProxy("127.0.0.1"))

	// Reset
	ResetTrustedProxies()

	// Now defaults should be used again (env still set by t.Setenv but reset re-reads it)
	// Actually since t.Setenv stays set, after reset it re-reads "8.8.8.8"
	// Test that reset clears state so next call re-initializes
	assert.True(t, IsTrustedProxy("8.8.8.8"))

	// Verify internal state was reset
	ResetTrustedProxies()
	_ = os.Unsetenv("BI_TRUSTED_PROXIES")
	assert.True(t, IsTrustedProxy("127.0.0.1")) // defaults restored
}

func TestResetTrustedProxies_DetectsEnvChange(t *testing.T) {
	ResetTrustedProxies()
	trustedProxies = nil

	t.Setenv("BI_TRUSTED_PROXIES", "10.0.0.1")
	assert.True(t, IsTrustedProxy("10.0.0.1"))
	assert.False(t, IsTrustedProxy("8.8.8.8"))

	// Change env and reset
	ResetTrustedProxies()
	t.Setenv("BI_TRUSTED_PROXIES", "8.8.8.8")
	assert.True(t, IsTrustedProxy("8.8.8.8"))
	assert.False(t, IsTrustedProxy("10.0.0.1"))
}

func TestIsTrustedProxy_Default(t *testing.T) {
	parseOnce = sync.Once{}
	trustedProxies = nil

	_ = os.Unsetenv("BI_TRUSTED_PROXIES")

	// Local loopback and RFC 1918 should be trusted by default
	assert.True(t, IsTrustedProxy("127.0.0.1"))
	assert.True(t, IsTrustedProxy("10.0.0.1"))
	assert.True(t, IsTrustedProxy("192.168.1.50"))
	assert.True(t, IsTrustedProxy("::1"))

	// Public IPs should not be trusted
	assert.False(t, IsTrustedProxy("8.8.8.8"))
	assert.False(t, IsTrustedProxy("203.0.113.1"))
}

func TestIsTrustedProxy_EnvConfigured(t *testing.T) {
	parseOnce = sync.Once{}
	trustedProxies = nil

	t.Setenv("BI_TRUSTED_PROXIES", "8.8.8.8,192.0.2.0/24")

	assert.True(t, IsTrustedProxy("8.8.8.8"))
	assert.True(t, IsTrustedProxy("192.0.2.55"))
	assert.False(t, IsTrustedProxy("127.0.0.1")) // not in custom env, so no longer trusted
}

func TestClientIPFromXFF_UntrustedClient(t *testing.T) {
	parseOnce = sync.Once{}
	trustedProxies = nil
	_ = os.Unsetenv("BI_TRUSTED_PROXIES")

	// Single public IP — returned directly
	assert.Equal(t, "8.8.8.8", ClientIPFromXFF("8.8.8.8"))

	// Client behind a trusted proxy
	assert.Equal(t, "8.8.8.8", ClientIPFromXFF("8.8.8.8, 10.0.0.1"))

	// Client behind multiple trusted proxies
	assert.Equal(t, "8.8.8.8", ClientIPFromXFF("8.8.8.8, 10.0.0.1, 192.168.1.1"))

	// Spoofed: attacker claims 1.2.3.4; the real client IP 8.8.8.8 was
	// appended by the proxy and is the first untrusted entry from the right.
	assert.Equal(t, "8.8.8.8", ClientIPFromXFF("1.2.3.4, 8.8.8.8, 10.0.0.1"))
}

func TestClientIPFromXFF_AllTrusted(t *testing.T) {
	parseOnce = sync.Once{}
	trustedProxies = nil
	_ = os.Unsetenv("BI_TRUSTED_PROXIES")

	assert.Equal(t, "192.168.1.1", ClientIPFromXFF("192.168.1.1,10.0.0.1,127.0.0.1"))
}

func TestClientIPFromXFF_Empty(t *testing.T) {
	parseOnce = sync.Once{}
	trustedProxies = nil
	_ = os.Unsetenv("BI_TRUSTED_PROXIES")

	assert.Equal(t, "", ClientIPFromXFF(""))
}

func TestClientIPFromXFF_WhitespaceAndNoTrailing(t *testing.T) {
	parseOnce = sync.Once{}
	trustedProxies = nil
	_ = os.Unsetenv("BI_TRUSTED_PROXIES")

	assert.Equal(t, "8.8.8.8", ClientIPFromXFF("8.8.8.8, 10.0.0.1"))

	// Single-entry without trailing comma
	assert.Equal(t, "8.8.8.8", ClientIPFromXFF("8.8.8.8"))
}
