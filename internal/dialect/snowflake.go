// Package dialect defines SQL dialect interfaces for different database engines.
package dialect

import "fmt"

// SnowflakeDialect implements the Dialect interface for Snowflake.
type SnowflakeDialect struct {
	BaseDialect
}

// Snowflake is the default global instance of SnowflakeDialect.
var Snowflake = SnowflakeDialect{
	BaseDialect: BaseDialect{
		QuoteLeft:  "\"",
		QuoteRight: "\"",
	},
}

// Name returns the dialect name.
func (SnowflakeDialect) Name() string {
	return "snowflake"
}

// QuoteStringLiteral escapes backslash (a string escape in Snowflake by
// default) before doubling single quotes, so a value ending in "\" cannot
// break out.
func (SnowflakeDialect) QuoteStringLiteral(value string) string {
	return quoteBackslashEscapedLiteral(value)
}

// Placeholder returns the parameter placeholder for the given index.
func (SnowflakeDialect) Placeholder(_ int) string {
	return "?"
}

// DateTrunc returns the date truncation expression.
func (d SnowflakeDialect) DateTrunc(part, column string) string {
	return fmt.Sprintf("DATE_TRUNC('%s', %s)", part, d.QuoteIdent(column))
}

// CalendarPart returns CAST(EXTRACT(...)) AS INTEGER buckets.
func (d SnowflakeDialect) CalendarPart(part, column string) string {
	return CalendarPartLookup(d, part, column,
		"CAST(EXTRACT(YEAR FROM %s) AS INTEGER)",
		"CAST(EXTRACT(QUARTER FROM %s) AS INTEGER)",
		"CAST(EXTRACT(MONTH FROM %s) AS INTEGER)",
		"CAST(EXTRACT(DAY FROM %s) AS INTEGER)",
		"CAST(EXTRACT(HOUR FROM %s) AS INTEGER)",
	)
}

// ILike returns Snowflake's native case-insensitive LIKE.
func (SnowflakeDialect) ILike(column, placeholder string) string {
	return fmt.Sprintf("%s ILIKE %s", column, placeholder)
}

var _ Dialect = SnowflakeDialect{}
