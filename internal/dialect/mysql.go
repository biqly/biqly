package dialect

import (
	"fmt"
	"strings"
)

// MySQLDialect implements the Dialect interface for MySQL.
type MySQLDialect struct{}

func (d MySQLDialect) Name() string {
	return "mysql"
}

func (d MySQLDialect) QuoteIdent(identifier string) string {
	parts := strings.Split(identifier, ".")
	quoted := make([]string, len(parts))
	for i, part := range parts {
		quoted[i] = "`" + part + "`"
	}
	return strings.Join(quoted, ".")
}

func (d MySQLDialect) Placeholder(index int) string {
	return "?"
}

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

func (d MySQLDialect) DateTrunc(part, column string) string {
	// MySQL doesn't have DATE_TRUNC, use DATE_FORMAT workaround
	return fmt.Sprintf("DATE_FORMAT(%s, '%%Y-%%m-%%d %%H:%%i:%%s')", d.QuoteIdent(column))
}

func (d MySQLDialect) ILike(column, placeholder string) string {
	// MySQL uses LOWER() + LIKE for case-insensitive matching
	return fmt.Sprintf("LOWER(%s) LIKE LOWER(%s)", d.QuoteIdent(column), placeholder)
}

func (d MySQLDialect) CastType(sqlType string) string {
	return strings.ToUpper(sqlType)
}

func (d MySQLDialect) Aggregate(fn, column string) string {
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
