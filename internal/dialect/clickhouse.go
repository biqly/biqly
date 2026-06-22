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

// WindowFunc maps analytic functions to ClickHouse spelling. ClickHouse has no
// LAG/LEAD; lagInFrame/leadInFrame are the equivalents (they require an explicit
// frame, which the compiler supplies). ClickHouse window-function names are
// lower-case. percent_rank/cume_dist have no native ClickHouse equivalent, so
// they are rejected (ok=false) rather than emitted as broken SQL.
func (ClickHouseDialect) WindowFunc(fn string, args []string) (string, bool) {
	switch fn {
	case "lag":
		if len(args) == 0 {
			return "", false
		}
		return "lagInFrame(" + strings.Join(args, ", ") + ")", true
	case "lead":
		if len(args) == 0 {
			return "", false
		}
		return "leadInFrame(" + strings.Join(args, ", ") + ")", true
	case "row_number", "rank", "dense_rank":
		return fn + "()", true
	case "ntile":
		if len(args) != 1 {
			return "", false
		}
		return "ntile(" + args[0] + ")", true
	case "first_value", "last_value":
		if len(args) != 1 {
			return "", false
		}
		return fn + "(" + args[0] + ")", true
	}
	return "", false
}

// Compile-time check
var _ Dialect = ClickHouseDialect{}
