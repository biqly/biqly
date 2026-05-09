package query

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/biqly/biqly/internal/dialect"
	"github.com/biqly/biqly/internal/semantic"
)

// normalizeSQL removes whitespace differences for comparison.
func normalizeSQL(sql string) string {
	fields := strings.Fields(sql)
	return strings.Join(fields, " ")
}

// TestGolden_PostgresSimpleSelect verifies compiler output matches golden SQL.
func TestGolden_PostgresSimpleSelect(t *testing.T) {
	golden, err := os.ReadFile(filepath.Join("..", "..", "testdata", "sql", "postgres", "group_by_metric.sql"))
	if err != nil {
		t.Skip("golden file not found")
	}

	model := &semantic.SemanticModel{
		Name:       "sales_orders",
		BaseSchema: "sales",
		BaseTable:  "salesorderheader",
		Dimensions: []semantic.Dimension{
			{Name: "country", ColumnRef: "salesterritory.countryregioncode", Type: "text"},
			{Name: "order_date", ColumnRef: "salesorderheader.orderdate", Type: "date"},
		},
		Metrics: []semantic.Metric{
			{Name: "order_count", Expression: "salesorderheader.salesorderid", Aggregation: "count"},
		},
		Joins: []semantic.Join{
			{
				Name:         "salesorderheader_salesterritory",
				FromTable:    "salesorderheader",
				FromColumn:   "territoryid",
				ToTable:      "salesterritory",
				ToColumn:     "territoryid",
				JoinType:     "LEFT",
				Relationship: "many_to_one",
			},
		},
	}

	lq := LogicalQuery{
		ModelID: "sales_orders",
		Select: []SelectItem{
			{Type: "dimension", Name: "country"},
			{Type: "metric", Name: "order_count"},
		},
		Filters: []Filter{{Field: "order_date", Operator: OpGte, Value: "2011-01-01"}},
		GroupBy: []GroupBy{{Field: "country"}},
		OrderBy: []OrderBy{{Field: "order_count", Direction: "desc"}},
		Limit:   100,
	}

	compiler := NewCompiler(dialect.PostgresDialect{})
	cq, err := compiler.Compile(context.Background(), lq, model)
	if err != nil {
		t.Fatalf("compilation failed: %v", err)
	}

	expected := normalizeSQL(string(golden))
	actual := normalizeSQL(cq.SQL)

	if expected != actual {
		t.Errorf("SQL mismatch.\nExpected:\n%s\n\nGot:\n%s", expected, actual)
	}
}

// TestGolden_MySQLSimpleSelect verifies MySQL compiler output.
func TestGolden_MySQLSimpleSelect(t *testing.T) {
	golden, err := os.ReadFile(filepath.Join("..", "..", "testdata", "sql", "mysql", "simple_select.sql"))
	if err != nil {
		t.Skip("golden file not found")
	}

	model := &semantic.SemanticModel{
		Name:       "products",
		BaseSchema: "store",
		BaseTable:  "products",
		Dimensions: []semantic.Dimension{
			{Name: "name", ColumnRef: "products.name", Type: "text"},
		},
	}

	lq := LogicalQuery{
		ModelID: "products",
		Select:  []SelectItem{{Type: "dimension", Name: "name"}},
		Filters: []Filter{{Field: "name", Operator: OpContains, Value: "test"}},
		Limit:   50,
	}

	compiler := NewCompiler(dialect.MySQLDialect{})
	cq, err := compiler.Compile(context.Background(), lq, model)
	if err != nil {
		t.Fatalf("MySQL compilation failed: %v", err)
	}

	expected := normalizeSQL(string(golden))
	actual := normalizeSQL(cq.SQL)

	if expected != actual {
		t.Errorf("MySQL SQL mismatch.\nExpected:\n%s\n\nGot:\n%s", expected, actual)
	}
}
