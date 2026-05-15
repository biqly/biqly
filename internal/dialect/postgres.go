// Package dialect defines SQL dialect interfaces for different database engines.
package dialect

import (
	"fmt"
	"strings"
)

// PostgresDialect implements the Dialect interface for PostgreSQL.
type PostgresDialect struct {
	BaseDialect
}

// Name returns the dialect name.
func (d PostgresDialect) Name() string {
	return "postgres"
}

// QuoteIdentSegment quotes one identifier segment; dots are literal (column names like Emp.StartDate).
func (d PostgresDialect) QuoteIdentSegment(identifier string) string {
	return "\"" + strings.ReplaceAll(identifier, "\"", "\"\"") + "\""
}

// QuoteIdent quotes a qualified name by splitting on '.' (schema.table or a.b.c).
func (d PostgresDialect) QuoteIdent(identifier string) string {
	return QuoteIdentQualified(d, identifier)
}

// Placeholder returns the parameter placeholder for the given index.
func (d PostgresDialect) Placeholder(index int) string {
	return fmt.Sprintf("$%d", index)
}

// DateTrunc returns the date truncation expression.
func (d PostgresDialect) DateTrunc(part, column string) string {
	return fmt.Sprintf("DATE_TRUNC('%s', %s)", part, d.QuoteIdent(column))
}

// CalendarPart returns CAST(EXTRACT(...)) AS INTEGER for scalar year/quarter/month buckets.
func (d PostgresDialect) CalendarPart(part, column string) string {
	return CalendarPartLookup(d, part, column,
		"CAST(EXTRACT(YEAR FROM %s) AS INTEGER)",
		"CAST(EXTRACT(QUARTER FROM %s) AS INTEGER)",
		"CAST(EXTRACT(MONTH FROM %s) AS INTEGER)",
	)
}

// ILike returns a case-insensitive LIKE expression.
// column must be a SQL expression (e.g. already-quoted "schema"."col").
func (d PostgresDialect) ILike(column, placeholder string) string {
	return fmt.Sprintf("%s ILIKE %s", column, placeholder)
}

// Aggregate formats an aggregation function call.
func (d PostgresDialect) Aggregate(fn, column string) string {
	return d.BaseDialect.Aggregate(d, fn, column)
}

var _ Dialect = PostgresDialect{}
