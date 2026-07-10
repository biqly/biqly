package handlers

import (
	"bytes"
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

func TestAIConversationsCreateListDelete(t *testing.T) {
	db, state := setupMockDB(t)
	now := time.Date(2026, 6, 12, 17, 55, 0, 0, time.UTC)
	state.queries = []queryMock{
		// Idempotency ledger check: empty rows (key not found)
		{Pattern: "conversation_write_requests", Cols: []string{"response_status", "payload_hash"}, Rows: [][]driver.Value{}},
		// New conversation insert
		{
			Pattern: "INSERT INTO ai_conversations",
			Cols:    []string{"id", "snapshot_version", "created_at", "updated_at"},
			Rows:    [][]driver.Value{{"conv-1", int64(1), now, now}},
		},
		{
			Pattern: "FROM ai_conversations",
			Cols: []string{
				"id", "user_id", "datasource_id", "model_id", "context_enabled", "title",
				"snapshot_version", "created_at", "updated_at",
				"message_id", "message_remote_id", "message_ordinal", "message_role", "message_content",
				"message_ai_response", "message_result_summary", "message_created_at",
			},
			Rows: [][]driver.Value{{
				"conv-1", "user-1", "ds-1", "model-1", true, "Tweets",
				int64(1), now, now, nil, nil, nil, nil, nil, nil, nil, nil,
			}},
		},
	}
	state.execs = []execMock{
		{Pattern: "conversation_write_requests", RowsAffected: 1},
		{Pattern: "DELETE FROM ai_conversations", RowsAffected: 1},
	}
	h := newAIHandlerWithRepo(metadata.NewRepository(db))

	createBody := []byte(`{"datasource_id":"ds-1","model_id":"model-1","title":"Tweets","context_enabled":true}`)
	rec := httptest.NewRecorder()
	h.CreateConversation(rec, newConversationRequest(http.MethodPost, "/api/ai/conversations", createBody))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var created metadata.AIConversation
	require.NoError(t, sonic.Unmarshal(rec.Body.Bytes(), &created))
	assert.Equal(t, "conv-1", created.ID)
	assert.Equal(t, "user-1", created.UserID)

	rec = httptest.NewRecorder()
	h.ListConversations(rec, newConversationRequest(http.MethodGet, "/api/ai/conversations", nil))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var list []metadata.AIConversation
	require.NoError(t, sonic.Unmarshal(rec.Body.Bytes(), &list))
	require.Len(t, list, 1)
	assert.Equal(t, "conv-1", list[0].ID)

	router := chi.NewRouter()
	router.Delete("/api/ai/conversations/{id}", h.DeleteConversation)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, newConversationRequest(http.MethodDelete, "/api/ai/conversations/conv-1", nil))
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestAIConversationsRequireUser(t *testing.T) {
	h := newAIHandlerWithRepo(nil)

	rec := httptest.NewRecorder()
	h.ListConversations(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/ai/conversations", http.NoBody))

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func newConversationRequest(method string, target string, body []byte) *http.Request {
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		reader = bytes.NewReader(body)
	}
	ctx := middleware.WithUserID(context.Background(), "user-1")
	req := httptest.NewRequestWithContext(ctx, method, target, reader)
	req.Header.Set("Content-Type", "application/json")
	return req
}
