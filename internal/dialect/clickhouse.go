// Package dialect defines SQL dialect interfaces for different database engines.
package dialect

import (
	"fmt"
	"strings"
)

// ClickHouseDialect implements the Dialect interface for ClickHouse.
type ClickHouseDialect struct {
	BaseDialect
}

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
	return QuoteIdentQualified(d, identifier)
}

// Placeholder returns the parameter placeholder for the given index.
func (d ClickHouseDialect) Placeholder(index int) string {
	return "?"
}

// LimitOffset generates the LIMIT/OFFSET clause.
func (d ClickHouseDialect) LimitOffset(limit, offset int) string {
	return StandardLimitOffset(limit, offset)
}

// DateTrunc returns the date truncation expression.
func (d ClickHouseDialect) DateTrunc(part, column string) string {
	quoted := d.QuoteIdent(column)
	tc := titleCase(strings.ToLower(part))
	return fmt.Sprintf("toStartOf%s(%s)", tc, quoted)
}

// CalendarPart returns toYear / toQuarter / toMonth for UInt-sized calendar integers.
func (d ClickHouseDialect) CalendarPart(part, column string) string {
	return CalendarPartLookup(d, part, column, "toYear(%s)", "toQuarter(%s)", "toMonth(%s)")
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

// Aggregate formats an aggregation function call.
func (d ClickHouseDialect) Aggregate(fn, column string) string {
	return AggregateClickHouseSQL(d, fn, column)
}

// Compile-time check
var _ Dialect = ClickHouseDialect{}
