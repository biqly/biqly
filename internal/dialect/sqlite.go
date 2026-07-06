// Package dialect defines SQL dialect interfaces for different database engines.
package dialect

import (
	"fmt"
	"strings"
)

// SQLiteDialect implements the Dialect interface for SQLite.
type SQLiteDialect struct {
	BaseDialect
}

// SQLite is the default global instance of SQLiteDialect.
var SQLite = SQLiteDialect{
	BaseDialect: BaseDialect{
		QuoteLeft:  "\"",
		QuoteRight: "\"",
	},
}

// Name returns the dialect name.
func (SQLiteDialect) Name() string {
	return "sqlite"
}

// Placeholder returns the parameter placeholder for the given index.
func (SQLiteDialect) Placeholder(_ int) string {
	return "?"
}

// sqliteDateTrunc renders SQLite date() modifiers that truncate expr to part.
func sqliteDateTrunc(part, expr string) string {
	switch strings.ToLower(strings.TrimSpace(part)) {
	case "day":
		return fmt.Sprintf("date(%s)", expr)
	case "week":
		// 'weekday 0' advances to the next Sunday (or stays), -6 days lands on Monday.
		return fmt.Sprintf("date(%s, 'weekday 0', '-6 days')", expr)
	case "month":
		return fmt.Sprintf("date(%s, 'start of month')", expr)
	case "quarter":
		return fmt.Sprintf("date(%s, 'start of month', '-' || ((CAST(strftime('%%m', %s) AS INTEGER) - 1) %% 3) || ' months')", expr, expr)
	case "year":
		return fmt.Sprintf("date(%s, 'start of year')", expr)
	default:
		return fmt.Sprintf("datetime(%s)", expr)
	}
}

// DateTrunc returns the date truncation expression.
func (d SQLiteDialect) DateTrunc(part, column string) string {
	return sqliteDateTrunc(part, d.QuoteIdent(column))
}

// DateTruncPlaceholder truncates a bind-parameter timestamp.
func (SQLiteDialect) DateTruncPlaceholder(part, placeholder string) string {
	return sqliteDateTrunc(part, placeholder)
}

// CalendarPart returns strftime-based integer buckets for year/quarter/month.
func (d SQLiteDialect) CalendarPart(part, column string) string {
	return CalendarPartLookup(d, part, column,
		"CAST(strftime('%%Y', %s) AS INTEGER)",
		"(CAST(strftime('%%m', %s) AS INTEGER) + 2) / 3",
		"CAST(strftime('%%m', %s) AS INTEGER)",
		"CAST(strftime('%%d', %s) AS INTEGER)",
		"CAST(strftime('%%H', %s) AS INTEGER)",
	)
}

// ILike returns a case-insensitive LIKE expression. SQLite LIKE is
// case-insensitive for ASCII by default.
func (SQLiteDialect) ILike(column, placeholder string) string {
	return fmt.Sprintf("%s LIKE %s", column, placeholder)
}

// ExplainSQL uses SQLite's EXPLAIN QUERY PLAN form.
func (SQLiteDialect) ExplainSQL(sql string) string {
	return "EXPLAIN QUERY PLAN " + sql
}

var _ Dialect = SQLiteDialect{}
