package dialect

import (
	"strings"
	"testing"
)

func TestDateTruncPlaceholderPerDialect(t *testing.T) {
	tests := []struct {
		name    string
		dialect Dialect
		want    string
	}{
		{"postgres", Postgres, "DATE_TRUNC('quarter', $1::timestamptz)"},
		{"mysql", MySQL, "DATE_TRUNC('quarter', CAST(? AS TIMESTAMP))"},
		{"sqlserver", SQLServer, "DATE_TRUNC('quarter', CAST(@p1 AS TIMESTAMP))"},
		{"clickhouse", ClickHouse, "DATE_TRUNC('quarter', CAST(? AS TIMESTAMP))"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ph := tt.dialect.Placeholder(1)
			got := tt.dialect.DateTruncPlaceholder("quarter", ph)
			if got != tt.want {
				t.Errorf("DateTruncPlaceholder(quarter, %q) = %q, want %q", ph, got, tt.want)
			}
		})
	}
}

func TestCalendarPartYearIntegerSQL(t *testing.T) {
	col := "sales.salesorderheader.orderdate"
	tests := []struct {
		name    string
		dialect Dialect
		contain []string
	}{
		{
			name:    "postgres",
			dialect: Postgres,
			contain: []string{"EXTRACT(YEAR FROM", "AS INTEGER)", `"salesorderheader"."orderdate"`},
		},
		{name: "mysql", dialect: MySQL, contain: []string{"YEAR(", "`salesorderheader`.`orderdate`"}},
		{name: "sqlserver", dialect: SQLServer, contain: []string{"YEAR(", "[salesorderheader].[orderdate]"}},
		{name: "clickhouse", dialect: ClickHouse, contain: []string{"toYear(", "`salesorderheader`.`orderdate`"}},
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
		{name: "postgres", dialect: Postgres, want: "COUNT(*)"},
		{name: "mysql", dialect: MySQL, want: "COUNT(*)"},
		{name: "sqlserver", dialect: SQLServer, want: "COUNT(*)"},
		{name: "clickhouse", dialect: ClickHouse, want: "count()"},
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
	got := SQLServer.LimitOffset(100, 0)
	want := "OFFSET 0 ROWS FETCH NEXT 100 ROWS ONLY"
	if got != want {
		t.Fatalf("LimitOffset(100, 0) = %q, want %q", got, want)
	}
}

func TestDefaultOrderBy(t *testing.T) {
	tests := []struct {
		name    string
		dialect Dialect
		want    string
	}{
		{name: "postgres", dialect: Postgres, want: ""},
		{name: "mysql", dialect: MySQL, want: ""},
		{name: "sqlserver", dialect: SQLServer, want: "(SELECT NULL)"},
		{name: "clickhouse", dialect: ClickHouse, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.dialect.DefaultOrderBy()
			if got != tt.want {
				t.Errorf("DefaultOrderBy() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSelectWithLimit(t *testing.T) {
	cols := []string{"id", "name"}
	table := "users"
	limit := 5

	tests := []struct {
		name    string
		dialect Dialect
		want    string
	}{
		{
			name:    "postgres",
			dialect: Postgres,
			want:    "SELECT id, name FROM users LIMIT 5",
		},
		{
			name:    "mysql",
			dialect: MySQL,
			want:    "SELECT id, name FROM users LIMIT 5",
		},
		{
			name:    "sqlserver",
			dialect: SQLServer,
			want:    "SELECT TOP (5) id, name FROM users",
		},
		{
			name:    "clickhouse",
			dialect: ClickHouse,
			want:    "SELECT id, name FROM users LIMIT 5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.dialect.SelectWithLimit(cols, table, limit)
			if got != tt.want {
				t.Errorf("SelectWithLimit() = %q, want %q", got, tt.want)
			}
		})
	}
}
