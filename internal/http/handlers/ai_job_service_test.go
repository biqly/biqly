package handlers

import (
	"encoding/json"
	"testing"

	"github.com/biqly/biqly/internal/ai"
	"github.com/biqly/biqly/internal/config"
	"github.com/biqly/biqly/internal/queue"
	"github.com/bytedance/sonic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateAIJobRequestDescribe(t *testing.T) {
	valid, err := sonic.ConfigStd.Marshal(ai.DescribeRequest{
		DatasourceID: "ds_1",
		Schema:       "public",
		Table:        "users",
	})
	require.NoError(t, err)
	require.NoError(t, validateAIJobRequest("describe", valid))

	missingTable, err := sonic.ConfigStd.Marshal(ai.DescribeRequest{DatasourceID: "ds_1"})
	require.NoError(t, err)
	require.EqualError(t, validateAIJobRequest("describe", missingTable), "datasource_id and table are required")
}

func TestValidateAIJobRequestRejectsUnknownKind(t *testing.T) {
	err := validateAIJobRequest("unknown", json.RawMessage(`{}`))
	require.EqualError(t, err, "invalid kind")
}

func TestRouteAIJobDisabled(t *testing.T) {
	cfg := config.AgentConfig{Enabled: false}
	route := routeAIJob(cfg, "ws-1")
	assert.Equal(t, queue.AIJobSubject, route.LegacySubject)
	assert.Empty(t, route.AgentSubject)
	assert.False(t, route.AgentAuthoritative)
}

func TestRouteAIJobShadowRunsLegacyAuthoritativePlusAgentShadow(t *testing.T) {
	cfg := config.AgentConfig{Enabled: true, Mode: config.AgentModeShadow, JobSubject: "biqly.agent.jobs"}
	route := routeAIJob(cfg, "ws-1")
	assert.Equal(t, queue.AIJobSubject, route.LegacySubject, "legacy stays authoritative in shadow mode")
	assert.Equal(t, "biqly.agent.jobs", route.AgentSubject, "the agent still runs, just for comparison")
	assert.False(t, route.AgentAuthoritative)
}

func TestRouteAIJobActiveAllowlistedIsAgentAuthoritative(t *testing.T) {
	cfg := config.AgentConfig{
		Enabled: true, Mode: config.AgentModeActive, JobSubject: "biqly.agent.jobs",
		WorkspaceAllowlist: []string{"ws-1", "ws-2"},
	}
	route := routeAIJob(cfg, "ws-1")
	assert.Empty(t, route.LegacySubject)
	assert.Equal(t, "biqly.agent.jobs", route.AgentSubject)
	assert.True(t, route.AgentAuthoritative)
}

func TestRouteAIJobActiveNonAllowlistedFallsBackToLegacy(t *testing.T) {
	cfg := config.AgentConfig{
		Enabled: true, Mode: config.AgentModeActive, JobSubject: "biqly.agent.jobs",
		WorkspaceAllowlist: []string{"ws-1", "ws-2"},
	}
	route := routeAIJob(cfg, "ws-unlisted")
	assert.Equal(t, queue.AIJobSubject, route.LegacySubject)
	assert.Empty(t, route.AgentSubject)
	assert.False(t, route.AgentAuthoritative)
}

// "default": active mode with no allowlist rolls the agent out to everyone.
func TestRouteAIJobActiveEmptyAllowlistIsDefaultForEveryone(t *testing.T) {
	cfg := config.AgentConfig{Enabled: true, Mode: config.AgentModeActive, JobSubject: "biqly.agent.jobs"}
	route := routeAIJob(cfg, "any-workspace")
	assert.Empty(t, route.LegacySubject)
	assert.Equal(t, "biqly.agent.jobs", route.AgentSubject)
	assert.True(t, route.AgentAuthoritative)
}

func TestRouteAIJobUnknownModeFallsBackToLegacy(t *testing.T) {
	cfg := config.AgentConfig{Enabled: true, Mode: "not-a-real-mode"}
	route := routeAIJob(cfg, "ws-1")
	assert.Equal(t, queue.AIJobSubject, route.LegacySubject)
	assert.False(t, route.AgentAuthoritative)
}

func TestShouldFallbackToLegacyPreExecuteFailureWhenEnabled(t *testing.T) {
	cfg := config.AgentConfig{LegacyFallbackEnabled: true}
	route := AIJobRoute{AgentAuthoritative: true}
	assert.True(t, shouldFallbackToLegacy(cfg, route, false))
}

func TestShouldFallbackToLegacyPreExecuteFailureWhenDisabled(t *testing.T) {
	cfg := config.AgentConfig{LegacyFallbackEnabled: false}
	route := AIJobRoute{AgentAuthoritative: true}
	assert.False(t, shouldFallbackToLegacy(cfg, route, false))
}

func TestShouldFallbackToLegacyPostExecuteNeverFallsBack(t *testing.T) {
	cfg := config.AgentConfig{LegacyFallbackEnabled: true}
	route := AIJobRoute{AgentAuthoritative: true}
	assert.False(t, shouldFallbackToLegacy(cfg, route, true), "no fallback once Query Execute has started")
}

func TestShouldFallbackToLegacyNeverAppliesWhenLegacyIsAuthoritative(t *testing.T) {
	cfg := config.AgentConfig{LegacyFallbackEnabled: true}
	route := AIJobRoute{AgentAuthoritative: false}
	assert.False(t, shouldFallbackToLegacy(cfg, route, false), "there is nothing to fall back from when legacy is already authoritative")
}
