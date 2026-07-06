package dialect

import "testing"

func TestDialectName(t *testing.T) {
	tests := []struct {
		dialect Dialect
		want    string
	}{
		{Postgres, "postgres"},
		{MySQL, "mysql"},
		{SQLServer, "sqlserver"},
		{ClickHouse, "clickhouse"},
	}
	for _, tt := range tests {
		if got := tt.dialect.Name(); got != tt.want {
			t.Errorf("Name() = %q, want %q", got, tt.want)
		}
	}
}

func TestPlaceholder(t *testing.T) {
	tests := []struct {
		name    string
		dialect Dialect
		index   int
		want    string
	}{
		{"postgres", Postgres, 3, "$3"},
		{"mysql", MySQL, 3, "?"},
		{"sqlserver", SQLServer, 3, "@p3"},
		{"clickhouse", ClickHouse, 3, "?"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.dialect.Placeholder(tt.index); got != tt.want {
				t.Errorf("Placeholder(%d) = %q, want %q", tt.index, got, tt.want)
			}
		})
	}
}

func TestQuoteIdentQualified(t *testing.T) {
	tests := []struct {
		name    string
		dialect Dialect
		ident   string
		want    string
	}{
		{"postgres", Postgres, "sales.orders", `"sales"."orders"`},
		{"mysql", MySQL, "sales.orders", "`sales`.`orders`"},
		{"sqlserver", SQLServer, "sales.orders", "[sales].[orders]"},
		{"clickhouse", ClickHouse, "sales.orders", "`sales`.`orders`"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.dialect.QuoteIdent(tt.ident); got != tt.want {
				t.Errorf("QuoteIdent(%q) = %q, want %q", tt.ident, got, tt.want)
			}
		})
	}
}

func TestLimitOffset(t *testing.T) {
	tests := []struct {
		name    string
		dialect Dialect
		limit   int
		offset  int
		want    string
	}{
		{"postgres limit+offset", Postgres, 10, 20, "LIMIT 10 OFFSET 20"},
		{"postgres limit only", Postgres, 10, 0, "LIMIT 10"},
		{"postgres none", Postgres, 0, 0, ""},
		{"mysql limit+offset", MySQL, 10, 20, "LIMIT 10 OFFSET 20"},
		{"clickhouse limit+offset", ClickHouse, 10, 20, "LIMIT 10 OFFSET 20"},
		{"sqlserver limit+offset", SQLServer, 10, 20, "OFFSET 20 ROWS FETCH NEXT 10 ROWS ONLY"},
		{"sqlserver offset only", SQLServer, 0, 20, "OFFSET 20 ROWS"},
		{"sqlserver none", SQLServer, 0, 0, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.dialect.LimitOffset(tt.limit, tt.offset); got != tt.want {
				t.Errorf("LimitOffset(%d, %d) = %q, want %q", tt.limit, tt.offset, got, tt.want)
			}
		})
	}
}

func TestDateTrunc(t *testing.T) {
	col := "orderdate"
	tests := []struct {
		name    string
		dialect Dialect
		part    string
		want    string
	}{
		{"postgres day", Postgres, "day", `DATE_TRUNC('day', "orderdate")`},
		{"postgres month", Postgres, "month", `DATE_TRUNC('month', "orderdate")`},
		{"mysql day", MySQL, "day", "DATE(`orderdate`)"},
		{"mysql week", MySQL, "week", "DATE_SUB(DATE(`orderdate`), INTERVAL WEEKDAY(`orderdate`) DAY)"},
		{"mysql month", MySQL, "month", "DATE_FORMAT(`orderdate`, '%Y-%m-01')"},
		{"mysql quarter", MySQL, "quarter", "MAKEDATE(YEAR(`orderdate`), 1) + INTERVAL (QUARTER(`orderdate`) - 1) QUARTER"},
		{"mysql year", MySQL, "year", "MAKEDATE(YEAR(`orderdate`), 1)"},
		{"mysql default", MySQL, "hour", "DATE_FORMAT(`orderdate`, '%Y-%m-%d %H:%i:%s')"},
		{"clickhouse day", ClickHouse, "day", "toStartOfDay(`orderdate`)"},
		{"clickhouse month", ClickHouse, "month", "toStartOfMonth(`orderdate`)"},
		{"sqlserver month", SQLServer, "month", "DATEADD(month, DATEDIFF(month, 0, [orderdate]), 0)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.dialect.DateTrunc(tt.part, col); got != tt.want {
				t.Errorf("DateTrunc(%q, %q) = %q, want %q", tt.part, col, got, tt.want)
			}
		})
	}
}

