package dialect

import "testing"

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
