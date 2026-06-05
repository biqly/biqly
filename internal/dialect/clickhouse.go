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

// ClickHouse is the default global instance of ClickHouseDialect.
var ClickHouse = ClickHouseDialect{
	BaseDialect: BaseDialect{
		QuoteLeft:      "`",
		QuoteRight:     "`",
		ClickHouseAggs: true,
	},
}

// Name returns the dialect name.
func (ClickHouseDialect) Name() string {
	return "clickhouse"
}

// Placeholder returns the parameter placeholder for the given index.
func (ClickHouseDialect) Placeholder(_ int) string {
	return "?"
}

// LimitOffset generates the LIMIT/OFFSET clause.
func (ClickHouseDialect) LimitOffset(limit, offset int) string {
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
func (ClickHouseDialect) ILike(column, placeholder string) string {
	return fmt.Sprintf("lower(%s) LIKE lower(%s)", column, placeholder)
}

// Compile-time check
var _ Dialect = ClickHouseDialect{}
