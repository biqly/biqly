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

// QuoteIdent quotes a SQL identifier with dialect-specific delimiters.
func (d MySQLDialect) QuoteIdent(identifier string) string {
	parts := strings.Split(identifier, ".")
	quoted := make([]string, len(parts))
	for i, part := range parts {
		quoted[i] = "`" + part + "`"
	}
	return strings.Join(quoted, ".")
}

// Placeholder returns the parameter placeholder for the given index.
func (d MySQLDialect) Placeholder(index int) string {
	return "?"
}

// LimitOffset generates the LIMIT/OFFSET clause.
func (d MySQLDialect) LimitOffset(limit, offset int) string {
	var parts []string
	if limit > 0 {
		parts = append(parts, fmt.Sprintf("LIMIT %d", limit))
	}
	if offset > 0 {
		parts = append(parts, fmt.Sprintf("OFFSET %d", offset))
	}
	return strings.Join(parts, " ")
}

// DateTrunc returns the date truncation expression.
func (d MySQLDialect) DateTrunc(part, column string) string {
	// MySQL doesn't have DATE_TRUNC, use DATE_FORMAT workaround
	return fmt.Sprintf("DATE_FORMAT(%s, '%%Y-%%m-%%d %%H:%%i:%%s')", d.QuoteIdent(column))
}

// ILike returns a case-insensitive LIKE expression.
func (d MySQLDialect) ILike(column, placeholder string) string {
	// MySQL uses LOWER() + LIKE for case-insensitive matching
	return fmt.Sprintf("LOWER(%s) LIKE LOWER(%s)", d.QuoteIdent(column), placeholder)
}

// CastType returns the dialect-specific SQL type name for casting.
func (d MySQLDialect) CastType(sqlType string) string {
	return strings.ToUpper(sqlType)
}

// Aggregate formats an aggregation function call.
func (d MySQLDialect) Aggregate(fn, column string) string {
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

// Compile-time check
var _ Dialect = MySQLDialect{}
