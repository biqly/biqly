package metadata

import (
	"context"
	"time"

	platformdb "github.com/biqly/biqly/internal/platform/db"
)

const (
	AIOutcomeSuccess       = "success"
	AIOutcomePartial       = "partial"
	AIOutcomeFailed        = "failed"
	AIOutcomeClarification = "clarification"
	AIOutcomeUnknown       = "unknown"
)

// AIMetricsSummary is the centralized AI text-to-SQL operations dashboard.
type AIMetricsSummary struct {
	TotalQueries       int     `json:"total_queries"`
	SuccessCount       int     `json:"success_count"`
	FailedCount        int     `json:"failed_count"`
	PartialCount       int     `json:"partial_count"`
	ClarificationCount int     `json:"clarification_count"`
	SuccessRate        float64 `json:"success_rate"`
	FailureRate        float64 `json:"failure_rate"`
	AvgRetryCount      float64 `json:"avg_retry_count"`
	AvgLatencyMs       float64 `json:"avg_latency_ms"`
	TotalCost          float64 `json:"total_cost"`
	TotalTokens        int     `json:"total_tokens"`
	PositiveFeedback   int     `json:"positive_feedback"`
	NegativeFeedback   int     `json:"negative_feedback"`
}

// AIMetricsDayRow is daily AI pipeline aggregates for charts.
type AIMetricsDayRow struct {
	Date               string  `json:"date"`
	TotalQueries       int     `json:"total_queries"`
	SuccessCount       int     `json:"success_count"`
	FailedCount        int     `json:"failed_count"`
	PartialCount       int     `json:"partial_count"`
	ClarificationCount int     `json:"clarification_count"`
	FailureRate        float64 `json:"failure_rate"`
	AvgRetryCount      float64 `json:"avg_retry_count"`
	AvgLatencyMs       float64 `json:"avg_latency_ms"`
	TotalCost          float64 `json:"total_cost"`
	TotalTokens        int     `json:"total_tokens"`
}

// GetAIMetricsDashboard returns summary + daily rows for the trailing window.
func (r *Repository) GetAIMetricsDashboard(ctx context.Context, days int) (summary AIMetricsSummary, daily []AIMetricsDayRow, err error) {
	if days <= 0 {
		days = 30
	}
	interval := days

	daily, err = platformdb.QuerySliceErr(ctx, r.db, "AI metrics daily", `
		SELECT DATE(created_at),
			COUNT(*),
			COUNT(*) FILTER (WHERE outcome_status = 'success'),
			COUNT(*) FILTER (WHERE outcome_status = 'failed'),
			COUNT(*) FILTER (WHERE outcome_status = 'partial'),
			COUNT(*) FILTER (WHERE outcome_status = 'clarification'),
			COALESCE(AVG(retry_count), 0),
			COALESCE(AVG(latency_ms), 0),
			COALESCE(SUM(cost_usd), 0),
			COALESCE(SUM(token_count), 0)
		FROM ai_query_history
		WHERE created_at >= NOW() - ($1::int * INTERVAL '1 day')
		GROUP BY DATE(created_at)
		ORDER BY DATE(created_at) DESC
	`, []any{interval}, scanAIMetricsDayRow)
	if err != nil {
		return summary, nil, err
	}

	err = r.db.QueryRowContext(ctx, `
		SELECT COUNT(*),
			COUNT(*) FILTER (WHERE outcome_status = 'success'),
			COUNT(*) FILTER (WHERE outcome_status = 'failed'),
			COUNT(*) FILTER (WHERE outcome_status = 'partial'),
			COUNT(*) FILTER (WHERE outcome_status = 'clarification'),
			COALESCE(AVG(retry_count), 0),
			COALESCE(AVG(latency_ms), 0),
			COALESCE(SUM(cost_usd), 0),
			COALESCE(SUM(token_count), 0),
			COUNT(*) FILTER (WHERE user_rating = 'positive'),
			COUNT(*) FILTER (WHERE user_rating = 'negative')
		FROM ai_query_history
		WHERE created_at >= NOW() - ($1::int * INTERVAL '1 day')
	`, interval).Scan(
		&summary.TotalQueries,
		&summary.SuccessCount,
		&summary.FailedCount,
		&summary.PartialCount,
		&summary.ClarificationCount,
		&summary.AvgRetryCount,
		&summary.AvgLatencyMs,
		&summary.TotalCost,
		&summary.TotalTokens,
		&summary.PositiveFeedback,
		&summary.NegativeFeedback,
	)
	if err != nil {
		return summary, daily, err
	}
	if summary.TotalQueries > 0 {
		summary.SuccessRate = float64(summary.SuccessCount) / float64(summary.TotalQueries)
		summary.FailureRate = float64(summary.FailedCount) / float64(summary.TotalQueries)
	}
	return summary, daily, nil
}

func scanAIMetricsDayRow(s platformdb.Scanner) (AIMetricsDayRow, error) {
	var d AIMetricsDayRow
	var dateVal time.Time
	var total, success, failed, partial, clarification int
	if err := s.Scan(&dateVal, &total, &success, &failed, &partial, &clarification,
		&d.AvgRetryCount, &d.AvgLatencyMs, &d.TotalCost, &d.TotalTokens); err != nil {
		return AIMetricsDayRow{}, err
	}
	d.Date = dateVal.Format("2006-01-02")
	d.TotalQueries = total
	d.SuccessCount = success
	d.FailedCount = failed
	d.PartialCount = partial
	d.ClarificationCount = clarification
	if total > 0 {
		d.FailureRate = float64(failed) / float64(total)
	}
	return d, nil
}
