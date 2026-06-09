package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"slices"

	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/i18n"
	"github.com/biqly/biqly/internal/metadata"
)

// tableSampleRowLimit caps how many rows the table sample-data preview returns.
const tableSampleRowLimit = 50

// MetadataHandler exposes endpoints for browsing and editing introspected metadata.
type MetadataHandler struct {
	deps *app.CatalogDeps
}

// NewMetadataHandler creates a new metadata handler.
func NewMetadataHandler(deps *app.CatalogDeps) *MetadataHandler {
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

func (*MetadataHandler) searchMetadata(w http.ResponseWriter, r *http.Request, search func(context.Context, string, string) (any, error), errMsg string) {
	datasourceID := r.URL.Query().Get("datasource_id")
	q := r.URL.Query().Get("q")
	if datasourceID == "" || q == "" {
		writeError(w, http.StatusBadRequest, "datasource_id and q parameters are required")
		return
	}
	result, err := search(r.Context(), datasourceID, q)
	if err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, errMsg, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// SearchColumns finds columns by name or description across a datasource.
func (h *MetadataHandler) SearchColumns(w http.ResponseWriter, r *http.Request) {
	h.searchMetadata(w, r, func(ctx context.Context, datasourceID, q string) (any, error) {
		return h.deps.MetaRepo.SearchColumns(ctx, datasourceID, q)
	}, "failed to search columns")
}

// SearchTables finds tables by name or description across a datasource.
func (h *MetadataHandler) SearchTables(w http.ResponseWriter, r *http.Request) {
	h.searchMetadata(w, r, func(ctx context.Context, datasourceID, q string) (any, error) {
		return h.deps.MetaRepo.SearchTables(ctx, datasourceID, q)
	}, "failed to search tables")
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

type sampleColumn struct {
	Name string `json:"name"`
}

type sampleData struct {
	Columns []sampleColumn `json:"columns"`
	Rows    [][]any        `json:"rows"`
}

// GetTableSample returns up to tableSampleRowLimit rows from a single
// introspected table so the UI can preview real data.
// GET /datasources/{id}/tables/{schema}/{table}/sample
func (h *MetadataHandler) GetTableSample(w http.ResponseWriter, r *http.Request) {
	datasourceID, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	schemaName, ok := requireURLParam(w, r, "schema")
	if !ok {
		return
	}
	tableName, ok := requireURLParam(w, r, "table")
	if !ok {
		return
	}
	ctx := r.Context()

	// Confine sampling to tables we have introspected: gives a clean 404 for
	// unknown tables and keeps the query off arbitrary identifiers. The query
	// below is built from the matched record's own fields (sourced from our
	// metadata DB), never the raw request strings.
	known, err := h.deps.MetaRepo.ListTables(ctx, datasourceID, schemaName)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to list tables", err)
		return
	}
	idx := slices.IndexFunc(known, func(t metadata.Table) bool { return t.TableName == tableName })
	if idx < 0 {
		writeEntityNotFound(w, "table")
		return
	}
	table := known[idx]

	resolved, err := h.deps.ResolveDatasourceDB(ctx, datasourceID)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to open connection", err)
		return
	}
	defer func() { _ = resolved.DB.Close() }()

	// Identifiers are quoted through the dialect and taken from the validated
	// metadata record, so no request input reaches the query.
	d := resolved.Driver.Dialect()
	tableRef := d.QuoteIdentSegment(table.SchemaName) + "." + d.QuoteIdentSegment(table.TableName)
	query := d.SelectWithLimit([]string{"*"}, tableRef, tableSampleRowLimit)

	sample, err := querySampleRows(ctx, resolved.DB, query)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to fetch sample rows", err)
		return
	}
	writeJSON(w, http.StatusOK, sample)
}

// querySampleRows runs query and scans every column into a generic grid,
// converting driver []byte cells (e.g. MySQL text) to strings so they
// serialize as readable JSON rather than base64.
func querySampleRows(ctx context.Context, db *sql.DB, query string) (_ *sampleData, err error) {
	rows, err := db.QueryContext(ctx, query) // nosemgrep: go.lang.security.audit.database.string-formatted-query.string-formatted-query
	if err != nil {
		return nil, fmt.Errorf("query sample rows: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	colNames, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("read columns: %w", err)
	}
	cols := make([]sampleColumn, len(colNames))
	for i, name := range colNames {
		cols[i] = sampleColumn{Name: name}
	}

	out := make([][]any, 0, tableSampleRowLimit)
	for rows.Next() {
		cells := make([]any, len(colNames))
		ptrs := make([]any, len(colNames))
		for i := range cells {
			ptrs[i] = &cells[i]
		}
		if scanErr := rows.Scan(ptrs...); scanErr != nil {
			return nil, fmt.Errorf("scan sample row: %w", scanErr)
		}
		for i, c := range cells {
			if b, ok := c.([]byte); ok {
				cells[i] = string(b)
			}
		}
		out = append(out, cells)
	}
	if scanErr := rows.Err(); scanErr != nil {
		return nil, fmt.Errorf("iterate sample rows: %w", scanErr)
	}
	return &sampleData{Columns: cols, Rows: out}, nil
}
