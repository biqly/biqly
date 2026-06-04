package query

import (
	"testing"

	pkgsemantic "github.com/biqly/biqly/pkg/semantic"
)

func TestValidateExpression(t *testing.T) {
	tests := []struct {
		expr    string
		isValid bool
	}{
		// Valid expressions
		{"[total_amount] - [discount]", true},
		{"COALESCE([amount], 0)", true},
		{"CASE WHEN price > 0 THEN price ELSE 0 END", true},
		{"1 + 2 * 3 / 4", true},
		{"(a + b) * c", true},
		{"UPPER(CONCAT(first_name, ' ', last_name))", true},
		{"first_name || ' ' || last_name", true},
		{"amount IS NULL", true},
		{"amount IS NOT NULL", true},
		{"price BETWEEN 10 AND 20", true},
		{"status IN ('active', 'pending')", true},
		{"name LIKE 'John%'", true},

		// Invalid expressions (banned keywords / syntax)
		{"1; DROP TABLE users", false},
		{"(SELECT * FROM users)", false},
		{"exec xp_cmdshell", false},
		{"SELECT 1", false},
		{"INSERT INTO users VALUES (1)", false},
		{"UPDATE users SET name = 'foo'", false},
		{"DELETE FROM users", false},
		{"DROP TABLE users", false},
		{"ALTER TABLE users ADD COLUMN age INT", false},
		{"CREATE TABLE users (id INT)", false},

		// Comments (forbidden)
		{"amount -- some comment", false},
		{"amount /* some comment */ + tax", false},

		// Semicolons (forbidden)
		{"amount; tax", false},

		// Invalid syntax
		{"(amount", false},
		{"COALESCE(amount,", false},
		{"CASE WHEN price > 0 THEN price END", true},
		{"CASE WHEN price > 0 THEN price", false},
	}

	for _, tc := range tests {
		t.Run(tc.expr, func(t *testing.T) {
			err := ValidateExpression(tc.expr)
			if tc.isValid && err != nil {
				t.Errorf("expected valid for %q, got error: %v", tc.expr, err)
			} else if !tc.isValid && err == nil {
				t.Errorf("expected invalid for %q, got no error", tc.expr)
			}
		})
	}
}

func TestParseExpressionProducesSemanticAST(t *testing.T) {
	tests := []struct {
		name   string
		expr   string
		assert func(t *testing.T, got pkgsemantic.ExprNode)
	}{
		{
			name: "metric refs in brackets",
			expr: "[gross_revenue] - [discount_amount]",
			assert: func(t *testing.T, got pkgsemantic.ExprNode) {
				t.Helper()
				bin, ok := got.(pkgsemantic.BinaryExpr)
				if !ok {
					t.Fatalf("ParseExpression() = %T, want semantic.BinaryExpr", got)
				}
				if bin.Op != pkgsemantic.OpSubtract {
					t.Fatalf("ParseExpression() op = %q, want %q", bin.Op, pkgsemantic.OpSubtract)
				}
				left, ok := bin.Left.(pkgsemantic.MetricRefExpr)
				if !ok || left.Name != "gross_revenue" {
					t.Fatalf("ParseExpression() left = %#v, want MetricRefExpr gross_revenue", bin.Left)
				}
				right, ok := bin.Right.(pkgsemantic.MetricRefExpr)
				if !ok || right.Name != "discount_amount" {
					t.Fatalf("ParseExpression() right = %#v, want MetricRefExpr discount_amount", bin.Right)
				}
			},
		},
		{
			name: "bare identifiers become column refs",
			expr: "revenue - cost",
			assert: func(t *testing.T, got pkgsemantic.ExprNode) {
				t.Helper()
				bin, ok := got.(pkgsemantic.BinaryExpr)
				if !ok {
					t.Fatalf("ParseExpression() = %T, want semantic.BinaryExpr", got)
				}
				if bin.Op != pkgsemantic.OpSubtract {
					t.Fatalf("ParseExpression() op = %q, want %q", bin.Op, pkgsemantic.OpSubtract)
				}
				left, ok := bin.Left.(pkgsemantic.ColumnRefExpr)
				if !ok || left.Column != "revenue" || left.Table != "" {
					t.Fatalf("ParseExpression() left = %#v, want bare ColumnRefExpr revenue", bin.Left)
				}
				right, ok := bin.Right.(pkgsemantic.ColumnRefExpr)
				if !ok || right.Column != "cost" || right.Table != "" {
					t.Fatalf("ParseExpression() right = %#v, want bare ColumnRefExpr cost", bin.Right)
				}
			},
		},
		{
			name: "function call",
			expr: "COALESCE(email, 'N/A')",
			assert: func(t *testing.T, got pkgsemantic.ExprNode) {
				t.Helper()
				call, ok := got.(pkgsemantic.FunctionCallExpr)
				if !ok {
					t.Fatalf("ParseExpression() = %T, want semantic.FunctionCallExpr", got)
				}
				if call.Name != "COALESCE" || len(call.Args) != 2 {
					t.Fatalf("ParseExpression() call = %#v, want COALESCE with 2 args", call)
				}
			},
		},
		{
			name: "case expression",
			expr: "CASE WHEN x > 0 THEN 'positive' ELSE 'negative' END",
			assert: func(t *testing.T, got pkgsemantic.ExprNode) {
				t.Helper()
				caseExpr, ok := got.(pkgsemantic.CaseExpr)
				if !ok {
					t.Fatalf("ParseExpression() = %T, want semantic.CaseExpr", got)
				}
				if len(caseExpr.Conditions) != 1 || caseExpr.ElseExpr == nil {
					t.Fatalf("ParseExpression() case = %#v, want one condition and else", caseExpr)
				}
			},
		},
		{
			name: "qualified column ref",
			expr: "orders.total_amount",
			assert: func(t *testing.T, got pkgsemantic.ExprNode) {
				t.Helper()
				ref, ok := got.(pkgsemantic.ColumnRefExpr)
				if !ok {
					t.Fatalf("ParseExpression() = %T, want semantic.ColumnRefExpr", got)
				}
				if ref.Table != "orders" || ref.Column != "total_amount" {
					t.Fatalf("ParseExpression() ref = %#v, want orders.total_amount", ref)
				}
			},
		},
		{
			name: "string concatenation with operator ||",
			expr: "first_name || ' ' || last_name",
			assert: func(t *testing.T, got pkgsemantic.ExprNode) {
				t.Helper()
				bin, ok := got.(pkgsemantic.BinaryExpr)
				if !ok {
					t.Fatalf("ParseExpression() = %T, want semantic.BinaryExpr", got)
				}
				if bin.Op != pkgsemantic.OpConcat {
					t.Fatalf("ParseExpression() op = %q, want %q", bin.Op, pkgsemantic.OpConcat)
				}
				// Verify left-associative nesting: (first_name || ' ') || last_name
				left, ok := bin.Left.(pkgsemantic.BinaryExpr)
				if !ok || left.Op != pkgsemantic.OpConcat {
					t.Fatalf("ParseExpression() left = %T, want concat BinaryExpr", bin.Left)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseExpression(tt.expr)
			if err != nil {
				t.Fatalf("ParseExpression(%q) error = %v", tt.expr, err)
			}
			tt.assert(t, got)
		})
	}
}
