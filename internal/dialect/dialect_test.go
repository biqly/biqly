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

func TestDefaultOrderBy(t *testing.T) {
	tests := []struct {
		name    string
		dialect Dialect
		want    string
	}{
		{name: "postgres", dialect: PostgresDialect{}, want: ""},
		{name: "mysql", dialect: MySQLDialect{}, want: ""},
		{name: "sqlserver", dialect: SQLServerDialect{}, want: "(SELECT NULL)"},
		{name: "clickhouse", dialect: ClickHouseDialect{}, want: ""},
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
			dialect: PostgresDialect{},
			want:    "SELECT id, name FROM users LIMIT 5",
		},
		{
			name:    "mysql",
			dialect: MySQLDialect{},
			want:    "SELECT id, name FROM users LIMIT 5",
		},
		{
			name:    "sqlserver",
			dialect: SQLServerDialect{},
			want:    "SELECT TOP (5) id, name FROM users",
		},
		{
			name:    "clickhouse",
			dialect: ClickHouseDialect{},
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
