package semantic

import (
	"testing"

	pkgsemantic "github.com/biqly/biqly/pkg/semantic"
)

func TestExpressionASTStorageRoundTrip(t *testing.T) {
	expr := &pkgsemantic.BinaryExpr{
		Op:    pkgsemantic.OpSubtract,
		Left:  &pkgsemantic.ColumnRefExpr{Table: "orders", Column: "revenue"},
		Right: &pkgsemantic.ColumnRefExpr{Table: "orders", Column: "cost"},
	}

	encoded, err := encodeExprNodeJSON(expr)
	if err != nil {
		t.Fatalf("encodeExprNodeJSON() error = %v", err)
	}
	if len(encoded) == 0 {
		t.Fatal("encodeExprNodeJSON() returned empty JSON")
	}

	decoded := decodeExprNodeJSON(encoded)
	if decoded == nil {
		t.Fatal("decodeExprNodeJSON() returned nil")
	}
	if _, ok := decoded.(*pkgsemantic.BinaryExpr); !ok {
		t.Fatalf("decodeExprNodeJSON() = %T, want *semantic.BinaryExpr", decoded)
	}
}

func TestDecodeExprNodeJSONFailOpen(t *testing.T) {
	if got := decodeExprNodeJSON([]byte(`{"type":"unknown"}`)); got != nil {
		t.Fatalf("decodeExprNodeJSON() = %#v, want nil for invalid AST JSON", got)
	}
}
