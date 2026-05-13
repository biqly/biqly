package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/go-chi/chi/v5"
)

// FewShotExample is the wire format for a curated few-shot example (alias for metadata row type).
type FewShotExample = metadata.FewShotCuratedRow

// AIExamplesHandler handles few-shot example CRUD and feedback operations.
type AIExamplesHandler struct {
	deps *app.Dependencies
}

// NewAIExamplesHandler creates a new handler for AI examples and feedback.
func NewAIExamplesHandler(deps *app.Dependencies) *AIExamplesHandler {
	return &AIExamplesHandler{deps: deps}
}

// ListExamples returns all few-shot examples, optionally filtered.
func (h *AIExamplesHandler) ListExamples(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	datasourceID := r.URL.Query().Get("datasource_id")
	modelID := r.URL.Query().Get("model_id")

	examples, err := h.deps.MetaRepo.ListFewShotCurated(ctx, datasourceID, modelID)
	if err != nil {
		slog.ErrorContext(ctx, "list few-shot examples failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list examples")
		return
	}
	if examples == nil {
		examples = []metadata.FewShotCuratedRow{}
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

	id, err := h.deps.MetaRepo.InsertFewShotCurated(r.Context(), metadata.FewShotCuratedInsert{
		DatasourceID: input.DatasourceID,
		ModelID:      input.ModelID,
		Question:     input.Question,
		LogicalQuery: input.LogicalQuery,
		Tags:         input.Tags,
		Dialect:      input.Dialect,
	})
	if err != nil {
		slog.ErrorContext(r.Context(), "create few-shot example failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create example")
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
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	writeJSON(w, http.StatusCreated, example)
}

// DeleteExample deletes a few-shot example by ID.
func (h *AIExamplesHandler) DeleteExample(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "example id is required")
		return
	}
	ok, err := h.deps.MetaRepo.DeleteFewShotCurated(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "delete few-shot example failed", "id", id, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to delete example")
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "example not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// UpdateExample updates an existing few-shot example.
func (h *AIExamplesHandler) UpdateExample(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
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
	if err := h.deps.MetaRepo.UpdateFewShotCurated(r.Context(), id, metadata.FewShotCuratedUpdate{
		Question:     input.Question,
		LogicalQuery: input.LogicalQuery,
		Tags:         input.Tags,
		Dialect:      input.Dialect,
	}); err != nil {
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

	ctx := r.Context()
	if err := h.deps.MetaRepo.InsertAIFeedback(ctx, input.Question, input.DatasourceID, input.Rating, input.Categories, input.Text); err != nil {
		slog.ErrorContext(ctx, "submit feedback failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to submit feedback")
		return
	}
	_ = h.deps.MetaRepo.UpdateLatestAIQueryHistoryRating(ctx, input.DatasourceID, input.Rating)

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
		slog.ErrorContext(ctx, "get model success rates failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to get model statistics")
		return
	}
	if stats == nil {
		stats = []metadata.ModelSuccessRateRow{}
	}
	writeJSON(w, http.StatusOK, stats)
}

// DayUsage is one day in the AI usage breakdown.
type DayUsage = metadata.AIUsageDayRow

// GetAIUsage returns aggregated AI usage statistics.
func (h *AIExamplesHandler) GetAIUsage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	daily, summary, err := h.deps.MetaRepo.GetAIUsageLast30Days(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "get AI usage failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to get usage data")
		return
	}
	if daily == nil {
		daily = []metadata.AIUsageDayRow{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"summary": summary, "daily": daily})
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
		slog.ErrorContext(ctx, "get example IDs failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to get example IDs")
		return
	}
	if ids == nil {
		ids = []string{}
	}
	writeJSON(w, http.StatusOK, ids)
}
