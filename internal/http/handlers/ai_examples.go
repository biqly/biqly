package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/biqly/biqly/internal/app"
	bimw "github.com/biqly/biqly/internal/http/middleware"
	"github.com/biqly/biqly/internal/metadata"
)

// FewShotExample is the wire format for a curated few-shot example (alias for metadata row type).
type FewShotExample = metadata.FewShotCuratedRow

type createFewShotExampleRequest struct {
	DatasourceID string          `json:"datasource_id"`
	ModelID      string          `json:"model_id,omitempty"`
	Question     string          `json:"question"`
	LogicalQuery json.RawMessage `json:"logical_query"`
	Tags         []string        `json:"tags"`
	Dialect      string          `json:"dialect"`
	Locale       string          `json:"locale,omitempty"`
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	IsFewShot    *bool           `json:"is_few_shot"`
	IsFavorite   *bool           `json:"is_favorite"`
}

type updateFewShotExampleRequest struct {
	Question     string          `json:"question"`
	LogicalQuery json.RawMessage `json:"logical_query"`
	Tags         []string        `json:"tags"`
	Dialect      string          `json:"dialect"`
	Locale       string          `json:"locale,omitempty"`
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	IsFewShot    *bool           `json:"is_few_shot"`
	IsFavorite   *bool           `json:"is_favorite"`
}

type submitAIFeedbackRequest struct {
	Question     string   `json:"question"`
	DatasourceID string   `json:"datasource_id"`
	Rating       string   `json:"rating"`
	Categories   []string `json:"categories"`
	Text         string   `json:"text"`
}

// AIExamplesHandler handles few-shot example CRUD and feedback operations.
type AIExamplesHandler struct {
	deps       *app.AIDeps
	authClient *bimw.AuthClient
}

// NewAIExamplesHandler creates a new handler for AI examples and feedback.
func NewAIExamplesHandler(deps *app.AIDeps) *AIExamplesHandler {
	return &AIExamplesHandler{deps: deps}
}

// SetAuthClient sets the auth service client.
func (h *AIExamplesHandler) SetAuthClient(c *bimw.AuthClient) {
	h.authClient = c
}

// ListExamples returns all few-shot examples, optionally filtered.
func (h *AIExamplesHandler) ListExamples(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	datasourceID := r.URL.Query().Get("datasource_id")
	modelID := r.URL.Query().Get("model_id")

	examples, err := h.deps.MetaRepo.ListFewShotCurated(ctx, datasourceID, modelID)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to list examples", err)
		return
	}
	writeJSON(w, http.StatusOK, examples)
}

// CreateExample creates a new few-shot example.
func (h *AIExamplesHandler) CreateExample(w http.ResponseWriter, r *http.Request) {
	input, ok := decodeJSON[createFewShotExampleRequest](w, r)
	if !ok {
		return
	}
	if input.Question == "" || input.DatasourceID == "" || len(input.LogicalQuery) == 0 {
		writeError(w, http.StatusBadRequest, "datasource_id, question, and logical_query are required")
		return
	}
	if input.Dialect == "" {
		input.Dialect = "postgresql"
	}
	name := input.Name
	if name == "" {
		name = input.Question
	}
	isFewShot := true
	if input.IsFewShot != nil {
		isFewShot = *input.IsFewShot
	}
	isFavorite := input.IsFavorite != nil && *input.IsFavorite

	id, err := h.deps.MetaRepo.InsertFewShotCurated(r.Context(), metadata.FewShotCuratedInsert{
		DatasourceID: input.DatasourceID,
		ModelID:      input.ModelID,
		Question:     input.Question,
		LogicalQuery: input.LogicalQuery,
		Tags:         input.Tags,
		Dialect:      input.Dialect,
		Locale:       input.Locale,
		Name:         name,
		Description:  input.Description,
		IsFewShot:    isFewShot,
		IsFavorite:   isFavorite,
	})
	if err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "failed to create example", err)
		return
	}
	now := time.Now()
	example := FewShotExample{
		ID:           id,
		DatasourceID: input.DatasourceID,
		ModelID:      input.ModelID,
		Question:     input.Question,
		LogicalQuery: input.LogicalQuery,
		Tags:         input.Tags,
		Dialect:      input.Dialect,
		Locale:       input.Locale,
		CreatedAt:    now,
		UpdatedAt:    now,
		Name:         name,
		Description:  input.Description,
		IsFewShot:    isFewShot,
		IsFavorite:   isFavorite,
	}
	writeJSON(w, http.StatusCreated, example)
}

