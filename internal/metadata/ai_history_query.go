package metadata

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/lib/pq"
)

const aiHistorySelectCols = `id, datasource_id, model_id, user_id, question, prompt_context,
		       ai_response, logical_query, confidence_score, warnings, outcome_status,
		       retry_count, needs_clarification, model_used, prompt_tokens, completion_tokens,
		       token_count, cost_usd, latency_ms, created_at, ab_experiment_id, ab_variant_id`

// AIHistoryListFilter drives paginated ai_query_history listing.
type AIHistoryListFilter struct {
	UserID        string
	DatasourceID  string
	DatasourceIDs []string
	ModelID       string
	Status        string
	Search        string
	Page          int
	PageSize      int
}

// AIHistoryListResult is a page of AI history rows plus total matching count.
type AIHistoryListResult struct {
	Entries []AIQueryHistoryEntry
	Total   int
}

func (f AIHistoryListFilter) offset() int {
	if f.Page < 1 {
		return 0
	}
	return (f.Page - 1) * f.PageSize
}

func buildAIHistoryWhere(filter AIHistoryListFilter) (string, []any) {
	var (
		parts []string
		args  []any
	)
	parts = append(parts, "TRUE")

	if filter.UserID != "" {
		args = append(args, filter.UserID)
		parts = append(parts, fmt.Sprintf("user_id = $%d", len(args)))
	}
	if filter.DatasourceID != "" {
		args = append(args, filter.DatasourceID)
		parts = append(parts, fmt.Sprintf("datasource_id = $%d::uuid", len(args)))
	} else if len(filter.DatasourceIDs) > 0 {
		args = append(args, pq.Array(filter.DatasourceIDs))
		parts = append(parts, fmt.Sprintf("datasource_id = ANY($%d::uuid[])", len(args)))
	}
	if filter.ModelID != "" {
		args = append(args, filter.ModelID)
		parts = append(parts, fmt.Sprintf("model_id = $%d::uuid", len(args)))
	}
	switch filter.Status {
	case "clarification":
		parts = append(parts, "needs_clarification = TRUE")
	case "success":
		parts = append(parts, "needs_clarification = FALSE", "outcome_status = 'success'")
	case "error":
		parts = append(parts, "needs_clarification = FALSE", "(outcome_status IS NULL OR outcome_status <> 'success')")
	}
	if q := strings.TrimSpace(filter.Search); q != "" {
		args = append(args, "%"+q+"%")
		parts = append(parts, fmt.Sprintf("question ILIKE $%d", len(args)))
	}

	return strings.Join(parts, " AND "), args
}

// ListAIQueryHistoryFiltered returns one page of AI history rows and the total
// matching count for the same filter (newest first).
func (r *Repository) ListAIQueryHistoryFiltered(ctx context.Context, filter AIHistoryListFilter) (AIHistoryListResult, error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 10
	}

	where, args := buildAIHistoryWhere(filter)

	var total int
	countQ := `SELECT COUNT(*) FROM ai_query_history WHERE ` + where // nosemgrep: go.lang.security.audit.database.string-formatted-query.string-formatted-query
	if err := r.db.QueryRowContext(ctx, countQ, args...).Scan(&total); err != nil {
		return AIHistoryListResult{}, fmt.Errorf("count AI history: %w", err)
	}

	listArgs := append(append([]any(nil), args...), filter.PageSize, filter.offset())
	limitArg := len(args) + 1
	offsetArg := len(args) + 2
	listQ := `SELECT ` + aiHistorySelectCols + ` FROM ai_query_history WHERE ` + where + //nolint:gosec // nosemgrep: go.lang.security.audit.database.string-formatted-query.string-formatted-query
		` ORDER BY created_at DESC LIMIT $` + strconv.Itoa(limitArg) +
		` OFFSET $` + strconv.Itoa(offsetArg)

	rows, err := r.db.QueryContext(ctx, listQ, listArgs...)
	if err != nil {
		return AIHistoryListResult{}, fmt.Errorf("list AI history filtered: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			_ = err
		}
	}()

	entries := make([]AIQueryHistoryEntry, 0, filter.PageSize)
	for rows.Next() {
		entry, err := scanAIHistoryEntry(rows)
		if err != nil {
			return AIHistoryListResult{}, err
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return AIHistoryListResult{}, err
	}
	if entries == nil {
		entries = []AIQueryHistoryEntry{}
	}
	return AIHistoryListResult{Entries: entries, Total: total}, nil
}

// GetAIQueryHistoryByID returns one AI query history row by primary key.
func (r *Repository) GetAIQueryHistoryByID(ctx context.Context, id string) (*AIQueryHistoryEntry, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+aiHistorySelectCols+` FROM ai_query_history WHERE id = $1::uuid`, id)
	entry, err := scanAIHistoryEntry(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, fmt.Errorf("get AI history by id: %w", err)
	}
	return &entry, nil
}
