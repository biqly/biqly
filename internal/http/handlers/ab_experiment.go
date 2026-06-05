package handlers

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/biqly/biqly/internal/ai/abtest"
	"github.com/biqly/biqly/internal/ai/prompt"
	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/i18n"
)

type abExperimentDetailResponse struct {
	Experiment *abtest.Experiment `json:"experiment"`
	Variants   []abtest.Variant   `json:"variants"`
}

type createExperimentRequest struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	TemplateName string `json:"template_name"`
	Locale       string `json:"locale"`
}

type updateExperimentRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type updateStatusRequest struct {
	Status string `json:"status"`
}

type createVariantRequest struct {
	Name            string `json:"name"`
	TemplateVersion int    `json:"template_version"`
	TrafficPct      int    `json:"traffic_pct"`
	IsControl       bool   `json:"is_control"`
}

type updateVariantRequest struct {
	Name            string `json:"name"`
	TemplateVersion int    `json:"template_version"`
	TrafficPct      int    `json:"traffic_pct"`
	IsControl       bool   `json:"is_control"`
}

type dailyMetric struct {
	Date    string                     `json:"date"`
	Metrics []abtest.ExperimentMetrics `json:"metrics"`
}

// ABExperimentHandler handles REST endpoints for prompt A/B testing.
type ABExperimentHandler struct {
	deps        *app.AIDeps
	repo        *abtest.Repository
	collector   *abtest.MetricsCollector
	recommender *abtest.Recommender
	router      *abtest.TrafficRouter
}

// NewABExperimentHandler creates a new ABExperimentHandler.
func NewABExperimentHandler(deps *app.AIDeps) *ABExperimentHandler {
	var repo *abtest.Repository
	var collector *abtest.MetricsCollector
	var recommender *abtest.Recommender
	var router *abtest.TrafficRouter

	if deps != nil {
		if deps.MetaRepo != nil {
			repo = abtest.NewRepository(deps.MetaRepo.DB())
			collector = abtest.NewMetricsCollector(repo)
			recommender = abtest.NewRecommender(repo, collector)
		}
		router = deps.ABRouter
	}

	return &ABExperimentHandler{
		deps:        deps,
		repo:        repo,
		collector:   collector,
		recommender: recommender,
		router:      router,
	}
}

func (h *ABExperimentHandler) checkInitialized(w http.ResponseWriter) bool {
	if h.repo == nil {
		writeError(w, http.StatusServiceUnavailable, "A/B testing repository is not initialized")
		return false
	}
	return true
}

// Create creates a new draft A/B experiment.
func (h *ABExperimentHandler) Create(w http.ResponseWriter, r *http.Request) {
	if !h.checkInitialized(w) {
		return
	}
	ctx := r.Context()
	req, ok := decodeJSON[createExperimentRequest](w, r)
	if !ok {
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	templateName := strings.TrimSpace(req.TemplateName)
	if templateName == "" {
		writeError(w, http.StatusBadRequest, "template_name is required")
		return
	}

	knownTemplates := prompt.KnownPromptTemplateNames()
	validTemplate := false
	for _, kt := range knownTemplates {
		if kt == templateName {
			validTemplate = true
			break
		}
	}
	if !validTemplate {
		writeError(w, http.StatusBadRequest, "invalid template_name")
		return
	}

	localeStr := strings.TrimSpace(strings.ToLower(req.Locale))
	if localeStr == "" {
		localeStr = "en"
	}
	loc, okLoc := i18n.ParseSupportedLocale(localeStr)
	if !okLoc {
		writeError(w, http.StatusBadRequest, "unsupported locale")
		return
	}

	exp := &abtest.Experiment{
		Name:         name,
		Description:  strings.TrimSpace(req.Description),
		TemplateName: templateName,
		Locale:       string(loc),
		Status:       abtest.ExperimentStatusDraft,
	}

	if err := h.repo.CreateExperiment(ctx, exp); err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to create experiment", err)
		return
	}

	writeJSON(w, http.StatusCreated, exp)
}

// List returns A/B experiments, optionally filtered by ?status=running|draft|paused|completed.
func (h *ABExperimentHandler) List(w http.ResponseWriter, r *http.Request) {
	if !h.checkInitialized(w) {
		return
	}
	ctx := r.Context()
	status := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("status")))

	exps, err := h.repo.ListExperiments(ctx, status)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to list experiments", err)
		return
	}

	writeJSON(w, http.StatusOK, exps)
}

// Get returns details for one experiment including its variants.
func (h *ABExperimentHandler) Get(w http.ResponseWriter, r *http.Request) {
	if !h.checkInitialized(w) {
		return
	}
	ctx := r.Context()
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}

	exp, err := h.repo.GetExperiment(ctx, id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeEntityNotFound(w, "Experiment")
			return
		}
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to get experiment", err)
		return
	}

	variants, err := h.repo.ListVariants(ctx, id)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to list variants", err)
		return
	}

	writeJSON(w, http.StatusOK, abExperimentDetailResponse{
		Experiment: exp,
		Variants:   variants,
	})
}

