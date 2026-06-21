package semantic

import (
	"strings"
	"testing"

	"github.com/bytedance/sonic"
)

func TestExprNodeJSONRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		node ExprNode
	}{
		{
			name: "literal",
			node: &LiteralExpr{Value: "paid"},
		},
		{
			name: "column ref",
			node: &ColumnRefExpr{Table: "orders", Column: "total_amount"},
		},
		{
			name: "metric ref",
			node: &MetricRefExpr{Name: "gross_revenue"},
		},
		{
			name: "dimension ref",
			node: &DimensionRefExpr{Name: "customer_country"},
		},
		{
			name: "binary",
			node: &BinaryExpr{
				Op:    OpSubtract,
				Left:  &MetricRefExpr{Name: "gross_revenue"},
				Right: &MetricRefExpr{Name: "discount_amount"},
			},
		},
		{
			name: "unary",
			node: &UnaryExpr{
				Op:   OpNegate,
				Expr: &ColumnRefExpr{Column: "discount_amount"},
			},
		},
		{
			name: "function call",
			node: &FunctionCallExpr{
				Name: "COALESCE",
				Args: []ExprNode{
					&ColumnRefExpr{Column: "email"},
					&LiteralExpr{Value: "N/A"},
				},
			},
		},
		{
			name: "case",
			node: &CaseExpr{
				Conditions: []CaseWhen{
					{
						When: &BinaryExpr{
							Op:    OpGt,
							Left:  &ColumnRefExpr{Column: "total_amount"},
							Right: &LiteralExpr{Value: float64(0)},
						},
						Then: &LiteralExpr{Value: "positive"},
					},
				},
				ElseExpr: &LiteralExpr{Value: "negative"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := sonic.ConfigStd.Marshal(tt.node)
			if err != nil {
				t.Fatalf("sonic.ConfigStd.Marshal(%T) error = %v", tt.node, err)
			}
			if !strings.Contains(string(data), `"type"`) {
				t.Fatalf("sonic.ConfigStd.Marshal(%T) = %s, want type discriminator", tt.node, data)
			}

			got, err := UnmarshalExprNode(data)
			if err != nil {
				t.Fatalf("UnmarshalExprNode(%s) error = %v", data, err)
			}
			gotJSON, err := sonic.ConfigStd.Marshal(got)
			if err != nil {
				t.Fatalf("sonic.ConfigStd.Marshal(roundtrip %T) error = %v", got, err)
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

func TestRequireExprTypeError(t *testing.T) {
	err := requireExprType("wrong", "literal")
	if err == nil {
		t.Fatal("requireExprType should return error on mismatch")
	}
	if !strings.Contains(err.Error(), `"wrong"`) || !strings.Contains(err.Error(), `"literal"`) {
		t.Fatalf("requireExprType error = %v, want type info", err)
	}
}

func TestUnmarshalExprNodeEmpty(t *testing.T) {
	_, err := UnmarshalExprNode(nil)
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("UnmarshalExprNode(nil) error = %v, want 'empty'", err)
	}
	_, err = UnmarshalExprNode([]byte("null"))
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("UnmarshalExprNode(null) error = %v, want 'empty'", err)
	}
	_, err = UnmarshalExprNode([]byte{})
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("UnmarshalExprNode(empty) error = %v, want 'empty'", err)
	}
}

// UnmarshalJSON with wrong type triggers requireExprType error for each node type.
func TestUnmarshalJSONWrongType(t *testing.T) {
	tests := []struct {
		name string
		expr ExprNode
	}{
		{"LiteralExpr", &LiteralExpr{}},
		{"ColumnRefExpr", &ColumnRefExpr{}},
		{"MetricRefExpr", &MetricRefExpr{}},
		{"DimensionRefExpr", &DimensionRefExpr{}},
		{"BinaryExpr", &BinaryExpr{}},
		{"UnaryExpr", &UnaryExpr{}},
		{"FunctionCallExpr", &FunctionCallExpr{}},
		{"CaseExpr", &CaseExpr{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := []byte(`{"type":"bogus"}`)
			var err error
			switch e := tt.expr.(type) {
			case *LiteralExpr:
				err = e.UnmarshalJSON(data)
			case *ColumnRefExpr:
				err = e.UnmarshalJSON(data)
			case *MetricRefExpr:
				err = e.UnmarshalJSON(data)
			case *DimensionRefExpr:
				err = e.UnmarshalJSON(data)
			case *BinaryExpr:
				err = e.UnmarshalJSON(data)
			case *UnaryExpr:
				err = e.UnmarshalJSON(data)
			case *FunctionCallExpr:
				err = e.UnmarshalJSON(data)
			case *CaseExpr:
				err = e.UnmarshalJSON(data)
			}
			if err == nil {
				t.Fatalf("%T.UnmarshalJSON with wrong type should error", tt.expr)
			}
			if !strings.Contains(err.Error(), `"bogus"`) {
				t.Fatalf("%T.UnmarshalJSON error = %v, want type info", tt.expr, err)
			}
		})
	}
}

func TestCaseExprUnmarshalJSONInvalidCondition(t *testing.T) {
	// toCaseWhen error: invalid "when" field
	data := []byte(`{"type":"case","conditions":[{"when":null,"then":{"type":"literal","value":"yes"}}]}`)
	var expr CaseExpr
	err := expr.UnmarshalJSON(data)
	if err == nil {
		t.Fatal("expected error for null when in condition")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Fatalf("error = %v, want 'empty'", err)
	}
}

func TestCaseExprUnmarshalJSONInvalidThen(t *testing.T) {
	// toCaseWhen error: invalid "then" field
	data := []byte(`{"type":"case","conditions":[{"when":{"type":"literal","value":1},"then":null}]}`)
	var expr CaseExpr
	err := expr.UnmarshalJSON(data)
	if err == nil {
		t.Fatal("expected error for null then in condition")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Fatalf("error = %v, want 'empty'", err)
	}
}

func TestCaseExprUnmarshalJSONInvalidElse(t *testing.T) {
	// CaseExpr.UnmarshalJSON: else unmarshal error
	data := []byte(`{"type":"case","conditions":[{"when":{"type":"literal","value":1},"then":{"type":"literal","value":"yes"}}],"else":"invalid"}`)
	var expr CaseExpr
	err := expr.UnmarshalJSON(data)
	if err == nil {
		t.Fatal("expected error for invalid else")
	}
	if !strings.Contains(err.Error(), "unmarshal case else") {
		t.Fatalf("error = %v, want 'unmarshal case else'", err)
	}
}

func TestBinaryExprUnmarshalJSONInvalidChild(t *testing.T) {
	// BinaryExpr.UnmarshalJSON: left child unmarshal error
	data := []byte(`{"type":"binary","op":"add","left":null,"right":{"type":"literal","value":1}}`)
	var expr BinaryExpr
	err := expr.UnmarshalJSON(data)
	if err == nil {
		t.Fatal("expected error for null left child")
	}
	if !strings.Contains(err.Error(), "unmarshal binary left") {
		t.Fatalf("error = %v, want 'unmarshal binary left'", err)
	}

	// Right child unmarshal error
	data = []byte(`{"type":"binary","op":"add","left":{"type":"literal","value":1},"right":null}`)
	err = expr.UnmarshalJSON(data)
	if err == nil {
		t.Fatal("expected error for null right child")
	}
	if !strings.Contains(err.Error(), "unmarshal binary right") {
		t.Fatalf("error = %v, want 'unmarshal binary right'", err)
	}
}

func TestUnaryExprUnmarshalJSONInvalidChild(t *testing.T) {
	data := []byte(`{"type":"unary","op":"negate","expr":null}`)
	var expr UnaryExpr
	err := expr.UnmarshalJSON(data)
	if err == nil {
		t.Fatal("expected error for null expr child")
	}
	if !strings.Contains(err.Error(), "unmarshal unary expr") {
		t.Fatalf("error = %v, want 'unmarshal unary expr'", err)
	}
}

func TestBinaryExprUnmarshalJSONSonicError(t *testing.T) {
	// Bad op field type triggers sonic unmarshal error in BinaryExpr.UnmarshalJSON
	data := []byte(`{"type":"binary","op":123,"left":{"type":"literal","value":1},"right":{"type":"literal","value":2}}`)
	var expr BinaryExpr
	err := expr.UnmarshalJSON(data)
	if err == nil {
		t.Fatal("expected error for non-string op field")
	}
}

func TestUnaryExprUnmarshalJSONSonicError(t *testing.T) {
	// Bad op field type triggers sonic unmarshal error in UnaryExpr.UnmarshalJSON
	data := []byte(`{"type":"unary","op":123,"expr":{"type":"literal","value":1}}`)
	var expr UnaryExpr
	err := expr.UnmarshalJSON(data)
	if err == nil {
		t.Fatal("expected error for non-string op field")
	}
}

func TestFunctionCallExprUnmarshalJSONInvalidArg(t *testing.T) {
	data := []byte(`{"type":"function_call","name":"CONCAT","args":[{"type":"literal","value":"hello"},null]}`)
	var expr FunctionCallExpr
	err := expr.UnmarshalJSON(data)
	if err == nil {
		t.Fatal("expected error for null arg")
	}
	if !strings.Contains(err.Error(), "unmarshal function arg 1") {
		t.Fatalf("error = %v, want 'unmarshal function arg 1'", err)
	}
}

// UnmarshalExprNode type-specific unmarshal errors: valid type discriminator
// but invalid field types cause sonic unmarshal to fail.
func TestUnmarshalExprNodeTypeUnmarshalErrors(t *testing.T) {
	tests := []struct {
		name string
		json string
	}{
		{"column_ref", `{"type":"column_ref","column":123}`},
		{"metric_ref", `{"type":"metric_ref","name":123}`},
		{"dimension_ref", `{"type":"dimension_ref","name":123}`},
		{"binary", `{"type":"binary","op":"add","left":null,"right":null}`},
		{"unary", `{"type":"unary","op":"negate","expr":null}`},
		{"function_call", `{"type":"function_call","name":123}`},
		{"case", `{"type":"case","conditions":"not_an_array"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := UnmarshalExprNode([]byte(tt.json))
			if err == nil {
				t.Fatalf("UnmarshalExprNode(%s) error = nil, want error", tt.json)
			}
		})
	}
}
