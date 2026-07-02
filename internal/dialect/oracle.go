// Package dialect defines SQL dialect interfaces for different database engines.
package dialect

import (
	"fmt"
	"strconv"
	"strings"
)

// OracleDialect implements the Dialect interface for Oracle Database (12c+).
type OracleDialect struct {
	BaseDialect
}

// Oracle is the default global instance of OracleDialect.
var Oracle = OracleDialect{
	BaseDialect: BaseDialect{
		QuoteLeft:  "\"",
		QuoteRight: "\"",
		// EXPLAIN PLAN FOR writes to PLAN_TABLE, which fails on read-only
		// credentials — skip dry-run like SQL Server does.
		ExplainDisabled: true,
	},
}

// Name returns the dialect name.
func (OracleDialect) Name() string {
	return "oracle"
}

// Placeholder returns the numbered bind placeholder for the given index.
func (OracleDialect) Placeholder(index int) string {
	return ":" + strconv.Itoa(index)
}

// LimitOffset generates Oracle 12c+ OFFSET/FETCH clauses.
func (OracleDialect) LimitOffset(limit, offset int) string {
	switch {
	case limit > 0 && offset > 0:
		return "OFFSET " + strconv.Itoa(offset) + " ROWS FETCH NEXT " + strconv.Itoa(limit) + " ROWS ONLY"
	case limit > 0:
		return "FETCH FIRST " + strconv.Itoa(limit) + " ROWS ONLY"
	case offset > 0:
		return "OFFSET " + strconv.Itoa(offset) + " ROWS"
	default:
		return ""
	}
}

func oracleTruncFormat(part string) string {
	switch strings.ToLower(strings.TrimSpace(part)) {
	case "day":
		return "DD"
	case "week":
		return "IW"
	case "month":
		return "MM"
	case "quarter":
		return "Q"
	case "year":
		return "YYYY"
	default:
		return "DD"
	}
}

// DateTrunc returns TRUNC(col, 'fmt').
func (d OracleDialect) DateTrunc(part, column string) string {
	return fmt.Sprintf("TRUNC(%s, '%s')", d.QuoteIdent(column), oracleTruncFormat(part))
}

// DateTruncPlaceholder truncates a bind-parameter timestamp.
func (OracleDialect) DateTruncPlaceholder(part, placeholder string) string {
	return fmt.Sprintf("TRUNC(CAST(%s AS TIMESTAMP), '%s')", placeholder, oracleTruncFormat(part))
}

// CalendarPart returns EXTRACT-based buckets; Oracle EXTRACT has no QUARTER,
// so quarter uses TO_CHAR(d, 'Q').
func (d OracleDialect) CalendarPart(part, column string) string {
	return CalendarPartLookup(d, part, column,
		"EXTRACT(YEAR FROM %s)",
		"TO_NUMBER(TO_CHAR(%s, 'Q'))",
		"EXTRACT(MONTH FROM %s)",
	)
}

// ILike returns a case-insensitive LIKE via UPPER on both sides.
func (OracleDialect) ILike(column, placeholder string) string {
	return fmt.Sprintf("UPPER(%s) LIKE UPPER(%s)", column, placeholder)
}

// SelectWithLimit formats an Oracle SELECT capped with FETCH FIRST.
func (OracleDialect) SelectWithLimit(columns []string, table string, limit int) string {
	sql := "SELECT " + strings.Join(columns, ", ") + " FROM " + table
	if limit > 0 {
		sql += " FETCH FIRST " + strconv.Itoa(limit) + " ROWS ONLY"
	}
	return sql
}

var _ Dialect = OracleDialect{}
