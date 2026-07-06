// Package dialect defines SQL dialect interfaces for different database engines.
package dialect

import (
	"fmt"
	"strconv"
)

// PostgresDialect implements the Dialect interface for PostgreSQL.
type PostgresDialect struct {
	BaseDialect
}

// Postgres is the default global instance of PostgresDialect.
var Postgres = PostgresDialect{
	BaseDialect: BaseDialect{
		QuoteLeft:  "\"",
		QuoteRight: "\"",
	},
}

// Name returns the dialect name.
func (PostgresDialect) Name() string {
	return "postgres"
}

// Placeholder returns the parameter placeholder for the given index.
func (PostgresDialect) Placeholder(index int) string {
	return "$" + strconv.Itoa(index)
}

// DateTrunc returns the date truncation expression.
func (d PostgresDialect) DateTrunc(part, column string) string {
	return fmt.Sprintf("DATE_TRUNC('%s', %s)", part, d.QuoteIdent(column))
}

// DateTruncPlaceholder casts the placeholder to timestamptz (PostgreSQL's
// preferred timestamp-with-tz type) before truncating.
func (PostgresDialect) DateTruncPlaceholder(part, placeholder string) string {
	return fmt.Sprintf("DATE_TRUNC('%s', %s::timestamptz)", part, placeholder)
}

// CalendarPart returns CAST(EXTRACT(...)) AS INTEGER for scalar year/quarter/month buckets.
func (d PostgresDialect) CalendarPart(part, column string) string {
	return CalendarPartLookup(d, part, column,
		"CAST(EXTRACT(YEAR FROM %s) AS INTEGER)",
		"CAST(EXTRACT(QUARTER FROM %s) AS INTEGER)",
		"CAST(EXTRACT(MONTH FROM %s) AS INTEGER)",
		"CAST(EXTRACT(DAY FROM %s) AS INTEGER)",
		"CAST(EXTRACT(HOUR FROM %s) AS INTEGER)",
	)
}

// ILike returns a case-insensitive LIKE expression.
// column must be a SQL expression (e.g. already-quoted "schema"."col").
func (PostgresDialect) ILike(column, placeholder string) string {
	return fmt.Sprintf("%s ILIKE %s", column, placeholder)
}

var _ Dialect = PostgresDialect{}
