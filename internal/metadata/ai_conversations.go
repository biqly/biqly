package metadata

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	platformdb "github.com/biqly/biqly/internal/platform/db"
	"github.com/bytedance/sonic"
)

// Conversation snapshot conflicts and idempotency errors.
var (
	ErrConversationVersionConflict = errors.New("conversation version conflict")
	ErrConversationMessageConflict = errors.New("conversation message conflict")
	ErrIdempotencyKeyConflict      = errors.New("idempotency key conflict")
)

// ConversationSnapshotWrite is the input for an atomic conversation snapshot save.
type ConversationSnapshotWrite struct {
	Conversation    AIConversation
	ExpectedVersion int64
	IdempotencyKey  string
	PayloadHash     string
}

// ConversationSnapshotResult is the outcome of a snapshot save.
type ConversationSnapshotResult struct {
	Conversation AIConversation
	StatusCode   int
}

// CreateAIConversation inserts or updates a persisted AI conversation.
func (r *Repository) CreateAIConversation(ctx context.Context, conv *AIConversation) error {
	if conv == nil {
		return errors.New("create AI conversation: nil conversation")
	}
	const query = `
		INSERT INTO ai_conversations (
			id, user_id, datasource_id, model_id, context_enabled, title
		)
		VALUES (
			COALESCE(NULLIF($1, ''), gen_random_uuid()::text),
			$2, $3, NULLIF($4, '')::uuid, $5, NULLIF($6, '')
		)
		ON CONFLICT (id) DO UPDATE SET
			user_id = EXCLUDED.user_id,
			datasource_id = EXCLUDED.datasource_id,
			model_id = EXCLUDED.model_id,
			context_enabled = EXCLUDED.context_enabled,
			title = EXCLUDED.title,
			updated_at = now()
		RETURNING id, created_at, updated_at
	`
	if err := r.db.QueryRowContext(ctx, query,
		conv.ID,
		conv.UserID,
		conv.DatasourceID,
		platformdb.NullUUIDPtr(conv.ModelID),
		conv.ContextEnabled,
		platformdb.NullIfEmptyPtr(conv.Title),
	).Scan(&conv.ID, &conv.CreatedAt, &conv.UpdatedAt); err != nil {
		return fmt.Errorf("create AI conversation: %w", err)
	}
	return nil
}

// CreateAIConversationMessage inserts or updates one persisted conversation turn.
func (r *Repository) CreateAIConversationMessage(ctx context.Context, msg *AIConversationMessage) error {
	if msg == nil {
		return errors.New("create AI conversation message: nil message")
	}
	aiResponse, err := nullableJSON(msg.AIResponse)
	if err != nil {
		return fmt.Errorf("encode AI conversation message response: %w", err)
	}
	const query = `
		INSERT INTO ai_conversation_messages (
			id, conversation_id, role, content, ai_response, result_summary
		)
		VALUES (
			COALESCE(NULLIF($1, ''), gen_random_uuid()::text),
			$2, $3, $4, $5::jsonb, NULLIF($6, '')
		)
		ON CONFLICT (id) DO UPDATE SET
			role = EXCLUDED.role,
			content = EXCLUDED.content,
			ai_response = EXCLUDED.ai_response,
			result_summary = EXCLUDED.result_summary
		RETURNING id, created_at
	`
	if err := r.db.QueryRowContext(ctx, query,
		msg.ID,
		msg.ConversationID,
		msg.Role,
		msg.Content,
		aiResponse,
		platformdb.NullIfEmptyPtr(msg.ResultSummary),
	).Scan(&msg.ID, &msg.CreatedAt); err != nil {
		return fmt.Errorf("create AI conversation message: %w", err)
	}
	return nil
}

