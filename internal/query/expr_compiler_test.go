package query

import (
	"testing"

	"github.com/biqly/biqly/internal/dialect"
	"github.com/biqly/biqly/internal/semantic"
	pkgsemantic "github.com/biqly/biqly/pkg/semantic"
)

func TestCompileExprAcrossDialects(t *testing.T) {
	resolver := NewSchemaResolver(&semantic.SemanticModel{BaseSchema: "public"}, nil)

	tests := []struct {
		name string
		expr pkgsemantic.ExprNode
		want map[string]string
	}{
		{
			name: "literal string",
			expr: pkgsemantic.LiteralExpr{Value: "paid"},
			want: map[string]string{
				"postgres":   "'paid'",
				"mysql":      "'paid'",
				"sqlserver":  "'paid'",
				"clickhouse": "'paid'",
			},
		},
		{
			name: "column ref",
			expr: pkgsemantic.ColumnRefExpr{Table: "orders", Column: "total_amount"},
			want: map[string]string{
				"postgres":   `"orders"."total_amount"`,
				"mysql":      "`orders`.`total_amount`",
				"sqlserver":  "[orders].[total_amount]",
				"clickhouse": "`orders`.`total_amount`",
			},
		},
		{
			name: "nested arithmetic",
			expr: pkgsemantic.BinaryExpr{
				Op: pkgsemantic.OpMultiply,
				Left: pkgsemantic.BinaryExpr{
					Op:    pkgsemantic.OpSubtract,
					Left:  pkgsemantic.ColumnRefExpr{Column: "revenue"},
					Right: pkgsemantic.ColumnRefExpr{Column: "cost"},
				},
				Right: pkgsemantic.ColumnRefExpr{Column: "tax_rate"},
			},
			want: map[string]string{
				"postgres":   `(("revenue" - "cost") * "tax_rate")`,
				"mysql":      "((`revenue` - `cost`) * `tax_rate`)",
				"sqlserver":  "(([revenue] - [cost]) * [tax_rate])",
				"clickhouse": "((`revenue` - `cost`) * `tax_rate`)",
			},
		},
		{
			name: "concat override",
			expr: pkgsemantic.BinaryExpr{
				Op:    pkgsemantic.OpConcat,
				Left:  pkgsemantic.ColumnRefExpr{Column: "first_name"},
				Right: pkgsemantic.ColumnRefExpr{Column: "last_name"},
			},
			want: map[string]string{
				"postgres":   `("first_name" || "last_name")`,
				"mysql":      "CONCAT(`first_name`, `last_name`)",
				"sqlserver":  "([first_name] + [last_name])",
				"clickhouse": "concat(`first_name`, `last_name`)",
			},
		},
		{
			name: "unary not",
			expr: pkgsemantic.UnaryExpr{
				Op:   pkgsemantic.OpNot,
				Expr: pkgsemantic.ColumnRefExpr{Column: "is_active"},
			},
			want: map[string]string{
				"postgres":   `(NOT "is_active")`,
				"mysql":      "(NOT `is_active`)",
				"sqlserver":  "(NOT [is_active])",
				"clickhouse": "(NOT `is_active`)",
			},
		},
		{
			name: "function call",
			expr: pkgsemantic.FunctionCallExpr{
				Name: "ROUND",
				Args: []pkgsemantic.ExprNode{
					pkgsemantic.BinaryExpr{
						Op:    pkgsemantic.OpDivide,
						Left:  pkgsemantic.ColumnRefExpr{Column: "revenue"},
						Right: pkgsemantic.FunctionCallExpr{Name: "NULLIF", Args: []pkgsemantic.ExprNode{pkgsemantic.ColumnRefExpr{Column: "quantity"}, pkgsemantic.LiteralExpr{Value: int64(0)}}},
					},
					pkgsemantic.LiteralExpr{Value: int64(2)},
				},
			},
			want: map[string]string{
				"postgres":   `ROUND(("revenue" / NULLIF("quantity", 0)), 2)`,
				"mysql":      "ROUND((`revenue` / NULLIF(`quantity`, 0)), 2)",
				"sqlserver":  "ROUND(([revenue] / NULLIF([quantity], 0)), 2)",
				"clickhouse": "round((`revenue` / nullif(`quantity`, 0)), 2)",
			},
		},
		{
			name: "date trunc",
			expr: pkgsemantic.FunctionCallExpr{
				Name: "DATE_TRUNC",
				Args: []pkgsemantic.ExprNode{
					pkgsemantic.LiteralExpr{Value: "month"},
					pkgsemantic.ColumnRefExpr{Column: "created_at"},
				},
			},
			want: map[string]string{
				"postgres":   `DATE_TRUNC('month', "created_at")`,
				"mysql":      "DATE_FORMAT(`created_at`, '%Y-%m-01')",
				"sqlserver":  "DATEADD(month, DATEDIFF(month, 0, [created_at]), 0)",
				"clickhouse": "toStartOfMonth(`created_at`)",
			},
		},
		{
			name: "case expression",
			expr: pkgsemantic.CaseExpr{
				Conditions: []pkgsemantic.CaseWhen{
					{
						When: pkgsemantic.BinaryExpr{
							Op:    pkgsemantic.OpGt,
							Left:  pkgsemantic.ColumnRefExpr{Column: "total_amount"},
							Right: pkgsemantic.LiteralExpr{Value: int64(0)},
						},
						Then: pkgsemantic.LiteralExpr{Value: "positive"},
					},
				},
				ElseExpr: pkgsemantic.LiteralExpr{Value: "negative"},
			},
			want: map[string]string{
				"postgres":   `CASE WHEN ("total_amount" > 0) THEN 'positive' ELSE 'negative' END`,
				"mysql":      "CASE WHEN (`total_amount` > 0) THEN 'positive' ELSE 'negative' END",
				"sqlserver":  "CASE WHEN ([total_amount] > 0) THEN 'positive' ELSE 'negative' END",
				"clickhouse": "CASE WHEN (`total_amount` > 0) THEN 'positive' ELSE 'negative' END",
			},
		},
	}

	dialects := []dialect.Dialect{
		dialect.PostgresDialect{},
		dialect.MySQLDialect{},
		dialect.SQLServerDialect{},
		dialect.ClickHouseDialect{},
	}
	for _, tt := range tests {
		for _, d := range dialects {
			t.Run(tt.name+"/"+d.Name(), func(t *testing.T) {
				got := CompileExpr(tt.expr, d, resolver)
				if want := tt.want[d.Name()]; got != want {
					t.Fatalf("CompileExpr(%s, %s) = %s, want %s", tt.name, d.Name(), got, want)
				}
			})
		}
	}
}
