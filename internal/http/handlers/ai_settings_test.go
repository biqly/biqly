package handlers

import (
	"context"
	"database/sql/driver"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/biqly/biqly/internal/metadata"
	"github.com/bytedance/sonic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRuntimeSettingsIncludesAmbiguityEnvDefaults(t *testing.T) {
	db, state := setupMockDB(t)
	state.queries = []queryMock{
		{Pattern: "FROM ai_runtime_config", Cols: []string{"value"}, Rows: [][]driver.Value{}},
	}
	h := newAIHandlerWithRepo(metadata.NewRepository(db))

	rec := httptest.NewRecorder()
	h.RuntimeSettings(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/ai/settings", http.NoBody))

	require.Equal(t, http.StatusOK, rec.Code)
	var resp aiRuntimeSettingsResponse
	require.NoError(t, sonic.Unmarshal(rec.Body.Bytes(), &resp))
	assert.False(t, resp.Ambiguity.TieredEnabled)
	assert.Equal(t, 1, resp.Ambiguity.MaxLLMTierPerQuestion)
	assert.False(t, resp.Ambiguity.DBOverride)
	assert.Equal(t, "environment", resp.Ambiguity.Source)
}

func TestRuntimeSettingsIncludesAmbiguityDBOverride(t *testing.T) {
	db, state := setupMockDB(t)
	stored := `{"tiered_enabled":true,"max_llm_tier_per_question":2}`
	state.queries = []queryMock{
		{Pattern: "FROM ai_runtime_config", Cols: []string{"value"}, Rows: [][]driver.Value{{stored}}},
	}
	h := newAIHandlerWithRepo(metadata.NewRepository(db))

	rec := httptest.NewRecorder()
	h.RuntimeSettings(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/ai/settings", http.NoBody))

	require.Equal(t, http.StatusOK, rec.Code)
	var resp aiRuntimeSettingsResponse
	require.NoError(t, sonic.Unmarshal(rec.Body.Bytes(), &resp))
	assert.True(t, resp.Ambiguity.TieredEnabled)
	assert.Equal(t, 2, resp.Ambiguity.MaxLLMTierPerQuestion)
	assert.True(t, resp.Ambiguity.DBOverride)
	assert.Equal(t, "database", resp.Ambiguity.Source)
}
