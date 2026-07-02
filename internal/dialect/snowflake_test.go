package dialect

import "testing"

func TestSnowflakeDialect(t *testing.T) {
	d := Snowflake
	if d.Name() != "snowflake" {
		t.Errorf("Name() = %q", d.Name())
	}
	if got := d.Placeholder(2); got != "?" {
		t.Errorf("Placeholder = %q", got)
	}
	if got := d.QuoteIdent("sales.orders"); got != `"sales"."orders"` {
		t.Errorf("QuoteIdent = %q", got)
	}
	if got := d.LimitOffset(10, 5); got != "LIMIT 10 OFFSET 5" {
		t.Errorf("LimitOffset = %q", got)
	}
	if got := d.DateTrunc("month", "created_at"); got != `DATE_TRUNC('month', "created_at")` {
		t.Errorf("DateTrunc = %q", got)
	}
	if got := d.CalendarPart("quarter", "d"); got != `CAST(EXTRACT(QUARTER FROM "d") AS INTEGER)` {
		t.Errorf("CalendarPart = %q", got)
	}
	if got := d.ILike(`"name"`, "?"); got != `"name" ILIKE ?` {
		t.Errorf("ILike = %q", got)
	}
	if got := d.ExplainSQL("SELECT 1"); got != "EXPLAIN SELECT 1" {
		t.Errorf("ExplainSQL = %q", got)
	}
}
