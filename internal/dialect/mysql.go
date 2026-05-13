// Package dialect defines SQL dialect interfaces for different database engines.
package dialect

import (
	"fmt"
	"strings"
)

// MySQLDialect implements the Dialect interface for MySQL.
type MySQLDialect struct{}

// Name returns the dialect name.
func (d MySQLDialect) Name() string {
	return "mysql"
}

// QuoteIdentSegment quotes one identifier segment; backticks inside are doubled.
func (d MySQLDialect) QuoteIdentSegment(identifier string) string {
	return "`" + strings.ReplaceAll(identifier, "`", "``") + "`"
}

// QuoteIdent quotes a qualified name by splitting on '.' .
func (d MySQLDialect) QuoteIdent(identifier string) string {
	return QuoteIdentQualified(d, identifier)
}

// Placeholder returns the parameter placeholder for the given index.
func (d MySQLDialect) Placeholder(index int) string {
	return "?"
}

// LimitOffset generates the LIMIT/OFFSET clause.
func (d MySQLDialect) LimitOffset(limit, offset int) string {
	return StandardLimitOffset(limit, offset)
}

// DateTrunc returns the date truncation expression. MySQL has no native
// DATE_TRUNC so each grain uses its idiomatic workaround.
func (d MySQLDialect) DateTrunc(part, column string) string {
	q := d.QuoteIdent(column)
	switch strings.ToLower(strings.TrimSpace(part)) {
	case "day":
		return fmt.Sprintf("DATE(%s)", q)
	case "week":
		// ISO-ish start of week (Monday). WEEKDAY returns 0 for Monday.
		return fmt.Sprintf("DATE_SUB(DATE(%s), INTERVAL WEEKDAY(%s) DAY)", q, q)
	case "month":
		return fmt.Sprintf("DATE_FORMAT(%s, '%%Y-%%m-01')", q)
	case "quarter":
		return fmt.Sprintf("MAKEDATE(YEAR(%s), 1) + INTERVAL (QUARTER(%s) - 1) QUARTER", q, q)
	case "year":
		return fmt.Sprintf("MAKEDATE(YEAR(%s), 1)", q)
	default:
		return fmt.Sprintf("DATE_FORMAT(%s, '%%Y-%%m-%%d %%H:%%i:%%s')", q)
	}
}

// CalendarPart returns YEAR / QUARTER / MONTH for integer grouping.
func (d MySQLDialect) CalendarPart(part, column string) string {
	q := d.QuoteIdent(column)
	switch strings.ToLower(strings.TrimSpace(part)) {
	case "year":
		return fmt.Sprintf("YEAR(%s)", q)
	case "quarter":
		return fmt.Sprintf("QUARTER(%s)", q)
	case "month":
		return fmt.Sprintf("MONTH(%s)", q)
	default:
		return d.DateTrunc(part, column)
	}
}

// ILike returns a case-insensitive LIKE expression.
// column must be a SQL expression (e.g. already-quoted identifiers).
func (d MySQLDialect) ILike(column, placeholder string) string {
	return fmt.Sprintf("LOWER(%s) LIKE LOWER(%s)", column, placeholder)
}

// CastType returns the dialect-specific SQL type name for casting.
func (d MySQLDialect) CastType(sqlType string) string {
	return CastTypeUpper(sqlType)
}

// Aggregate formats an aggregation function call.
func (d MySQLDialect) Aggregate(fn, column string) string {
	return AggregateStandardSQL(d, fn, column)
}

// ExplainSQL prefixes the statement with EXPLAIN; MySQL plans without executing.
func (d MySQLDialect) ExplainSQL(sql string) string {
	return "EXPLAIN " + sql
}

// Compile-time check
var _ Dialect = MySQLDialect{}
