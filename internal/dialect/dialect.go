// Package dialect defines SQL dialect interfaces for different database engines.
package dialect

// CoreDialect defines the database-specific SQL generation behavior used by the query compiler.
type CoreDialect interface {
	// Name returns the dialect name (e.g. "postgres", "mysql").
	Name() string

	// QuoteIdent quotes an identifier (table name, column name) to prevent SQL injection.
	QuoteIdent(identifier string) string

	// QuoteIdentSegment quotes a single catalog name as returned by the database (e.g. a
	// column "Emp.StartDate") without splitting on '.' . Use QuoteIdent for qualified
	// refs like schema.table expressed as one string with dot separators.
	QuoteIdentSegment(identifier string) string

	// Placeholder returns the parameter placeholder for the given index (1-based).
	Placeholder(index int) string

	// LimitOffset generates the LIMIT/OFFSET clause.
	LimitOffset(limit, offset int) string

	// DateTrunc returns the date truncation expression for the given date part and column.
	DateTrunc(part, column string) string

	// DateTruncPlaceholder returns a date-truncation expression where the
	// truncated value comes from a bind placeholder (e.g. "$1"). Dialects
	// own the timestamp cast so callers don't hardcode `::timestamptz`.
	DateTruncPlaceholder(part, placeholder string) string

	// CalendarPart returns an integer grouping expression for calendar parts supported by time-grain
	// dimensions: year (e.g. 2024), quarter (1–4), month (1–12). part is lower-case: year, quarter, month.
	CalendarPart(part, column string) string

	// ILike returns a case-insensitive LIKE expression.
	ILike(column, placeholder string) string

	// CastType returns the dialect-specific type name for casting.
	CastType(sqlType string) string

	// Aggregate formats an aggregation function call.
	Aggregate(fn, column string) string

	// WindowFunc renders the head of a pure analytic window function (everything
	// before OVER) for fn with pre-rendered args. ok=false means the dialect has
	// no portable spelling and the caller must reject the query rather than emit
	// broken SQL. Plain aggregate window heads (sum/avg/count/min/max) are built
	// via Aggregate, not here. fn is lower-case; recognised names: row_number,
	// rank, dense_rank, percent_rank, cume_dist, ntile, lag, lead, first_value,
	// last_value.
	WindowFunc(fn string, args []string) (string, bool)

	// ExplainSQL wraps a SELECT statement so the database parses/plans it without
	// returning rows (e.g. "EXPLAIN <sql>"). Returning "" indicates the dialect
	// does not support a single-statement dry-run; callers should skip the check.
	ExplainSQL(sql string) string
}

// SampleDialect defines the database-specific query generation used by the data sampler.
type SampleDialect interface {
	// DefaultOrderBy returns the default ORDER BY clause part when pagination is used but no sorting is defined.
	DefaultOrderBy() string

	// SelectWithLimit generates a SELECT query for sample projection.
	SelectWithLimit(columns []string, table string, limit int) string
}

// Dialect defines database-specific SQL generation behavior by composing CoreDialect and SampleDialect.
type Dialect interface {
	CoreDialect
	SampleDialect
}
