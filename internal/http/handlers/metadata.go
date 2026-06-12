package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/biqly/biqly/internal/app"
	bimw "github.com/biqly/biqly/internal/http/middleware"
	"github.com/biqly/biqly/internal/i18n"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/go-chi/chi/v5"
)

// tableSampleRowLimit caps how many rows the table sample-data preview returns.
const tableSampleRowLimit = 50

// MetadataHandler exposes endpoints for browsing and editing introspected metadata.
type MetadataHandler struct {
	deps          *app.CatalogDeps
	accessChecker metadataDatasourceAccessChecker
}

type metadataDatasourceAccessChecker interface {
	CheckDatasourceAccess(ctx context.Context, userID, datasourceID, level string) (bool, error)
}

// NewMetadataHandler creates a new metadata handler.
func NewMetadataHandler(deps *app.CatalogDeps) *MetadataHandler {
	return &MetadataHandler{deps: deps}
}

// SetDatasourceAccessChecker enables handler-level checks for metadata routes
// that carry table/column IDs instead of datasource IDs.
func (h *MetadataHandler) SetDatasourceAccessChecker(checker metadataDatasourceAccessChecker) {
	h.accessChecker = checker
}

// ResolveDatasourceID extracts the datasource ID from the request using URL parameters,
// query parameters, or performing an entity lookup (for tables/columns).
func (h *MetadataHandler) ResolveDatasourceID(r *http.Request) (string, error) {
	ctx := r.Context()
	// 1. Direct URL params
	if id := chi.URLParam(r, "datasourceID"); id != "" {
		return id, nil
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		// 2. Query param
		if qid := r.URL.Query().Get("datasource_id"); qid != "" {
			return qid, nil
		}
		return "", errors.New("datasource id required")
	}

	path := r.URL.Path
	if strings.Contains(path, "/metadata/tables/") {
		t, err := h.deps.MetaRepo.GetTable(ctx, id)
		if err != nil {
			return "", err
		}
		return t.DatasourceID, nil
	}
	if strings.Contains(path, "/metadata/columns/") {
		c, err := h.deps.MetaRepo.GetColumn(ctx, id)
		if err != nil {
			return "", err
		}
		return c.DatasourceID, nil
	}
	return id, nil
}

// CheckDatasourceAccess checks if the current user has the given access level on the datasource.
func (h *MetadataHandler) CheckDatasourceAccess(ctx context.Context, datasourceID, level string) (bool, error) {
	if h.accessChecker == nil || bimw.HasRole(ctx, bimw.RoleSuperAdmin) {
		return true, nil
	}
	userID := bimw.UserID(ctx)
	if userID == "" {
		return false, errors.New("authentication required")
	}
	return h.accessChecker.CheckDatasourceAccess(ctx, userID, datasourceID, level)
}

func (h *MetadataHandler) requireDatasourceAccess(w http.ResponseWriter, r *http.Request, datasourceID, level string) bool {
	allowed, err := h.CheckDatasourceAccess(r.Context(), datasourceID, level)
	if err != nil {
		if err.Error() == "authentication required" {
			writeError(w, http.StatusUnauthorized, "authentication required")
		} else {
			writeError(w, http.StatusServiceUnavailable, "datasource access check failed")
		}
		return false
	}
	if !allowed {
		writeError(w, http.StatusForbidden, "datasource access denied")
		return false
	}
	return true
}

func (h *MetadataHandler) requireTableAccess(w http.ResponseWriter, r *http.Request, id, level string) bool {
	t, err := h.deps.MetaRepo.GetTable(r.Context(), id)
	if err != nil {
		writeEntityNotFound(w, "table")
		return false
	}
	return h.requireDatasourceAccess(w, r, t.DatasourceID, level)
}

func (h *MetadataHandler) requireColumnAccess(w http.ResponseWriter, r *http.Request, id, level string) bool {
	c, err := h.deps.MetaRepo.GetColumn(r.Context(), id)
	if err != nil {
		writeEntityNotFound(w, "column")
		return false
	}
	return h.requireDatasourceAccess(w, r, c.DatasourceID, level)
}

