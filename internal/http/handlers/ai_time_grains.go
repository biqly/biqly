package handlers

import (
	"net/http"
	"strings"

	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/go-chi/chi/v5"
)

type updateTimeGrainRequest struct {
	Suffix       string   `json:"suffix"`
	RequiresTime bool     `json:"requires_time"`
	Synonyms     []string `json:"synonyms"`
}

// AITimeGrainsHandler serves HTTP CRUD operations for time grain synonyms and suffixes.
type AITimeGrainsHandler struct {
	deps *app.Dependencies
}

// NewAITimeGrainsHandler creates a new AITimeGrainsHandler.
func NewAITimeGrainsHandler(deps *app.Dependencies) *AITimeGrainsHandler {
	return &AITimeGrainsHandler{deps: deps}
}

// ListTimeGrains returns the current customizable time grains.
func (h *AITimeGrainsHandler) ListTimeGrains(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	grains, err := h.deps.MetaRepo.ListTimeGrains(ctx)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to list time grains", err)
		return
	}
	writeJSON(w, http.StatusOK, grains)
}

// UpdateTimeGrain updates the suffix, requires_time and synonyms for a specific grain.
func (h *AITimeGrainsHandler) UpdateTimeGrain(w http.ResponseWriter, r *http.Request) {
	grain := strings.TrimSpace(chi.URLParam(r, "grain"))
	if grain == "" {
		writeError(w, http.StatusBadRequest, "grain is required")
		return
	}

	input, ok := decodeJSON[updateTimeGrainRequest](w, r)
	if !ok {
		return
	}

	suffix := strings.TrimSpace(input.Suffix)
	if suffix == "" {
		writeError(w, http.StatusBadRequest, "suffix is required")
		return
	}

	// Clean synonyms list
	var synonyms []string
	for _, syn := range input.Synonyms {
		s := strings.TrimSpace(syn)
		if s != "" {
			synonyms = append(synonyms, s)
		}
	}

	tg := metadata.TimeGrain{
		Grain:        grain,
		Suffix:       suffix,
		RequiresTime: input.RequiresTime,
		Synonyms:     synonyms,
	}

	ctx := r.Context()
	if err := h.deps.MetaRepo.UpdateTimeGrain(ctx, tg); err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to update time grain", err)
		return
	}

	if h.deps.TimeGrains != nil {
		h.deps.TimeGrains.Invalidate()
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