func TestCalendarPartMonthQuarterAndFallback(t *testing.T) {
	col := "orderdate"
	tests := []struct {
		name    string
		dialect Dialect
		part    string
		want    string
	}{
		{"postgres month", Postgres, "month", `CAST(EXTRACT(MONTH FROM "orderdate") AS INTEGER)`},
		{"postgres quarter", Postgres, "quarter", `CAST(EXTRACT(QUARTER FROM "orderdate") AS INTEGER)`},
		{"mysql month", MySQL, "month", "MONTH(`orderdate`)"},
		{"mysql quarter", MySQL, "quarter", "QUARTER(`orderdate`)"},
		{"sqlserver quarter", SQLServer, "quarter", "DATEPART(quarter, [orderdate])"},
		{"clickhouse month", ClickHouse, "month", "toMonth(`orderdate`)"},
		{"clickhouse quarter", ClickHouse, "quarter", "toQuarter(`orderdate`)"},
		// Day is a first-class integer part (day-of-month) as of the *_day
		// grain filter fix; week remains a DateTrunc fallback.
		{"postgres day", Postgres, "day", `CAST(EXTRACT(DAY FROM "orderdate") AS INTEGER)`},
		{"mysql day", MySQL, "day", "DAYOFMONTH(`orderdate`)"},
		{"postgres week fallback", Postgres, "week", `DATE_TRUNC('week', "orderdate")`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.dialect.CalendarPart(tt.part, col); got != tt.want {
				t.Errorf("CalendarPart(%q, %q) = %q, want %q", tt.part, col, got, tt.want)
			}
		})
	}
}

func TestILike(t *testing.T) {
	tests := []struct {
		name    string
		dialect Dialect
		want    string
	}{
		{"postgres", Postgres, `"name" ILIKE $1`},
		{"mysql", MySQL, "LOWER(`name`) LIKE LOWER(?)"},
		{"sqlserver", SQLServer, "[name] LIKE @p1"},
		{"clickhouse", ClickHouse, "lower(`name`) LIKE lower(?)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			col := tt.dialect.QuoteIdent("name")
			ph := tt.dialect.Placeholder(1)
			if got := tt.dialect.ILike(col, ph); got != tt.want {
				t.Errorf("ILike(%q, %q) = %q, want %q", col, ph, got, tt.want)
			}
		})
	}
}

func TestCastType(t *testing.T) {
	for _, d := range []Dialect{Postgres, MySQL, SQLServer, ClickHouse} {
		if got := d.CastType("integer"); got != "INTEGER" {
			t.Errorf("%s CastType(integer) = %q, want INTEGER", d.Name(), got)
		}
	}
}

func TestExplainSQL(t *testing.T) {
	const sql = "SELECT 1"
	tests := []struct {
		name    string
		dialect Dialect
		want    string
	}{
		{"postgres", Postgres, "EXPLAIN SELECT 1"},
		{"mysql", MySQL, "EXPLAIN SELECT 1"},
		{"clickhouse", ClickHouse, "EXPLAIN SELECT 1"},
		// SQL Server has no single-statement EXPLAIN: returns "" so callers skip the dry-run.
		{"sqlserver", SQLServer, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.dialect.ExplainSQL(sql); got != tt.want {
				t.Errorf("ExplainSQL(%q) = %q, want %q", sql, got, tt.want)
			}
		})
	}
}

func TestAggregateAllFunctions(t *testing.T) {
	const col = "amount"
	tests := []struct {
		name      string
		dialect   Dialect
		fn        string
		column    string
		wantStd   string
		wantClick string
	}{
		{"count", Postgres, "count", col, `COUNT("amount")`, ""},
		{"count_distinct std", Postgres, "count_distinct", col, `COUNT(DISTINCT "amount")`, ""},
		{"sum std", Postgres, "sum", col, `SUM("amount")`, ""},
		{"avg std", Postgres, "avg", col, `AVG("amount")`, ""},
		{"min std", Postgres, "min", col, `MIN("amount")`, ""},
		{"max std", Postgres, "max", col, `MAX("amount")`, ""},
		{"unknown std defaults to count", Postgres, "median", col, `COUNT("amount")`, ""},
		{"custom passthrough", Postgres, "custom", "amount * 2", "amount * 2", ""},
		{"none passthrough", Postgres, "none", "amount", "amount", ""},
		{"empty passthrough", Postgres, "", "amount", "amount", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.dialect.Aggregate(tt.fn, tt.column); got != tt.wantStd {
				t.Errorf("Aggregate(%q, %q) = %q, want %q", tt.fn, tt.column, got, tt.wantStd)
			}
		})
	}
}

