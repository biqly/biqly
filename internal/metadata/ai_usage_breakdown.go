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

// ListAIUsageByUserAndModel returns usage grouped by user_id and model_used.
func (r *Repository) ListAIUsageByUserAndModel(ctx context.Context, days int) ([]AIUsageByUserModelRow, error) {
	if days <= 0 {
		days = 30
	}
	q := `
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
		ORDER BY total_tokens DESC, query_count DESC
	`
	return platformdb.QuerySliceErr(ctx, r.db, "list AI usage by user and model", q, []any{days}, scanAIUsageByUserModelRow)
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
