package dialect

import "testing"

func TestOracleDialect(t *testing.T) {
	d := Oracle
	if d.Name() != "oracle" {
		t.Errorf("Name() = %q", d.Name())
	}
	if got := d.Placeholder(3); got != ":3" {
		t.Errorf("Placeholder = %q", got)
	}
	if got := d.QuoteIdent("sales.orders"); got != `"sales"."orders"` {
		t.Errorf("QuoteIdent = %q", got)
	}
	limits := []struct {
		limit, offset int
		want          string
	}{
		{10, 5, "OFFSET 5 ROWS FETCH NEXT 10 ROWS ONLY"},
		{10, 0, "FETCH FIRST 10 ROWS ONLY"},
		{0, 5, "OFFSET 5 ROWS"},
		{0, 0, ""},
	}
	for _, tc := range limits {
		if got := d.LimitOffset(tc.limit, tc.offset); got != tc.want {
			t.Errorf("LimitOffset(%d,%d) = %q, want %q", tc.limit, tc.offset, got, tc.want)
		}
	}
	truncs := map[string]string{
		"day":     `TRUNC("d", 'DD')`,
		"week":    `TRUNC("d", 'IW')`,
		"month":   `TRUNC("d", 'MM')`,
		"quarter": `TRUNC("d", 'Q')`,
		"year":    `TRUNC("d", 'YYYY')`,
	}
	for part, want := range truncs {
		if got := d.DateTrunc(part, "d"); got != want {
			t.Errorf("DateTrunc(%s) = %q, want %q", part, got, want)
		}
	}
	if got := d.DateTruncPlaceholder("month", ":1"); got != "TRUNC(CAST(:1 AS TIMESTAMP), 'MM')" {
		t.Errorf("DateTruncPlaceholder = %q", got)
	}
	parts := map[string]string{
		"year":    `EXTRACT(YEAR FROM "d")`,
		"quarter": `TO_NUMBER(TO_CHAR("d", 'Q'))`,
		"month":   `EXTRACT(MONTH FROM "d")`,
	}
	for part, want := range parts {
		if got := d.CalendarPart(part, "d"); got != want {
			t.Errorf("CalendarPart(%s) = %q, want %q", part, got, want)
		}
	}
	if got := d.ILike(`"name"`, ":1"); got != `UPPER("name") LIKE UPPER(:1)` {
		t.Errorf("ILike = %q", got)
	}
	if got := d.ExplainSQL("SELECT 1"); got != "" {
		t.Errorf("ExplainSQL = %q, want empty (skip dry-run)", got)
	}
	if got := d.SelectWithLimit([]string{`"a"`}, `"t"`, 5); got != `SELECT "a" FROM "t" FETCH FIRST 5 ROWS ONLY` {
		t.Errorf("SelectWithLimit = %q", got)
	}
}
