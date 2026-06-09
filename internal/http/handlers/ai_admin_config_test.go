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
	return &AIHandler{deps: (&app.Dependencies{MetaRepo: repo, Config: cfg}).AIDeps()}
}

// DB overrides take precedence over environment defaults; unset fields keep them.
func TestEffectiveAmbiguityConfigAppliesOverrides(t *testing.T) {
	h := newAIHandlerWithRepo(nil)
	h.ambiguityOverridesCache.overrides = ambiguityOverrides{TieredEnabled: new(true)}
	h.ambiguityOverridesCache.expires = time.Now().Add(time.Minute)

	eff := h.effectiveAmbiguityConfig(context.Background())
	assert.True(t, eff.TieredEnabled, "override should flip tiered flag")
	assert.Equal(t, 1, eff.MaxLLMTierPerQuestion, "unset override keeps env default")
}

// With no stored row, GET reports the environment defaults and no DB override.
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
}

// Both knobs are required and the LLM tier budget is range-checked.
func TestUpdateAdminRuntimeConfigValidation(t *testing.T) {
	h := newAIHandlerWithRepo(nil)
	for name, body := range map[string]string{
		"missing fields": `{"ambiguity":{}}`,
		"out of range":   `{"ambiguity":{"tiered_enabled":true,"max_llm_tier_per_question":99}}`,
	} {
		rec := httptest.NewRecorder()
		h.UpdateAdminRuntimeConfig(rec, httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/ai/admin/config", strings.NewReader(body)))
		assert.Equal(t, http.StatusBadRequest, rec.Code, name)
	}
}
