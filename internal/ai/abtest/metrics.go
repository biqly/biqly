package abtest

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// MetricsCollector computes performance metrics for experiment variants.
type MetricsCollector struct {
	repo *Repository
}

// NewMetricsCollector creates a new MetricsCollector.
func NewMetricsCollector(repo *Repository) *MetricsCollector {
	return &MetricsCollector{repo: repo}
}

// ComputeMetrics returns aggregated metrics for each variant of an experiment within a period.
func (m *MetricsCollector) ComputeMetrics(
	ctx context.Context,
	experimentID string,
	periodStart, periodEnd time.Time,
) (metrics []ExperimentMetrics, err error) {
	query := `
		SELECT
			COALESCE(ab_variant_id, ''),
			COUNT(*) AS total_queries,
			COALESCE(COUNT(*) FILTER (WHERE confidence_score >= 0.7 AND (warnings IS NULL OR cardinality(warnings) = 0)) / NULLIF(COUNT(*), 0)::float, 0.0) AS success_rate,
			COALESCE(COUNT(*) FILTER (WHERE confidence_score >= 0.7) / NULLIF(COUNT(*), 0)::float, 0.0) AS validator_pass_rate,
			COALESCE(AVG(confidence_score), 0.0) AS avg_confidence,
			COALESCE(COUNT(*) FILTER (WHERE user_rating = 'negative') / NULLIF(COUNT(*) FILTER (WHERE user_rating IS NOT NULL), 0)::float, 0.0) AS user_correction_rate,
			COALESCE(COUNT(*) FILTER (WHERE user_rating = 'positive') / NULLIF(COUNT(*) FILTER (WHERE user_rating IS NOT NULL), 0)::float, 0.0) AS positive_feedback_rate,
			COALESCE(COUNT(*) FILTER (WHERE outcome_status = 'success') / NULLIF(COUNT(*), 0)::float, 0.0) AS execution_success_rate,
			COALESCE(AVG(cost_usd), 0.0) AS avg_cost_usd,
			COALESCE(AVG(latency_ms), 0.0) AS avg_latency_ms,
			COALESCE(STDDEV_SAMP(cost_usd), 0.0) AS stddev_cost_usd,
			COALESCE(STDDEV_SAMP(latency_ms), 0.0) AS stddev_latency_ms,
			COALESCE(SUM(token_count), 0) AS total_tokens
		FROM ai_query_history
		WHERE ab_experiment_id = $1 AND created_at BETWEEN $2 AND $3
		GROUP BY ab_variant_id
	`
	rows, err := m.repo.db.QueryContext(ctx, query, experimentID, periodStart, periodEnd)
	if err != nil {
		return nil, fmt.Errorf("query ab experiment metrics: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close ab experiment metrics rows: %w", closeErr))
		}
	}()

	for rows.Next() {
		var em ExperimentMetrics
		var totalTokens sql.NullInt64
		em.ExperimentID = experimentID
		em.PeriodStart = periodStart
		em.PeriodEnd = periodEnd

		err := rows.Scan(
			&em.VariantID,
			&em.TotalQueries,
			&em.SuccessRate,
			&em.ValidatorPassRate,
			&em.AvgConfidence,
			&em.UserCorrectionRate,
			&em.PositiveFeedbackRate,
			&em.ExecutionSuccessRate,
			&em.AvgCostUSD,
			&em.AvgLatencyMs,
			&em.StdDevCostUSD,
			&em.StdDevLatencyMs,
			&totalTokens,
		)
		if err != nil {
			return nil, fmt.Errorf("scan ab experiment metrics: %w", err)
		}
		if totalTokens.Valid {
			em.TotalTokens = int(totalTokens.Int64)
		}
		metrics = append(metrics, em)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ab experiment metrics: %w", err)
	}
	return metrics, nil
}