// Update updates name/description of an experiment.
func (h *ABExperimentHandler) Update(w http.ResponseWriter, r *http.Request) {
	if !h.checkInitialized(w) {
		return
	}
	ctx := r.Context()
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}

	req, ok := decodeJSON[updateExperimentRequest](w, r)
	if !ok {
		return
	}

	exp, err := h.repo.GetExperiment(ctx, id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeEntityNotFound(w, "Experiment")
			return
		}
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to get experiment", err)
		return
	}

	exp.Name = strings.TrimSpace(req.Name)
	exp.Description = strings.TrimSpace(req.Description)

	if exp.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	if err := h.repo.UpdateExperiment(ctx, exp); err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to update experiment", err)
		return
	}

	writeJSON(w, http.StatusOK, exp)
}

// UpdateStatus transitions experiment status.
func (h *ABExperimentHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	if !h.checkInitialized(w) {
		return
	}
	ctx := r.Context()
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}

	req, ok := decodeJSON[updateStatusRequest](w, r)
	if !ok {
		return
	}

	newStatus := abtest.ExperimentStatus(strings.TrimSpace(strings.ToLower(req.Status)))
	if newStatus != abtest.ExperimentStatusDraft &&
		newStatus != abtest.ExperimentStatusRunning &&
		newStatus != abtest.ExperimentStatusPaused &&
		newStatus != abtest.ExperimentStatusCompleted {
		writeError(w, http.StatusBadRequest, "invalid status")
		return
	}

	exp, err := h.repo.GetExperiment(ctx, id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeEntityNotFound(w, "Experiment")
			return
		}
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to get experiment", err)
		return
	}

	if exp.Status == newStatus {
		writeJSON(w, http.StatusOK, exp)
		return
	}

	variants, err := h.repo.ListVariants(ctx, id)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to list variants", err)
		return
	}

	// Validate status transition
	switch newStatus {
	case abtest.ExperimentStatusRunning:
		if err := abtest.ValidateVariantsForAllocation(variants); err != nil {
			writeError(w, http.StatusBadRequest, "cannot start experiment: "+err.Error())
			return
		}
		if exp.StartedAt == nil {
			now := time.Now()
			exp.StartedAt = &now
		}
	case abtest.ExperimentStatusCompleted:
		if exp.EndedAt == nil {
			now := time.Now()
			exp.EndedAt = &now
		}
	case abtest.ExperimentStatusDraft, abtest.ExperimentStatusPaused:
		// No action needed for other status values
	}

	exp.Status = newStatus

	if err := h.repo.UpdateExperiment(ctx, exp); err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to update experiment status", err)
		return
	}

	// Invalidate traffic router cache
	if h.router != nil {
		h.router.Invalidate(exp.TemplateName, exp.Locale)
	}

	writeJSON(w, http.StatusOK, exp)
}

// AddVariant adds a new variant to a draft experiment.
func (h *ABExperimentHandler) AddVariant(w http.ResponseWriter, r *http.Request) {
	if !h.checkInitialized(w) {
		return
	}
	ctx := r.Context()
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}

	req, ok := decodeJSON[createVariantRequest](w, r)
	if !ok {
		return
	}

	exp, err := h.repo.GetExperiment(ctx, id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeEntityNotFound(w, "Experiment")
			return
		}
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to get experiment", err)
		return
	}

	if exp.Status != abtest.ExperimentStatusDraft {
		writeError(w, http.StatusBadRequest, "cannot add variants to non-draft experiment")
		return
	}

	variant := &abtest.Variant{
		ExperimentID:    id,
		Name:            strings.TrimSpace(req.Name),
		TemplateVersion: req.TemplateVersion,
		TrafficPct:      req.TrafficPct,
		IsControl:       req.IsControl,
	}

	if variant.Name == "" {
		writeError(w, http.StatusBadRequest, "variant name is required")
		return
	}

	if err := h.repo.AddVariant(ctx, variant); err != nil {
		writeInternalError(ctx, w, http.StatusBadRequest, err.Error(), err)
		return
	}

	writeJSON(w, http.StatusCreated, variant)
}

