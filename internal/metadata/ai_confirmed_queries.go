package metadata

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	platformdb "github.com/biqly/biqly/internal/platform/db"
)

// ConfirmedQueriesCandidatePool caps how many active confirmed pairs we load before ranking.
const ConfirmedQueriesCandidatePool = 50

// ConfirmedQueryRow is a user-confirmed NL→query pair for few-shot recall.
type ConfirmedQueryRow struct {
	ID                string
	DatasourceID      string
	ModelID           string
	UserID            string
	QuestionHash      string
	NLQuery           string
	SQLQuery          string
	SemanticModelHash string
	QuestionEmbedding []float32
	IsActive          bool
}

// ConfirmedQueryUpsert is input for storing a confirmed query pair.
type ConfirmedQueryUpsert struct {
	DatasourceID      string
	ModelID           string
	UserID            string
	QuestionHash      string
	NLQuery           string
	SQLQuery          string
	SemanticModelHash string
	QuestionEmbedding []float32
}

// AIQueryHistoryFeedbackRow is the latest history row used when recording feedback.
type AIQueryHistoryFeedbackRow struct {
	ID           string
	ModelID      string
	LogicalQuery []byte
}

// QuestionHash returns a stable SHA-256 hex digest for a natural-language question.
func QuestionHash(question string) string {
	normalized := strings.TrimSpace(strings.ToLower(question))
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])
}

// SemanticModelHash returns the version pin used to invalidate stale confirmed pairs.
func SemanticModelHash(modelID string, version int) string {
	if modelID == "" {
		return ""
	}
	return fmt.Sprintf("%s@%d", modelID, version)
}

// ConfirmedQueryAdminRow is the admin-facing view of a confirmed pair,
// including inactive rows and the confirmation timestamp.
type ConfirmedQueryAdminRow struct {
	ID                string
	DatasourceID      string
	ModelID           string
	UserID            string
	NLQuery           string
	SQLQuery          string
	SemanticModelHash string
	IsActive          bool
	ConfirmedAt       time.Time
}

// ConfirmedQueriesAdminListParams scopes the admin review listing query.
type ConfirmedQueriesAdminListParams struct {
	DatasourceID string
	Limit        int
	Offset       int
	SortBy       string
	SortDir      string
}

func confirmedQueriesAdminOrderClause(sortBy, sortDir string) string {
	col := "created_at"
	switch sortBy {
	case "question":
		col = "question"
	case "status":
		col = "is_active"
	case "confirmed_at":
		col = "created_at"
	}
	dir := "DESC"
	if sortDir == "asc" {
		dir = "ASC"
	}
	return col + " " + dir
}

// GetLatestAIQueryHistoryForFeedback returns the newest history row for feedback correlation.
func (r *Repository) GetLatestAIQueryHistoryForFeedback(ctx context.Context, datasourceID, userID, question string) (*AIQueryHistoryFeedbackRow, error) {
	var row AIQueryHistoryFeedbackRow
	var modelID, lq []byte
	err := r.db.QueryRowContext(ctx, `
		SELECT id::text, COALESCE(model_id::text, ''), logical_query
		FROM ai_query_history
		WHERE datasource_id = $1::uuid AND user_id = $2 AND question = $3
		  AND logical_query IS NOT NULL
		ORDER BY created_at DESC
		LIMIT 1
	`, datasourceID, userID, question).Scan(&row.ID, &modelID, &lq)
	if err != nil {
		return nil, err
	}
	row.ModelID = string(modelID)
	row.LogicalQuery = lq
	return &row, nil
}

func scanConfirmedQueryRow(s platformdb.Scanner) (ConfirmedQueryRow, error) {
	var row ConfirmedQueryRow
	var embeddingRaw []byte
	if err := s.Scan(
		&row.ID, &row.DatasourceID, &row.ModelID, &row.UserID,
		&row.QuestionHash, &row.NLQuery, &row.SQLQuery, &row.SemanticModelHash,
		&embeddingRaw, &row.IsActive,
	); err != nil {
		return row, fmt.Errorf("scan confirmed query: %w", err)
	}
	vec, err := decodeEmbedding(embeddingRaw)
	if err != nil {
		return row, err
	}
	row.QuestionEmbedding = vec
	return row, nil
}
