package dialect

import "testing"

// TestQuoteStringLiteral verifies that string literals are quoted safely per
// dialect. The key security property is that a value ending in a backslash
// cannot break out of the quote on dialects where backslash is a C-style escape
// (MySQL/ClickHouse/Snowflake/Databricks), while standard-SQL dialects only
// double single quotes.
func TestQuoteStringLiteral(t *testing.T) {
	tests := []struct {
		name    string
		dialect Dialect
		input   string
		want    string
	}{
		// Standard SQL: only single quotes are doubled; backslash is literal.
		{"postgres plain", Postgres, "abc", "'abc'"},
		{"postgres quote", Postgres, "O'Brien", "'O''Brien'"},
		{"postgres backslash left literal", Postgres, `a\b`, `'a\b'`},
		{"sqlserver quote", SQLServer, "O'Brien", "'O''Brien'"},
		{"sqlite backslash literal", SQLite, `a\b`, `'a\b'`},
		{"oracle backslash literal", Oracle, `a\b`, `'a\b'`},

		// Backslash-escaping dialects: backslash is doubled, then quotes doubled.
		{"mysql plain", MySQL, "abc", "'abc'"},
		{"mysql quote", MySQL, "O'Brien", "'O''Brien'"},
		{"mysql backslash", MySQL, `a\b`, `'a\\b'`},
		{"clickhouse backslash", ClickHouse, `a\b`, `'a\\b'`},
		{"snowflake backslash", Snowflake, `a\b`, `'a\\b'`},
		{"databricks backslash", Databricks, `a\b`, `'a\\b'`},

		// The breakout payload: a trailing backslash before a quote. On MySQL the
		// backslash must be escaped so the following doubled-quote is not consumed
		// as an escaped quote (which would let the payload escape the literal).
		{"mysql breakout neutralized", MySQL, `\' OR 1=1 -- `, `'\\'' OR 1=1 -- '`},
		{"postgres breakout neutralized", Postgres, `\' OR 1=1 -- `, `'\'' OR 1=1 -- '`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.dialect.QuoteStringLiteral(tt.input); got != tt.want {
				t.Errorf("QuoteStringLiteral(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
