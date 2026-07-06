package handlers

import (
	"context"
	"database/sql/driver"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/config"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/bytedance/sonic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newAIHandlerWithRepo(repo *metadata.Repository) *AIHandler {
	cfg := &config.Config{}
	cfg.AI.Ambiguity = config.AmbiguityConfig{TieredEnabled: false, MaxLLMTierPerQuestion: 1}
	cfg.AI.Memory = config.AIMemoryConfig{RecallEnabled: true, RecallLimit: 5}
	cfg.PII = config.PIIConfig{Enabled: true, DetectionThreshold: 0.6}
	cfg.NATS.Concurrency = 2
	cfg.Agent = config.AgentConfig{
		Enabled: false, Mode: config.AgentModeShadow, MaxSteps: 6,
		MaxClarificationRounds: 2, Timeout: 45 * time.Second, MaxRows: 1000,
	}
	return &AIHandler{deps: (&app.Dependencies{MetaRepo: repo, Config: cfg}).AIDeps()}
}

// Agent overrides overlay the env defaults; unset fields keep them.
func TestEffectiveAgentConfigAppliesOverrides(t *testing.T) {
	h := newAIHandlerWithRepo(nil)
	h.agentOverridesCache.cached = agentOverrides{Enabled: new(true), MaxSteps: new(2)}
	h.agentOverridesCache.expires = time.Now().Add(time.Minute)

	eff := h.effectiveAgentConfig(context.Background())
	assert.True(t, eff.Enabled, "override should enable the agent")
	assert.Equal(t, 2, eff.MaxSteps, "override should set max_steps")
	assert.Equal(t, config.AgentModeShadow, eff.Mode, "unset override keeps env default")
	assert.Equal(t, 45*time.Second, eff.Timeout, "unset override keeps env default")
}

func TestEffectiveQueueConfigAppliesOverrides(t *testing.T) {
	h := newAIHandlerWithRepo(nil)
	h.queueOverridesCache.cached = queueOverrides{Concurrency: new(5)}
	h.queueOverridesCache.expires = time.Now().Add(time.Minute)

	eff := h.EffectiveConcurrency(context.Background())
	assert.Equal(t, 5, eff, "override should set concurrency to 5")
}

// DB overrides take precedence over environment defaults; unset fields keep them.
func TestEffectiveAmbiguityConfigAppliesOverrides(t *testing.T) {
	h := newAIHandlerWithRepo(nil)
	h.ambiguityOverridesCache.cached = ambiguityOverrides{TieredEnabled: new(true)}
	h.ambiguityOverridesCache.expires = time.Now().Add(time.Minute)

	eff := h.effectiveAmbiguityConfig(context.Background())
	assert.True(t, eff.TieredEnabled, "override should flip tiered flag")
	assert.Equal(t, 1, eff.MaxLLMTierPerQuestion, "unset override keeps env default")
}

// The extended ambiguity knobs overlay the env defaults the same way.
func TestEffectiveAmbiguityConfigAppliesExtendedOverrides(t *testing.T) {
	h := newAIHandlerWithRepo(nil)
	h.ambiguityOverridesCache.cached = ambiguityOverrides{
		CheckEnabled:        new(false),
		ConfidenceThreshold: new(0.9),
		MaxOptions:          new(3),
	}
	h.ambiguityOverridesCache.expires = time.Now().Add(time.Minute)

	eff := h.effectiveAmbiguityConfig(context.Background())
	assert.False(t, eff.CheckEnabled)
	assert.Equal(t, 0.9, eff.ConfidenceThreshold)
	assert.Equal(t, 3, eff.MaxOptions)
}

// Memory overrides overlay the env defaults; unset fields keep them.
func TestEffectiveMemoryConfigAppliesOverrides(t *testing.T) {
	h := newAIHandlerWithRepo(nil)
	h.memoryOverridesCache.cached = memoryOverrides{RecallEnabled: new(false)}
	h.memoryOverridesCache.expires = time.Now().Add(time.Minute)

	eff := h.effectiveMemoryConfig(context.Background())
	assert.False(t, eff.RecallEnabled, "override should disable recall")
	assert.Equal(t, 5, eff.RecallLimit, "unset override keeps env default")
}

