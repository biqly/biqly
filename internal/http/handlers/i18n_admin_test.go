package handlers

import (
	"context"
	"database/sql/driver"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bytedance/sonic"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/biqly/biqly/internal/metadata"
)

func i18nRequestWithLocale(method, target, locale string, body string) *http.Request {
	var reader = http.NoBody
	req := httptest.NewRequestWithContext(context.Background(), method, target, reader)
	if body != "" {
		req = httptest.NewRequestWithContext(context.Background(), method, target, strings.NewReader(body))
	}
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("locale", locale)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func TestAdminUpsertI18nLocalesValidation(t *testing.T) {
	h := newAIHandlerWithRepo(nil)
	cases := []struct {
		name string
		body string
	}{
		{"empty", `{"locales":[]}`},
		{"bad locale", `{"locales":[{"locale":"DE!","label":"x","short_label":"X","enabled":true}]}`},
		{"missing labels", `{"locales":[{"locale":"de","label":"","short_label":"","enabled":true}]}`},
		{"disable default", `{"locales":[{"locale":"en","label":"English","short_label":"EN","enabled":false}]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.AdminUpsertI18nLocales(rec, httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/ai/admin/i18n/locales", strings.NewReader(tc.body)))
			require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
		})
	}
}

func TestAdminUpsertI18nLocalesPersists(t *testing.T) {
	db, state := setupMockDB(t)
	state.execs = []execMock{{Pattern: "INSERT INTO i18n_locales", RowsAffected: 1}}
	h := newAIHandlerWithRepo(metadata.NewRepository(db))

	body := `{"locales":[{"locale":"de","label":"Deutsch","short_label":"DE","question_signals":[" wie viele "],"enabled":true}]}`
	rec := httptest.NewRecorder()
	h.AdminUpsertI18nLocales(rec, httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/ai/admin/i18n/locales", strings.NewReader(body)))

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	insert := findCall(state.calls, "INSERT INTO i18n_locales")
	require.NotNil(t, insert)
	assert.Equal(t, "de", insert.Args[0])
}

func TestAdminGetI18nBundleFallsBackToEmbedded(t *testing.T) {
	db, state := setupMockDB(t)
	state.queries = []queryMock{
		{Pattern: "FROM i18n_bundles", Cols: []string{"locale", "bundle", "version", "updated_at"}, Rows: [][]driver.Value{}},
	}
	h := newAIHandlerWithRepo(metadata.NewRepository(db))

	rec := httptest.NewRecorder()
	h.AdminGetI18nBundle(rec, i18nRequestWithLocale(http.MethodGet, "/api/ai/admin/i18n/bundles/tr", "tr", ""))

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp i18nBundleResponse
	require.NoError(t, sonic.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "embedded", resp.Source)
	assert.Contains(t, string(resp.Bundle), "ambiguity_reason")
}

func TestAdminUpsertI18nBundleValidatesLeaves(t *testing.T) {
	h := newAIHandlerWithRepo(nil)
	rec := httptest.NewRecorder()
	h.AdminUpsertI18nBundle(rec, i18nRequestWithLocale(http.MethodPut, "/api/ai/admin/i18n/bundles/de", "de", `{"clarification":{"count":42}}`))
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "clarification.count")
}

func TestAdminUpsertI18nBundlePersists(t *testing.T) {
	db, state := setupMockDB(t)
	now := time.Now()
	state.execs = []execMock{{Pattern: "INSERT INTO i18n_bundles", RowsAffected: 1}}
	state.queries = []queryMock{
		{Pattern: "FROM i18n_bundles", Cols: []string{"locale", "bundle", "version", "updated_at"},
			Rows: [][]driver.Value{{"de", `{"clarification":{"ambiguity_reason":"Mehrdeutig."}}`, 1, now}}},
	}
	h := newAIHandlerWithRepo(metadata.NewRepository(db))

	rec := httptest.NewRecorder()
	h.AdminUpsertI18nBundle(rec, i18nRequestWithLocale(http.MethodPut, "/api/ai/admin/i18n/bundles/de", "de", `{"clarification":{"ambiguity_reason":"Mehrdeutig."}}`))

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.NotNil(t, findCall(state.calls, "INSERT INTO i18n_bundles"))
	var resp i18nBundleResponse
	require.NoError(t, sonic.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "database", resp.Source)
	assert.Equal(t, 1, resp.Version)
}

func TestAdminI18nCoverageReportsMissingKeys(t *testing.T) {
	db, state := setupMockDB(t)
	// No DB bundles (the mock matches by query text, not args, so both the
	// reference and target lookups must share one outcome): reference resolves
	// to the embedded EN catalog, target "de" has no catalog at all.
	state.queries = []queryMock{
		{Pattern: "FROM i18n_bundles", Cols: []string{"locale", "bundle", "version", "updated_at"}, Rows: [][]driver.Value{}},
	}
	h := newAIHandlerWithRepo(metadata.NewRepository(db))

	rec := httptest.NewRecorder()
	h.AdminI18nCoverage(rec, i18nRequestWithLocale(http.MethodGet, "/api/ai/admin/i18n/coverage/de", "de", ""))

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp i18nCoverageResponse
	require.NoError(t, sonic.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "de", resp.Locale)
	assert.Greater(t, resp.TotalKeys, 100, "embedded EN catalog should provide the reference key set")
	assert.Equal(t, 0, resp.Translated)
	assert.Equal(t, float64(0), resp.CoveragePct)
	assert.Contains(t, resp.MissingKeys, "clarification.needs_clarification_warning")
	assert.Contains(t, resp.MissingKeys, "clarification.ambiguity_reason")
	assert.Len(t, resp.MissingKeys, resp.TotalKeys)
}
