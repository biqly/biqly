package dialect

import (
	"fmt"
	"strings"
)

// PostgresDialect implements the Dialect interface for PostgreSQL.
type PostgresDialect struct{}

func (d PostgresDialect) Name() string {
	return "postgres"
}

func (d PostgresDialect) QuoteIdent(identifier string) string {
	// Handle schema.table format: "schema"."table".
	// Internal double quotes are escaped by doubling per the SQL standard so a
	// crafted identifier cannot break out of the quoted name.
	parts := strings.Split(identifier, ".")
	quoted := make([]string, len(parts))
	for i, part := range parts {
		quoted[i] = "\"" + strings.ReplaceAll(part, "\"", "\"\"") + "\""
	}
	return strings.Join(quoted, ".")
}

func (d PostgresDialect) Placeholder(index int) string {
	return fmt.Sprintf("$%d", index)
}

func (d PostgresDialect) LimitOffset(limit, offset int) string {
	var parts []string
	if limit > 0 {
		parts = append(parts, fmt.Sprintf("LIMIT %d", limit))
	}
	if offset > 0 {
		parts = append(parts, fmt.Sprintf("OFFSET %d", offset))
	}
	return strings.Join(parts, " ")
}

func (d PostgresDialect) DateTrunc(part, column string) string {
	return fmt.Sprintf("DATE_TRUNC('%s', %s)", part, d.QuoteIdent(column))
}

func (d PostgresDialect) ILike(column, placeholder string) string {
	return fmt.Sprintf("%s ILIKE %s", d.QuoteIdent(column), placeholder)
}

func (d PostgresDialect) CastType(sqlType string) string {
	return strings.ToUpper(sqlType)
}

func (d PostgresDialect) Aggregate(fn, column string) string {
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
