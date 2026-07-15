package dashboard

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateShareToken(t *testing.T) {
	a, err := GenerateShareToken()
	require.NoError(t, err)
	b, err := GenerateShareToken()
	require.NoError(t, err)
	assert.NotEqual(t, a, b)
	assert.GreaterOrEqual(t, len(a), 43) // 32 bytes base64url, unpadded
	assert.False(t, strings.ContainsAny(a, "+/="), "must be URL-safe")
}

func TestHashShareToken(t *testing.T) {
	h := HashShareToken("fixed-input")
	assert.Equal(t, HashShareToken("fixed-input"), h)
	assert.Len(t, h, 64) // sha256 hex
	assert.NotEqual(t, HashShareToken("other"), h)
}