// ListAIConversations returns a user's newest conversations with messages.
func (r *Repository) ListAIConversations(ctx context.Context, userID string, limit int) ([]AIConversation, error) {
	if limit <= 0 {
		limit = 50
	}
	const query = `
		WITH limited_conversations AS (
			SELECT id, user_id, datasource_id, model_id, context_enabled, title, snapshot_version, created_at, updated_at
			FROM ai_conversations
			WHERE user_id = $1
			ORDER BY updated_at DESC
			LIMIT $2
		)
		SELECT c.id, c.user_id, c.datasource_id, c.model_id, c.context_enabled, c.title,
		       c.snapshot_version, c.created_at, c.updated_at,
		       m.id AS message_id, m.remote_id AS message_remote_id, m.ordinal AS message_ordinal,
		       m.role AS message_role, m.content AS message_content,
		       m.ai_response AS message_ai_response, m.result_summary AS message_result_summary,
		       m.created_at AS message_created_at
		FROM limited_conversations c
		LEFT JOIN ai_conversation_messages m ON m.conversation_id = c.id AND m.deleted_at IS NULL
		ORDER BY c.updated_at DESC, COALESCE(m.ordinal, 0), m.created_at ASC
	`
	rows, err := r.db.QueryContext(ctx, query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("list AI conversations: %w", err)
	}
	defer func() { _ = rows.Close() }()

	conversationsByID := make(map[string]int)
	conversations := make([]AIConversation, 0)
	for rows.Next() {
		conv, msg, err := scanAIConversationRow(rows)
		if err != nil {
			return nil, err
		}
		index, ok := conversationsByID[conv.ID]
		if !ok {
			conversations = append(conversations, conv)
			index = len(conversations) - 1
			conversationsByID[conv.ID] = index
		}
		if msg != nil {
			conversations[index].Messages = append(conversations[index].Messages, *msg)
		}
	}
	return conversations, rows.Err()
}

// DeleteAIConversation deletes a conversation only when it belongs to userID.
func (r *Repository) DeleteAIConversation(ctx context.Context, id string, userID string) (bool, error) {
	res, err := r.db.ExecContext(ctx, `DELETE FROM ai_conversations WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return false, fmt.Errorf("delete AI conversation: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("delete AI conversation rows affected: %w", err)
	}
	return rows > 0, nil
}

// ConversationBelongsToUser reports whether the conversation exists and is
// owned by userID. Used to scope conversation-derived listings (e.g. agent
// runs) to their owner.
func (r *Repository) ConversationBelongsToUser(ctx context.Context, id, userID string) (bool, error) {
	if id == "" || userID == "" {
		return false, nil
	}
	var exists bool
	err := r.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM ai_conversations WHERE id = $1 AND user_id = $2)`,
		id, userID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("conversation belongs to user: %w", err)
	}
	return exists, nil
}

func scanAIConversationRow(s platformdb.Scanner) (AIConversation, *AIConversationMessage, error) {
	var conv AIConversation
	var modelID, title sql.NullString
	var msgID, msgRemoteID, msgRole, msgContent, msgResultSummary sql.NullString
	var msgOrdinal sql.NullInt64
	var msgAIResponse []byte
	var msgCreatedAt sql.NullTime
	if err := s.Scan(
		&conv.ID, &conv.UserID, &conv.DatasourceID, &modelID, &conv.ContextEnabled, &title,
		&conv.SnapshotVersion, &conv.CreatedAt, &conv.UpdatedAt,
		&msgID, &msgRemoteID, &msgOrdinal, &msgRole, &msgContent, &msgAIResponse,
		&msgResultSummary, &msgCreatedAt,
	); err != nil {
		return conv, nil, fmt.Errorf("scan AI conversation row: %w", err)
	}
	if modelID.Valid {
		conv.ModelID = new(modelID.String)
	}
	if title.Valid {
		conv.Title = new(title.String)
	}
	if !msgID.Valid {
		return conv, nil, nil
	}
	// remote_id/ordinal must round-trip: the client dedupes snapshot upserts by
	// remote_id, and regenerating it on every load re-inserts the whole history
	// (replay-chain duplication).
	msg := &AIConversationMessage{
		ID:             msgID.String,
		RemoteID:       msgRemoteID.String,
		ConversationID: conv.ID,
		Ordinal:        int(msgOrdinal.Int64),
		Role:           msgRole.String,
		Content:        msgContent.String,
		CreatedAt:      msgCreatedAt.Time,
	}
	if msgResultSummary.Valid {
		msg.ResultSummary = new(msgResultSummary.String)
	}
	if len(msgAIResponse) > 0 {
		if err := sonic.Unmarshal(msgAIResponse, &msg.AIResponse); err != nil {
			return conv, nil, fmt.Errorf("decode AI conversation message response: %w", err)
		}
	}
	return conv, msg, nil
}

