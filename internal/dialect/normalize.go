package dialect

// Normalize replaces a zero-value concrete dialect — one built as a bare struct
// literal (e.g. PostgresDialect{}) with an empty QuoteLeft — with the package's
// fully-initialized global instance, so quoting/placeholder behavior is correct.
// An already-initialized dialect (non-empty QuoteLeft) is returned unchanged.
//
// This is the single source of truth for the normalization the query compiler
// and expression compiler both need; keeping it here prevents the two call
// sites from drifting on which dialects they cover.
func Normalize(d Dialect) Dialect {
	switch concrete := d.(type) {
	case PostgresDialect:
		if concrete.QuoteLeft == "" {
			return Postgres
		}
	case MySQLDialect:
		if concrete.QuoteLeft == "" {
			return MySQL
		}
	case SQLServerDialect:
		if concrete.QuoteLeft == "" {
			return SQLServer
		}
	case ClickHouseDialect:
		if concrete.QuoteLeft == "" {
			return ClickHouse
		}
	case SQLiteDialect:
		if concrete.QuoteLeft == "" {
			return SQLite
		}
	case SnowflakeDialect:
		if concrete.QuoteLeft == "" {
			return Snowflake
		}
	case DatabricksDialect:
		if concrete.QuoteLeft == "" {
			return Databricks
		}
	case OracleDialect:
		if concrete.QuoteLeft == "" {
			return Oracle
		}
	}
	return d
}
