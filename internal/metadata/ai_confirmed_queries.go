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

// UpsertConfirmedQuery stores or refreshes a confirmed NL→query pair atomically.
// Uses INSERT … ON CONFLICT DO UPDATE so concurrent calls targeting the same
// logical key (datasource_id, question_hash, semantic_model_hash, model_id)
// never produce duplicate rows or a unique-violation.
func (r *Repository) UpsertConfirmedQuery(ctx context.Context, in ConfirmedQueryUpsert) error {
	embeddingJSON, err := encodeEmbedding(in.QuestionEmbedding)
	if err != nil {
		return err
	}
	var modelID any = in.ModelID
	if in.ModelID == "" {
		modelID = nil
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO ai_confirmed_queries (
			datasource_id, model_id, user_id, question_hash, nl_query, sql_query,
			semantic_model_hash, question_embedding, is_active, confirmed_at
		) VALUES (
			$1::uuid, $2::uuid, NULLIF($3, ''), $4, $5, $6, $7, $8::jsonb, true, NOW()
		)
		ON CONFLICT (
			datasource_id, question_hash, semantic_model_hash,
			COALESCE(model_id, '00000000-0000-0000-0000-000000000000'::uuid)
		)
		DO UPDATE SET
			nl_query            = EXCLUDED.nl_query,
			sql_query           = EXCLUDED.sql_query,
			question_embedding  = EXCLUDED.question_embedding,
			user_id             = EXCLUDED.user_id,
			is_active           = true,
			confirmed_at        = NOW()
	`, in.DatasourceID, modelID, in.UserID, in.QuestionHash, in.NLQuery, in.SQLQuery, in.SemanticModelHash, embeddingJSON)
	if err != nil {
		return fmt.Errorf("upsert confirmed query: %w", err)
	}
	return nil
}

// ListActiveConfirmedQueries returns recent active confirmed pairs for recall ranking.
func (r *Repository) ListActiveConfirmedQueries(ctx context.Context, datasourceID, modelID, semanticModelHash string, limit int) ([]ConfirmedQueryRow, error) {
	if limit <= 0 {
		limit = ConfirmedQueriesCandidatePool
	}
	q := `
		SELECT id::text, datasource_id::text, COALESCE(model_id::text, ''), COALESCE(user_id, ''),
			question_hash, nl_query, sql_query, semantic_model_hash, question_embedding, is_active
		FROM ai_confirmed_queries
		WHERE datasource_id = $1::uuid
		  AND is_active = true
		  AND semantic_model_hash = $2
		  AND ($3::uuid IS NULL OR model_id = $3::uuid OR model_id IS NULL)
		ORDER BY confirmed_at DESC
		LIMIT $4
	`
	var modelArg any
	if modelID != "" {
		modelArg = modelID
	}
	return platformdb.QuerySliceErr(ctx, r.db, "list active confirmed queries", q,
		[]any{datasourceID, semanticModelHash, modelArg, limit},
		scanConfirmedQueryRow)
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
	col := "confirmed_at"
	switch sortBy {
	case "question":
		col = "nl_query"
	case "status":
		col = "is_active"
	case "confirmed_at":
		col = "confirmed_at"
	}
	dir := "DESC"
	if sortDir == "asc" {
		dir = "ASC"
	}
	return col + " " + dir
}

// ListConfirmedQueriesForAdmin returns confirmed pairs for a datasource
// regardless of model hash or active state (admin review listing).
func (r *Repository) ListConfirmedQueriesForAdmin(ctx context.Context, p ConfirmedQueriesAdminListParams) ([]ConfirmedQueryAdminRow, error) {
	limit := p.Limit
	if limit <= 0 {
		limit = ConfirmedQueriesCandidatePool
	}
	offset := max(p.Offset, 0)
	order := confirmedQueriesAdminOrderClause(p.SortBy, p.SortDir)
	q := `
		SELECT id::text, datasource_id::text, COALESCE(model_id::text, ''), COALESCE(user_id, ''),
			nl_query, sql_query, semantic_model_hash, is_active, confirmed_at
		FROM ai_confirmed_queries
		WHERE datasource_id = $1::uuid
		ORDER BY ` + order + `
		LIMIT $2 OFFSET $3
	`
	return platformdb.QuerySliceErr(ctx, r.db, "list confirmed queries for admin", q,
		[]any{p.DatasourceID, limit, offset},
		func(s platformdb.Scanner) (ConfirmedQueryAdminRow, error) {
			var row ConfirmedQueryAdminRow
			if err := s.Scan(
				&row.ID, &row.DatasourceID, &row.ModelID, &row.UserID,
				&row.NLQuery, &row.SQLQuery, &row.SemanticModelHash,
				&row.IsActive, &row.ConfirmedAt,
			); err != nil {
				return row, fmt.Errorf("scan confirmed query admin row: %w", err)
			}
			return row, nil
		})
}

// CountConfirmedQueriesForAdmin returns how many confirmed pairs exist for a datasource.
func (r *Repository) CountConfirmedQueriesForAdmin(ctx context.Context, datasourceID string) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*)::int FROM ai_confirmed_queries WHERE datasource_id = $1::uuid
	`, datasourceID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count confirmed queries for admin: %w", err)
	}
	return n, nil
}

// SetConfirmedQueryActive toggles a single confirmed pair's recall eligibility.
// It returns the number of rows updated (0 when the id does not exist).
func (r *Repository) SetConfirmedQueryActive(ctx context.Context, id string, active bool) (int64, error) {
	res, err := r.db.ExecContext(ctx, `
		UPDATE ai_confirmed_queries SET is_active = $2 WHERE id = $1::uuid
	`, id, active)
	if err != nil {
		return 0, fmt.Errorf("set confirmed query active: %w", err)
	}
	return res.RowsAffected()
}

// DeactivateConfirmedQueriesExceptHash marks stale pairs inactive after a model publish.
func (r *Repository) DeactivateConfirmedQueriesExceptHash(ctx context.Context, modelID, semanticModelHash string) (int64, error) {
	res, err := r.db.ExecContext(ctx, `
		UPDATE ai_confirmed_queries
		SET is_active = false
		WHERE model_id = $1::uuid
		  AND is_active = true
		  AND semantic_model_hash <> $2
	`, modelID, semanticModelHash)
	if err != nil {
		return 0, fmt.Errorf("deactivate confirmed queries: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return n, nil
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
