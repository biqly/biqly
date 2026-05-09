// Package dialect defines SQL dialect interfaces for different database engines.
package dialect

import (
	"fmt"
	"strings"
)

// ClickHouseDialect implements the Dialect interface for ClickHouse.
type ClickHouseDialect struct{}

// Name returns the dialect name.
func (d ClickHouseDialect) Name() string {
	return "clickhouse"
}

// QuoteIdent quotes a SQL identifier with dialect-specific delimiters.
func (d ClickHouseDialect) QuoteIdent(identifier string) string {
	parts := strings.Split(identifier, ".")
	quoted := make([]string, len(parts))
	for i, part := range parts {
		quoted[i] = "`" + part + "`"
	}
	return strings.Join(quoted, ".")
}

// Placeholder returns the parameter placeholder for the given index.
func (d ClickHouseDialect) Placeholder(index int) string {
	return "?"
}

// LimitOffset generates the LIMIT/OFFSET clause.
func (d ClickHouseDialect) LimitOffset(limit, offset int) string {
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
func (d ClickHouseDialect) DateTrunc(part, column string) string {
	quoted := d.QuoteIdent(column)
	tc := titleCase(strings.ToLower(part))
	return fmt.Sprintf("toStartOf%s(%s)", tc, quoted)
}

func titleCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// ILike returns a case-insensitive LIKE expression.
func (d ClickHouseDialect) ILike(column, placeholder string) string {
	// ClickHouse is case-sensitive, use lower()
	return fmt.Sprintf("lower(%s) LIKE lower(%s)", d.QuoteIdent(column), placeholder)
}

// CastType returns the dialect-specific SQL type name for casting.
func (d ClickHouseDialect) CastType(sqlType string) string {
	return strings.ToUpper(sqlType)
}

// Aggregate formats an aggregation function call.
func (d ClickHouseDialect) Aggregate(fn, column string) string {
	if strings.ToLower(fn) == "count" && column == "*" {
		return "count()"
	}
	quotedCol := d.QuoteIdent(column)
	switch strings.ToLower(fn) {
	case "count":
		return fmt.Sprintf("count(%s)", quotedCol)
	case "count_distinct":
		return fmt.Sprintf("uniq(%s)", quotedCol)
	case "sum":
		return fmt.Sprintf("sum(%s)", quotedCol)
	case "avg":
		return fmt.Sprintf("avg(%s)", quotedCol)
	case "min":
		return fmt.Sprintf("min(%s)", quotedCol)
	case "max":
		return fmt.Sprintf("max(%s)", quotedCol)
	default:
		return fmt.Sprintf("count(%s)", quotedCol)
	}
}

// Compile-time check
var _ Dialect = ClickHouseDialect{}
