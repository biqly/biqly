package dialect

import (
	"fmt"
	"strconv"
	"strings"
)

// BaseDialect provides shared Dialect method implementations. Embed in concrete dialect types
// and implement Name, QuoteIdentSegment, Placeholder, DateTrunc, CalendarPart, and ILike.
type BaseDialect struct {
	QuoteLeft       string
	QuoteRight      string
	ExplainDisabled bool
	ClickHouseAggs  bool
}

// QuoteIdentSegment quotes one identifier segment; the right quote character is escaped by doubling it.
func (b BaseDialect) QuoteIdentSegment(identifier string) string {
	left := b.QuoteLeft
	right := b.QuoteRight
	if left == "" {
		left = "\""
	}
	if right == "" {
		right = "\""
	}
	return left + strings.ReplaceAll(identifier, right, right+right) + right
}

// QuoteIdent quotes a qualified name by splitting on '.' (schema.table or a.b.c).
func (b BaseDialect) QuoteIdent(identifier string) string {
	return QuoteIdentQualified(b, identifier)
}

// CastType returns the upper-cased SQL type name.
func (BaseDialect) CastType(sqlType string) string {
	return CastTypeUpper(sqlType)
}

// DateTruncPlaceholder casts the placeholder to a timestamp via standard SQL
// `CAST(... AS TIMESTAMP)` and wraps it in `DATE_TRUNC('part', ...)`. Dialects
// without DATE_TRUNC or with a different timestamp keyword override this.
func (BaseDialect) DateTruncPlaceholder(part, placeholder string) string {
	return fmt.Sprintf("DATE_TRUNC('%s', CAST(%s AS TIMESTAMP))", part, placeholder)
}

// LimitOffset generates a standard LIMIT/OFFSET clause.
func (BaseDialect) LimitOffset(limit, offset int) string {
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
func (b BaseDialect) Aggregate(fn, column string) string {
	if b.ClickHouseAggs {
		return AggregateClickHouseSQL(b, fn, column)
	}
	return AggregateStandardSQL(b, fn, column)
}

// WindowFunc renders a pure analytic window-function head using standard ANSI
// SQL:2003 spelling, which PostgreSQL, MySQL 8+, and SQL Server all accept.
// Dialects whose syntax differs (e.g. ClickHouse) override this. ok=false marks
// an unrecognised function so the caller rejects the query.
func (BaseDialect) WindowFunc(fn string, args []string) (string, bool) {
	switch fn {
	case "row_number", "rank", "dense_rank", "percent_rank", "cume_dist":
		return strings.ToUpper(fn) + "()", true
	case "ntile":
		if len(args) != 1 {
			return "", false
		}
		return "NTILE(" + args[0] + ")", true
	case "lag", "lead":
		if len(args) == 0 {
			return "", false
		}
		return strings.ToUpper(fn) + "(" + strings.Join(args, ", ") + ")", true
	case "first_value", "last_value":
		if len(args) != 1 {
			return "", false
		}
		return strings.ToUpper(fn) + "(" + args[0] + ")", true
	}
	return "", false
}

// CalendarPartLookup maps year/quarter/month/day-of-month/hour to dialect-specific expressions.
func CalendarPartLookup(d Dialect, part, column string, yearFmt, quarterFmt, monthFmt, dayFmt, hourFmt string) string {
	q := d.QuoteIdent(column)
	switch strings.ToLower(strings.TrimSpace(part)) {
	case "year":
		return fmt.Sprintf(yearFmt, q)
	case "quarter":
		return fmt.Sprintf(quarterFmt, q)
	case "month":
		return fmt.Sprintf(monthFmt, q)
	case "day":
		return fmt.Sprintf(dayFmt, q)
	case "hour":
		return fmt.Sprintf(hourFmt, q)
	default:
		return d.DateTrunc(part, column)
	}
}

// DefaultOrderBy returns an empty string by default.
func (BaseDialect) DefaultOrderBy() string {
	return ""
}

// SelectWithLimit formats a standard SELECT with limit query.
func (BaseDialect) SelectWithLimit(columns []string, table string, limit int) string {
	var limitStr string
	if limit > 0 {
		limitStr = " LIMIT " + strconv.Itoa(limit)
	}
	return "SELECT " + strings.Join(columns, ", ") + " FROM " + table + limitStr
}
