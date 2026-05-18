package dialect

import (
	"strings"
	"testing"
)

func TestCalendarPartYearIntegerSQL(t *testing.T) {
	col := "sales.salesorderheader.orderdate"
	tests := []struct {
		name    string
		dialect Dialect
		contain []string
	}{
		{
			name:    "postgres",
			dialect: PostgresDialect{},
			contain: []string{"EXTRACT(YEAR FROM", "AS INTEGER)", `"salesorderheader"."orderdate"`},
		},
		{name: "mysql", dialect: MySQLDialect{}, contain: []string{"YEAR(", "`salesorderheader`.`orderdate`"}},
		{name: "sqlserver", dialect: SQLServerDialect{}, contain: []string{"YEAR(", "[salesorderheader].[orderdate]"}},
		{name: "clickhouse", dialect: ClickHouseDialect{}, contain: []string{"toYear(", "`salesorderheader`.`orderdate`"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.dialect.CalendarPart("year", col)
			for _, sub := range tt.contain {
				if !strings.Contains(got, sub) {
					t.Errorf("CalendarPart(year, %q) = %q, want substring %q", col, got, sub)
				}
			}
		})
	}
}

func TestAggregateCountStar(t *testing.T) {
	tests := []struct {
		name    string
		dialect Dialect
		want    string
	}{
		{name: "postgres", dialect: PostgresDialect{}, want: "COUNT(*)"},
		{name: "mysql", dialect: MySQLDialect{}, want: "COUNT(*)"},
		{name: "sqlserver", dialect: SQLServerDialect{}, want: "COUNT(*)"},
		{name: "clickhouse", dialect: ClickHouseDialect{}, want: "count()"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.dialect.Aggregate("count", "*")
			if got != tt.want {
				t.Errorf("Aggregate(%q, %q) = %q, want %q", "count", "*", got, tt.want)
			}
		})
	}
}

func TestSQLServerLimitOffsetIncludesOffsetForLimitOnly(t *testing.T) {
	got := SQLServerDialect{}.LimitOffset(100, 0)
	want := "OFFSET 0 ROWS FETCH NEXT 100 ROWS ONLY"
	if got != want {
		t.Fatalf("LimitOffset(100, 0) = %q, want %q", got, want)
	}
}
