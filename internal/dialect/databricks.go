// Package dialect defines SQL dialect interfaces for different database engines.
package dialect

import (
	"fmt"
	"strings"
)

// DatabricksDialect implements the Dialect interface for Databricks (Spark SQL).
type DatabricksDialect struct {
	BaseDialect
}

// Databricks is the default global instance of DatabricksDialect.
var Databricks = DatabricksDialect{
	BaseDialect: BaseDialect{
		QuoteLeft:  "`",
		QuoteRight: "`",
	},
}

// Name returns the dialect name.
func (DatabricksDialect) Name() string {
	return "databricks"
}

// Placeholder returns the parameter placeholder for the given index.
func (DatabricksDialect) Placeholder(_ int) string {
	return "?"
}

// DateTrunc returns Spark's date_trunc with an upper-case format unit.
func (d DatabricksDialect) DateTrunc(part, column string) string {
	return fmt.Sprintf("date_trunc('%s', %s)", strings.ToUpper(part), d.QuoteIdent(column))
}

// DateTruncPlaceholder truncates a bind-parameter timestamp.
func (DatabricksDialect) DateTruncPlaceholder(part, placeholder string) string {
	return fmt.Sprintf("date_trunc('%s', CAST(%s AS TIMESTAMP))", strings.ToUpper(part), placeholder)
}

// CalendarPart returns Spark's year/quarter/month functions.
func (d DatabricksDialect) CalendarPart(part, column string) string {
	return CalendarPartLookup(d, part, column, "year(%s)", "quarter(%s)", "month(%s)", "hour(%s)")
}

// ILike returns Spark SQL's native case-insensitive LIKE.
func (DatabricksDialect) ILike(column, placeholder string) string {
	return fmt.Sprintf("%s ILIKE %s", column, placeholder)
}

var _ Dialect = DatabricksDialect{}
