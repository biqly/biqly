package app

import (
	"context"
	"testing"

	"github.com/biqly/biqly/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// NewAgentDependencies must fail fast on missing required config rather
// than connecting to a database it will never use — the agent service has
// nothing to do without a queue to consume jobs from.
func TestNewAgentDependenciesRequiresNATSURL(t *testing.T) {
	cfg := &config.Config{}
	_, err := NewAgentDependencies(context.Background(), cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "BI_NATS_URL")
}
