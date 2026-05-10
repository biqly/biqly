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

// QuoteIdentSegment quotes one identifier segment; backticks inside are doubled.
func (d ClickHouseDialect) QuoteIdentSegment(identifier string) string {
	return "`" + strings.ReplaceAll(identifier, "`", "``") + "`"
}

// QuoteIdent quotes a qualified name by splitting on '.' .
func (d ClickHouseDialect) QuoteIdent(identifier string) string {
	parts := strings.Split(identifier, ".")
	quoted := make([]string, len(parts))
	for i, part := range parts {
		quoted[i] = d.QuoteIdentSegment(part)
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

// CalendarPart returns toYear / toQuarter / toMonth for UInt-sized calendar integers.
func (d ClickHouseDialect) CalendarPart(part, column string) string {
	q := d.QuoteIdent(column)
	switch strings.ToLower(strings.TrimSpace(part)) {
	case "year":
		return fmt.Sprintf("toYear(%s)", q)
	case "quarter":
		return fmt.Sprintf("toQuarter(%s)", q)
	case "month":
		return fmt.Sprintf("toMonth(%s)", q)
	default:
		return d.DateTrunc(part, column)
	}
}

func titleCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// ILike returns a case-insensitive LIKE expression.
// column must be a SQL expression (e.g. already-quoted identifiers).
func (d ClickHouseDialect) ILike(column, placeholder string) string {
	return fmt.Sprintf("lower(%s) LIKE lower(%s)", column, placeholder)
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

// ExplainSQL prefixes the statement with EXPLAIN; ClickHouse plans without executing.
func (d ClickHouseDialect) ExplainSQL(sql string) string {
	return "EXPLAIN " + sql
}

// Compile-time check
var _ Dialect = ClickHouseDialect{}
