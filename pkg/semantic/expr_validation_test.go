package semantic

import (
	"strings"
	"testing"
)

func validateExprStrictAllowedMaps() (map[string]bool, map[string]bool, map[string]bool) {
	allowedCols := map[string]bool{
		"orders.revenue":      true,
		"orders.cost":         true,
		"orders.id":           true,
		"customers.name":      true,
		"customers.region":    true,
		"orders.total_amount": true,
	}
	allowedMets := map[string]bool{
		"total_revenue": true,
		"order_count":   true,
	}
	allowedDims := map[string]bool{
		"customer_region": true,
		"order_date":      true,
	}
	return allowedCols, allowedMets, allowedDims
}

type validateExprStrictCase struct {
	name         string
	expr         ExprNode
	allowMetrics bool
	wantErr      string
}

func validateExprStrictRefCases() []validateExprStrictCase {
	return []validateExprStrictCase{
		{
			name:         "valid literal",
			expr:         &LiteralExpr{Value: 42.5},
			allowMetrics: false,
			wantErr:      "",
		},
		{
			name:         "valid column ref table qualified",
			expr:         &ColumnRefExpr{Table: "orders", Column: "revenue"},
			allowMetrics: false,
			wantErr:      "",
		},
		{
			name:         "valid column ref unqualified",
			expr:         &ColumnRefExpr{Column: "revenue"},
			allowMetrics: false,
			wantErr:      "",
		},
		{
			name:         "invalid column ref",
			expr:         &ColumnRefExpr{Column: "missing_column"},
			allowMetrics: false,
			wantErr:      "unknown column reference: missing_column",
		},
		{
			name:         "valid dimension ref",
			expr:         &DimensionRefExpr{Name: "customer_region"},
			allowMetrics: false,
			wantErr:      "",
		},
		{
			name:         "invalid dimension ref",
			expr:         &DimensionRefExpr{Name: "missing_dimension"},
			allowMetrics: false,
			wantErr:      "unknown dimension reference: missing_dimension",
		},
		{
			name:         "metric ref allowed",
			expr:         &MetricRefExpr{Name: "total_revenue"},
			allowMetrics: true,
			wantErr:      "",
		},
		{
			name:         "metric ref forbidden",
			expr:         &MetricRefExpr{Name: "total_revenue"},
			allowMetrics: false,
			wantErr:      "metric reference not allowed in this context: total_revenue",
		},
		{
			name:         "invalid metric ref",
			expr:         &MetricRefExpr{Name: "missing_metric"},
			allowMetrics: true,
			wantErr:      "unknown metric reference: missing_metric",
		},
	}
}

func validateExprStrictFunctionCases() []validateExprStrictCase {
	return []validateExprStrictCase{
		{
			name: "disallowed function",
			expr: &FunctionCallExpr{
				Name: "DROP_DATABASE",
				Args: []ExprNode{&LiteralExpr{Value: "db"}},
			},
			allowMetrics: false,
			wantErr:      "disallowed function call: DROP_DATABASE",
		},
		{
			name: "valid function call UPPER",
			expr: &FunctionCallExpr{
				Name: "UPPER",
				Args: []ExprNode{&ColumnRefExpr{Column: "revenue"}},
			},
			allowMetrics: false,
			wantErr:      "",
		},
		{
			name: "invalid function call UPPER arity",
			expr: &FunctionCallExpr{
				Name: "UPPER",
				Args: []ExprNode{&ColumnRefExpr{Column: "revenue"}, &ColumnRefExpr{Column: "cost"}},
			},
			allowMetrics: false,
			wantErr:      "function UPPER requires exactly 1 arguments, got 2",
		},
		{
			name: "valid function call ROUND 1 arg",
			expr: &FunctionCallExpr{
				Name: "ROUND",
				Args: []ExprNode{&ColumnRefExpr{Column: "revenue"}},
			},
			allowMetrics: false,
			wantErr:      "",
		},
		{
			name: "valid function call ROUND 2 args",
			expr: &FunctionCallExpr{
				Name: "ROUND",
				Args: []ExprNode{&ColumnRefExpr{Column: "revenue"}, &LiteralExpr{Value: 2}},
			},
			allowMetrics: false,
			wantErr:      "",
		},
		{
			name: "invalid function call ROUND arity",
			expr: &FunctionCallExpr{
				Name: "ROUND",
				Args: []ExprNode{},
			},
			allowMetrics: false,
			wantErr:      "function ROUND requires 1 or 2 arguments, got 0",
		},
		{
			name: "valid variadic function call COALESCE",
			expr: &FunctionCallExpr{
				Name: "COALESCE",
				Args: []ExprNode{&ColumnRefExpr{Column: "revenue"}, &ColumnRefExpr{Column: "cost"}, &LiteralExpr{Value: 0}},
			},
			allowMetrics: false,
			wantErr:      "",
		},
		{
			name: "invalid variadic function call COALESCE empty",
			expr: &FunctionCallExpr{
				Name: "COALESCE",
				Args: []ExprNode{},
			},
			allowMetrics: false,
			wantErr:      "function COALESCE requires at least 1 argument, got 0",
		},
		{
			name: "aggregate SUM forbidden in dimension context",
			expr: &FunctionCallExpr{
				Name: "SUM",
				Args: []ExprNode{&ColumnRefExpr{Column: "revenue"}},
			},
			allowMetrics: false,
			wantErr:      "aggregate function not allowed in this context: SUM",
		},
		{
			name: "aggregate SUM allowed in custom metric context",
			expr: &FunctionCallExpr{
				Name: "SUM",
				Args: []ExprNode{&ColumnRefExpr{Column: "revenue"}},
			},
			allowMetrics: true,
			wantErr:      "",
		},
		{
			name:         "nesting depth overflow",
			expr:         buildDeepExpr(12),
			allowMetrics: false,
			wantErr:      "expression nesting depth exceeds limit of 10",
		},
	}
}