func TestAggregateClickHouseSpellings(t *testing.T) {
	const col = "amount"
	tests := []struct {
		fn   string
		want string
	}{
		{"count", "count(`amount`)"},
		{"count_distinct", "uniq(`amount`)"},
		{"sum", "sum(`amount`)"},
		{"avg", "avg(`amount`)"},
		{"min", "min(`amount`)"},
		{"max", "max(`amount`)"},
		{"median", "count(`amount`)"},
	}
	for _, tt := range tests {
		t.Run(tt.fn, func(t *testing.T) {
			if got := ClickHouse.Aggregate(tt.fn, col); got != tt.want {
				t.Errorf("ClickHouse.Aggregate(%q, %q) = %q, want %q", tt.fn, col, got, tt.want)
			}
		})
	}
}

func TestSelectWithLimitNoLimit(t *testing.T) {
	// Base dialects omit the LIMIT clause when limit <= 0.
	got := Postgres.SelectWithLimit([]string{"id"}, "users", 0)
	if got != "SELECT id FROM users" {
		t.Errorf("SelectWithLimit(limit=0) = %q", got)
	}
	// SQL Server omits TOP when limit <= 0.
	gotMS := SQLServer.SelectWithLimit([]string{"id"}, "users", 0)
	if gotMS != "SELECT id FROM users" {
		t.Errorf("SQLServer.SelectWithLimit(limit=0) = %q", gotMS)
	}
}

func TestDefaultOrderByPerDialect(t *testing.T) {
	if got := Postgres.DefaultOrderBy(); got != "" {
		t.Errorf("Postgres.DefaultOrderBy() = %q, want empty", got)
	}
	if got := SQLServer.DefaultOrderBy(); got != "(SELECT NULL)" {
		t.Errorf("SQLServer.DefaultOrderBy() = %q", got)
	}
}

func TestWindowFunc_ANSIStandard(t *testing.T) {
	d := PostgresDialect{}
	cases := []struct {
		fn   string
		args []string
		want string
	}{
		{"row_number", nil, "ROW_NUMBER()"},
		{"dense_rank", nil, "DENSE_RANK()"},
		{"percent_rank", nil, "PERCENT_RANK()"},
		{"cume_dist", nil, "CUME_DIST()"},
		{"ntile", []string{"4"}, "NTILE(4)"},
		{"lag", []string{`"t"."v"`, "1"}, `LAG("t"."v", 1)`},
		{"lead", []string{`"t"."v"`, "2"}, `LEAD("t"."v", 2)`},
		{"first_value", []string{`"t"."v"`}, `FIRST_VALUE("t"."v")`},
		{"last_value", []string{`"t"."v"`}, `LAST_VALUE("t"."v")`},
	}
	for _, c := range cases {
		got, ok := d.WindowFunc(c.fn, c.args)
		if !ok || got != c.want {
			t.Errorf("WindowFunc(%q,%v) = (%q,%v), want (%q,true)", c.fn, c.args, got, ok, c.want)
		}
	}
	if _, ok := d.WindowFunc("median", nil); ok {
		t.Error("expected unknown window function to be rejected")
	}
}

func TestWindowFunc_ClickHouseDerivesLagLeadAndRejects(t *testing.T) {
	d := ClickHouseDialect{}
	if got, ok := d.WindowFunc("lag", []string{"`t`.`v`", "1"}); !ok || got != "lagInFrame(`t`.`v`, 1)" {
		t.Errorf("ClickHouse lag = (%q,%v), want lagInFrame(...)", got, ok)
	}
	if got, ok := d.WindowFunc("lead", []string{"`t`.`v`", "1"}); !ok || got != "leadInFrame(`t`.`v`, 1)" {
		t.Errorf("ClickHouse lead = (%q,%v), want leadInFrame(...)", got, ok)
	}
	if got, ok := d.WindowFunc("row_number", nil); !ok || got != "row_number()" {
		t.Errorf("ClickHouse row_number = (%q,%v), want row_number()", got, ok)
	}
	if _, ok := d.WindowFunc("percent_rank", nil); ok {
		t.Error("ClickHouse has no percent_rank; expected rejection (ok=false)")
	}
}
