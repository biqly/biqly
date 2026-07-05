package handlers

import (
	"context"
	"database/sql/driver"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/bytedance/sonic"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testInstructionID = "7c9e6679-7425-40de-944b-e07fc1f90ae7"

func newInstructionsHandlerWithRepo(repo *metadata.Repository) *AIInstructionsHandler {
	return NewAIInstructionsHandler((&app.Dependencies{MetaRepo: repo}).AIDeps())
}

// List returns all instructions (active and inactive) for the datasource.
func TestInstructionsListReturnsRows(t *testing.T) {
	db, state := setupMockDB(t)
	updatedAt := time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC)
	state.queries = []queryMock{
		{
			Pattern: "FROM ai_instructions",
			Cols:    []string{"id", "datasource_id", "model_id", "title", "body_md", "is_active", "created_at", "updated_at"},
			Rows: [][]driver.Value{
				{testInstructionID, testDatasourceID, "", "Fiscal year", "Starts in April.", true, updatedAt, updatedAt},
				{testConfirmedQueryID, testDatasourceID, "", "Deprecated", "Do not use.", false, updatedAt, updatedAt},
			},
		},
	}
	h := newInstructionsHandlerWithRepo(metadata.NewRepository(db))

	rec := httptest.NewRecorder()
	h.List(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/ai/instructions?datasource_id="+testDatasourceID, http.NoBody))

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var payload struct {
		Instructions []instructionResponse `json:"instructions"`
	}
	require.NoError(t, sonic.Unmarshal(rec.Body.Bytes(), &payload))
	require.Len(t, payload.Instructions, 2)
	assert.Equal(t, "Fiscal year", payload.Instructions[0].Title)
	assert.True(t, payload.Instructions[0].IsActive)
	assert.False(t, payload.Instructions[1].IsActive)
}

// datasource_id is required on list.
func TestInstructionsListRequiresDatasourceID(t *testing.T) {
	h := newInstructionsHandlerWithRepo(nil)
	rec := httptest.NewRecorder()
	h.List(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/ai/instructions", http.NoBody))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// Create inserts an active instruction and returns its id.
func TestInstructionsCreate(t *testing.T) {
	db, state := setupMockDB(t)
	state.queries = []queryMock{
		{Pattern: "INSERT INTO ai_instructions", Cols: []string{"id"}, Rows: [][]driver.Value{{testInstructionID}}},
	}
	h := newInstructionsHandlerWithRepo(metadata.NewRepository(db))

	body := `{"datasource_id":"` + testDatasourceID + `","title":"Fiscal year","body_md":"Starts in April."}`
	rec := httptest.NewRecorder()
	h.Create(rec, httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/ai/instructions", strings.NewReader(body)))

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var payload struct {
		ID string `json:"id"`
	}
	require.NoError(t, sonic.Unmarshal(rec.Body.Bytes(), &payload))
	assert.Equal(t, testInstructionID, payload.ID)
	// No follow-up deactivation update should run for an active create.
	assert.Nil(t, findCall(state.calls, "UPDATE ai_instructions"))
}

// Create with is_active=false inserts then deactivates the new row.
func TestInstructionsCreateInactive(t *testing.T) {
	db, state := setupMockDB(t)
	state.queries = []queryMock{
		{Pattern: "INSERT INTO ai_instructions", Cols: []string{"id"}, Rows: [][]driver.Value{{testInstructionID}}},
	}
	state.execs = []execMock{{Pattern: "UPDATE ai_instructions", RowsAffected: 1}}
	h := newInstructionsHandlerWithRepo(metadata.NewRepository(db))

	body := `{"datasource_id":"` + testDatasourceID + `","title":"Deprecated","body_md":"Do not use.","is_active":false}`
	rec := httptest.NewRecorder()
	h.Create(rec, httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/ai/instructions", strings.NewReader(body)))

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	call := findCall(state.calls, "UPDATE ai_instructions")
	require.NotNil(t, call, "expected a deactivating update")
	assert.Equal(t, false, call.Args[3])
}

// Create rejects a missing title.
func TestInstructionsCreateValidatesTitle(t *testing.T) {
	h := newInstructionsHandlerWithRepo(nil)
	body := `{"datasource_id":"` + testDatasourceID + `","body_md":"x"}`
	rec := httptest.NewRecorder()
	h.Create(rec, httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/ai/instructions", strings.NewReader(body)))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func instructionRoute(t *testing.T, h *AIInstructionsHandler, method, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := chi.NewRouter()
	r.Put("/ai/instructions/{id}", h.Update)
	r.Delete("/ai/instructions/{id}", h.Delete)
	var reader io.Reader = http.NoBody
	if body != "" {
		reader = strings.NewReader(body)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), method, "/ai/instructions/"+testInstructionID, reader))
	return rec
}

// Update writes the row and 404s on unknown ids.
func TestInstructionsUpdate(t *testing.T) {
	db, state := setupMockDB(t)
	state.execs = []execMock{{Pattern: "UPDATE ai_instructions", RowsAffected: 1}}
	h := newInstructionsHandlerWithRepo(metadata.NewRepository(db))

	body := `{"title":"Fiscal year","body_md":"Starts in April.","is_active":true}`
	rec := instructionRoute(t, h, http.MethodPut, body)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	state.execs = []execMock{{Pattern: "UPDATE ai_instructions", RowsAffected: 0}}
	rec = instructionRoute(t, h, http.MethodPut, body)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// Delete removes the row and 404s on unknown ids.
func TestInstructionsDelete(t *testing.T) {
	db, state := setupMockDB(t)
	state.execs = []execMock{{Pattern: "DELETE FROM ai_instructions", RowsAffected: 1}}
	h := newInstructionsHandlerWithRepo(metadata.NewRepository(db))

	rec := instructionRoute(t, h, http.MethodDelete, "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	state.execs = []execMock{{Pattern: "DELETE FROM ai_instructions", RowsAffected: 0}}
	rec = instructionRoute(t, h, http.MethodDelete, "")
	assert.Equal(t, http.StatusNotFound, rec.Code)
}
