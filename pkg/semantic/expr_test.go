package semantic

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestExprNodeJSONRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		node ExprNode
	}{
		{
			name: "literal",
			node: LiteralExpr{Value: "paid"},
		},
		{
			name: "column ref",
			node: ColumnRefExpr{Table: "orders", Column: "total_amount"},
		},
		{
			name: "metric ref",
			node: MetricRefExpr{Name: "gross_revenue"},
		},
		{
			name: "dimension ref",
			node: DimensionRefExpr{Name: "customer_country"},
		},
		{
			name: "binary",
			node: BinaryExpr{
				Op:    OpSubtract,
				Left:  MetricRefExpr{Name: "gross_revenue"},
				Right: MetricRefExpr{Name: "discount_amount"},
			},
		},
		{
			name: "unary",
			node: UnaryExpr{
				Op:   OpNegate,
				Expr: ColumnRefExpr{Column: "discount_amount"},
			},
		},
		{
			name: "function call",
			node: FunctionCallExpr{
				Name: "COALESCE",
				Args: []ExprNode{
					ColumnRefExpr{Column: "email"},
					LiteralExpr{Value: "N/A"},
				},
			},
		},
		{
			name: "case",
			node: CaseExpr{
				Conditions: []CaseWhen{
					{
						When: BinaryExpr{
							Op:    OpGt,
							Left:  ColumnRefExpr{Column: "total_amount"},
							Right: LiteralExpr{Value: float64(0)},
						},
						Then: LiteralExpr{Value: "positive"},
					},
				},
				ElseExpr: LiteralExpr{Value: "negative"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.node)
			if err != nil {
				t.Fatalf("json.Marshal(%T) error = %v", tt.node, err)
			}
			if !strings.Contains(string(data), `"type"`) {
				t.Fatalf("json.Marshal(%T) = %s, want type discriminator", tt.node, data)
			}

			got, err := UnmarshalExprNode(data)
			if err != nil {
				t.Fatalf("UnmarshalExprNode(%s) error = %v", data, err)
			}
			gotJSON, err := json.Marshal(got)
			if err != nil {
				t.Fatalf("json.Marshal(roundtrip %T) error = %v", got, err)
			}
			if string(gotJSON) != string(data) {
				t.Fatalf("ExprNode JSON round trip mismatch: got %s, want %s", gotJSON, data)
			}
		})
	}
}

func TestUnmarshalExprNodeRejectsUnknownType(t *testing.T) {
	_, err := UnmarshalExprNode([]byte(`{"type":"danger","sql":"DROP TABLE users"}`))
	if err == nil {
		t.Fatal(`UnmarshalExprNode({"type":"danger"}) error = nil, want error`)
	}
}

func TestAllowedFunctions(t *testing.T) {
	tests := []struct {
		name      string
		function  string
		wantArity int
	}{
		{name: "variadic coalesce", function: "COALESCE", wantArity: -1},
		{name: "single argument upper", function: "UPPER", wantArity: 1},
		{name: "round", function: "ROUND", wantArity: 2},
		{name: "date trunc", function: "DATE_TRUNC", wantArity: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := AllowedFunctions[tt.function]
			if !ok {
				t.Fatalf("AllowedFunctions[%q] missing", tt.function)
			}
			if got != tt.wantArity {
				t.Fatalf("AllowedFunctions[%q] = %d, want %d", tt.function, got, tt.wantArity)
			}
		})
	}
}
