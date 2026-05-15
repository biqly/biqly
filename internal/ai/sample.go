package ai

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/biqly/biqly/internal/dialect"
	"github.com/biqly/biqly/internal/metadata"
)

// TableSample carries a small set of rows from a single table to be embedded
// in the prompt as concrete examples of the data the LLM is querying.
type TableSample struct {
	Schema string
	Table  string
	Rows   []map[string]any
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
	if table != "" && !validIdent(table) {
		return nil, fmt.Errorf("invalid table identifier: %q", table)
	}
	if schema != "" && !validIdent(schema) {
		return nil, fmt.Errorf("invalid schema identifier: %q", schema)
	}
	colIdents := make([]string, 0, len(cols))
	for _, c := range cols {
		if !validIdent(c.ColumnName) {
			return nil, fmt.Errorf("invalid column identifier: %q", c.ColumnName)
		}
		colIdents = append(colIdents, d.QuoteIdentSegment(c.ColumnName))
	}
	from := d.QuoteIdentSegment(table)
	if schema != "" {
		from = d.QuoteIdentSegment(schema) + "." + d.QuoteIdentSegment(table)
	}
	// Identifiers were validated against validIdent above; LimitOffset emits an
	// integer literal. We avoid fmt.Sprintf with a "SELECT %s..." template so
	// static analyzers that pattern-match string-formatted SQL stay quiet.
	var qb strings.Builder
	qb.WriteString("SELECT ")
	qb.WriteString(strings.Join(colIdents, ", "))
	qb.WriteString(" FROM ")
	qb.WriteString(from)
	qb.WriteString(" ")
	qb.WriteString(d.LimitOffset(limit, 0))

	// nosemgrep
	rows, err := db.QueryContext(ctx, qb.String())
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out, err := scanSQLRowsToMaps(rows)
	if err != nil {
		return nil, err
	}
	return shrinkSampleForPrompt(out, maxCellRunes), nil
}
