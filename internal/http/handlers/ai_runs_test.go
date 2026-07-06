package handlers

import (
	"context"
	"database/sql/driver"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/biqly/biqly/internal/http/middleware"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/bytedance/sonic"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetAgentRunReturnsRunAndSteps(t *testing.T) {
	db, state := setupMockDB(t)
	now := time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)
	state.queries = []queryMock{
		{
			Pattern: "FROM agent_runs WHERE id",
			Cols: []string{
				"id", "conversation_id", "datasource_id", "model_id", "user_id",
				"question", "question_hash", "mode", "status", "confidence", "answer",
				"created_at", "updated_at",
			},
			Rows: [][]driver.Value{{
				"run-1", "conv-1", "ds-1", "model-1", "user-1",
				"monthly revenue", "hash-1", "interactive", "completed", 0.9, "Revenue was up.",
				now, now,
			}},
		},
		{
			Pattern: "FROM agent_steps",
			Cols:    []string{"seq", "kind", "status", "attempt", "duration_ms", "detail"},
			Rows: [][]driver.Value{
				{1, "table_route", "ok", 0, 12, ""},
				{2, "llm_generate", "ok", 0, 340, ""},
			},
		},
	}
	h := newAIHandlerWithRepo(metadata.NewRepository(db))

	router := chi.NewRouter()
	router.Get("/api/ai/runs/{id}", h.GetAgentRun)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, newRunsRequest("/api/ai/runs/run-1"))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var out struct {
		Run   map[string]any   `json:"run"`
		Steps []map[string]any `json:"steps"`
	}
	require.NoError(t, sonic.Unmarshal(rec.Body.Bytes(), &out))
	assert.Equal(t, "run-1", out.Run["id"])
	assert.Equal(t, "completed", out.Run["status"])
	require.Len(t, out.Steps, 2)
	assert.Equal(t, "table_route", out.Steps[0]["kind"])
	assert.Equal(t, "llm_generate", out.Steps[1]["kind"])
}

func TestGetAgentRunNotFound(t *testing.T) {
	db, state := setupMockDB(t)
	state.queries = []queryMock{
		{
			Pattern: "FROM agent_runs WHERE id",
			Cols: []string{
				"id", "conversation_id", "datasource_id", "model_id", "user_id",
				"question", "question_hash", "mode", "status", "confidence", "answer",
				"created_at", "updated_at",
			},
			Rows: [][]driver.Value{}, // no rows -> sql.ErrNoRows -> ErrAgentRunNotFound
		},
	}
	h := newAIHandlerWithRepo(metadata.NewRepository(db))

	router := chi.NewRouter()
	router.Get("/api/ai/runs/{id}", h.GetAgentRun)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, newRunsRequest("/api/ai/runs/missing"))
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
}

func TestListAgentRunsScopedToConversationOwner(t *testing.T) {
	db, state := setupMockDB(t)
	now := time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)
	state.queries = []queryMock{
		{
			Pattern: "SELECT EXISTS",
			Cols:    []string{"exists"},
			Rows:    [][]driver.Value{{true}},
		},
		{
			Pattern: "FROM agent_runs WHERE conversation_id",
			Cols: []string{
				"id", "conversation_id", "datasource_id", "model_id", "user_id",
				"question", "question_hash", "mode", "status", "confidence", "answer",
				"created_at", "updated_at",
			},
			Rows: [][]driver.Value{{
				"run-1", "conv-1", "ds-1", "", "user-1",
				"monthly revenue", "hash-1", "interactive", "completed", 0.9, "",
				now, now,
			}},
		},
	}
	h := newAIHandlerWithRepo(metadata.NewRepository(db))

	rec := httptest.NewRecorder()
	h.ListAgentRuns(rec, newRunsRequest("/api/ai/runs?conversation_id=conv-1"))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var out struct {
		Runs []map[string]any `json:"runs"`
	}
	require.NoError(t, sonic.Unmarshal(rec.Body.Bytes(), &out))
	require.Len(t, out.Runs, 1)
	assert.Equal(t, "run-1", out.Runs[0]["id"])
}

func TestListAgentRunsRejectsForeignConversation(t *testing.T) {
	db, state := setupMockDB(t)
	state.queries = []queryMock{
		{
			Pattern: "SELECT EXISTS",
			Cols:    []string{"exists"},
			Rows:    [][]driver.Value{{false}},
		},
	}
	h := newAIHandlerWithRepo(metadata.NewRepository(db))

	rec := httptest.NewRecorder()
	h.ListAgentRuns(rec, newRunsRequest("/api/ai/runs?conversation_id=other"))
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
}

func TestListAgentRunsRequiresConversationID(t *testing.T) {
	h := newAIHandlerWithRepo(nil)
	rec := httptest.NewRecorder()
	h.ListAgentRuns(rec, newRunsRequest("/api/ai/runs"))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func newRunsRequest(target string) *http.Request {
	ctx := middleware.WithUserID(context.Background(), "user-1")
	return httptest.NewRequestWithContext(ctx, http.MethodGet, target, http.NoBody)
}
