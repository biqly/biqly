package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/biqly/biqly/internal/app"
	"github.com/go-chi/chi/v5"
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
	datasourceID := chi.URLParam(r, "id")
	schemaName := r.URL.Query().Get("schema")

	tables, err := h.deps.MetaRepo.ListTables(r.Context(), datasourceID, schemaName)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list tables")
		return
	}
	writeJSON(w, http.StatusOK, tables)
}

// ListColumns returns columns for a datasource, scoped by schema/table query params.
func (h *MetadataHandler) ListColumns(w http.ResponseWriter, r *http.Request) {
	datasourceID := chi.URLParam(r, "id")
	schemaName := r.URL.Query().Get("schema")
	tableName := r.URL.Query().Get("table")

	cols, err := h.deps.MetaRepo.ListColumns(r.Context(), datasourceID, schemaName, tableName)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list columns")
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
		writeError(w, http.StatusInternalServerError, "failed to search columns")
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
		writeError(w, http.StatusInternalServerError, "failed to search tables")
		return
	}
	writeJSON(w, http.StatusOK, tables)
}

type updateDescriptionRequest struct {
	Description *string `json:"description"`
}

// UpdateTableDescription edits the description of a single table row.
func (h *MetadataHandler) UpdateTableDescription(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req updateDescriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.deps.MetaRepo.UpdateTableDescription(r.Context(), id, req.Description); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update table description")
		return
	}

	t, err := h.deps.MetaRepo.GetTable(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "table not found")
		return
	}
	writeJSON(w, http.StatusOK, t)
}

// UpdateColumnDescription edits the description of a single column row.
func (h *MetadataHandler) UpdateColumnDescription(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req updateDescriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.deps.MetaRepo.UpdateColumnDescription(r.Context(), id, req.Description); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update column description")
		return
	}

	c, err := h.deps.MetaRepo.GetColumn(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "column not found")
		return
	}
	writeJSON(w, http.StatusOK, c)
}
