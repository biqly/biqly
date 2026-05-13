// Package dialect defines SQL dialect interfaces for different database engines.
package dialect

import (
	"fmt"
	"strings"
)

// PostgresDialect implements the Dialect interface for PostgreSQL.
type PostgresDialect struct{}

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

// LimitOffset generates the LIMIT/OFFSET clause.
func (d PostgresDialect) LimitOffset(limit, offset int) string {
	return StandardLimitOffset(limit, offset)
}

// DateTrunc returns the date truncation expression.
func (d PostgresDialect) DateTrunc(part, column string) string {
	return fmt.Sprintf("DATE_TRUNC('%s', %s)", part, d.QuoteIdent(column))
}

// CalendarPart returns CAST(EXTRACT(...)) AS INTEGER for scalar year/quarter/month buckets.
func (d PostgresDialect) CalendarPart(part, column string) string {
	q := d.QuoteIdent(column)
	switch strings.ToLower(strings.TrimSpace(part)) {
	case "year":
		return fmt.Sprintf("CAST(EXTRACT(YEAR FROM %s) AS INTEGER)", q)
	case "quarter":
		return fmt.Sprintf("CAST(EXTRACT(QUARTER FROM %s) AS INTEGER)", q)
	case "month":
		return fmt.Sprintf("CAST(EXTRACT(MONTH FROM %s) AS INTEGER)", q)
	default:
		return d.DateTrunc(part, column)
	}
}

// ILike returns a case-insensitive LIKE expression.
// column must be a SQL expression (e.g. already-quoted "schema"."col").
func (d PostgresDialect) ILike(column, placeholder string) string {
	return fmt.Sprintf("%s ILIKE %s", column, placeholder)
}

// CastType returns the dialect-specific SQL type name for casting.
func (d PostgresDialect) CastType(sqlType string) string {
	return CastTypeUpper(sqlType)
}

// Aggregate formats an aggregation function call.
func (d PostgresDialect) Aggregate(fn, column string) string {
	return AggregateStandardSQL(d, fn, column)
}

// ExplainSQL prefixes the statement with EXPLAIN; PostgreSQL plans without executing.
func (d PostgresDialect) ExplainSQL(sql string) string {
	return "EXPLAIN " + sql
}
