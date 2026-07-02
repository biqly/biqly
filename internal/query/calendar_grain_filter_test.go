package query

import (
	"testing"

	"github.com/biqly/biqly/internal/dialect"
)

func TestIsDateOnlyCalendarValue(t *testing.T) {
	tests := []struct {
		value any
		want  bool
	}{
		{"2026-06-20", true},
		{"2026-06-20T00:00:00Z", false},
		{"2026-06", false},
		{20260620, false},
		{"", false},
	}
	for _, tt := range tests {
		if got := isDateOnlyCalendarValue(tt.value); got != tt.want {
			t.Errorf("isDateOnlyCalendarValue(%#v) = %v, want %v", tt.value, got, tt.want)
		}
	}
}

func TestCastColumnAsDate_sqlite(t *testing.T) {
	c := NewCompiler(dialect.SQLite)
	if got := c.castColumnAsDate("d"); got != `date("d")` {
		t.Errorf("got %q", got)
	}
}

func TestCastColumnAsDate_newDriversDefault(t *testing.T) {
	for _, d := range []dialect.Dialect{dialect.Snowflake, dialect.Databricks, dialect.Oracle} {
		c := NewCompiler(d)
		got := c.castColumnAsDate("d")
		want := "CAST(" + d.QuoteIdent("d") + " AS DATE)"
		if got != want {
			t.Errorf("%s: got %q, want %q", d.Name(), got, want)
		}
	}
}
