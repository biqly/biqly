package handlers

import (
	"net/http"
	"time"

	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/metadata"
	pkgmetadata "github.com/biqly/biqly/pkg/metadata"
)

type createGlossaryRequest struct {
	DatasourceID string                         `json:"datasource_id"`
	ModelID      string                         `json:"model_id,omitempty"`
	Term         string                         `json:"term"`
	Definition   string                         `json:"definition,omitempty"`
	MapsToType   string                         `json:"maps_to_type"`
	MapsToName   string                         `json:"maps_to_name"`
	Aliases      []string                       `json:"aliases,omitempty"`
	AIContext    *pkgmetadata.GlossaryAIContext `json:"ai_context,omitempty"`
}

type updateGlossaryRequest struct {
	Term       string                         `json:"term"`
	Definition string                         `json:"definition,omitempty"`
	MapsToType string                         `json:"maps_to_type"`
	MapsToName string                         `json:"maps_to_name"`
	Aliases    []string                       `json:"aliases,omitempty"`
	AIContext  *pkgmetadata.GlossaryAIContext `json:"ai_context,omitempty"`
	IsActive   *bool                          `json:"is_active,omitempty"`
}

// BusinessGlossaryTerm is the wire format for a curated glossary row.
type BusinessGlossaryTerm = metadata.BusinessGlossaryRow

// AIGlossaryHandler handles business glossary CRUD.
type AIGlossaryHandler struct {
	deps *app.AIDeps
}

// NewAIGlossaryHandler creates a glossary handler.
func NewAIGlossaryHandler(deps *app.AIDeps) *AIGlossaryHandler {
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
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to list glossary", err)
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

// CreateGlossary creates a glossary term.
func (h *AIGlossaryHandler) CreateGlossary(w http.ResponseWriter, r *http.Request) {
	input, ok := decodeJSON[createGlossaryRequest](w, r)
	if !ok {
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
		AIContext:    input.AIContext,
	})
	if err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "failed to create glossary term", err)
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
		AIContext:    input.AIContext,
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	})
}

// UpdateGlossary updates a glossary term.
func (h *AIGlossaryHandler) UpdateGlossary(w http.ResponseWriter, r *http.Request) {
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	input, ok := decodeJSON[updateGlossaryRequest](w, r)
	if !ok {
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
		AIContext:  input.AIContext,
		IsActive:   input.IsActive,
	}); err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "failed to update glossary term", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// DeleteGlossary deletes a glossary term.
func (h *AIGlossaryHandler) DeleteGlossary(w http.ResponseWriter, r *http.Request) {
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	ok, err := h.deps.MetaRepo.DeleteBusinessGlossary(r.Context(), id)
	if err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "failed to delete glossary term", err)
		return
	}
	if !ok {
		writeEntityNotFound(w, "glossary term")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
