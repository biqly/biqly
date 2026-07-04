package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/biqly/biqly/internal/semantic"
)

const modelFileContentType = "application/yaml"

// ExportModel serializes the current (draft) state of a semantic model into
// the portable YAML file format.
func (h *SemanticHandler) ExportModel(w http.ResponseWriter, r *http.Request) {
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	ctx := r.Context()
	model, err := h.deps.SemanticRepo.GetFullModel(ctx, id)
	if err != nil {
		writeEntityNotFound(w, "model")
		return
	}
	writeModelFile(w, r, model)
}

// ExportModelVersion serializes a published snapshot of a semantic model into
// the portable YAML file format, for download or version diffing.
func (h *SemanticHandler) ExportModelVersion(w http.ResponseWriter, r *http.Request) {
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	version, err := strconv.Atoi(chi.URLParam(r, "version"))
	if err != nil || version < 1 {
		writeError(w, http.StatusBadRequest, "invalid version")
		return
	}
	ctx := r.Context()
	model, err := h.deps.SemanticRepo.GetModelSnapshot(ctx, id, version)
	if err != nil {
		writeEntityNotFound(w, "model version")
		return
	}
	writeModelFile(w, r, model)
}

func writeModelFile(w http.ResponseWriter, r *http.Request, model *semantic.SemanticModel) {
	out, err := semantic.MarshalModelFile(semantic.NewModelFile(model))
	if err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "failed to serialize model", err)
		return
	}
	w.Header().Set("Content-Type", modelFileContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", model.Name+".yaml"))
	w.WriteHeader(http.StatusOK)
	// #nosec G705 -- YAML file download (application/yaml + attachment disposition), not HTML output.
	_, _ = w.Write(out)
}

// ListModelVersions returns published version metadata for a model.
func (h *SemanticHandler) ListModelVersions(w http.ResponseWriter, r *http.Request) {
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	ctx := r.Context()
	versions, err := h.deps.SemanticRepo.ListModelSnapshots(ctx, id)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to list model versions", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"versions": versions})
}

type importModelRequest struct {
	DatasourceID string `json:"datasource_id"`
	Content      string `json:"content"`
}

type importModelResponse struct {
	Model      *semantic.SemanticModel          `json:"model"`
	Validation semantic.PublishValidationResult `json:"validation"`
}

// ImportModel creates a new draft semantic model from a portable YAML/JSON
// model file. The file is validated against the datasource catalog before
// anything is persisted; validation failures return 422 with details.
func (h *SemanticHandler) ImportModel(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[importModelRequest](w, r)
	if !ok {
		return
	}
	if req.DatasourceID == "" || strings.TrimSpace(req.Content) == "" {
		writeError(w, http.StatusBadRequest, "datasource_id and content are required")
		return
	}
	file, err := semantic.ParseModelFile([]byte(req.Content))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	ctx := r.Context()
	model := file.Model(req.DatasourceID)
	model.ID = uuid.New().String()
	for i := range model.Dimensions {
		model.Dimensions[i].ID = uuid.New().String()
		model.Dimensions[i].ModelID = model.ID
	}
	for i := range model.Metrics {
		model.Metrics[i].ID = uuid.New().String()
		model.Metrics[i].ModelID = model.ID
	}
	for i := range model.Joins {
		model.Joins[i].ID = uuid.New().String()
		model.Joins[i].ModelID = model.ID
	}

	validation := semantic.ValidateContext(ctx, *model, semanticCatalogAdapter{repo: h.deps.MetaRepo})
	if !validation.Valid {
		writeJSON(w, http.StatusUnprocessableEntity, importModelResponse{Validation: validation})
		return
	}

	if err := h.persistGeneratedModel(ctx, model); err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to import model", err)
		return
	}
	for i := range model.Dimensions {
		d := &model.Dimensions[i]
		if len(d.EnumValues) == 0 {
			continue
		}
		if err := h.deps.SemanticRepo.ReplaceEnumMappings(ctx, model.ID, d.ID, d.EnumValues); err != nil {
			writeInternalError(ctx, w, http.StatusInternalServerError, "failed to import enum values", err)
			return
		}
	}

	full, err := h.deps.SemanticRepo.GetFullModel(ctx, model.ID)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to load imported model", err)
		return
	}
	writeJSON(w, http.StatusCreated, importModelResponse{Model: full, Validation: validation})
}
