package handlers

import (
	"net/http"

	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/i18n"
	"github.com/biqly/biqly/internal/metadata"
)

// MetadataHandler exposes endpoints for browsing and editing introspected metadata.
type MetadataHandler struct {
	deps *app.Dependencies
}

// NewMetadataHandler creates a new metadata handler.
func NewMetadataHandler(deps *app.Dependencies) *MetadataHandler {
	return &MetadataHandler{deps: deps}
}

// ListTables returns all introspected tables for a datasource (optionally filtered by schema).
func (h *MetadataHandler) ListTables(w http.ResponseWriter, r *http.Request) {
	datasourceID, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	schemaName := r.URL.Query().Get("schema")

	tables, err := h.deps.MetaRepo.ListTables(r.Context(), datasourceID, schemaName)
	if err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "failed to list tables", err)
		return
	}
	loc := i18n.FromContext(r.Context())
	if err := h.deps.MetaRepo.ApplyTableTranslations(r.Context(), tables, loc); err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "failed to apply table translations", err)
		return
	}
	writeJSON(w, http.StatusOK, tables)
}

// ListColumns returns columns for a datasource, scoped by schema/table query params.
func (h *MetadataHandler) ListColumns(w http.ResponseWriter, r *http.Request) {
	datasourceID, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	schemaName := r.URL.Query().Get("schema")
	tableName := r.URL.Query().Get("table")

	cols, err := h.deps.MetaRepo.ListColumns(r.Context(), datasourceID, schemaName, tableName)
	if err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "failed to list columns", err)
		return
	}
	loc := i18n.FromContext(r.Context())
	if err := h.deps.MetaRepo.ApplyColumnTranslations(r.Context(), cols, loc); err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "failed to apply column translations", err)
		return
	}
	writeJSON(w, http.StatusOK, cols)
}

// SearchColumns finds columns by name or description across a datasource.
func (h *MetadataHandler) SearchColumns(w http.ResponseWriter, r *http.Request) {
	datasourceID := r.URL.Query().Get("datasource_id")
	q := r.URL.Query().Get("q")
	if datasourceID == "" || q == "" {
		writeError(w, http.StatusBadRequest, "datasource_id and q parameters are required")
		return
	}
	cols, err := h.deps.MetaRepo.SearchColumns(r.Context(), datasourceID, q)
	if err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "failed to search columns", err)
		return
	}
	writeJSON(w, http.StatusOK, cols)
}

// SearchTables finds tables by name or description across a datasource.
func (h *MetadataHandler) SearchTables(w http.ResponseWriter, r *http.Request) {
	datasourceID := r.URL.Query().Get("datasource_id")
	q := r.URL.Query().Get("q")
	if datasourceID == "" || q == "" {
		writeError(w, http.StatusBadRequest, "datasource_id and q parameters are required")
		return
	}
	tables, err := h.deps.MetaRepo.SearchTables(r.Context(), datasourceID, q)
	if err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "failed to search tables", err)
		return
	}
	writeJSON(w, http.StatusOK, tables)
}

type updateDescriptionRequest struct {
	Description *string `json:"description,omitempty"`
	Label       *string `json:"label,omitempty"`
}

// UpdateTableDescription edits the description and/or label of a single table row.
func (h *MetadataHandler) UpdateTableDescription(w http.ResponseWriter, r *http.Request) {
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}

	req, ok := decodeJSON[updateDescriptionRequest](w, r)
	if !ok {
		return
	}

	if req.Description != nil || req.Label != nil {
		if err := h.deps.MetaRepo.UpdateTableDescriptionAndLabel(r.Context(), id, req.Description, req.Label); err != nil {
			writeInternalError(r.Context(), w, http.StatusInternalServerError, "failed to update table description", err)
			return
		}
	}

	t, err := h.deps.MetaRepo.GetTable(r.Context(), id)
	if err != nil {
		writeEntityNotFound(w, "table")
		return
	}
	writeJSON(w, http.StatusOK, t)
}

// UpdateColumnDescription edits the description of a single column row.
func (h *MetadataHandler) UpdateColumnDescription(w http.ResponseWriter, r *http.Request) {
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}

	req, ok := decodeJSON[updateDescriptionRequest](w, r)
	if !ok {
		return
	}

	if err := h.deps.MetaRepo.UpdateColumnDescription(r.Context(), id, req.Description); err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "failed to update column description", err)
		return
	}

	c, err := h.deps.MetaRepo.GetColumn(r.Context(), id)
	if err != nil {
		writeEntityNotFound(w, "column")
		return
	}
	writeJSON(w, http.StatusOK, c)
}

// translationUpsertRequest is the payload for PUT /metadata/{entity}/{id}/translations.
// Body shape: { "tr": { "description": "..." }, "en": { ... } }
type translationUpsertRequest map[string]map[string]string

func (h *MetadataHandler) putTranslations(w http.ResponseWriter, r *http.Request, entityType string) {
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	req, ok := decodeJSON[translationUpsertRequest](w, r)
	if !ok {
		return
	}
	for lang, fields := range *req {
		loc := i18n.ParseLocale(lang)
		for field, value := range fields {
			if field != metadata.TranslationFieldDescription && field != metadata.TranslationFieldLabel {
				writeError(w, http.StatusBadRequest, "unsupported translation field: "+field)
				return
			}
			err := h.deps.MetaRepo.UpsertTranslation(r.Context(), metadata.Translation{
				EntityType: entityType,
				EntityID:   id,
				Lang:       string(loc),
				Field:      field,
				Value:      value,
			})
			if err != nil {
				writeInternalError(r.Context(), w, http.StatusInternalServerError, "failed to upsert translation", err)
				return
			}
		}
	}
	rows, err := h.deps.MetaRepo.ListEntityTranslations(r.Context(), entityType, id)
	if err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "failed to list translations", err)
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

// GetTableTranslations returns every stored translation row for a table.
func (h *MetadataHandler) GetTableTranslations(w http.ResponseWriter, r *http.Request) {
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	rows, err := h.deps.MetaRepo.ListEntityTranslations(r.Context(), metadata.EntityTypeTable, id)
	if err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "failed to list table translations", err, "entity_id", id)
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

// PutTableTranslations upserts language-specific overrides for a table.
func (h *MetadataHandler) PutTableTranslations(w http.ResponseWriter, r *http.Request) {
	h.putTranslations(w, r, metadata.EntityTypeTable)
}

// GetColumnTranslations returns every stored translation row for a column.
func (h *MetadataHandler) GetColumnTranslations(w http.ResponseWriter, r *http.Request) {
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	rows, err := h.deps.MetaRepo.ListEntityTranslations(r.Context(), metadata.EntityTypeColumn, id)
	if err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "failed to list column translations", err, "entity_id", id)
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

// PutColumnTranslations upserts language-specific overrides for a column.
func (h *MetadataHandler) PutColumnTranslations(w http.ResponseWriter, r *http.Request) {
	h.putTranslations(w, r, metadata.EntityTypeColumn)
}
