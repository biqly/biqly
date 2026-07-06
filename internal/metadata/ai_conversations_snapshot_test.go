package metadata

import (
	"context"
	"database/sql/driver"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveAIConversationSnapshotCreatesNewConversation(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()
	now := time.Date(2026, 7, 6, 10, 0, 0, 0, time.UTC)

	// Idempotency ledger: first SELECT returns no rows (ErrNoRows simulated by empty result).
	// Mock driver returns io.EOF for empty rows, which the sql layer turns into ErrNoRows.
	// But our mock returns "no mock query matched" error instead — so we provide a SELECT
	// that matches the idempotency query with zero rows.
	state.queries = []queryMock{
		// Idempotency ledger check — zero rows means key not found (ErrNoRows in real DB)
		{Pattern: "conversation_write_requests", Cols: []string{"response_status", "payload_hash"}, Rows: [][]driver.Value{}},
		// New conversation insert returning generated id
		{Pattern: "INSERT INTO ai_conversations", Cols: []string{"id", "snapshot_version", "created_at", "updated_at"}, Rows: [][]driver.Value{{"conv-1", int64(1), now, now}}},
		// Message upsert
		{Pattern: "INSERT INTO ai_conversation_messages", Cols: []string{"id", "created_at"}, Rows: [][]driver.Value{{"srv-msg-1", now}}},
	}
	// Execs: insert idempotency, update idempotency
	state.execs = []execMock{
		{Pattern: "conversation_write_requests", RowsAffected: 1},
	}

	conv := AIConversation{
		UserID:         "user-1",
		DatasourceID:   "ds-1",
		ContextEnabled: true,
		Messages: []AIConversationMessage{
			{RemoteID: "rm-1", Ordinal: 0, Role: "user", Content: "hello"},
		},
	}
	in := ConversationSnapshotWrite{
		Conversation:    conv,
		ExpectedVersion: 0,
		IdempotencyKey:  "idem-1",
		PayloadHash:     "hash-1",
	}
	result, err := repo.SaveAIConversationSnapshot(ctx, "user-1", in)
	require.NoError(t, err)
	assert.Equal(t, "conv-1", result.Conversation.ID)
	assert.Equal(t, int64(1), result.Conversation.SnapshotVersion)
	assert.Equal(t, 201, result.StatusCode)
}

func TestSaveAIConversationSnapshotRejectsStaleVersion(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	// The conversation exists but has version 5, we pass expected 3
	state.queries = []queryMock{
		// Idempotency ledger: not found (empty rows)
		{Pattern: "conversation_write_requests", Cols: []string{"response_status", "payload_hash"}, Rows: [][]driver.Value{}},
		// SELECT FOR UPDATE returns current version 5
		{Pattern: "FOR UPDATE", Cols: []string{"snapshot_version"}, Rows: [][]driver.Value{{int64(5)}}},
	}

	conv := AIConversation{ID: "conv-1", UserID: "user-1", DatasourceID: "ds-1"}
	in := ConversationSnapshotWrite{
		Conversation:    conv,
		ExpectedVersion: 3, // stale
		IdempotencyKey:  "idem-1",
		PayloadHash:     "hash-1",
	}
	_, err := repo.SaveAIConversationSnapshot(ctx, "user-1", in)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrConversationVersionConflict))
}

func TestSaveAIConversationSnapshotReplaysStoredIdempotentResponse(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	// The idempotency ledger already has a completed entry with the same key + hash
	state.queries = []queryMock{
		{
			Pattern: "conversation_write_requests",
			Cols:    []string{"response_status", "payload_hash"},
			Rows:    [][]driver.Value{{int64(201), "hash-1"}},
		},
	}

	in := ConversationSnapshotWrite{
		Conversation:    AIConversation{ID: "conv-1", UserID: "user-1", DatasourceID: "ds-1"},
		ExpectedVersion: 0,
		IdempotencyKey:  "idem-1",
		PayloadHash:     "hash-1",
	}
	result, err := repo.SaveAIConversationSnapshot(ctx, "user-1", in)
	require.NoError(t, err)
	assert.Equal(t, 201, result.StatusCode)
}

func TestSaveAIConversationSnapshotRejectsIdempotencyConflict(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	// The idempotency ledger has a completed entry with a DIFFERENT hash
	state.queries = []queryMock{
		{
			Pattern: "conversation_write_requests",
			Cols:    []string{"response_status", "payload_hash"},
			Rows:    [][]driver.Value{{int64(201), "different-hash"}},
		},
	}

	in := ConversationSnapshotWrite{
		Conversation:    AIConversation{ID: "conv-1", UserID: "user-1", DatasourceID: "ds-1"},
		ExpectedVersion: 0,
		IdempotencyKey:  "idem-1",
		PayloadHash:     "hash-1",
	}
	_, err := repo.SaveAIConversationSnapshot(ctx, "user-1", in)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrIdempotencyKeyConflict))
}

func TestListAIConversationsExcludesSoftDeletedMessages(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()
	now := time.Date(2026, 7, 6, 10, 0, 0, 0, time.UTC)

	state.queries = []queryMock{
		{
			Pattern: "FROM ai_conversations",
			Cols: []string{
				"id", "user_id", "datasource_id", "model_id", "context_enabled", "title",
				"snapshot_version", "created_at", "updated_at",
				"message_id", "message_role", "message_content",
				"message_ai_response", "message_result_summary", "message_created_at",
			},
			Rows: [][]driver.Value{
				{"conv-1", "user-1", "ds-1", nil, true, "T", int64(1), now, now,
					"msg-1", "user", "hello", nil, nil, now},
			},
		},
	}

	conversations, err := repo.ListAIConversations(ctx, "user-1", 20)
	require.NoError(t, err)
	require.Len(t, conversations, 1)
	require.Len(t, conversations[0].Messages, 1)
	assert.Equal(t, "hello", conversations[0].Messages[0].Content)
	assert.Equal(t, int64(1), conversations[0].SnapshotVersion)
}
