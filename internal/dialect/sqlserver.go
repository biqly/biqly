package dialect

import (
	"fmt"
	"strings"
)

// SQLServerDialect implements the Dialect interface for SQL Server.
type SQLServerDialect struct{}

func (d SQLServerDialect) Name() string {
	return "sqlserver"
}

func (d SQLServerDialect) QuoteIdent(identifier string) string {
	parts := strings.Split(identifier, ".")
	quoted := make([]string, len(parts))
	for i, part := range parts {
		quoted[i] = "[" + part + "]"
	}
	return strings.Join(quoted, ".")
}

func (d SQLServerDialect) Placeholder(index int) string {
	return fmt.Sprintf("@p%d", index)
}

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

func (d SQLServerDialect) DateTrunc(part, column string) string {
	// SQL Server uses DATEADD/DATEDIFF for truncation
	return fmt.Sprintf("DATEADD(%s, DATEDIFF(%s, 0, %s), 0)", part, part, d.QuoteIdent(column))
}

func (d SQLServerDialect) ILike(column, placeholder string) string {
	// SQL Server is case-insensitive by default for LIKE
	return fmt.Sprintf("%s LIKE %s", d.QuoteIdent(column), placeholder)
}

func (d SQLServerDialect) CastType(sqlType string) string {
	return strings.ToUpper(sqlType)
}

func (d SQLServerDialect) Aggregate(fn, column string) string {
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
var _ Dialect = SQLServerDialect{}
