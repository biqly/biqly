package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizeDisplayNameForProfile(t *testing.T) {
	name, err := SanitizeDisplayName("  Ada Lovelace  ")
	assert.NoError(t, err)
	assert.Equal(t, "Ada Lovelace", name)

	_, err = SanitizeDisplayName("<script>")
	assert.Error(t, err)
}
