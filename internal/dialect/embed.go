package dialect

import (
	"fmt"
	"strconv"
	"strings"
)

// BaseDialect provides shared Dialect method implementations. Embed in concrete dialect types
// and implement Name, QuoteIdentSegment, Placeholder, DateTrunc, CalendarPart, and ILike.
type BaseDialect struct {
	ExplainDisabled bool
	ClickHouseAggs  bool
}

// CastType returns the upper-cased SQL type name.
func (b BaseDialect) CastType(sqlType string) string {
	return CastTypeUpper(sqlType)
}

// DateTruncPlaceholder casts the placeholder to a timestamp via standard SQL
// `CAST(... AS TIMESTAMP)` and wraps it in `DATE_TRUNC('part', ...)`. Dialects
// without DATE_TRUNC or with a different timestamp keyword override this.
func (b BaseDialect) DateTruncPlaceholder(part, placeholder string) string {
	return fmt.Sprintf("DATE_TRUNC('%s', CAST(%s AS TIMESTAMP))", part, placeholder)
}

// LimitOffset generates a standard LIMIT/OFFSET clause.
func (b BaseDialect) LimitOffset(limit, offset int) string {
	return StandardLimitOffset(limit, offset)
}

// ExplainSQL prefixes EXPLAIN unless disabled for the dialect.
func (b BaseDialect) ExplainSQL(sql string) string {
	if b.ExplainDisabled {
		return ""
	}
	return "EXPLAIN " + sql
}

// Aggregate formats an aggregation call using the dialect's conventions.
func (b BaseDialect) Aggregate(d Dialect, fn, column string) string {
	if b.ClickHouseAggs {
		return AggregateClickHouseSQL(d, fn, column)
	}
	return AggregateStandardSQL(d, fn, column)
}

// CalendarPartLookup maps year/quarter/month to dialect-specific expressions.
func CalendarPartLookup(d Dialect, part, column string, yearFmt, quarterFmt, monthFmt string) string {
	q := d.QuoteIdent(column)
	switch strings.ToLower(strings.TrimSpace(part)) {
	case "year":
		return fmt.Sprintf(yearFmt, q)
	case "quarter":
		return fmt.Sprintf(quarterFmt, q)
	case "month":
		return fmt.Sprintf(monthFmt, q)
	default:
		return d.DateTrunc(part, column)
	}
}

// DefaultOrderBy returns an empty string by default.
func (b BaseDialect) DefaultOrderBy() string {
	return ""
}

// SelectWithLimit formats a standard SELECT with limit query.
func (b BaseDialect) SelectWithLimit(columns []string, table string, limit int) string {
	var limitStr string
	if limit > 0 {
		limitStr = " LIMIT " + strconv.Itoa(limit)
	}
	return "SELECT " + strings.Join(columns, ", ") + " FROM " + table + limitStr
}
