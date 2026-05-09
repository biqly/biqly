// Package dialect defines SQL dialect interfaces for different database engines.
package dialect

// Dialect defines database-specific SQL generation behavior.
type Dialect interface {
	// Name returns the dialect name (e.g. "postgres", "mysql").
	Name() string

	// QuoteIdent quotes an identifier (table name, column name) to prevent SQL injection.
	QuoteIdent(identifier string) string

	// Placeholder returns the parameter placeholder for the given index (1-based).
	Placeholder(index int) string

	// LimitOffset generates the LIMIT/OFFSET clause.
	LimitOffset(limit, offset int) string

	// DateTrunc returns the date truncation expression for the given date part and column.
	DateTrunc(part, column string) string

	// ILike returns a case-insensitive LIKE expression.
	ILike(column, placeholder string) string

	// CastType returns the dialect-specific type name for casting.
	CastType(sqlType string) string

	// Aggregate formats an aggregation function call.
	Aggregate(fn, column string) string
}
