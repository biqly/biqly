package metadata

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/lib/pq"
)

// FewShotCuratedRow is the API shape for a row in few_shot_examples.
type FewShotCuratedRow struct {
	ID           string          `json:"id"`
	DatasourceID string          `json:"datasource_id"`
	ModelID      string          `json:"model_id,omitempty"`
	Question     string          `json:"question"`
	LogicalQuery json.RawMessage `json:"logical_query"`
	Tags         []string        `json:"tags"`
	Dialect      string          `json:"dialect"`
	CreatedBy    string          `json:"created_by,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

// FewShotCuratedInsert is input for creating a curated few-shot example.
type FewShotCuratedInsert struct {
	DatasourceID string
	ModelID      string
	Question     string
	LogicalQuery json.RawMessage
	Tags         []string
	Dialect      string
}

// FewShotCuratedUpdate is input for updating a curated few-shot example.
type FewShotCuratedUpdate struct {
	Question     string
	LogicalQuery json.RawMessage
	Tags         []string
	Dialect      string
}

// ListFewShotCurated returns few-shot examples, optionally filtered by datasource and model.
func (r *Repository) ListFewShotCurated(ctx context.Context, datasourceID, modelID string) ([]FewShotCuratedRow, error) {
	q := `SELECT id::text, datasource_id::text, COALESCE(model_id::text,''), question, logical_query,
		COALESCE(tags,'{}'), COALESCE(dialect,'postgresql'), COALESCE(created_by,''),
		created_at, updated_at FROM few_shot_examples`
	args := []any{}
	argPos := 1
	if datasourceID != "" {
		q += fmt.Sprintf(" WHERE datasource_id = $%d::uuid", argPos)
		args = append(args, datasourceID)
		argPos++
		if modelID != "" {
			q += fmt.Sprintf(" AND model_id = $%d::uuid", argPos)
			args = append(args, modelID)
			argPos++
		}
	}
	q += " ORDER BY created_at DESC"

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []FewShotCuratedRow
	for rows.Next() {
		var e FewShotCuratedRow
		var mid, createdBy string
		var tags pq.StringArray
		if err := rows.Scan(&e.ID, &e.DatasourceID, &mid, &e.Question, &e.LogicalQuery, &tags, &e.Dialect, &createdBy, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}
		e.ModelID = mid
		e.CreatedBy = createdBy
		e.Tags = []string(tags)
		out = append(out, e)
	}
	return out, rows.Err()
}

// InsertFewShotCurated inserts a row and returns the new id.
func (r *Repository) InsertFewShotCurated(ctx context.Context, in FewShotCuratedInsert) (string, error) {
	var id string
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO few_shot_examples (datasource_id, model_id, question, logical_query, tags, dialect)
		 VALUES ($1::uuid, NULLIF($2,'')::uuid, $3, $4, $5::text[], $6) RETURNING id::text`,
		in.DatasourceID, in.ModelID, in.Question, in.LogicalQuery, pq.Array(in.Tags), in.Dialect,
	).Scan(&id)
	if err != nil {
		return "", err
	}
	return id, nil
}

// DeleteFewShotCurated deletes by id. Returns false if no row matched.
func (r *Repository) DeleteFewShotCurated(ctx context.Context, id string) (bool, error) {
	res, err := r.db.ExecContext(ctx, `DELETE FROM few_shot_examples WHERE id = $1::uuid`, id)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// UpdateFewShotCurated updates a row by id.
func (r *Repository) UpdateFewShotCurated(ctx context.Context, id string, in FewShotCuratedUpdate) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE few_shot_examples SET question = $1, logical_query = $2, tags = $3::text[], dialect = $4, updated_at = NOW() WHERE id = $5::uuid`,
		in.Question, in.LogicalQuery, pq.Array(in.Tags), in.Dialect, id,
	)
	return err
}

// InsertAIFeedback records user feedback on an AI answer.
func (r *Repository) InsertAIFeedback(ctx context.Context, question, datasourceID, rating string, categories []string, text string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO ai_feedback (question, datasource_id, rating, categories, feedback_text) VALUES ($1, $2::uuid, $3, $4::text[], $5)`,
		question, datasourceID, rating, pq.Array(categories), text,
	)
	return err
}

