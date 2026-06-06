package security

import (
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

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