func (h *MetadataHandler) requireMetadataEntityAccess(w http.ResponseWriter, r *http.Request, entityType, id, level string) bool {
	switch entityType {
	case metadata.EntityTypeTable:
		return h.requireTableAccess(w, r, id, level)
	case metadata.EntityTypeColumn:
		return h.requireColumnAccess(w, r, id, level)
	default:
		writeError(w, http.StatusBadRequest, "unsupported metadata entity type")
		return false
	}
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
	// DisplayExpression sets the row display label template for a table;
	// an empty string clears it.
	DisplayExpression *string `json:"display_expression,omitempty"`
}

const maxDisplayExpressionRunes = 512

var displayExpressionColumnTokenRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func validateDisplayExpression(expr string) error {
	if expr == "" {
		return nil
	}
	if utf8.RuneCountInString(expr) > maxDisplayExpressionRunes {
		return fmt.Errorf("display_expression must be at most %d characters", maxDisplayExpressionRunes)
	}
	parts, err := splitDisplayExpressionParts(expr)
	if err != nil {
		return err
	}
	if len(parts) == 0 {
		return nil
	}
	for _, part := range parts {
		if isDisplayExpressionLiteral(part) {
			continue
		}
		if !displayExpressionColumnTokenRE.MatchString(part) {
			return fmt.Errorf("invalid display_expression token %q", part)
		}
	}
	return nil
}

func splitDisplayExpressionParts(expr string) ([]string, error) {
	parts := make([]string, 0, 4)
	var current strings.Builder
	var quote rune
	for _, ch := range expr {
		if quote != 0 {
			current.WriteRune(ch)
			if ch == quote {
				quote = 0
			}
			continue
		}
		switch ch {
		case '"', '\'':
			quote = ch
			current.WriteRune(ch)
		case '+':
			part := strings.TrimSpace(current.String())
			if part == "" {
				return nil, errors.New("display_expression contains an empty segment")
			}
			parts = append(parts, part)
			current.Reset()
		default:
			current.WriteRune(ch)
		}
	}
	if quote != 0 {
		return nil, errors.New("display_expression contains an unterminated literal")
	}
	part := strings.TrimSpace(current.String())
	if part == "" {
		return nil, errors.New("display_expression contains an empty segment")
	}
	parts = append(parts, part)
	return parts, nil
}

func isDisplayExpressionLiteral(part string) bool {
	if len(part) < 2 {
		return false
	}
	first := part[0]
	last := part[len(part)-1]
	return (first == '"' && last == '"') || (first == '\'' && last == '\'')
}

// UpdateTableDescription edits the description and/or label of a single table row.
func (h *MetadataHandler) UpdateTableDescription(w http.ResponseWriter, r *http.Request) {
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}

	if !h.requireTableAccess(w, r, id, "write") {
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
	if req.DisplayExpression != nil {
		expr := strings.TrimSpace(*req.DisplayExpression)
		if err := validateDisplayExpression(expr); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		var exprPtr *string
		if expr != "" {
			exprPtr = &expr
		}
		if err := h.deps.MetaRepo.UpdateTableDisplayExpression(r.Context(), id, exprPtr); err != nil {
			writeInternalError(r.Context(), w, http.StatusInternalServerError, "failed to update table display expression", err)
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

	if !h.requireColumnAccess(w, r, id, "write") {
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
	if !h.requireMetadataEntityAccess(w, r, entityType, id, "write") {
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
	if !h.requireMetadataEntityAccess(w, r, metadata.EntityTypeTable, id, "read") {
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
	if !h.requireMetadataEntityAccess(w, r, metadata.EntityTypeColumn, id, "read") {
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

	columns, err := h.deps.MetaRepo.ListColumns(ctx, datasourceID, table.SchemaName, table.TableName)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to list columns", err)
		return
	}
	piiConfig, err := h.tablePIIMaskingConfig(ctx, datasourceID)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to resolve pii policy", err)
		return
	}

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
	projection := buildTableRowsProjection(d, columns, piiConfig)
	if len(projection) == 0 {
		writeJSON(w, http.StatusOK, &sampleData{Columns: []sampleColumn{}, Rows: [][]any{}})
		return
	}
	query := d.SelectWithLimit(projection, tableRef, tableSampleRowLimit)

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
func querySampleRows(ctx context.Context, db *sql.DB, query string, args ...any) (_ *sampleData, err error) {
	rows, err := db.QueryContext(ctx, query, args...) // nosemgrep: go.lang.security.audit.database.string-formatted-query.string-formatted-query
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
