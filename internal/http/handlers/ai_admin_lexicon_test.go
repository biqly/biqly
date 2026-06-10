package handlers

import (
	"context"
	"database/sql/driver"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/biqly/biqly/internal/ai/lexicon"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/bytedance/sonic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminListLexiconRejectsUnknownDomain(t *testing.T) {
	h := newAIHandlerWithRepo(nil)
	rec := httptest.NewRecorder()
	h.AdminListLexicon(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/ai/admin/lexicon?domain=bogus", http.NoBody))
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAdminListLexiconReturnsWireEntries(t *testing.T) {
	db, state := setupMockDB(t)
	state.queries = []queryMock{{
		Pattern: "FROM ai_nl_lexicon",
		Cols:    []string{"locale", "domain", "key", "value", "is_active", "updated_at"},
		Rows: [][]driver.Value{
			{"tr", lexicon.DomainIntentToken, "count", `{"terms":["kaç","adet"]}`, true, time.Now()},
		},
	}}
	h := newAIHandlerWithRepo(metadata.NewRepository(db))

	rec := httptest.NewRecorder()
	h.AdminListLexicon(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/ai/admin/lexicon?domain=intent_token", http.NoBody))

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp lexiconListResponse
	require.NoError(t, sonic.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp.Entries, 1)
	assert.Equal(t, "tr", resp.Entries[0].Locale)
	assert.Equal(t, []string{"kaç", "adet"}, resp.Entries[0].Terms)
}

func TestAdminUpsertLexiconValidation(t *testing.T) {
	h := newAIHandlerWithRepo(nil)
	cases := []struct {
		name string
		body string
	}{
		{"empty entries", `{"entries":[]}`},
		{"bad locale", `{"entries":[{"locale":"Türkçe","domain":"intent_token","key":"count","terms":["kaç"]}]}`},
		{"unknown domain", `{"entries":[{"locale":"tr","domain":"bogus","key":"count","terms":["kaç"]}]}`},
		{"missing key", `{"entries":[{"locale":"tr","domain":"intent_token","key":"","terms":["kaç"]}]}`},
		{"temporal without keys", `{"entries":[{"locale":"de","domain":"temporal_phrase","key":"letzten monat","terms":["x"]}]}`},
		{"terms missing", `{"entries":[{"locale":"de","domain":"intent_token","key":"count"}]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.AdminUpsertLexicon(rec, httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/ai/admin/lexicon", strings.NewReader(tc.body)))
			require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
		})
	}
}

func TestAdminUpsertLexiconPersistsAndCounts(t *testing.T) {
	db, state := setupMockDB(t)
	state.execs = []execMock{{Pattern: "INSERT INTO ai_nl_lexicon", RowsAffected: 1}}
	h := newAIHandlerWithRepo(metadata.NewRepository(db))

	body := `{"entries":[
		{"locale":"de","domain":"temporal_phrase","key":"letzten monat","interpretation_keys":["prev_calendar_month","rolling_30d"]},
		{"locale":"de","domain":"intent_token","key":"count","terms":["wie viele","anzahl"]}
	]}`
	rec := httptest.NewRecorder()
	h.AdminUpsertLexicon(rec, httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/ai/admin/lexicon", strings.NewReader(body)))

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp lexiconUpsertResponse
	require.NoError(t, sonic.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, 2, resp.Updated)

	insert := findCall(state.calls, "INSERT INTO ai_nl_lexicon")
	require.NotNil(t, insert, "expected lexicon insert exec")
	require.Len(t, insert.Args, 5)
	assert.Equal(t, "de", insert.Args[0])
	assert.Equal(t, lexicon.DomainTemporalPhrase, insert.Args[1])
	assert.Equal(t, "letzten monat", insert.Args[2])
}

func TestAdminResetLexiconDomainRestoresDefaults(t *testing.T) {
	db, state := setupMockDB(t)
	state.execs = []execMock{
		{Pattern: "DELETE FROM ai_nl_lexicon", RowsAffected: 3},
		{Pattern: "INSERT INTO ai_nl_lexicon", RowsAffected: 1},
	}
	h := newAIHandlerWithRepo(metadata.NewRepository(db))

	rec := httptest.NewRecorder()
	h.AdminResetLexiconDomain(rec, httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/ai/admin/lexicon/reset", strings.NewReader(`{"domain":"temporal_phrase"}`)))

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp lexiconResetResponse
	require.NoError(t, sonic.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "temporal_phrase", resp.Domain)

	defaults, err := lexicon.DefaultMetadataEntries("temporal_phrase")
	require.NoError(t, err)
	assert.Equal(t, len(defaults), resp.Restored)
	require.NotNil(t, findCall(state.calls, "DELETE FROM ai_nl_lexicon"))
}
