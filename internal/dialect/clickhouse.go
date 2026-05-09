package dialect

import (
	"fmt"
	"strings"
)

// ClickHouseDialect implements the Dialect interface for ClickHouse.
type ClickHouseDialect struct{}

func (d ClickHouseDialect) Name() string {
	return "clickhouse"
}

func (d ClickHouseDialect) QuoteIdent(identifier string) string {
	parts := strings.Split(identifier, ".")
	quoted := make([]string, len(parts))
	for i, part := range parts {
		quoted[i] = "`" + part + "`"
	}
	return strings.Join(quoted, ".")
}

func (d ClickHouseDialect) Placeholder(index int) string {
	return "?"
}

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

func (d ClickHouseDialect) DateTrunc(part, column string) string {
	quoted := d.QuoteIdent(column)
	return fmt.Sprintf("toStartOf%s(%s)", strings.Title(strings.ToLower(part)), quoted)
}

func (d ClickHouseDialect) ILike(column, placeholder string) string {
	// ClickHouse is case-sensitive, use lower()
	return fmt.Sprintf("lower(%s) LIKE lower(%s)", d.QuoteIdent(column), placeholder)
}

func (d ClickHouseDialect) CastType(sqlType string) string {
	return strings.ToUpper(sqlType)
}

func (d ClickHouseDialect) Aggregate(fn, column string) string {
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
