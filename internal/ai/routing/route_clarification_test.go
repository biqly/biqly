package routing

import (
	"context"
	"testing"

	"github.com/biqly/biqly/internal/metadata"
)

// Tier-0 routing ambiguity: when a question matches two tables with equal,
// low scores, the router must not silently pick one — it must flag
// NeedsClarification and surface the competing candidates so the caller can
// ask the user which table to use.
func TestTableRouter_RouteNeedsClarificationForCompetingCandidates(t *testing.T) {
	// Two tables match the question only weakly and equally — via a column
	// description, never a table/column name. column_description weight is 1, so
	// each table scores low; routeConfidence stays below minRouteConfidence and
	// the two equal candidates make the routing genuinely ambiguous. "refund" is
	// neutral (not a revenue / readable-label / category-product token), so no
	// boost breaks the tie.
	refundNotes := "customer refund details"
	refundMemo := "refund processing notes"
	reader := fakeMetadataReader{
		tables: []metadata.Table{
			{DatasourceID: "ds1", SchemaName: "public", TableName: "orders", TableType: "BASE TABLE"},
			{DatasourceID: "ds1", SchemaName: "public", TableName: "tickets", TableType: "BASE TABLE"},
		},
		columns: []metadata.Column{
			{DatasourceID: "ds1", SchemaName: "public", TableName: "orders", ColumnName: "id", DataType: "uuid", IsPrimaryKey: true},
			{DatasourceID: "ds1", SchemaName: "public", TableName: "orders", ColumnName: "notes", DataType: "text", Description: &refundNotes},
			{DatasourceID: "ds1", SchemaName: "public", TableName: "tickets", ColumnName: "id", DataType: "uuid", IsPrimaryKey: true},
			{DatasourceID: "ds1", SchemaName: "public", TableName: "tickets", ColumnName: "memo", DataType: "text", Description: &refundMemo},
		},
	}
	router := NewTableRouter(reader)

	model, result, err := router.Route(context.Background(), "ds1", "refund", nil, true, true)
	if err != nil {
		t.Fatalf("Route() error = %v, want nil", err)
	}
	if model != nil {
		t.Fatalf("Route() model = %+v, want nil on clarification", model)
	}
	if !result.NeedsClarification {
		t.Fatalf("Route() NeedsClarification = false, want true for competing candidates")
	}
	if result.Confidence >= minRouteConfidence {
		t.Errorf("Route() confidence = %v, want < %v (ambiguous tie)", result.Confidence, minRouteConfidence)
	}
	if len(result.Candidates) < 2 {
		t.Fatalf("Route() candidates = %d, want >= 2 competing tables", len(result.Candidates))
	}
	// The two top candidates tie — that is exactly what makes routing ambiguous.
	if result.Candidates[0].Score != result.Candidates[1].Score {
		t.Errorf("top candidate scores = %v vs %v, want equal (competing)",
			result.Candidates[0].Score, result.Candidates[1].Score)
	}
}
