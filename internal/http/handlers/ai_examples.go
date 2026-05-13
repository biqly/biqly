package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/biqly/biqly/internal/app"
)

// AIExamplesHandler handles few-shot example CRUD and feedback operations.
type AIExamplesHandler struct {
	deps *app.Dependencies
}

// NewAIExamplesHandler creates a new handler for AI examples and feedback.
func NewAIExamplesHandler(deps *app.Dependencies) *AIExamplesHandler {
	return &AIExamplesHandler{deps: deps}
}

// FewShotExample is the wire format for a curated few-shot example.
type FewShotExample struct {
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

// ListExamples returns all few-shot examples, optionally filtered.
func (h *AIExamplesHandler) ListExamples(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	datasourceID := r.URL.Query().Get("datasource_id")
	modelID := r.URL.Query().Get("model_id")

	q := `SELECT id::text, datasource_id::text, COALESCE(model_id::text,''), question, logical_query,
		COALESCE(tags,'{}'), COALESCE(dialect,'postgresql'), COALESCE(created_by,''),
		created_at, updated_at FROM few_shot_examples`
	args := []any{}
	if datasourceID != "" {
		q += " WHERE datasource_id = $1::uuid"
		args = append(args, datasourceID)
		if modelID != "" {
			q += " AND model_id = $2::uuid"
			args = append(args, modelID)
		}
	}
	q += " ORDER BY created_at DESC"

	rows, err := h.deps.MetadataDB.QueryContext(ctx, q, args...)
	if err != nil {
		slog.ErrorContext(ctx, "list few-shot examples failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list examples")
		return
	}
	defer rows.Close()

	var examples []FewShotExample
	for rows.Next() {
		var e FewShotExample
		var mid, createdBy string
		if err := rows.Scan(&e.ID, &e.DatasourceID, &mid, &e.Question, &e.LogicalQuery, &e.Tags, &e.Dialect, &createdBy, &e.CreatedAt, &e.UpdatedAt); err != nil {
			slog.ErrorContext(ctx, "scan few-shot example failed", "error", err)
			continue
		}
		e.ModelID = mid
		e.CreatedBy = createdBy
		examples = append(examples, e)
	}
	if examples == nil {
		examples = []FewShotExample{}
	}
	writeJSON(w, http.StatusOK, examples)
}

// CreateExample creates a new few-shot example.
func (h *AIExamplesHandler) CreateExample(w http.ResponseWriter, r *http.Request) {
	var input struct {
		DatasourceID string          `json:"datasource_id"`
		ModelID      string          `json:"model_id,omitempty"`
		Question     string          `json:"question"`
		LogicalQuery json.RawMessage `json:"logical_query"`
		Tags         []string        `json:"tags"`
		Dialect      string          `json:"dialect"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if input.Question == "" || input.DatasourceID == "" || len(input.LogicalQuery) == 0 {
		writeError(w, http.StatusBadRequest, "datasource_id, question, and logical_query are required")
		return
	}
	if input.Dialect == "" {
		input.Dialect = "postgresql"
	}

	var id string
	err := h.deps.MetadataDB.QueryRowContext(r.Context(),
		"INSERT INTO few_shot_examples (datasource_id, model_id, question, logical_query, tags, dialect) VALUES ($1::uuid, NULLIF($2,'')::uuid, $3, $4, $5::text[], $6) RETURNING id::text",
		input.DatasourceID, input.ModelID, input.Question, input.LogicalQuery, pqStringArray(input.Tags), input.Dialect,
	).Scan(&id)
	if err != nil {
		slog.ErrorContext(r.Context(), "create few-shot example failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create example")
		return
	}
	example := FewShotExample{
		ID:           id,
		DatasourceID: input.DatasourceID,
		ModelID:      input.ModelID,
		Question:     input.Question,
		LogicalQuery: input.LogicalQuery,
		Tags:         input.Tags,
		Dialect:      input.Dialect,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	writeJSON(w, http.StatusCreated, example)
}

// DeleteExample deletes a few-shot example by ID.
func (h *AIExamplesHandler) DeleteExample(w http.ResponseWriter, r *http.Request) {
	id := extractUUIDFromPath(r)
	if id == "" {
		writeError(w, http.StatusBadRequest, "example id is required")
		return
	}
	_, err := h.deps.MetadataDB.ExecContext(r.Context(), "DELETE FROM few_shot_examples WHERE id = $1::uuid", id)
	if err != nil {
		slog.ErrorContext(r.Context(), "delete few-shot example failed", "id", id, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to delete example")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// UpdateExample updates an existing few-shot example.
func (h *AIExamplesHandler) UpdateExample(w http.ResponseWriter, r *http.Request) {
	id := extractUUIDFromPath(r)
	if id == "" {
		writeError(w, http.StatusBadRequest, "example id is required")
		return
	}
	var input struct {
		Question     string          `json:"question"`
		LogicalQuery json.RawMessage `json:"logical_query"`
		Tags         []string        `json:"tags"`
		Dialect      string          `json:"dialect"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if input.Question == "" || len(input.LogicalQuery) == 0 {
		writeError(w, http.StatusBadRequest, "question and logical_query are required")
		return
	}
	if input.Dialect == "" {
		input.Dialect = "postgresql"
	}
	_, err := h.deps.MetadataDB.ExecContext(r.Context(),
		"UPDATE few_shot_examples SET question = $1, logical_query = $2, tags = $3::text[], dialect = $4, updated_at = NOW() WHERE id = $5::uuid",
		input.Question, input.LogicalQuery, pqStringArray(input.Tags), input.Dialect, id,
	)
	if err != nil {
		slog.ErrorContext(r.Context(), "update few-shot example failed", "id", id, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update example")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// SubmitFeedback records user feedback on an AI query result.
func (h *AIExamplesHandler) SubmitFeedback(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Question     string   `json:"question"`
		DatasourceID string   `json:"datasource_id"`
		Rating       string   `json:"rating"`
		Categories   []string `json:"categories"`
		Text         string   `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if input.Rating != "positive" && input.Rating != "negative" {
		writeError(w, http.StatusBadRequest, "rating must be 'positive' or 'negative'")
		return
	}
	if input.DatasourceID == "" {
		writeError(w, http.StatusBadRequest, "datasource_id is required")
		return
	}

	_, err := h.deps.MetadataDB.ExecContext(r.Context(),
		"INSERT INTO ai_feedback (question, datasource_id, rating, categories, feedback_text) VALUES ($1, $2::uuid, $3, $4::text[], $5)",
		input.Question, input.DatasourceID, input.Rating, pqStringArray(input.Categories), input.Text,
	)
	if err != nil {
		slog.ErrorContext(r.Context(), "submit feedback failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to submit feedback")
		return
	}

	// Update the most recent ai_query_history entry for this datasource
	h.deps.MetadataDB.ExecContext(r.Context(),
		`UPDATE ai_query_history SET user_rating = $1
		 WHERE id = (SELECT id FROM ai_query_history WHERE datasource_id = $2 ORDER BY created_at DESC LIMIT 1)`,
		input.Rating, input.DatasourceID,
	)

	writeJSON(w, http.StatusOK, map[string]string{"status": "recorded"})
}

// GetModelSuccessRates returns per-model success/failure statistics.
func (h *AIExamplesHandler) GetModelSuccessRates(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	days := r.URL.Query().Get("days")
	if days == "" {
		days = "30"
	}

	type ModelStats struct {
		ModelID       string  `json:"model_id"`
		ModelName     string  `json:"model_name,omitempty"`
		TotalQueries  int     `json:"total_queries"`
		SuccessCount  int     `json:"success_count"`
		FailureCount  int     `json:"failure_count"`
		SuccessRate   float64 `json:"success_rate"`
		AvgConfidence float64 `json:"avg_confidence"`
		AvgLatencyMs  float64 `json:"avg_latency_ms"`
		PositiveCount int     `json:"positive_count"`
		NegativeCount int     `json:"negative_count"`
	}

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

	rows, err := h.deps.MetadataDB.QueryContext(ctx, q, days)
	if err != nil {
		slog.ErrorContext(ctx, "get model success rates failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to get model statistics")
		return
	}
	defer rows.Close()

	var stats []ModelStats
	for rows.Next() {
		var s ModelStats
		if err := rows.Scan(&s.ModelID, &s.TotalQueries, &s.SuccessCount, &s.FailureCount, &s.AvgConfidence, &s.AvgLatencyMs, &s.PositiveCount, &s.NegativeCount); err != nil {
			slog.ErrorContext(ctx, "scan model stats failed", "error", err)
			continue
		}
		if s.TotalQueries > 0 {
			s.SuccessRate = float64(s.SuccessCount) / float64(s.TotalQueries) * 100
		}
		stats = append(stats, s)
	}
	if stats == nil {
		stats = []ModelStats{}
	}
	writeJSON(w, http.StatusOK, stats)
}

// GetAIUsage returns aggregated AI usage statistics.
func (h *AIExamplesHandler) GetAIUsage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	type DayUsage struct {
		Date             string  `json:"date"`
		TotalQueries     int     `json:"total_queries"`
		PositiveFeedback int     `json:"positive_feedback"`
		NegativeFeedback int     `json:"negative_feedback"`
		AvgLatencyMs     float64 `json:"avg_latency_ms"`
		TotalCost        float64 `json:"total_cost"`
		TotalTokens      int     `json:"total_tokens"`
	}

	rows, err := h.deps.MetadataDB.QueryContext(ctx, `
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
		slog.ErrorContext(ctx, "get AI usage failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to get usage data")
		return
	}
	defer rows.Close()

	var daily []DayUsage
	for rows.Next() {
		var d DayUsage
		var dateVal time.Time
		if err := rows.Scan(&dateVal, &d.TotalQueries, &d.PositiveFeedback, &d.NegativeFeedback, &d.AvgLatencyMs, &d.TotalCost, &d.TotalTokens); err != nil {
			slog.ErrorContext(ctx, "scan usage row failed", "error", err)
			continue
		}
		d.Date = dateVal.Format("2006-01-02")
		daily = append(daily, d)
	}
	if daily == nil {
		daily = []DayUsage{}
	}

	var summary struct {
		TotalQueries int     `json:"total_queries"`
		SuccessRate  float64 `json:"success_rate"`
		AvgLatencyMs float64 `json:"avg_latency_ms"`
		TotalCost    float64 `json:"total_cost"`
	}
	_ = h.deps.MetadataDB.QueryRowContext(ctx, `
		SELECT COUNT(*),
			COALESCE(AVG(CASE WHEN user_rating IS NULL THEN 0.5 WHEN user_rating = 'positive' THEN 1.0 ELSE 0.0 END), 0),
			COALESCE(AVG(latency_ms), 0),
			COALESCE(SUM(cost_usd), 0)
		FROM ai_query_history WHERE created_at >= NOW() - INTERVAL '30 days'
	`).Scan(&summary.TotalQueries, &summary.SuccessRate, &summary.AvgLatencyMs, &summary.TotalCost)

	writeJSON(w, http.StatusOK, map[string]any{"summary": summary, "daily": daily})
}

// pqStringArray converts a Go string slice to a PostgreSQL TEXT[] string.
func pqStringArray(s []string) string {
	if len(s) == 0 {
		return "{}"
	}
	escaped := make([]string, len(s))
	for i, v := range s {
		escaped[i] = strings.ReplaceAll(strings.ReplaceAll(v, `\`, `\\`), `"`, `\"`)
	}
	return fmt.Sprintf(`{"%s"}`, strings.Join(escaped, `","`))
}

// GetExampleIDs returns a list of example IDs for a datasource/model.
func (h *AIExamplesHandler) GetExampleIDs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	datasourceID := r.URL.Query().Get("datasource_id")
	modelID := r.URL.Query().Get("model_id")

	if datasourceID == "" {
		writeError(w, http.StatusBadRequest, "datasource_id is required")
		return
	}

	q := "SELECT id::text FROM few_shot_examples WHERE datasource_id = $1::uuid"
	args := []any{datasourceID}
	if modelID != "" {
		q += " AND model_id = $2::uuid"
		args = append(args, modelID)
	}
	q += " ORDER BY created_at DESC LIMIT 10"

	rows, err := h.deps.MetadataDB.QueryContext(ctx, q, args...)
	if err != nil {
		slog.ErrorContext(ctx, "get example IDs failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to get example IDs")
		return
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			continue
		}
		ids = append(ids, id)
	}
	if ids == nil {
		ids = []string{}
	}
	writeJSON(w, http.StatusOK, ids)
}

// extractUUIDFromPath extracts the last path segment (used as UUID).
func extractUUIDFromPath(r *http.Request) string {
	path := r.URL.Path
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[i+1:]
		}
	}
	return path
}
