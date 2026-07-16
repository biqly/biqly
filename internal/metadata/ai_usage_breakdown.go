package metadata

import (
	"context"
	"fmt"

	platformdb "github.com/biqly/biqly/internal/platform/db"
)

// AIUsageByUserModelRow aggregates AI query usage per user and LLM model label.
type AIUsageByUserModelRow struct {
	UserID           string  `json:"user_id"`
	ModelUsed        string  `json:"model_used"`
	QueryCount       int     `json:"query_count"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	TotalCostUSD     float64 `json:"total_cost_usd"`
	AvgLatencyMs     float64 `json:"avg_latency_ms"`
}

const aiUsageBreakdownBaseQuery = `
		SELECT
			COALESCE(h.user_id, '') AS user_id,
			COALESCE(NULLIF(TRIM(h.model_used), ''), 'unknown') AS model_used,
			COUNT(*)::int AS query_count,
			COALESCE(SUM(h.prompt_tokens), 0)::int AS prompt_tokens,
			COALESCE(SUM(h.completion_tokens), 0)::int AS completion_tokens,
			COALESCE(SUM(h.token_count), 0)::int AS total_tokens,
			COALESCE(SUM(h.cost_usd), 0) AS total_cost_usd,
			COALESCE(AVG(h.latency_ms), 0) AS avg_latency_ms
		FROM ai_query_history h
		WHERE h.created_at >= NOW() - ($1::int * INTERVAL '1 day')
		GROUP BY h.user_id, COALESCE(NULLIF(TRIM(h.model_used), ''), 'unknown')
`

// ListAIUsageByUserAndModelPaged returns one page of usage grouped by user and model.
func (r *Repository) ListAIUsageByUserAndModelPaged(ctx context.Context, days, limit, offset int) ([]AIUsageByUserModelRow, error) {
	if days <= 0 {
		days = 30
	}
	if limit <= 0 {
		limit = 25
	}
	offset = max(offset, 0)
	q := `
		WITH breakdown AS (` + aiUsageBreakdownBaseQuery + `
		)
		SELECT user_id, model_used, query_count, prompt_tokens, completion_tokens,
			total_tokens, total_cost_usd, avg_latency_ms
		FROM breakdown
		ORDER BY total_tokens DESC, query_count DESC
		LIMIT $2 OFFSET $3
	`
	return platformdb.QuerySliceErr(ctx, r.db, "list AI usage by user and model", q, []any{days, limit, offset}, scanAIUsageByUserModelRow)
}

// CountAIUsageByUserAndModelGroups returns how many user/model groups exist in the window.
func (r *Repository) CountAIUsageByUserAndModelGroups(ctx context.Context, days int) (int, error) {
	if days <= 0 {
		days = 30
	}
	var n int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*)::int FROM (`+aiUsageBreakdownBaseQuery+`) breakdown
	`, days).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count AI usage breakdown groups: %w", err)
	}
	return n, nil
}

func scanAIUsageByUserModelRow(s platformdb.Scanner) (AIUsageByUserModelRow, error) {
	var row AIUsageByUserModelRow
	if err := s.Scan(
		&row.UserID, &row.ModelUsed,
		&row.QueryCount, &row.PromptTokens, &row.CompletionTokens, &row.TotalTokens,
		&row.TotalCostUSD, &row.AvgLatencyMs,
	); err != nil {
		return AIUsageByUserModelRow{}, err
	}
	return row, nil
}

// AIUserUsage aggregates one user's own AI query usage for a trailing window.
type AIUserUsage struct {
	QueryCount  int `json:"query_count"`
	TotalTokens int `json:"total_tokens"`
}

// GetAIUsageForUser returns query and token totals for a single user in the
// trailing window (personal stats, e.g. the Home page summary).
func (r *Repository) GetAIUsageForUser(ctx context.Context, userID string, days int) (AIUserUsage, error) {
	if days <= 0 {
		days = 30
	}
	var usage AIUserUsage
	err := r.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*)::int,
			COALESCE(SUM(token_count), 0)::int
		FROM ai_query_history
		WHERE user_id = $1
		  AND created_at >= NOW() - ($2::int * INTERVAL '1 day')
	`, userID, days).Scan(&usage.QueryCount, &usage.TotalTokens)
	if err != nil {
		return usage, fmt.Errorf("AI usage for user: %w", err)
	}
	return usage, nil
}

// AIUsageTotals is a rollup for admin dashboards.
type AIUsageTotals struct {
	QueryCount       int     `json:"query_count"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	TotalCostUSD     float64 `json:"total_cost_usd"`
	UniqueUsers      int     `json:"unique_users"`
	UniqueModels     int     `json:"unique_models"`
}

// GetAIUsageTotals returns aggregate token/cost stats for the trailing window.
func (r *Repository) GetAIUsageTotals(ctx context.Context, days int) (AIUsageTotals, error) {
	if days <= 0 {
		days = 30
	}
	var totals AIUsageTotals
	err := r.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*)::int,
			COALESCE(SUM(prompt_tokens), 0)::int,
			COALESCE(SUM(completion_tokens), 0)::int,
			COALESCE(SUM(token_count), 0)::int,
			COALESCE(SUM(cost_usd), 0),
			COUNT(DISTINCT user_id) FILTER (WHERE user_id IS NOT NULL AND user_id <> '')::int,
			COUNT(DISTINCT NULLIF(TRIM(model_used), ''))::int
		FROM ai_query_history
		WHERE created_at >= NOW() - ($1::int * INTERVAL '1 day')
	`, days).Scan(
		&totals.QueryCount,
		&totals.PromptTokens,
		&totals.CompletionTokens,
		&totals.TotalTokens,
		&totals.TotalCostUSD,
		&totals.UniqueUsers,
		&totals.UniqueModels,
	)
	if err != nil {
		return totals, fmt.Errorf("AI usage totals: %w", err)
	}
	return totals, nil
}
