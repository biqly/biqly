package dialect

import "testing"

// TestNormalize verifies a zero-value concrete dialect (bare struct literal) is
// replaced with its initialized global for every supported dialect, and that an
// already-initialized dialect passes through unchanged.
func TestNormalize(t *testing.T) {
	// Exercise every dialect branch; Name() is stable across zero/global so this
	// is a smoke check that normalization returns the right concrete type.
	cases := []struct {
		zero Dialect
		want string
	}{
		{PostgresDialect{}, "postgres"},
		{MySQLDialect{}, "mysql"},
		{SQLServerDialect{}, "sqlserver"},
		{ClickHouseDialect{}, "clickhouse"},
		{SQLiteDialect{}, "sqlite"},
		{SnowflakeDialect{}, "snowflake"},
		{DatabricksDialect{}, "databricks"},
		{OracleDialect{}, "oracle"},
	}
	for _, tt := range cases {
		if got := Normalize(tt.zero); got.Name() != tt.want {
			t.Errorf("Normalize(%s zero).Name() = %q, want %q", tt.want, got.Name(), tt.want)
		}
	}

	// Backtick dialects definitively prove normalization: a zero-value quotes
	// with the default double-quote, the normalized global uses backticks.
	if got := Normalize(MySQLDialect{}).QuoteIdent("x"); got != "`x`" {
		t.Errorf("normalized mysql QuoteIdent = %q, want %q", got, "`x`")
	}
	if got := Normalize(ClickHouseDialect{}).QuoteIdent("x"); got != "`x`" {
		t.Errorf("normalized clickhouse QuoteIdent = %q, want %q", got, "`x`")
	}
	// A zero-value mysql (unnormalized) would quote with the default double-quote.
	if got := (MySQLDialect{}).QuoteIdent("x"); got != `"x"` {
		t.Errorf("zero-value mysql QuoteIdent = %q, want %q", got, `"x"`)
	}

	// An already-initialized dialect passes through unchanged.
	if got := Normalize(Postgres); got.QuoteIdent("a.b") != Postgres.QuoteIdent("a.b") {
		t.Error("initialized dialect should pass through unchanged")
	}
}
