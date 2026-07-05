package handlers

import (
	"context"
	"database/sql/driver"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	bimw "github.com/biqly/biqly/internal/http/middleware"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/bytedance/sonic"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testConfirmedQueryID = "5b1f9d4e-6a7b-4c2d-9e3f-1a2b3c4d5e6f"
const testDatasourceID = "0f8fad5b-d9cb-469f-a165-70867728950e"

// The admin listing returns rows (active and inactive) for the datasource.
func TestAdminListConfirmedQueriesReturnsRows(t *testing.T) {
	db, state := setupMockDB(t)
	confirmedAt := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	state.queries = []queryMock{
		{
			Pattern: "LIMIT $2 OFFSET $3",
			Cols: []string{
				"id", "datasource_id", "model_id", "user_id",
				"nl_query", "sql_query", "semantic_model_hash", "is_active", "confirmed_at",
			},
			Rows: [][]driver.Value{
				{testConfirmedQueryID, testDatasourceID, "m-1", "u-1",
					"monthly sales", `{"select":[]}`, "m-1@2", false, confirmedAt},
			},
		},
		{Pattern: "SELECT COUNT(*)::int FROM ai_saved_queries", Cols: []string{"count"}, Rows: [][]driver.Value{{1}}},
	}
	h := newAIHandlerWithRepo(metadata.NewRepository(db))

	handler := bimw.Paginate(bimw.PaginationConfig{DefaultPage: 1, DefaultPageSize: 10, MaxPageSize: 100})(
		http.HandlerFunc(h.AdminListConfirmedQueries),
	)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec,
		httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/ai/confirmed-queries?datasource_id="+testDatasourceID, http.NoBody))

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var payload struct {
		Queries []confirmedQueryAdminResponse `json:"queries"`
		Total   int                           `json:"total"`
	}
	require.NoError(t, sonic.Unmarshal(rec.Body.Bytes(), &payload))
	require.Len(t, payload.Queries, 1)
	assert.Equal(t, 1, payload.Total)
	assert.Equal(t, "monthly sales", payload.Queries[0].NLQuery)
	assert.False(t, payload.Queries[0].IsActive)
	assert.True(t, confirmedAt.Equal(payload.Queries[0].ConfirmedAt))
}

// datasource_id is required and must be a UUID.
func TestAdminListConfirmedQueriesValidatesDatasourceID(t *testing.T) {
	h := newAIHandlerWithRepo(nil)
	for name, target := range map[string]string{
		"missing":    "/api/ai/confirmed-queries",
		"not-a-uuid": "/api/ai/confirmed-queries?datasource_id=abc",
	} {
		rec := httptest.NewRecorder()
		h.AdminListConfirmedQueries(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, target, http.NoBody))
		assert.Equal(t, http.StatusBadRequest, rec.Code, name)
	}
}

func deactivateVia(t *testing.T, h *AIHandler, id string) *httptest.ResponseRecorder {
	t.Helper()
	r := chi.NewRouter()
	r.Post("/ai/confirmed-queries/{id}/deactivate", h.AdminDeactivateConfirmedQuery)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/ai/confirmed-queries/"+id+"/deactivate", http.NoBody))
	return rec
}

// Deactivation flips is_active for the row and 404s on unknown ids.
func TestAdminDeactivateConfirmedQuery(t *testing.T) {
	db, state := setupMockDB(t)
	state.execs = []execMock{{Pattern: "UPDATE ai_saved_queries SET is_active", RowsAffected: 1}}
	h := newAIHandlerWithRepo(metadata.NewRepository(db))

	rec := deactivateVia(t, h, testConfirmedQueryID)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	call := findCall(state.calls, "UPDATE ai_saved_queries SET is_active")
	require.NotNil(t, call)
	require.Len(t, call.Args, 2)
	assert.Equal(t, testConfirmedQueryID, call.Args[0])
	assert.Equal(t, false, call.Args[1])

	state.execs = []execMock{{Pattern: "UPDATE ai_saved_queries SET is_active", RowsAffected: 0}}
	assert.Equal(t, http.StatusNotFound, deactivateVia(t, h, testConfirmedQueryID).Code)

	assert.Equal(t, http.StatusBadRequest, deactivateVia(t, h, "not-a-uuid").Code)
}