// UpdateVariant updates a variant in a draft experiment.
func (h *ABExperimentHandler) UpdateVariant(w http.ResponseWriter, r *http.Request) {
	if !h.checkInitialized(w) {
		return
	}
	ctx := r.Context()
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	variantID, ok := requireURLParam(w, r, "variantId")
	if !ok {
		return
	}

	req, ok := decodeJSON[updateVariantRequest](w, r)
	if !ok {
		return
	}

	exp, err := h.repo.GetExperiment(ctx, id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeEntityNotFound(w, "Experiment")
			return
		}
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to get experiment", err)
		return
	}

	if exp.Status != abtest.ExperimentStatusDraft {
		writeError(w, http.StatusBadRequest, "cannot modify variants on non-draft experiment")
		return
	}

	variant := &abtest.Variant{
		ID:              variantID,
		ExperimentID:    id,
		Name:            strings.TrimSpace(req.Name),
		TemplateVersion: req.TemplateVersion,
		TrafficPct:      req.TrafficPct,
		IsControl:       req.IsControl,
	}

	if variant.Name == "" {
		writeError(w, http.StatusBadRequest, "variant name is required")
		return
	}

	if err := h.repo.UpdateVariant(ctx, variant); err != nil {
		writeInternalError(ctx, w, http.StatusBadRequest, err.Error(), err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// DeleteVariant deletes a variant in a draft experiment.
func (h *ABExperimentHandler) DeleteVariant(w http.ResponseWriter, r *http.Request) {
	if !h.checkInitialized(w) {
		return
	}
	ctx := r.Context()
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	variantID, ok := requireURLParam(w, r, "variantId")
	if !ok {
		return
	}

	exp, err := h.repo.GetExperiment(ctx, id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeEntityNotFound(w, "Experiment")
			return
		}
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to get experiment", err)
		return
	}

	if exp.Status != abtest.ExperimentStatusDraft {
		writeError(w, http.StatusBadRequest, "cannot delete variants from non-draft experiment")
		return
	}

	if err := h.repo.DeleteVariant(ctx, variantID); err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to delete variant", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// GetMetrics returns aggregated metrics for an experiment.
func (h *ABExperimentHandler) GetMetrics(w http.ResponseWriter, r *http.Request) {
	if !h.checkInitialized(w) {
		return
	}
	ctx := r.Context()
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}

	exp, err := h.repo.GetExperiment(ctx, id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeEntityNotFound(w, "Experiment")
			return
		}
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to get experiment", err)
		return
	}

	start, end := parseStartEndQueries(r, exp)

	metrics, err := h.collector.ComputeMetrics(ctx, id, start, end)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to compute metrics", err)
		return
	}

	writeJSON(w, http.StatusOK, metrics)
}

// GetTimeseries returns daily timeseries performance metrics for the experiment.
func (h *ABExperimentHandler) GetTimeseries(w http.ResponseWriter, r *http.Request) {
	if !h.checkInitialized(w) {
		return
	}
	ctx := r.Context()
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}

	exp, err := h.repo.GetExperiment(ctx, id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeEntityNotFound(w, "Experiment")
			return
		}
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to get experiment", err)
		return
	}

	start, end := parseStartEndQueries(r, exp)

	// Clamp start date to 30 days ago to prevent performance/timeouts
	thirtyDaysAgo := time.Now().AddDate(0, 0, -30)
	if start.Before(thirtyDaysAgo) {
		start = thirtyDaysAgo
	}

	var series []dailyMetric

	// Step day-by-day
	for d := start; d.Before(end); d = d.AddDate(0, 0, 1) {
		dayStart := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, d.Location())
		dayEnd := dayStart.AddDate(0, 0, 1).Add(-time.Nanosecond)

		metrics, err := h.collector.ComputeMetrics(ctx, id, dayStart, dayEnd)
		if err != nil {
			writeInternalError(ctx, w, http.StatusInternalServerError, "failed to compute daily metrics", err)
			return
		}

		series = append(series, dailyMetric{
			Date:    dayStart.Format("2006-01-02"),
			Metrics: metrics,
		})
	}

	writeJSON(w, http.StatusOK, series)
}

// GetRecommendation returns automated winner recommendation analysis.
func (h *ABExperimentHandler) GetRecommendation(w http.ResponseWriter, r *http.Request) {
	if !h.checkInitialized(w) {
		return
	}
	ctx := r.Context()
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}

	rec, err := h.recommender.Recommend(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || strings.Contains(err.Error(), "not found") {
			writeEntityNotFound(w, "Experiment")
			return
		}
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to generate recommendation", err)
		return
	}

	writeJSON(w, http.StatusOK, rec)
}

func parseStartEndQueries(r *http.Request, exp *abtest.Experiment) (time.Time, time.Time) {
	startVal := r.URL.Query().Get("start")
	endVal := r.URL.Query().Get("end")

	start := exp.CreatedAt
	if exp.StartedAt != nil {
		start = *exp.StartedAt
	}
	if startVal != "" {
		if t, err := time.Parse(time.RFC3339, startVal); err == nil {
			start = t
		}
	}

	end := time.Now()
	if exp.EndedAt != nil {
		end = *exp.EndedAt
	}
	if endVal != "" {
		if t, err := time.Parse(time.RFC3339, endVal); err == nil {
			end = t
		}
	}

	return start, end
}
