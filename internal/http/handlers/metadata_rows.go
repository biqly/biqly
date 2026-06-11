package handlers

import (
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/biqly/biqly/internal/dialect"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/bytedance/sonic"
)

// tableBrowseMaxLimit caps a single rows page for the table browser.
const tableBrowseMaxLimit = 200

type tableRowsFilter struct {
	Column        string `json:"column"`
	Operator      string `json:"operator"`
	Value         string `json:"value"`
	CaseSensitive bool   `json:"case_sensitive,omitempty"`
}

type tableRowsRequest struct {
	Filters      []tableRowsFilter `json:"filters,omitempty"`
	OrderBy      string            `json:"order_by,omitempty"`
	OrderDir     string            `json:"order_dir,omitempty"`
	Limit        int               `json:"limit,omitempty"`
	Offset       int               `json:"offset,omitempty"`
	IncludeTotal bool              `json:"include_total,omitempty"`
}

type tableRowsResponse struct {
	Columns []sampleColumn `json:"columns"`
	Rows    [][]any        `json:"rows"`
	Total   *int64         `json:"total,omitempty"`
}

// filterValues splits a saved filter value into its OR-ed alternatives: the
// frontend stores multi-chip filters as a JSON string array, single chips raw.
func filterValues(raw string) []string {
	if strings.HasPrefix(raw, "[") && strings.HasSuffix(raw, "]") {
		var arr []string
		if err := sonic.ConfigStd.Unmarshal([]byte(raw), &arr); err == nil && len(arr) > 0 {
			return arr
		}
	}
	return []string{raw}
}

func filterPredicate(d dialect.Dialect, col string, f tableRowsFilter, value string, args *[]any) (string, error) {
	ph := func(v any) string {
		*args = append(*args, v)
		return d.Placeholder(len(*args))
	}
	switch f.Operator {
	case "eq":
		return col + " = " + ph(value), nil
	case "neq":
		return col + " <> " + ph(value), nil
	case "gt":
		return col + " > " + ph(value), nil
	case "gte":
		return col + " >= " + ph(value), nil
	case "lt":
		return col + " < " + ph(value), nil
	case "lte":
		return col + " <= " + ph(value), nil
	case "contains", "starts_with", "ends_with":
		pattern := value
		switch f.Operator {
		case "contains":
			pattern = "%" + value + "%"
		case "starts_with":
			pattern = value + "%"
		case "ends_with":
			pattern = "%" + value
		}
		p := ph(pattern)
		if f.CaseSensitive {
			return col + " LIKE " + p, nil
		}
		return d.ILike(col, p), nil
	default:
		return "", fmt.Errorf("unsupported filter operator %q", f.Operator)
	}
}

// buildTableRowsWhere renders the WHERE clause from validated filters. Column
// identifiers come from the introspected column set; values are always bound
// as parameters.
func buildTableRowsWhere(
	d dialect.Dialect,
	filters []tableRowsFilter,
	columnSet map[string]bool,
) (string, []any, error) {
	if len(filters) == 0 {
		return "", nil, nil
	}
	var clauses []string
	var args []any
	for _, f := range filters {
		if !columnSet[f.Column] {
			return "", nil, fmt.Errorf("unknown filter column %q", f.Column)
		}
		col := d.QuoteIdentSegment(f.Column)
		values := filterValues(f.Value)
		alts := make([]string, 0, len(values))
		for _, v := range values {
			pred, err := filterPredicate(d, col, f, v, &args)
			if err != nil {
				return "", nil, err
			}
			alts = append(alts, pred)
		}
		if len(alts) == 1 {
			clauses = append(clauses, alts[0])
		} else {
			clauses = append(clauses, "("+strings.Join(alts, " OR ")+")")
		}
	}
	return " WHERE " + strings.Join(clauses, " AND "), args, nil
}

// BrowseTableRows returns a filtered, sorted, paginated slice of a single
// introspected table plus an optional total count.
// POST /datasources/{id}/tables/{schema}/{table}/rows
func (h *MetadataHandler) BrowseTableRows(w http.ResponseWriter, r *http.Request) {
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
	req, ok := decodeJSON[tableRowsRequest](w, r)
	if !ok {
		return
	}
	ctx := r.Context()

	// Same containment as GetTableSample: identifiers in the final SQL come
	// from our introspected metadata records, never from request strings.
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
	columnSet := make(map[string]bool, len(columns))
	for _, c := range columns {
		columnSet[c.ColumnName] = true
	}

	resolved, err := h.deps.ResolveDatasourceDB(ctx, datasourceID)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to open connection", err)
		return
	}
	defer func() { _ = resolved.DB.Close() }()

	d := resolved.Driver.Dialect()
	tableRef := d.QuoteIdentSegment(table.SchemaName) + "." + d.QuoteIdentSegment(table.TableName)

	where, args, err := buildTableRowsWhere(d, req.Filters, columnSet)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	orderClause := ""
	if req.OrderBy != "" {
		if !columnSet[req.OrderBy] {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("unknown order_by column %q", req.OrderBy))
			return
		}
		dir := "ASC"
		if strings.EqualFold(req.OrderDir, "desc") {
			dir = "DESC"
		}
		orderClause = " ORDER BY " + d.QuoteIdentSegment(req.OrderBy) + " " + dir
	}

	limit := req.Limit
	if limit <= 0 {
		limit = tableSampleRowLimit
	}
	limit = min(limit, tableBrowseMaxLimit)
	offset := max(req.Offset, 0)

	query := "SELECT * FROM " + tableRef + where + orderClause + " " + d.LimitOffset(limit, offset)
	data, err := querySampleRows(ctx, resolved.DB, query, args...)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to fetch table rows", err)
		return
	}

	resp := tableRowsResponse{Columns: data.Columns, Rows: data.Rows}
	if req.IncludeTotal {
		var total int64
		// Identifiers come from validated metadata records; values are bound.
		countQuery := "SELECT COUNT(*) FROM " + tableRef + where                                   // nosemgrep: go.lang.security.audit.database.string-formatted-query.string-formatted-query
		if err := resolved.DB.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil { // nosemgrep: go.lang.security.audit.database.string-formatted-query.string-formatted-query
			writeInternalError(ctx, w, http.StatusInternalServerError, "failed to count table rows", err)
			return
		}
		resp.Total = &total
	}
	writeJSON(w, http.StatusOK, resp)
}