// SaveAIConversationSnapshot atomically persists a conversation snapshot within
// a single transaction. It enforces idempotency via the Idempotency-Key ledger,
// optimistic concurrency via snapshot_version, and message deduplication via
// (conversation_id, remote_id). The full snapshot rolls back on any failure.
func (r *Repository) SaveAIConversationSnapshot(
	ctx context.Context,
	userID string,
	in ConversationSnapshotWrite,
) (ConversationSnapshotResult, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return ConversationSnapshotResult{}, fmt.Errorf("begin snapshot tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// 1. Check idempotency ledger — replay if this request already completed.
	if replayed, ok, err := r.checkIdempotencyLedger(ctx, tx, in); ok {
		return replayed, err
	} else if err != nil {
		return ConversationSnapshotResult{}, err
	}

	// 2. Upsert conversation and lock for version check.
	conv := in.Conversation
	if err := upsertConversationInTx(ctx, tx, userID, &conv, in.ExpectedVersion); err != nil {
		return ConversationSnapshotResult{}, err
	}

	// 3. Reserve the idempotency key after the parent conversation exists.
	reservation := in
	reservation.Conversation = conv
	if err := r.reserveIdempotencyKey(ctx, tx, userID, reservation); err != nil {
		return ConversationSnapshotResult{}, err
	}

	// 4. Upsert messages by (conversation_id, remote_id).
	if err := upsertMessagesInTx(ctx, tx, &conv); err != nil {
		return ConversationSnapshotResult{}, err
	}

	// 5. Complete idempotency ledger and commit.
	statusCode := 201
	if _, err := tx.ExecContext(ctx, `
		UPDATE conversation_write_requests
		SET status = 'completed', response_status = $2, completed_at = now()
		WHERE idempotency_key = $1
	`, in.IdempotencyKey, statusCode); err != nil {
		return ConversationSnapshotResult{}, fmt.Errorf("complete idempotency ledger: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return ConversationSnapshotResult{}, fmt.Errorf("commit snapshot tx: %w", err)
	}
	return ConversationSnapshotResult{Conversation: conv, StatusCode: statusCode}, nil
}

// checkIdempotencyLedger returns (result, true, nil) if the key already exists
// (replay or conflict). Returns (_, false, nil) if the key is new.
func (*Repository) checkIdempotencyLedger(
	ctx context.Context,
	tx *sql.Tx,
	in ConversationSnapshotWrite,
) (ConversationSnapshotResult, bool, error) {
	var storedPayloadHash string
	var storedResponseStatus sql.NullInt64
	err := tx.QueryRowContext(ctx, `
		SELECT response_status, payload_hash
		FROM conversation_write_requests
		WHERE idempotency_key = $1
	`, in.IdempotencyKey).Scan(&storedResponseStatus, &storedPayloadHash)
	if err == nil {
		if storedPayloadHash != in.PayloadHash {
			return ConversationSnapshotResult{}, true, ErrIdempotencyKeyConflict
		}
		statusCode := 201
		if storedResponseStatus.Valid {
			statusCode = int(storedResponseStatus.Int64)
		}
		return ConversationSnapshotResult{Conversation: in.Conversation, StatusCode: statusCode}, true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return ConversationSnapshotResult{}, true, fmt.Errorf("query idempotency ledger: %w", err)
	}
	return ConversationSnapshotResult{}, false, nil
}

func (*Repository) reserveIdempotencyKey(
	ctx context.Context,
	tx *sql.Tx,
	userID string,
	in ConversationSnapshotWrite,
) error {
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO conversation_write_requests (idempotency_key, user_id, conversation_id, payload_hash, status)
		VALUES ($1, $2, $3, $4, 'processing')
	`, in.IdempotencyKey, userID, in.Conversation.ID, in.PayloadHash); err != nil {
		return fmt.Errorf("reserve idempotency key: %w", err)
	}
	return nil
}

func upsertConversationInTx(
	ctx context.Context,
	tx *sql.Tx,
	userID string,
	conv *AIConversation,
	expectedVersion int64,
) error {
	if conv.ID == "" {
		return insertConversationInTx(ctx, tx, userID, conv)
	}
	// Existing conversation — lock, check version, bump.
	var currentVersion int64
	err := tx.QueryRowContext(ctx, `
		SELECT snapshot_version FROM ai_conversations
		WHERE id = $1 AND user_id = $2
		FOR UPDATE
	`, conv.ID, userID).Scan(&currentVersion)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Client-provided ID is set but doesn't exist yet — create it
			// (e.g. client-side conversation hydration before first server save).
			return insertConversationInTx(ctx, tx, userID, conv)
		}
		return ErrConversationVersionConflict
	}
	if currentVersion != expectedVersion {
		return ErrConversationVersionConflict
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE ai_conversations
		SET datasource_id = $2::uuid, model_id = NULLIF($3, '')::uuid,
		    context_enabled = $4, title = NULLIF($5, ''),
		    snapshot_version = snapshot_version + 1, updated_at = now()
		WHERE id = $1 AND user_id = $6
	`, conv.ID, conv.DatasourceID, derefStringOrEmpty(conv.ModelID), conv.ContextEnabled, derefStringOrEmpty(conv.Title), userID); err != nil {
		return fmt.Errorf("update conversation: %w", err)
	}
	conv.SnapshotVersion = currentVersion + 1
	return nil
}

// insertConversationInTx inserts a new conversation row, accepting either a
// client-generated ID or an empty ID (DB generates via gen_random_uuid).
func insertConversationInTx(
	ctx context.Context,
	tx *sql.Tx,
	userID string,
	conv *AIConversation,
) error {
	return tx.QueryRowContext(ctx, `
		INSERT INTO ai_conversations (id, user_id, datasource_id, model_id, context_enabled, title, snapshot_version)
		VALUES (COALESCE(NULLIF($1, ''), gen_random_uuid()::text), $2, $3, NULLIF($4, '')::uuid, $5, NULLIF($6, ''), 1)
		RETURNING id::text, snapshot_version, created_at, updated_at
	`, conv.ID, userID, conv.DatasourceID, derefStringOrEmpty(conv.ModelID), conv.ContextEnabled, derefStringOrEmpty(conv.Title),
	).Scan(&conv.ID, &conv.SnapshotVersion, &conv.CreatedAt, &conv.UpdatedAt)
}

func upsertMessagesInTx(ctx context.Context, tx *sql.Tx, conv *AIConversation) error {
	for i := range conv.Messages {
		msg := &conv.Messages[i]
		msg.ConversationID = conv.ID
		aiResponse, err := nullableJSON(msg.AIResponse)
		if err != nil {
			return fmt.Errorf("encode message response: %w", err)
		}
		var msgID string
		var msgCreatedAt time.Time
		err = tx.QueryRowContext(ctx, `
			INSERT INTO ai_conversation_messages (
				id, conversation_id, remote_id, ordinal, role, content, ai_response, result_summary
			)
			VALUES (
				COALESCE(NULLIF($1, ''), gen_random_uuid()::text),
				$2, NULLIF($3, ''), $4, $5, $6, $7::jsonb, NULLIF($8, '')
			)
			ON CONFLICT (conversation_id, remote_id) WHERE remote_id IS NOT NULL
			DO UPDATE SET
				ordinal = EXCLUDED.ordinal,
				updated_at = now()
			RETURNING id::text, created_at
		`, "", conv.ID, msg.RemoteID, msg.Ordinal, msg.Role, msg.Content,
			aiResponse, derefStringOrEmpty(msg.ResultSummary),
		).Scan(&msgID, &msgCreatedAt)
		if err != nil {
			return fmt.Errorf("upsert conversation message: %w", err)
		}
		msg.ID = msgID
		msg.CreatedAt = msgCreatedAt
	}
	return nil
}
