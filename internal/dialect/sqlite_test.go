package dialect

import "testing"

func TestSQLiteDialect(t *testing.T) {
	d := SQLite
	if d.Name() != "sqlite" {
		t.Errorf("Name() = %q", d.Name())
	}
	if got := d.Placeholder(3); got != "?" {
		t.Errorf("Placeholder = %q", got)
	}
	if got := d.QuoteIdent("main.orders"); got != `"main"."orders"` {
		t.Errorf("QuoteIdent = %q", got)
	}
	if got := d.LimitOffset(10, 5); got != "LIMIT 10 OFFSET 5" {
		t.Errorf("LimitOffset = %q", got)
	}
	truncs := map[string]string{
		"day":     `date("created_at")`,
		"week":    `date("created_at", 'weekday 0', '-6 days')`,
		"month":   `date("created_at", 'start of month')`,
		"quarter": `date("created_at", 'start of month', '-' || ((CAST(strftime('%m', "created_at") AS INTEGER) - 1) % 3) || ' months')`,
		"year":    `date("created_at", 'start of year')`,
	}
	for part, want := range truncs {
		if got := d.DateTrunc(part, "created_at"); got != want {
			t.Errorf("DateTrunc(%s) = %q, want %q", part, got, want)
		}
	}
	if got := d.DateTruncPlaceholder("month", "?"); got != "date(?, 'start of month')" {
		t.Errorf("DateTruncPlaceholder = %q", got)
	}
	parts := map[string]string{
		"year":    `CAST(strftime('%Y', "d") AS INTEGER)`,
		"quarter": `(CAST(strftime('%m', "d") AS INTEGER) + 2) / 3`,
		"month":   `CAST(strftime('%m', "d") AS INTEGER)`,
	}
	for part, want := range parts {
		if got := d.CalendarPart(part, "d"); got != want {
			t.Errorf("CalendarPart(%s) = %q, want %q", part, got, want)
		}
	}
	if got := d.ILike(`"name"`, "?"); got != `"name" LIKE ?` {
		t.Errorf("ILike = %q", got)
	}
	if got := d.ExplainSQL("SELECT 1"); got != "EXPLAIN QUERY PLAN SELECT 1" {
		t.Errorf("ExplainSQL = %q", got)
	}
	if got := d.Aggregate("count", "*"); got != "COUNT(*)" {
		t.Errorf("Aggregate = %q", got)
	}
}
