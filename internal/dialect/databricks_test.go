package dialect

import "testing"

func TestDatabricksDialect(t *testing.T) {
	d := Databricks
	if d.Name() != "databricks" {
		t.Errorf("Name() = %q", d.Name())
	}
	if got := d.Placeholder(2); got != "?" {
		t.Errorf("Placeholder = %q", got)
	}
	if got := d.QuoteIdent("sales.orders"); got != "`sales`.`orders`" {
		t.Errorf("QuoteIdent = %q", got)
	}
	if got := d.LimitOffset(10, 5); got != "LIMIT 10 OFFSET 5" {
		t.Errorf("LimitOffset = %q", got)
	}
	if got := d.DateTrunc("month", "created_at"); got != "date_trunc('MONTH', `created_at`)" {
		t.Errorf("DateTrunc = %q", got)
	}
	if got := d.DateTruncPlaceholder("month", "?"); got != "date_trunc('MONTH', CAST(? AS TIMESTAMP))" {
		t.Errorf("DateTruncPlaceholder = %q", got)
	}
	if got := d.CalendarPart("year", "d"); got != "year(`d`)" {
		t.Errorf("CalendarPart = %q", got)
	}
	if got := d.ILike("`name`", "?"); got != "`name` ILIKE ?" {
		t.Errorf("ILike = %q", got)
	}
	if got := d.ExplainSQL("SELECT 1"); got != "EXPLAIN SELECT 1" {
		t.Errorf("ExplainSQL = %q", got)
	}
}
