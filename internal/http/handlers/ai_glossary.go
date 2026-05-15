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

// BusinessGlossaryTerm is the wire format for a curated glossary row.
type BusinessGlossaryTerm = metadata.BusinessGlossaryRow

// AIGlossaryHandler handles business glossary CRUD.
type AIGlossaryHandler struct {
	deps *app.Dependencies
}

// NewAIGlossaryHandler creates a glossary handler.
func NewAIGlossaryHandler(deps *app.Dependencies) *AIGlossaryHandler {
	return &AIGlossaryHandler{deps: deps}
}

// ListGlossary returns glossary terms for a datasource.
func (h *AIGlossaryHandler) ListGlossary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	datasourceID := r.URL.Query().Get("datasource_id")
	if datasourceID == "" {
		writeError(w, http.StatusBadRequest, "datasource_id is required")
		return
	}
	modelID := r.URL.Query().Get("model_id")

	rows, err := h.deps.MetaRepo.ListBusinessGlossary(ctx, datasourceID, modelID)
	if err != nil {
		slog.ErrorContext(ctx, "list business glossary failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list glossary")
		return
	}
	if rows == nil {
		rows = []metadata.BusinessGlossaryRow{}
	}
	writeJSON(w, http.StatusOK, rows)
}

// CreateGlossary creates a glossary term.
func (h *AIGlossaryHandler) CreateGlossary(w http.ResponseWriter, r *http.Request) {
	var input struct {
		DatasourceID string   `json:"datasource_id"`
		ModelID      string   `json:"model_id,omitempty"`
		Term         string   `json:"term"`
		Definition   string   `json:"definition,omitempty"`
		MapsToType   string   `json:"maps_to_type"`
		MapsToName   string   `json:"maps_to_name"`
		Aliases      []string `json:"aliases,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if input.DatasourceID == "" || input.Term == "" || input.MapsToType == "" || input.MapsToName == "" {
		writeError(w, http.StatusBadRequest, "datasource_id, term, maps_to_type, and maps_to_name are required")
		return
	}
	switch input.MapsToType {
	case "dimension", "metric", "model":
	default:
		writeError(w, http.StatusBadRequest, "maps_to_type must be dimension, metric, or model")
		return
	}

	id, err := h.deps.MetaRepo.InsertBusinessGlossary(r.Context(), metadata.BusinessGlossaryInsert{
		DatasourceID: input.DatasourceID,
		ModelID:      input.ModelID,
		Term:         input.Term,
		Definition:   input.Definition,
		MapsToType:   input.MapsToType,
		MapsToName:   input.MapsToName,
		Aliases:      input.Aliases,
	})
	if err != nil {
		slog.ErrorContext(r.Context(), "create business glossary failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create glossary term")
		return
	}
	now := time.Now()
	writeJSON(w, http.StatusCreated, BusinessGlossaryTerm{
		ID:           id,
		DatasourceID: input.DatasourceID,
		ModelID:      input.ModelID,
		Term:         input.Term,
		Definition:   input.Definition,
		MapsToType:   input.MapsToType,
		MapsToName:   input.MapsToName,
		Aliases:      input.Aliases,
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	})
}

// UpdateGlossary updates a glossary term.
func (h *AIGlossaryHandler) UpdateGlossary(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}
	var input struct {
		Term       string   `json:"term"`
		Definition string   `json:"definition,omitempty"`
		MapsToType string   `json:"maps_to_type"`
		MapsToName string   `json:"maps_to_name"`
		Aliases    []string `json:"aliases,omitempty"`
		IsActive   *bool    `json:"is_active,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if input.Term == "" || input.MapsToType == "" || input.MapsToName == "" {
		writeError(w, http.StatusBadRequest, "term, maps_to_type, and maps_to_name are required")
		return
	}
	switch input.MapsToType {
	case "dimension", "metric", "model":
	default:
		writeError(w, http.StatusBadRequest, "maps_to_type must be dimension, metric, or model")
		return
	}
	if err := h.deps.MetaRepo.UpdateBusinessGlossary(r.Context(), id, metadata.BusinessGlossaryUpdate{
		Term:       input.Term,
		Definition: input.Definition,
		MapsToType: input.MapsToType,
		MapsToName: input.MapsToName,
		Aliases:    input.Aliases,
		IsActive:   input.IsActive,
	}); err != nil {
		slog.ErrorContext(r.Context(), "update business glossary failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update glossary term")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// DeleteGlossary deletes a glossary term.
func (h *AIGlossaryHandler) DeleteGlossary(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}
	ok, err := h.deps.MetaRepo.DeleteBusinessGlossary(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "delete business glossary failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to delete glossary term")
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "glossary term not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
