package semantic

import (
	"testing"

	pkgsemantic "github.com/biqly/biqly/pkg/semantic"
)

func TestDecodeModelSnapshotHydratesExpressionASTs(t *testing.T) {
	raw := []byte(`{
		"id":"model-1",
		"datasource_id":"ds-1",
		"name":"orders",
		"base_schema":"public",
		"base_table":"orders",
		"is_active":true,
		"status":"published",
		"version":3,
		"dimensions":[{
			"id":"dim-1",
			"model_id":"model-1",
			"name":"net_revenue",
			"column_ref":"orders.revenue",
			"type":"number",
			"is_active":true,
			"calculated_expression":"orders.revenue - orders.cost",
			"calculated_expr":{
				"type":"binary",
				"op":"-",
				"left":{"type":"column_ref","table":"orders","column":"revenue"},
				"right":{"type":"column_ref","table":"orders","column":"cost"}
			}
		}],
		"metrics":[{
			"id":"metric-1",
			"model_id":"model-1",
			"name":"total_revenue",
			"expression":"orders.revenue",
			"aggregation":"sum",
			"is_active":true,
			"expr":{"type":"column_ref","table":"orders","column":"revenue"}
		}]
	}`)

	model, err := decodeModelSnapshot(raw)
	if err != nil {
		t.Fatalf("decodeModelSnapshot() error = %v", err)
	}
	if model.ID != "model-1" || model.Version != 3 {
		t.Fatalf("decoded model = %#v", model)
	}
	if _, ok := model.Dimensions[0].CalculatedExpr.(*pkgsemantic.BinaryExpr); !ok {
		t.Fatalf("dimension CalculatedExpr = %T, want *BinaryExpr", model.Dimensions[0].CalculatedExpr)
	}
	if _, ok := model.Metrics[0].Expr.(*pkgsemantic.ColumnRefExpr); !ok {
		t.Fatalf("metric Expr = %T, want *ColumnRefExpr", model.Metrics[0].Expr)
	}
}

func TestDecodeModelSnapshotIgnoresInvalidStoredExpressionAST(t *testing.T) {
	raw := []byte(`{
		"id":"model-1",
		"datasource_id":"ds-1",
		"name":"orders",
		"base_schema":"public",
		"base_table":"orders",
		"is_active":true,
		"status":"published",
		"version":3,
		"metrics":[{
			"id":"metric-1",
			"model_id":"model-1",
			"name":"total_revenue",
			"expression":"*",
			"aggregation":"count",
			"is_active":true,
			"expr":{"type":"unknown"}
		}]
	}`)

	model, err := decodeModelSnapshot(raw)
	if err != nil {
		t.Fatalf("decodeModelSnapshot() error = %v", err)
	}
	if model.Metrics[0].Expr != nil {
		t.Fatalf("metric Expr = %#v, want nil for invalid stored AST", model.Metrics[0].Expr)
	}
}
