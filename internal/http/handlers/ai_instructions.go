package handlers

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/metadata"
)

// AIInstructionsHandler serves the free-form business rules ("instructions")
// surface: datasource-scoped markdown rules injected into the text-to-SQL
// prompt as a "## Business Rules" block.
type AIInstructionsHandler struct {
	deps *app.AIDeps
}

// NewAIInstructionsHandler creates an AIInstructionsHandler.
func NewAIInstructionsHandler(deps *app.AIDeps) *AIInstructionsHandler {
	return &AIInstructionsHandler{deps: deps}
}

type instructionPayload struct {
	DatasourceID string `json:"datasource_id"`
	ModelID      string `json:"model_id,omitempty"`
	Title        string `json:"title"`
	BodyMD       string `json:"body_md"`
	IsActive     *bool  `json:"is_active,omitempty"`
}

func (p *instructionPayload) validate() string {
	if strings.TrimSpace(p.Title) == "" {
		return "title is required"
	}
	if strings.TrimSpace(p.BodyMD) == "" {
		return "body_md is required"
	}
	return ""
}

type instructionResponse struct {
	ID           string `json:"id"`
	DatasourceID string `json:"datasource_id"`
	ModelID      string `json:"model_id,omitempty"`
	Title        string `json:"title"`
	BodyMD       string `json:"body_md"`
	IsActive     bool   `json:"is_active"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

func instructionRowToResponse(row metadata.InstructionRow) instructionResponse {
	return instructionResponse{
		ID:           row.ID,
		DatasourceID: row.DatasourceID,
		ModelID:      row.ModelID,
		Title:        row.Title,
		BodyMD:       row.BodyMD,
		IsActive:     row.IsActive,
		CreatedAt:    row.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:    row.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

// List returns instructions for a datasource, including inactive ones so the
// admin can manage them.
func (h *AIInstructionsHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	datasourceID, ok := requireQueryParam(w, r, "datasource_id")
	if !ok {
		return
	}
	rows, err := h.deps.MetaRepo.ListInstructions(ctx, datasourceID)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to list instructions", err)
		return
	}
	out := make([]instructionResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, instructionRowToResponse(row))
	}
	writeJSON(w, http.StatusOK, map[string]any{"instructions": out})
}

// Create stores a new instruction. New instructions are active by default; pass
// is_active=false to create a deactivated rule.
func (h *AIInstructionsHandler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	payload, ok := decodeJSON[instructionPayload](w, r)
	if !ok {
		return
	}
	if payload.DatasourceID == "" {
		writeError(w, http.StatusBadRequest, "datasource_id is required")
		return
	}
	if msg := payload.validate(); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	title := strings.TrimSpace(payload.Title)
	id, err := h.deps.MetaRepo.InsertInstruction(ctx, metadata.InstructionInsert{
		DatasourceID: payload.DatasourceID,
		ModelID:      payload.ModelID,
		Title:        title,
		BodyMD:       payload.BodyMD,
	})
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to create instruction", err)
		return
	}
	// InsertInstruction creates an active row; honor an explicit is_active=false.
	if payload.IsActive != nil && !*payload.IsActive {
		if err := h.deps.MetaRepo.UpdateInstruction(ctx, id, metadata.InstructionUpdate{
			Title:    title,
			BodyMD:   payload.BodyMD,
			IsActive: false,
		}); err != nil {
			writeInternalError(ctx, w, http.StatusInternalServerError, "failed to create instruction", err)
			return
		}
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": id})
}

// Update replaces the editable fields of an instruction.
func (h *AIInstructionsHandler) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	payload, ok := decodeJSON[instructionPayload](w, r)
	if !ok {
		return
	}
	if msg := payload.validate(); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	isActive := true
	if payload.IsActive != nil {
		isActive = *payload.IsActive
	}
	err := h.deps.MetaRepo.UpdateInstruction(ctx, chi.URLParam(r, "id"), metadata.InstructionUpdate{
		Title:    strings.TrimSpace(payload.Title),
		BodyMD:   payload.BodyMD,
		IsActive: isActive,
	})
	if err != nil {
		if errors.Is(err, metadata.ErrInstructionNotFound) {
			writeEntityNotFound(w, "instruction")
			return
		}
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to update instruction", err)
		return
	}
	writeOK(w)
}

// Delete removes an instruction.
func (h *AIInstructionsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := h.deps.MetaRepo.DeleteInstruction(ctx, chi.URLParam(r, "id")); err != nil {
		if errors.Is(err, metadata.ErrInstructionNotFound) {
			writeEntityNotFound(w, "instruction")
			return
		}
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to delete instruction", err)
		return
	}
	writeOK(w)
}
