package handlers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/biqly/biqly/internal/catalog/dbt"
	"github.com/biqly/biqly/internal/semantic"
	"github.com/google/uuid"
)

type dbtImportedModel struct {
	Model      *semantic.SemanticModel          `json:"model"`
	Validation semantic.PublishValidationResult `json:"validation"`
}

type dbtImportResponse struct {
	ImportedModels []dbtImportedModel `json:"imported_models"`
	Skipped        []string           `json:"skipped,omitempty"`
	Warnings       []string           `json:"warnings,omitempty"`
}

// ImportDbtProject converts dbt manifest.json (+ optional catalog.json) into
// draft semantic models for a datasource. Models are created as drafts only —
// callers review and publish via the modeling UI.
//
// POST /api/catalog/dbt/import
func (h *SemanticHandler) ImportDbtProject(w http.ResponseWriter, r *http.Request) {
	datasourceID := strings.TrimSpace(r.URL.Query().Get("datasource_id"))
	if datasourceID == "" {
		writeError(w, http.StatusBadRequest, "datasource_id is required")
		return
	}

	manifest, catalogData, ok := readDBTImportFiles(w, r)
	if !ok {
		return
	}

	project, err := dbt.ParseProject(manifest, catalogData)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx := r.Context()
	existingNames, err := h.existingModelNames(ctx, datasourceID)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to list existing models", err)
		return
	}

	converted := dbt.ConvertProject(project, dbt.ConvertOptions{
		DatasourceID:  datasourceID,
		ExistingNames: existingNames,
	})

	resp := dbtImportResponse{
		ImportedModels: make([]dbtImportedModel, 0, len(converted.Models)),
		Skipped:        converted.Skipped,
		Warnings:       converted.Warnings,
	}
	catalog := semanticCatalogAdapter{repo: h.deps.MetaRepo}

	for _, model := range converted.Models {
		imported, skipWarn, err := h.persistDbtModel(ctx, model, catalog)
		if err != nil {
			writeInternalError(ctx, w, http.StatusInternalServerError, "failed to import dbt model", err)
			return
		}
		if skipWarn != "" {
			resp.Skipped = append(resp.Skipped, model.Name)
			resp.Warnings = append(resp.Warnings, skipWarn)
			continue
		}
		resp.ImportedModels = append(resp.ImportedModels, imported)
		if !imported.Validation.Valid {
			for _, e := range imported.Validation.Errors {
				resp.Warnings = append(resp.Warnings, model.Name+": "+e)
			}
		}
	}

	status := http.StatusCreated
	if len(resp.ImportedModels) == 0 {
		status = http.StatusUnprocessableEntity
	}
	writeJSON(w, status, resp)
}

func readDBTImportFiles(w http.ResponseWriter, r *http.Request) (manifest, catalog []byte, ok bool) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "invalid multipart request")
		return nil, nil, false
	}

	manifest, err := readMultipartFile(r, "manifest")
	if errors.Is(err, http.ErrMissingFile) {
		writeError(w, http.StatusBadRequest, "manifest file is required")
		return nil, nil, false
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read manifest file")
		return nil, nil, false
	}

	catalog, err = readMultipartFile(r, "catalog")
	if errors.Is(err, http.ErrMissingFile) {
		return manifest, nil, true
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read catalog file")
		return nil, nil, false
	}
	return manifest, catalog, true
}

func readMultipartFile(r *http.Request, field string) ([]byte, error) {
	file, _, err := r.FormFile(field)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	return io.ReadAll(file)
}

func (h *SemanticHandler) existingModelNames(ctx context.Context, datasourceID string) ([]string, error) {
	existing, err := h.deps.SemanticRepo.ListModels(ctx, datasourceID)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(existing))
	for _, m := range existing {
		names = append(names, m.Name)
	}
	return names, nil
}

func rebindModelChildren(model *semantic.SemanticModel) {
	for i := range model.Dimensions {
		if model.Dimensions[i].ID == "" {
			model.Dimensions[i].ID = uuid.New().String()
		}
		model.Dimensions[i].ModelID = model.ID
	}
	for i := range model.Metrics {
		if model.Metrics[i].ID == "" {
			model.Metrics[i].ID = uuid.New().String()
		}
		model.Metrics[i].ModelID = model.ID
	}
	for i := range model.Joins {
		if model.Joins[i].ID == "" {
			model.Joins[i].ID = uuid.New().String()
		}
		model.Joins[i].ModelID = model.ID
	}
}

// persistDbtModel saves one converted draft. Catalog validation failures are
// returned as warnings on the imported model (publish re-validates) — dbt
// tables may not be synced into biqly metadata yet.
func (h *SemanticHandler) persistDbtModel(ctx context.Context, model *semantic.SemanticModel, catalog semanticCatalogAdapter) (dbtImportedModel, string, error) {
	if strings.TrimSpace(model.Name) == "" || strings.TrimSpace(model.BaseTable) == "" {
		return dbtImportedModel{}, "skipped model with empty name or base_table", nil
	}
	rebindModelChildren(model)

	if err := h.persistGeneratedModel(ctx, model); err != nil {
		return dbtImportedModel{}, "", err
	}
	for i := range model.Dimensions {
		d := &model.Dimensions[i]
		if len(d.EnumValues) == 0 {
			continue
		}
		if err := h.deps.SemanticRepo.ReplaceEnumMappings(ctx, model.ID, d.ID, d.EnumValues); err != nil {
			return dbtImportedModel{}, "", h.cleanupDbtDraft(ctx, model.ID, err)
		}
	}

	validation := semantic.ValidateContext(ctx, *model, catalog)
	full, err := h.deps.SemanticRepo.GetFullModel(ctx, model.ID)
	if err != nil {
		return dbtImportedModel{}, "", h.cleanupDbtDraft(ctx, model.ID, err)
	}
	return dbtImportedModel{Model: full, Validation: validation}, "", nil
}

func (h *SemanticHandler) cleanupDbtDraft(ctx context.Context, modelID string, cause error) error {
	if err := h.deps.SemanticRepo.DeleteModel(ctx, modelID); err != nil {
		return fmt.Errorf("%w; delete incomplete draft: %w", cause, err)
	}
	return cause
}
