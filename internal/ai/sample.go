package ai

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/biqly/biqly/internal/dialect"
	"github.com/biqly/biqly/internal/metadata"
)

// ExcludePIIColumns drops every column carrying a PII annotation. It is the
// single choke point for the rule "raw values of PII-flagged columns never
// leave the cluster in an LLM prompt" — used by every sample path that feeds
// an external provider (table describe, NL→SQL sample rows). Column names and
// types can still reach the prompt via the metadata list; only the sampled
// *values* of PII columns are withheld.
func ExcludePIIColumns(cols []metadata.Column) []metadata.Column {
	out := make([]metadata.Column, 0, len(cols))
	for _, c := range cols {
		if c.PIIType != nil {
			continue
		}
		out = append(out, c)
	}
	return out
}

// FetchTableSample reads up to `limit` rows from schema.table, projecting only
// the listed columns, and returns them as map[col]value. Identifiers must pass
// the project's allowlist regex (validIdent) — we do not bind identifiers, so
// any other input would be unsafe to interpolate. Cell values longer than
// maxCellRunes are truncated for prompt budget control.
func FetchTableSample(ctx context.Context, db *sql.DB, d dialect.Dialect, cols []metadata.Column, schema, table string, limit, maxCellRunes int) ([]map[string]any, error) {
	if limit <= 0 {
		return nil, nil
	}
	query, err := buildTableSampleSQL(d, cols, schema, table, limit)
	if err != nil {
		return nil, err
	}

	// nosemgrep
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out, err := scanSQLRowsToMaps(rows, limit)
	if err != nil {
		return nil, err
	}
	return shrinkSampleForPrompt(out, maxCellRunes), nil
}

func buildTableSampleSQL(d dialect.Dialect, cols []metadata.Column, schema, table string, limit int) (string, error) {
	if table != "" && !validIdent(table) {
		return "", fmt.Errorf("invalid table identifier: %q", table)
	}
	if schema != "" && !validIdent(schema) {
		return "", fmt.Errorf("invalid schema identifier: %q", schema)
	}
	colIdents := make([]string, 0, len(cols))
	for _, c := range cols {
		if !validIdent(c.ColumnName) {
			return "", fmt.Errorf("invalid column identifier: %q", c.ColumnName)
		}
		colIdents = append(colIdents, d.QuoteIdentSegment(c.ColumnName))
	}
	from := d.QuoteIdentSegment(table)
	if schema != "" {
		from = d.QuoteIdentSegment(schema) + "." + d.QuoteIdentSegment(table)
	}
	return d.SelectWithLimit(colIdents, from, limit), nil
}
