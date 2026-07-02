package query

import (
	"context"
	"strings"
	"testing"

	"github.com/biqly/biqly/internal/dialect"
	"github.com/biqly/biqly/internal/semantic"
	pkgsemantic "github.com/biqly/biqly/pkg/semantic"
)

type compileExprDialectCase struct {
	name string
	expr pkgsemantic.ExprNode
	want map[string]string
}

func compileExprAcrossDialectsBasicCases() []compileExprDialectCase {
	return []compileExprDialectCase{
		{
			name: "literal string",
			expr: &pkgsemantic.LiteralExpr{Value: "paid"},
			want: map[string]string{
				"postgres":   "'paid'",
				"mysql":      "'paid'",
				"sqlserver":  "'paid'",
				"clickhouse": "'paid'",
			},
		},
		{
			name: "column ref",
			expr: &pkgsemantic.ColumnRefExpr{Table: "orders", Column: "total_amount"},
			want: map[string]string{
				"postgres":   `"orders"."total_amount"`,
				"mysql":      "`orders`.`total_amount`",
				"sqlserver":  "[orders].[total_amount]",
				"clickhouse": "`orders`.`total_amount`",
			},
		},
		{
			name: "nested arithmetic",
			expr: &pkgsemantic.BinaryExpr{
				Op: pkgsemantic.OpMultiply,
				Left: &pkgsemantic.BinaryExpr{
					Op:    pkgsemantic.OpSubtract,
					Left:  &pkgsemantic.ColumnRefExpr{Column: "revenue"},
					Right: &pkgsemantic.ColumnRefExpr{Column: "cost"},
				},
				Right: &pkgsemantic.ColumnRefExpr{Column: "tax_rate"},
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
			expr: &pkgsemantic.BinaryExpr{
				Op:    pkgsemantic.OpConcat,
				Left:  &pkgsemantic.ColumnRefExpr{Column: "first_name"},
				Right: &pkgsemantic.ColumnRefExpr{Column: "last_name"},
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
			expr: &pkgsemantic.UnaryExpr{
				Op:   pkgsemantic.OpNot,
				Expr: &pkgsemantic.ColumnRefExpr{Column: "is_active"},
			},
			want: map[string]string{
				"postgres":   `(NOT "is_active")`,
				"mysql":      "(NOT `is_active`)",
				"sqlserver":  "(NOT [is_active])",
				"clickhouse": "(NOT `is_active`)",
			},
		},
	}
}

func compileExprAcrossDialectsAdvancedCases() []compileExprDialectCase {
	return []compileExprDialectCase{
		{
			name: "function call",
			expr: &pkgsemantic.FunctionCallExpr{
				Name: "ROUND",
				Args: []pkgsemantic.ExprNode{
					&pkgsemantic.BinaryExpr{
						Op:    pkgsemantic.OpDivide,
						Left:  &pkgsemantic.ColumnRefExpr{Column: "revenue"},
						Right: &pkgsemantic.FunctionCallExpr{Name: "NULLIF", Args: []pkgsemantic.ExprNode{&pkgsemantic.ColumnRefExpr{Column: "quantity"}, &pkgsemantic.LiteralExpr{Value: int64(0)}}},
					},
					&pkgsemantic.LiteralExpr{Value: int64(2)},
				},
			},
			want: map[string]string{
				"postgres":   `ROUND(("revenue" * 1.0 / NULLIF("quantity", 0)), 2)`,
				"mysql":      "ROUND((`revenue` * 1.0 / NULLIF(`quantity`, 0)), 2)",
				"sqlserver":  "ROUND(([revenue] * 1.0 / NULLIF([quantity], 0)), 2)",
				"clickhouse": "round((`revenue` * 1.0 / nullif(`quantity`, 0)), 2)",
			},
		},
		{
			name: "date trunc",
			expr: &pkgsemantic.FunctionCallExpr{
				Name: "DATE_TRUNC",
				Args: []pkgsemantic.ExprNode{
					&pkgsemantic.LiteralExpr{Value: "month"},
					&pkgsemantic.ColumnRefExpr{Column: "created_at"},
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
			expr: &pkgsemantic.CaseExpr{
				Conditions: []pkgsemantic.CaseWhen{
					{
						When: &pkgsemantic.BinaryExpr{
							Op:    pkgsemantic.OpGt,
							Left:  &pkgsemantic.ColumnRefExpr{Column: "total_amount"},
							Right: &pkgsemantic.LiteralExpr{Value: int64(0)},
						},
						Then: &pkgsemantic.LiteralExpr{Value: "positive"},
					},
				},
				ElseExpr: &pkgsemantic.LiteralExpr{Value: "negative"},
			},
			want: map[string]string{
				"postgres":   `CASE WHEN ("total_amount" > 0) THEN 'positive' ELSE 'negative' END`,
				"mysql":      "CASE WHEN (`total_amount` > 0) THEN 'positive' ELSE 'negative' END",
				"sqlserver":  "CASE WHEN ([total_amount] > 0) THEN 'positive' ELSE 'negative' END",
				"clickhouse": "CASE WHEN (`total_amount` > 0) THEN 'positive' ELSE 'negative' END",
			},
		},
	}
}

func TestCompileExprAcrossDialects(t *testing.T) {
	resolver := NewSchemaResolver(&semantic.SemanticModel{BaseSchema: "public"}, nil)
	tests := append(compileExprAcrossDialectsBasicCases(), compileExprAcrossDialectsAdvancedCases()...)

	dialects := []dialect.Dialect{
		dialect.PostgresDialect{},
		dialect.MySQLDialect{},
		dialect.SQLServerDialect{},
		dialect.ClickHouseDialect{},
	}
	for _, tt := range tests {
		for _, d := range dialects {
			t.Run(tt.name+"/"+d.Name(), func(t *testing.T) {
				got, err := CompileExpr(tt.expr, d, resolver, nil, nil)
				if err != nil {
					t.Fatalf("unexpected error compiling: %v", err)
				}
				if want := tt.want[d.Name()]; got != want {
					t.Fatalf("CompileExpr(%s, %s) = %s, want %s", tt.name, d.Name(), got, want)
				}
			})
		}
	}
}

func TestCompileExprRejectsUnsupportedDateTruncGrain(t *testing.T) {
	expr := &pkgsemantic.FunctionCallExpr{
		Name: "DATE_TRUNC",
		Args: []pkgsemantic.ExprNode{
			&pkgsemantic.LiteralExpr{Value: "month'); DROP TABLE orders; --"},
			&pkgsemantic.ColumnRefExpr{Column: "created_at"},
		},
	}

	_, err := CompileExpr(expr, dialect.PostgresDialect{}, nil, nil, nil)
	if err == nil {
		t.Fatal("expected unsupported DATE_TRUNC grain error")
	}
	if !strings.Contains(err.Error(), "unsupported DATE_TRUNC grain") {
		t.Fatalf("expected unsupported grain error, got %v", err)
	}
}

func TestCompileDimensionRejectsUnsupportedTimeGrain(t *testing.T) {
	model := &semantic.SemanticModel{
		Name:       "orders",
		BaseSchema: "public",
		BaseTable:  "orders",
		Dimensions: []semantic.Dimension{
			{
				Name:      "created_at",
				ColumnRef: "orders.created_at",
				TimeGrain: "month'); DROP TABLE orders; --",
				Type:      "time",
			},
		},
	}
	lq := LogicalQuery{
		ModelID: "orders",
		Select:  []SelectItem{{Type: "dimension", Name: "created_at"}},
		Limit:   10,
	}

	_, err := NewCompiler(dialect.PostgresDialect{}).CompileWithPermissions(context.Background(), &lq, model, nil, nil)
	if err == nil {
		t.Fatal("expected unsupported time grain error")
	}
	if !strings.Contains(err.Error(), "unsupported time grain") {
		t.Fatalf("expected unsupported time grain error, got %v", err)
	}
}

func TestCompileExprSafetyNet(t *testing.T) {
	expr := &pkgsemantic.ColumnRefExpr{Column: "1; DROP TABLE users"}
	got, err := CompileExpr(expr, dialect.SQLServerDialect{}, nil, nil, nil)
	if err == nil {
		t.Fatalf("expected CompileExpr to return error for unsafe SQL")
	}
	if got != "" {
		t.Fatalf("expected CompileExpr to return empty string for unsafe SQL, got: %q", got)
	}
}

func TestCompileExprParameterization(t *testing.T) {
	expr := &pkgsemantic.BinaryExpr{
		Op:    pkgsemantic.OpEq,
		Left:  &pkgsemantic.ColumnRefExpr{Column: "status"},
		Right: &pkgsemantic.LiteralExpr{Value: "active"},
	}
	args := []any{}
	got, err := CompileExpr(expr, dialect.Postgres, nil, &args, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantSQL := `("status" = $1)`
	if got != wantSQL {
		t.Errorf("got SQL %q, want %q", got, wantSQL)
	}
	if len(args) != 1 || args[0] != "active" {
		t.Errorf("got args %v, want ['active']", args)
	}
}

func TestCompileExprPIIMasking(t *testing.T) {
	expr := &pkgsemantic.ColumnRefExpr{Column: "email"}
	piiConfig := &PIIMaskingConfig{
		ColumnAccess: map[string]string{
			"email": "masked",
		},
		ColumnTypes: map[string]string{
			"email": "email",
		},
	}
	got, err := CompileExpr(expr, dialect.Postgres, nil, nil, piiConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "email") && !strings.Contains(got, "CONCAT") {
		t.Errorf("expected masked email SQL, got %q", got)
	}

	piiConfigHidden := &PIIMaskingConfig{
		ColumnAccess: map[string]string{
			"email": "hidden",
		},
	}
	gotHidden, err := CompileExpr(expr, dialect.Postgres, nil, nil, piiConfigHidden)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotHidden != "'***'" {
		t.Errorf("expected hidden literal, got %q", gotHidden)
	}
}

func TestNormalizeExprDialect_newDrivers(t *testing.T) {
	cases := []struct {
		zero dialect.Dialect
		want string
	}{
		{dialect.SQLiteDialect{}, "sqlite"},
		{dialect.SnowflakeDialect{}, "snowflake"},
		{dialect.DatabricksDialect{}, "databricks"},
		{dialect.OracleDialect{}, "oracle"},
	}
	for _, tc := range cases {
		got := normalizeExprDialect(tc.zero)
		if got.Name() != tc.want {
			t.Errorf("normalizeExprDialect(%T).Name() = %q, want %q", tc.zero, got.Name(), tc.want)
		}
		// Zero-value structs must be replaced by the canonical instance so
		// QuoteIdent works; verify a quote round-trip does not produce "".
		if q := got.QuoteIdent("x"); q == "x" || q == "" {
			t.Errorf("%T: QuoteIdent broken after normalize: %q", tc.zero, q)
		}
	}
}

func TestDialectFunctions_oracleSubstr(t *testing.T) {
	m, ok := dialectFunctions["SUBSTRING"]
	if !ok || m["oracle"] != "SUBSTR" {
		t.Errorf("SUBSTRING oracle mapping = %v", m)
	}
}