// UpdateLatestAIQueryHistoryRating sets user_rating on the most recent ai_query_history row for a datasource.
func (r *Repository) UpdateLatestAIQueryHistoryRating(ctx context.Context, datasourceID, rating string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE ai_query_history SET user_rating = $1
		 WHERE id = (SELECT id FROM ai_query_history WHERE datasource_id = $2::uuid ORDER BY created_at DESC LIMIT 1)`,
		rating, datasourceID,
	)
	return err
}

// ModelSuccessRateRow is aggregated AI query stats per model_id label.
type ModelSuccessRateRow struct {
	ModelID       string  `json:"model_id"`
	TotalQueries  int     `json:"total_queries"`
	SuccessCount  int     `json:"success_count"`
	FailureCount  int     `json:"failure_count"`
	SuccessRate   float64 `json:"success_rate"`
	AvgConfidence float64 `json:"avg_confidence"`
	AvgLatencyMs  float64 `json:"avg_latency_ms"`
	PositiveCount int     `json:"positive_count"`
	NegativeCount int     `json:"negative_count"`
}

// ListModelSuccessRates returns per-model aggregates for the last N days (days is a decimal string, e.g. "30").
func (r *Repository) ListModelSuccessRates(ctx context.Context, days string) ([]ModelSuccessRateRow, error) {
	q := `
		SELECT 
			COALESCE(h.model_id, 'unknown') AS model_id,
			COUNT(*) AS total_queries,
			COUNT(*) FILTER (WHERE h.confidence_score >= 0.7 AND (h.warnings IS NULL OR jsonb_array_length(h.warnings) = 0)) AS success_count,
			COUNT(*) FILTER (WHERE h.confidence_score < 0.7 OR (h.warnings IS NOT NULL AND jsonb_array_length(h.warnings) > 0)) AS failure_count,
			COALESCE(AVG(h.confidence_score), 0) AS avg_confidence,
			COALESCE(AVG(h.latency_ms), 0) AS avg_latency_ms,
			COUNT(*) FILTER (WHERE h.user_rating = 'positive') AS positive_count,
			COUNT(*) FILTER (WHERE h.user_rating = 'negative') AS negative_count
		FROM ai_query_history h
		WHERE h.created_at >= NOW() - ($1 || ' days')::INTERVAL
		GROUP BY COALESCE(h.model_id, 'unknown')
		ORDER BY total_queries DESC
	`
	rows, err := r.db.QueryContext(ctx, q, days)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var stats []ModelSuccessRateRow
	for rows.Next() {
		var s ModelSuccessRateRow
		if err := rows.Scan(&s.ModelID, &s.TotalQueries, &s.SuccessCount, &s.FailureCount, &s.AvgConfidence, &s.AvgLatencyMs, &s.PositiveCount, &s.NegativeCount); err != nil {
			return nil, err
		}
		if s.TotalQueries > 0 {
			s.SuccessRate = float64(s.SuccessCount) / float64(s.TotalQueries) * 100
		}
		stats = append(stats, s)
	}
	return stats, rows.Err()
}

// AIUsageDayRow is daily AI usage aggregates.
type AIUsageDayRow struct {
	Date             string  `json:"date"`
	TotalQueries     int     `json:"total_queries"`
	PositiveFeedback int     `json:"positive_feedback"`
	NegativeFeedback int     `json:"negative_feedback"`
	AvgLatencyMs     float64 `json:"avg_latency_ms"`
	TotalCost        float64 `json:"total_cost"`
	TotalTokens      int     `json:"total_tokens"`
}

// AIUsageSummary aggregates the last 30 days of AI query history.
type AIUsageSummary struct {
	TotalQueries int     `json:"total_queries"`
	SuccessRate  float64 `json:"success_rate"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
	TotalCost    float64 `json:"total_cost"`
}

// GetAIUsageLast30Days returns per-day breakdown and a summary for the trailing 30 days.
func (r *Repository) GetAIUsageLast30Days(ctx context.Context) (daily []AIUsageDayRow, summary AIUsageSummary, err error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT DATE(created_at) AS usage_date, COUNT(*),
			COUNT(*) FILTER (WHERE user_rating = 'positive'),
			COUNT(*) FILTER (WHERE user_rating = 'negative'),
			COALESCE(AVG(latency_ms), 0),
			COALESCE(SUM(cost_usd), 0),
			COALESCE(SUM(token_count), 0)
		FROM ai_query_history
		WHERE created_at >= NOW() - INTERVAL '30 days'
		GROUP BY DATE(created_at)
		ORDER BY usage_date DESC
	`)
	if err != nil {
		return nil, summary, err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var d AIUsageDayRow
		var dateVal time.Time
		if err := rows.Scan(&dateVal, &d.TotalQueries, &d.PositiveFeedback, &d.NegativeFeedback, &d.AvgLatencyMs, &d.TotalCost, &d.TotalTokens); err != nil {
			return nil, summary, err
		}
		d.Date = dateVal.Format("2006-01-02")
		daily = append(daily, d)
	}
	if err := rows.Err(); err != nil {
		return nil, summary, err
	}

	err = r.db.QueryRowContext(ctx, `
		SELECT COUNT(*),
			COALESCE(AVG(CASE WHEN user_rating IS NULL THEN 0.5 WHEN user_rating = 'positive' THEN 1.0 ELSE 0.0 END), 0),
			COALESCE(AVG(latency_ms), 0),
			COALESCE(SUM(cost_usd), 0)
		FROM ai_query_history WHERE created_at >= NOW() - INTERVAL '30 days'
	`).Scan(&summary.TotalQueries, &summary.SuccessRate, &summary.AvgLatencyMs, &summary.TotalCost)
	if err != nil {
		return daily, summary, err
	}
	return daily, summary, nil
}

// ListFewShotExampleIDs returns example ids for a datasource (and optional model), newest first.
func (r *Repository) ListFewShotExampleIDs(ctx context.Context, datasourceID, modelID string, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 10
	}
	q := "SELECT id::text FROM few_shot_examples WHERE datasource_id = $1::uuid"
	args := []any{datasourceID}
	if modelID != "" {
		q += " AND model_id = $2::uuid"
		args = append(args, modelID)
	}
	q += fmt.Sprintf(" ORDER BY created_at DESC LIMIT %d", limit)

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