// DeleteExample deletes a few-shot example by ID.
func (h *AIExamplesHandler) DeleteExample(w http.ResponseWriter, r *http.Request) {
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	ok, err := h.deps.MetaRepo.DeleteFewShotCurated(r.Context(), id)
	if err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "failed to delete example", err)
		return
	}
	if !ok {
		writeEntityNotFound(w, "example")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// UpdateExample updates an existing few-shot example.
func (h *AIExamplesHandler) UpdateExample(w http.ResponseWriter, r *http.Request) {
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	input, ok := decodeJSON[updateFewShotExampleRequest](w, r)
	if !ok {
		return
	}
	if input.Question == "" || len(input.LogicalQuery) == 0 {
		writeError(w, http.StatusBadRequest, "question and logical_query are required")
		return
	}
	if input.Dialect == "" {
		input.Dialect = "postgresql"
	}
	name := input.Name
	if name == "" {
		name = input.Question
	}
	isFewShot := true
	if input.IsFewShot != nil {
		isFewShot = *input.IsFewShot
	}
	isFavorite := input.IsFavorite != nil && *input.IsFavorite
	if err := h.deps.MetaRepo.UpdateFewShotCurated(r.Context(), id, metadata.FewShotCuratedUpdate{
		Question:     input.Question,
		LogicalQuery: input.LogicalQuery,
		Tags:         input.Tags,
		Dialect:      input.Dialect,
		Locale:       input.Locale,
		Name:         name,
		Description:  input.Description,
		IsFewShot:    isFewShot,
		IsFavorite:   isFavorite,
	}); err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "failed to update example", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// SubmitFeedback records user feedback on an AI query result.
func (h *AIExamplesHandler) SubmitFeedback(w http.ResponseWriter, r *http.Request) {
	input, ok := decodeJSON[submitAIFeedbackRequest](w, r)
	if !ok {
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

	ctx := r.Context()
	if err := h.deps.MetaRepo.InsertAIFeedback(ctx, input.Question, input.DatasourceID, input.Rating, input.Categories, input.Text); err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to submit feedback", err)
		return
	}
	if err := h.deps.MetaRepo.UpdateLatestAIQueryHistoryRating(ctx, input.DatasourceID, input.Rating, bimw.UserID(ctx), input.Question); err != nil {
		slog.WarnContext(ctx, "update latest AI query history rating", "datasource_id", input.DatasourceID, "err", err)
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "recorded"})
}

// ModelStats is the wire shape for per-model AI statistics.
type ModelStats = metadata.ModelSuccessRateRow

// GetModelSuccessRates returns per-model success/failure statistics.
func (h *AIExamplesHandler) GetModelSuccessRates(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	days := r.URL.Query().Get("days")
	if days == "" {
		days = "30"
	}

	stats, err := h.deps.MetaRepo.ListModelSuccessRates(ctx, days)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to get model statistics", err)
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

// DayUsage is one day in the AI usage breakdown.
type DayUsage = metadata.AIUsageDayRow

// GetAIUsage returns aggregated AI usage and pipeline telemetry for the dashboard.
func (h *AIExamplesHandler) GetAIUsage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	summary, daily, err := h.deps.MetaRepo.GetAIMetricsDashboard(ctx, 30)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to get usage data", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"summary": summary, "daily": daily})
}

// GetAIUsageBreakdown returns per-user, per-LLM-model token aggregates for admins.
func (h *AIExamplesHandler) GetAIUsageBreakdown(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := bimw.UserID(ctx)
	hasViewDetails := canViewAIHistoryDetails(ctx, h.authClient, userID)

	if !hasViewDetails {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	days := 30
	if v := r.URL.Query().Get("days"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 365 {
			days = n
		}
	}
	totals, err := h.deps.MetaRepo.GetAIUsageTotals(ctx, days)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to get usage totals", err)
		return
	}
	rows, err := h.deps.MetaRepo.ListAIUsageByUserAndModel(ctx, days)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to get usage breakdown", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"days":   days,
		"totals": totals,
		"rows":   rows,
	})
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

	ids, err := h.deps.MetaRepo.ListFewShotExampleIDs(ctx, datasourceID, modelID, 10)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to get example IDs", err)
		return
	}
	writeJSON(w, http.StatusOK, ids)
}
