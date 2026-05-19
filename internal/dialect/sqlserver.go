// Package dialect defines SQL dialect interfaces for different database engines.
package dialect

import (
	"fmt"
	"strconv"
	"strings"
)

// SQLServerDialect implements the Dialect interface for SQL Server.
type SQLServerDialect struct {
	BaseDialect
}

// Name returns the dialect name.
func (d SQLServerDialect) Name() string {
	return "sqlserver"
}

// QuoteIdentSegment quotes one identifier segment; ']' is escaped per T-SQL rules.
func (d SQLServerDialect) QuoteIdentSegment(identifier string) string {
	return "[" + strings.ReplaceAll(identifier, "]", "]]") + "]"
}

// QuoteIdent quotes a qualified name by splitting on '.' .
func (d SQLServerDialect) QuoteIdent(identifier string) string {
	return QuoteIdentQualified(d, identifier)
}

// Placeholder returns the parameter placeholder for the given index.
func (d SQLServerDialect) Placeholder(index int) string {
	return "@p" + strconv.Itoa(index)
}

// LimitOffset generates the LIMIT/OFFSET clause.
func (d SQLServerDialect) LimitOffset(limit, offset int) string {
	var parts []string
	parts = append(parts, "OFFSET "+strconv.Itoa(offset)+" ROWS")
	if limit > 0 {
		parts = append(parts, "FETCH NEXT "+strconv.Itoa(limit)+" ROWS ONLY")
	}
	if limit <= 0 && offset <= 0 {
		return ""
	}
	return strings.Join(parts, " ")
}

// DateTrunc returns the date truncation expression.
func (d SQLServerDialect) DateTrunc(part, column string) string {
	// SQL Server uses DATEADD/DATEDIFF for truncation
	return fmt.Sprintf("DATEADD(%s, DATEDIFF(%s, 0, %s), 0)", part, part, d.QuoteIdent(column))
}

// CalendarPart returns YEAR / DATEPART(quarter|month) for integer grouping.
func (d SQLServerDialect) CalendarPart(part, column string) string {
	return CalendarPartLookup(d, part, column, "YEAR(%s)", "DATEPART(quarter, %s)", "MONTH(%s)")
}

// ILike returns a case-insensitive LIKE expression.
// column must be a SQL expression (e.g. already-quoted identifiers).
func (d SQLServerDialect) ILike(column, placeholder string) string {
	return fmt.Sprintf("%s LIKE %s", column, placeholder)
}

// Aggregate formats an aggregation function call.
func (d SQLServerDialect) Aggregate(fn, column string) string {
	return d.BaseDialect.Aggregate(d, fn, column)
}

// ExplainSQL returns "" because SQL Server has no single-statement EXPLAIN form
// (SHOWPLAN/NOEXEC require batch-level SET commands the driver may not honor in
// QueryContext). Callers should treat empty as "skip dry-run".
func (d SQLServerDialect) ExplainSQL(_ string) string {
	return ""
}

// Compile-time check
var _ Dialect = SQLServerDialect{}
