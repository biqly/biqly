package metadata

import (
	"context"
	"database/sql/driver"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAIConversationRepositoryCRUD(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()
	now := time.Date(2026, 6, 12, 17, 45, 0, 0, time.UTC)

	state.queries = []queryMock{
		{
			Pattern: "INSERT INTO ai_conversations",
			Cols:    []string{"id", "created_at", "updated_at"},
			Rows:    [][]driver.Value{{"conv-1", now, now}},
		},
		{
			Pattern: "INSERT INTO ai_conversation_messages",
			Cols:    []string{"id", "created_at"},
			Rows:    [][]driver.Value{{"msg-1", now}},
		},
		{
			Pattern: "FROM ai_conversations",
			Cols: []string{
				"id", "user_id", "datasource_id", "model_id", "context_enabled", "title",
				"snapshot_version", "created_at", "updated_at",
				"message_id", "message_remote_id", "message_ordinal", "message_role",
				"message_content", "message_ai_response", "message_result_summary",
				"message_created_at",
			},
			Rows: [][]driver.Value{{
				"conv-1", "user-1", "ds-1", "model-1", true, "Tweets",
				int64(0), now, now, "msg-1", "remote-1", int64(3), "assistant", "May 20 won",
				[]byte(`{"sql":"SELECT 1"}`), "date=2026-05-20, tweet_count=2932", now,
			}},
		},
	}
	state.execs = []execMock{{Pattern: "DELETE FROM ai_conversations", RowsAffected: 1}}

	conv := &AIConversation{
		UserID:         "user-1",
		DatasourceID:   "ds-1",
		ModelID:        new("model-1"),
		ContextEnabled: true,
		Title:          new("Tweets"),
	}
	require.NoError(t, repo.CreateAIConversation(ctx, conv))
	assert.Equal(t, "conv-1", conv.ID)

	msg := &AIConversationMessage{
		ConversationID: "conv-1",
		Role:           "assistant",
		Content:        "May 20 won",
		AIResponse:     map[string]any{"sql": "SELECT 1"},
		ResultSummary:  new("date=2026-05-20, tweet_count=2932"),
	}
	require.NoError(t, repo.CreateAIConversationMessage(ctx, msg))
	assert.Equal(t, "msg-1", msg.ID)

	conversations, err := repo.ListAIConversations(ctx, "user-1", 20)
	require.NoError(t, err)
	require.Len(t, conversations, 1)
	assert.Equal(t, "conv-1", conversations[0].ID)
	require.Len(t, conversations[0].Messages, 1)
	assert.Equal(t, "assistant", conversations[0].Messages[0].Role)
	// remote_id/ordinal must round-trip; a lost remote_id makes the client mint
	// a new one and re-insert the whole history on the next snapshot save.
	assert.Equal(t, "remote-1", conversations[0].Messages[0].RemoteID)
	assert.Equal(t, 3, conversations[0].Messages[0].Ordinal)
	assert.Equal(t, map[string]any{"sql": "SELECT 1"}, conversations[0].Messages[0].AIResponse)
	assert.Equal(t, "date=2026-05-20, tweet_count=2932", *conversations[0].Messages[0].ResultSummary)

	deleted, err := repo.DeleteAIConversation(ctx, "conv-1", "user-1")
	require.NoError(t, err)
	assert.True(t, deleted)
}