func TestValidateExprStrict(t *testing.T) {
	allowedCols, allowedMets, allowedDims := validateExprStrictAllowedMaps()
	tests := append(validateExprStrictRefCases(), validateExprStrictFunctionCases()...)
	tests = append(tests, validateExprStrictCaseCases()...)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateExprStrict(tt.expr, allowedCols, allowedMets, allowedDims, tt.allowMetrics, 0)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("ValidateExprStrict() error = %v, want nil", err)
				}
			} else {
				if err == nil {
					t.Errorf("ValidateExprStrict() error = nil, want %q", tt.wantErr)
				} else if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("ValidateExprStrict() error = %v, want to contain %q", err, tt.wantErr)
				}
			}
		})
	}
}

func validateExprStrictCaseCases() []validateExprStrictCase {
	return []validateExprStrictCase{
		{
			name: "valid case expr with else",
			expr: &CaseExpr{
				Conditions: []CaseWhen{
					{
						When: &BinaryExpr{
							Op:    OpGt,
							Left:  &ColumnRefExpr{Column: "revenue"},
							Right: &LiteralExpr{Value: float64(100)},
						},
						Then: &LiteralExpr{Value: "high"},
					},
				},
				ElseExpr: &LiteralExpr{Value: "low"},
			},
			allowMetrics: false,
			wantErr:      "",
		},
		{
			name: "valid case expr without else",
			expr: &CaseExpr{
				Conditions: []CaseWhen{
					{
						When: &ColumnRefExpr{Column: "region"},
						Then: &LiteralExpr{Value: "EMEA"},
					},
				},
			},
			allowMetrics: false,
			wantErr:      "",
		},
		{
			name: "invalid case when condition",
			expr: &CaseExpr{
				Conditions: []CaseWhen{
					{
						When: &MetricRefExpr{Name: "total_revenue"},
						Then: &LiteralExpr{Value: "high"},
					},
				},
			},
			allowMetrics: false,
			wantErr:      "invalid when condition 0: metric reference not allowed",
		},
		{
			name: "invalid case then result",
			expr: &CaseExpr{
				Conditions: []CaseWhen{
					{
						When: &LiteralExpr{Value: true},
						Then: &MetricRefExpr{Name: "total_revenue"},
					},
				},
			},
			allowMetrics: false,
			wantErr:      "invalid then result 0: metric reference not allowed",
		},
		{
			name: "invalid case else result",
			expr: &CaseExpr{
				Conditions: []CaseWhen{
					{
						When: &LiteralExpr{Value: true},
						Then: &LiteralExpr{Value: "high"},
					},
				},
				ElseExpr: &MetricRefExpr{Name: "total_revenue"},
			},
			allowMetrics: false,
			wantErr:      "invalid else result: metric reference not allowed",
		},
		{
			name:         "empty node returns nil",
			expr:         nil,
			allowMetrics: false,
			wantErr:      "",
		},
		{
			name: "binary expr invalid left",
			expr: &BinaryExpr{
				Op:    OpAdd,
				Left:  &MetricRefExpr{Name: "total_revenue"},
				Right: &LiteralExpr{Value: 1},
			},
			allowMetrics: false,
			wantErr:      "metric reference not allowed",
		},
		{
			name: "binary expr valid but right invalid",
			expr: &BinaryExpr{
				Op:    OpAdd,
				Left:  &LiteralExpr{Value: 1},
				Right: &MetricRefExpr{Name: "total_revenue"},
			},
			allowMetrics: false,
			wantErr:      "metric reference not allowed",
		},
		{
			name: "function call invalid arg",
			expr: &FunctionCallExpr{
				Name: "COALESCE",
				Args: []ExprNode{
					&LiteralExpr{Value: "good"},
					&MetricRefExpr{Name: "total_revenue"},
				},
			},
			allowMetrics: false,
			wantErr:      "metric reference not allowed",
		},
	}
}

type unknownTestExpr struct{}

func (*unknownTestExpr) exprSealed() {}

func TestValidateExprStrictUnknownType(t *testing.T) {
	allowedCols, allowedMets, allowedDims := validateExprStrictAllowedMaps()
	err := ValidateExprStrict(&unknownTestExpr{}, allowedCols, allowedMets, allowedDims, false, 0)
	if err == nil {
		t.Fatal("expected error for unknown type")
	}
	if !strings.Contains(err.Error(), "unknown expression node type") {
		t.Fatalf("error = %v, want unknown type", err)
	}
}

func buildDeepExpr(depth int) ExprNode {
	if depth <= 0 {
		return &LiteralExpr{Value: 1}
	}
	return &UnaryExpr{
		Op:   OpNegate,
		Expr: buildDeepExpr(depth - 1),
	}
}