// With no stored rows, GET reports the environment defaults and no DB override.
func TestAdminRuntimeConfigReturnsEnvDefaults(t *testing.T) {
	db, state := setupMockDB(t)
	state.queries = []queryMock{
		{Pattern: "FROM ai_runtime_config", Cols: []string{"value"}, Rows: [][]driver.Value{}},
	}
	h := newAIHandlerWithRepo(metadata.NewRepository(db))

	rec := httptest.NewRecorder()
	h.AdminRuntimeConfig(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/ai/admin/config", http.NoBody))

	require.Equal(t, http.StatusOK, rec.Code)
	var resp adminRuntimeConfigResponse
	require.NoError(t, sonic.Unmarshal(rec.Body.Bytes(), &resp))
	assert.False(t, resp.Ambiguity.TieredEnabled)
	assert.Equal(t, 1, resp.Ambiguity.MaxLLMTierPerQuestion)
	assert.False(t, resp.Ambiguity.DBOverride)
	assert.Equal(t, "environment", resp.Ambiguity.Source)
	assert.Equal(t, "environment", resp.Ambiguity.Sources["tiered_enabled"])

	assert.True(t, resp.PII.Enabled)
	assert.Equal(t, 0.6, resp.PII.DetectionThreshold)
	assert.False(t, resp.PII.DBOverride)

	assert.True(t, resp.Memory.RecallEnabled)
	assert.Equal(t, 5, resp.Memory.RecallLimit)
	assert.False(t, resp.Memory.DBOverride)

	assert.Equal(t, 2, resp.Queue.Concurrency)
	assert.False(t, resp.Queue.DBOverride)

	assert.False(t, resp.Agent.Enabled)
	assert.Equal(t, config.AgentModeShadow, resp.Agent.Mode)
	assert.Equal(t, 6, resp.Agent.MaxSteps)
	assert.Equal(t, 45, resp.Agent.TimeoutSeconds)
	assert.False(t, resp.Agent.DBOverride)
	assert.Equal(t, "environment", resp.Agent.Source)
}

// PUT accepts the agent domain and writes a single override row.
func TestUpdateAdminRuntimeConfigPersistsAgent(t *testing.T) {
	db, state := setupMockDB(t)
	stored := `{"enabled":true,"mode":"active","max_steps":2}`
	state.execs = []execMock{{Pattern: "INSERT INTO ai_runtime_config", RowsAffected: 1}}
	state.queries = []queryMock{
		{Pattern: "FROM ai_runtime_config", Cols: []string{"value"}, Rows: [][]driver.Value{{stored}}},
	}
	h := newAIHandlerWithRepo(metadata.NewRepository(db))

	body := strings.NewReader(`{"agent":{"enabled":true,"mode":"active","max_steps":2}}`)
	rec := httptest.NewRecorder()
	h.UpdateAdminRuntimeConfig(rec, httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/ai/admin/config", body))

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	upsert := findCall(state.calls, "INSERT INTO ai_runtime_config")
	require.NotNil(t, upsert, "expected upsert exec")
	require.Len(t, upsert.Args, 2)
	assert.Equal(t, agentRuntimeConfigKey, upsert.Args[0])

	var resp adminRuntimeConfigResponse
	require.NoError(t, sonic.Unmarshal(rec.Body.Bytes(), &resp))
	assert.True(t, resp.Agent.Enabled)
	assert.Equal(t, "active", resp.Agent.Mode)
	assert.Equal(t, 2, resp.Agent.MaxSteps)
	assert.True(t, resp.Agent.DBOverride)
	assert.Equal(t, "database", resp.Agent.Source)
	assert.Equal(t, "database", resp.Agent.Sources["max_steps"])
	assert.Equal(t, "environment", resp.Agent.Sources["max_rows"])
}

// PUT persists the overrides, invalidates the cache, and echoes the stored values.
func TestUpdateAdminRuntimeConfigPersistsAndReloads(t *testing.T) {
	db, state := setupMockDB(t)
	stored := `{"tiered_enabled":true,"max_llm_tier_per_question":2}`
	state.execs = []execMock{{Pattern: "INSERT INTO ai_runtime_config", RowsAffected: 1}}
	state.queries = []queryMock{
		{Pattern: "FROM ai_runtime_config", Cols: []string{"value"}, Rows: [][]driver.Value{{stored}}},
	}
	h := newAIHandlerWithRepo(metadata.NewRepository(db))

	body := strings.NewReader(`{"ambiguity":{"tiered_enabled":true,"max_llm_tier_per_question":2}}`)
	rec := httptest.NewRecorder()
	h.UpdateAdminRuntimeConfig(rec, httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/ai/admin/config", body))

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	upsert := findCall(state.calls, "INSERT INTO ai_runtime_config")
	require.NotNil(t, upsert, "expected upsert exec")
	require.Len(t, upsert.Args, 2)
	assert.Equal(t, ambiguityRuntimeConfigKey, upsert.Args[0])

	var resp adminRuntimeConfigResponse
	require.NoError(t, sonic.Unmarshal(rec.Body.Bytes(), &resp))
	assert.True(t, resp.Ambiguity.TieredEnabled)
	assert.Equal(t, 2, resp.Ambiguity.MaxLLMTierPerQuestion)
	assert.True(t, resp.Ambiguity.DBOverride)
	assert.Equal(t, "database", resp.Ambiguity.Source)
	assert.Equal(t, "database", resp.Ambiguity.Sources["tiered_enabled"])
	assert.Equal(t, "environment", resp.Ambiguity.Sources["check_enabled"])
}

// PUT accepts the pii and memory domains and writes one row per domain.
func TestUpdateAdminRuntimeConfigPersistsPIIAndMemory(t *testing.T) {
	db, state := setupMockDB(t)
	state.execs = []execMock{
		{Pattern: "INSERT INTO ai_runtime_config", RowsAffected: 1},
		{Pattern: "INSERT INTO ai_runtime_config", RowsAffected: 1},
	}
	state.queries = []queryMock{
		{Pattern: "FROM ai_runtime_config", Cols: []string{"value"}, Rows: [][]driver.Value{}},
	}
	h := newAIHandlerWithRepo(metadata.NewRepository(db))

	body := strings.NewReader(`{"pii":{"detection_threshold":0.8},"memory":{"recall_enabled":false,"recall_limit":3}}`)
	rec := httptest.NewRecorder()
	h.UpdateAdminRuntimeConfig(rec, httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/ai/admin/config", body))

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	keys := make([]string, 0, 2)
	for i := range state.calls {
		if strings.Contains(state.calls[i].Op, "INSERT INTO ai_runtime_config") {
			require.NotEmpty(t, state.calls[i].Args)
			if key, ok := state.calls[i].Args[0].(string); ok {
				keys = append(keys, key)
			}
		}
	}
	assert.ElementsMatch(t, []string{piiRuntimeConfigKey, memoryRuntimeConfigKey}, keys)
}

// Invalid values are rejected with field-specific 400s; nothing is persisted.
func TestUpdateAdminRuntimeConfigValidation(t *testing.T) {
	h := newAIHandlerWithRepo(nil)
	for name, tc := range map[string]struct {
		body    string
		wantMsg string
	}{
		"empty request":            {body: `{}`, wantMsg: "at least one config domain"},
		"unknown top-level domain": {body: `{"bogus":{}}`, wantMsg: "unknown or malformed"},
		"unknown ambiguity field":  {body: `{"ambiguity":{"tiered":true}}`, wantMsg: "ambiguity contains an unknown"},
		"llm tier out of range":    {body: `{"ambiguity":{"max_llm_tier_per_question":99}}`, wantMsg: "max_llm_tier_per_question"},
		"confidence out of range":  {body: `{"ambiguity":{"confidence_threshold":1.5}}`, wantMsg: "confidence_threshold"},
		"max options out of range": {body: `{"ambiguity":{"max_options":0}}`, wantMsg: "max_options"},
		"pii threshold zero":       {body: `{"pii":{"detection_threshold":0}}`, wantMsg: "detection_threshold"},
		"pii unknown field":        {body: `{"pii":{"enabled":false}}`, wantMsg: "pii contains an unknown"},
		"recall limit out of range": {
			body:    `{"memory":{"recall_limit":99}}`,
			wantMsg: "recall_limit",
		},
		"queue concurrency zero": {
			body:    `{"queue":{"concurrency":0}}`,
			wantMsg: "concurrency must be between 1 and 10",
		},
		"queue concurrency out of range": {
			body:    `{"queue":{"concurrency":11}}`,
			wantMsg: "concurrency must be between 1 and 10",
		},
		"agent unknown mode": {
			body:    `{"agent":{"mode":"eager"}}`,
			wantMsg: "agent.mode must be",
		},
		"agent max_steps out of range": {
			body:    `{"agent":{"max_steps":7}}`,
			wantMsg: "agent.max_steps must be between 1 and 6",
		},
		"agent clarification rounds out of range": {
			body:    `{"agent":{"max_clarification_rounds":3}}`,
			wantMsg: "agent.max_clarification_rounds must be between 0 and 2",
		},
		"agent timeout out of range": {
			body:    `{"agent":{"timeout_seconds":46}}`,
			wantMsg: "agent.timeout_seconds must be between 1 and 45",
		},
		"agent max_rows out of range": {
			body:    `{"agent":{"max_rows":1001}}`,
			wantMsg: "agent.max_rows must be between 1 and 1000",
		},
		"agent unknown field": {
			body:    `{"agent":{"job_subject":"x"}}`,
			wantMsg: "agent contains an unknown",
		},
	} {
		rec := httptest.NewRecorder()
		h.UpdateAdminRuntimeConfig(rec, httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/ai/admin/config", strings.NewReader(tc.body)))
		assert.Equal(t, http.StatusBadRequest, rec.Code, name)
		assert.Contains(t, rec.Body.String(), tc.wantMsg, name)
	}
}

// An empty domain object clears every override: the row is replaced with {}
// and the effective config falls back to the environment defaults.
func TestUpdateAdminRuntimeConfigEmptyDomainClearsOverrides(t *testing.T) {
	db, state := setupMockDB(t)
	state.execs = []execMock{{Pattern: "INSERT INTO ai_runtime_config", RowsAffected: 1}}
	state.queries = []queryMock{
		{Pattern: "FROM ai_runtime_config", Cols: []string{"value"}, Rows: [][]driver.Value{{`{}`}}},
	}
	h := newAIHandlerWithRepo(metadata.NewRepository(db))

	rec := httptest.NewRecorder()
	h.UpdateAdminRuntimeConfig(rec, httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/ai/admin/config", strings.NewReader(`{"ambiguity":{}}`)))

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp adminRuntimeConfigResponse
	require.NoError(t, sonic.Unmarshal(rec.Body.Bytes(), &resp))
	assert.False(t, resp.Ambiguity.DBOverride, "cleared row must report env source")
	assert.False(t, resp.Ambiguity.TieredEnabled, "effective value falls back to env default")
}

// An expired cache window refreshes from the database; invalidate forces the
// next load to refresh immediately (this is the ≤TTL propagation guarantee).
func TestRuntimeOverridesRefreshAfterExpiryAndInvalidate(t *testing.T) {
	db, state := setupMockDB(t)
	state.queries = []queryMock{
		{Pattern: "FROM ai_runtime_config", Cols: []string{"value"}, Rows: [][]driver.Value{{`{"tiered_enabled":true}`}}},
	}
	repo := metadata.NewRepository(db)

	var store runtimeOverrides[ambiguityOverrides]
	store.cached = ambiguityOverrides{TieredEnabled: new(false)}
	store.expires = time.Now().Add(-time.Second) // window already expired

	ov := store.load(context.Background(), repo, ambiguityRuntimeConfigKey)
	require.NotNil(t, ov.TieredEnabled)
	assert.True(t, *ov.TieredEnabled, "expired cache must refresh from the DB row")

	store.invalidate()
	assert.False(t, time.Now().Before(store.expires), "invalidate must expire the cache window")
}

// effectivePIIConfig overlays the DB threshold; the env kill switch is untouched.
func TestEffectivePIIConfigAppliesThresholdOverride(t *testing.T) {
	db, state := setupMockDB(t)
	state.queries = []queryMock{
		{Pattern: "FROM ai_runtime_config", Cols: []string{"value"}, Rows: [][]driver.Value{{`{"detection_threshold":0.85}`}}},
	}
	repo := metadata.NewRepository(db)

	base := config.PIIConfig{Enabled: true, DetectionThreshold: 0.6, SampleDataLimit: 50}
	eff := effectivePIIConfig(context.Background(), repo, base)
	assert.Equal(t, 0.85, eff.DetectionThreshold)
	assert.True(t, eff.Enabled)
	assert.Equal(t, 50, eff.SampleDataLimit)
}
