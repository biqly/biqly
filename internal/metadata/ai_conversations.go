package metadata

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	platformdb "github.com/biqly/biqly/internal/platform/db"
	"github.com/bytedance/sonic"
)

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
			$2, $3, NULLIF($4, ''), $5, NULLIF($6, '')
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
		platformdb.NullIfEmptyPtr(conv.ModelID),
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
			SELECT id, user_id, datasource_id, model_id, context_enabled, title, created_at, updated_at
			FROM ai_conversations
			WHERE user_id = $1
			ORDER BY updated_at DESC
			LIMIT $2
		)
		SELECT c.id, c.user_id, c.datasource_id, c.model_id, c.context_enabled, c.title,
		       c.created_at, c.updated_at,
		       m.id AS message_id, m.role AS message_role, m.content AS message_content,
		       m.ai_response AS message_ai_response, m.result_summary AS message_result_summary,
		       m.created_at AS message_created_at
		FROM limited_conversations c
		LEFT JOIN ai_conversation_messages m ON m.conversation_id = c.id
		ORDER BY c.updated_at DESC, m.created_at ASC
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

func scanAIConversationRow(s platformdb.Scanner) (AIConversation, *AIConversationMessage, error) {
	var conv AIConversation
	var modelID, title sql.NullString
	var msgID, msgRole, msgContent, msgResultSummary sql.NullString
	var msgAIResponse []byte
	var msgCreatedAt sql.NullTime
	if err := s.Scan(
		&conv.ID, &conv.UserID, &conv.DatasourceID, &modelID, &conv.ContextEnabled, &title,
		&conv.CreatedAt, &conv.UpdatedAt,
		&msgID, &msgRole, &msgContent, &msgAIResponse, &msgResultSummary, &msgCreatedAt,
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
	msg := &AIConversationMessage{
		ID:             msgID.String,
		ConversationID: conv.ID,
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
