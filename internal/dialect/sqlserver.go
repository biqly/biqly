// Package dialect defines SQL dialect interfaces for different database engines.
package dialect

import (
	"fmt"
	"strings"
)

// SQLServerDialect implements the Dialect interface for SQL Server.
type SQLServerDialect struct{}

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
	parts := strings.Split(identifier, ".")
	quoted := make([]string, len(parts))
	for i, part := range parts {
		quoted[i] = d.QuoteIdentSegment(part)
	}
	return strings.Join(quoted, ".")
}

// Placeholder returns the parameter placeholder for the given index.
func (d SQLServerDialect) Placeholder(index int) string {
	return fmt.Sprintf("@p%d", index)
}

// LimitOffset generates the LIMIT/OFFSET clause.
func (d SQLServerDialect) LimitOffset(limit, offset int) string {
	// SQL Server uses OFFSET ... ROWS FETCH NEXT ... ROWS ONLY
	// Requires ORDER BY clause
	var parts []string
	if offset > 0 {
		parts = append(parts, fmt.Sprintf("OFFSET %d ROWS", offset))
	}
	if limit > 0 {
		parts = append(parts, fmt.Sprintf("FETCH NEXT %d ROWS ONLY", limit))
	}
	if len(parts) == 0 && limit > 0 {
		return fmt.Sprintf("OFFSET 0 ROWS FETCH NEXT %d ROWS ONLY", limit)
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
	q := d.QuoteIdent(column)
	switch strings.ToLower(strings.TrimSpace(part)) {
	case "year":
		return fmt.Sprintf("YEAR(%s)", q)
	case "quarter":
		return fmt.Sprintf("DATEPART(quarter, %s)", q)
	case "month":
		return fmt.Sprintf("MONTH(%s)", q)
	default:
		return d.DateTrunc(part, column)
	}
}

// ILike returns a case-insensitive LIKE expression.
// column must be a SQL expression (e.g. already-quoted identifiers).
func (d SQLServerDialect) ILike(column, placeholder string) string {
	return fmt.Sprintf("%s LIKE %s", column, placeholder)
}

// CastType returns the dialect-specific SQL type name for casting.
func (d SQLServerDialect) CastType(sqlType string) string {
	return strings.ToUpper(sqlType)
}

// Aggregate formats an aggregation function call.
func (d SQLServerDialect) Aggregate(fn, column string) string {
	if strings.ToLower(fn) == "count" && column == "*" {
		return "COUNT(*)"
	}
	quotedCol := d.QuoteIdent(column)
	switch strings.ToLower(fn) {
	case "count":
		return fmt.Sprintf("COUNT(%s)", quotedCol)
	case "count_distinct":
		return fmt.Sprintf("COUNT(DISTINCT %s)", quotedCol)
	case "sum":
		return fmt.Sprintf("SUM(%s)", quotedCol)
	case "avg":
		return fmt.Sprintf("AVG(%s)", quotedCol)
	case "min":
		return fmt.Sprintf("MIN(%s)", quotedCol)
	case "max":
		return fmt.Sprintf("MAX(%s)", quotedCol)
	default:
		return fmt.Sprintf("COUNT(%s)", quotedCol)
	}
}

// ExplainSQL returns "" because SQL Server has no single-statement EXPLAIN form
// (SHOWPLAN/NOEXEC require batch-level SET commands the driver may not honor in
// QueryContext). Callers should treat empty as "skip dry-run".
func (d SQLServerDialect) ExplainSQL(_ string) string {
	return ""
}

// Compile-time check
var _ Dialect = SQLServerDialect{}
