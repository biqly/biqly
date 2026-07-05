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

// SQLServer is the default global instance of SQLServerDialect.
var SQLServer = SQLServerDialect{
	BaseDialect: BaseDialect{
		QuoteLeft:       "[",
		QuoteRight:      "]",
		ExplainDisabled: true,
	},
}

// Name returns the dialect name.
func (SQLServerDialect) Name() string {
	return "sqlserver"
}

// Placeholder returns the parameter placeholder for the given index.
func (SQLServerDialect) Placeholder(index int) string {
	return "@p" + strconv.Itoa(index)
}

// LimitOffset generates the LIMIT/OFFSET clause.
func (SQLServerDialect) LimitOffset(limit, offset int) string {
	parts := make([]string, 0, 2)
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
	return CalendarPartLookup(d, part, column, "YEAR(%s)", "DATEPART(quarter, %s)", "MONTH(%s)", "DATEPART(hour, %s)")
}

// ILike returns a case-insensitive LIKE expression.
// column must be a SQL expression (e.g. already-quoted identifiers).
func (SQLServerDialect) ILike(column, placeholder string) string {
	return fmt.Sprintf("%s LIKE %s", column, placeholder)
}

// ExplainSQL returns "" because SQL Server has no single-statement EXPLAIN form
// (SHOWPLAN/NOEXEC require batch-level SET commands the driver may not honor in
// QueryContext). Callers should treat empty as "skip dry-run".
func (SQLServerDialect) ExplainSQL(_ string) string {
	return ""
}

// DefaultOrderBy returns "(SELECT NULL)" for SQL Server.
func (SQLServerDialect) DefaultOrderBy() string {
	return "(SELECT NULL)"
}

// SelectWithLimit formats a SQL Server SELECT TOP (n) query.
func (SQLServerDialect) SelectWithLimit(columns []string, table string, limit int) string {
	var topStr string
	if limit > 0 {
		topStr = "TOP (" + strconv.Itoa(limit) + ") "
	}
	return "SELECT " + topStr + strings.Join(columns, ", ") + " FROM " + table
}

// Compile-time check
var _ Dialect = SQLServerDialect{}
