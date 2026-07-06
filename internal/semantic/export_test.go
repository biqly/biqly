package semantic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func strPtr(s string) *string { return &s }

func sampleModel() *SemanticModel {
	return &SemanticModel{
		ID:           "m-1",
		DatasourceID: "ds-1",
		Name:         "orders",
		Label:        strPtr("Orders"),
		Description:  strPtr("Order facts"),
		BaseSchema:   "public",
		BaseTable:    "orders",
		Synonyms:     []string{"siparişler"},
		Version:      3,
		Status:       "published",
		IsActive:     true,
		Dimensions: []Dimension{
			{
				ID: "d-2", ModelID: "m-1", Name: "status", ColumnRef: "orders.status", Type: "text", IsActive: true,
				EnumValues: []EnumMapping{{RawValue: "1", Label: "Açık", SortOrder: 1}},
			},
			{ID: "d-1", ModelID: "m-1", Name: "created_at", ColumnRef: "orders.created_at", Type: "date", TimeGrain: "day", IsActive: true},
			{ID: "d-3", ModelID: "m-1", Name: "inactive_dim", ColumnRef: "orders.x", Type: "text", IsActive: false},
		},
		Metrics: []Metric{
			{ID: "mt-1", ModelID: "m-1", Name: "revenue", Expression: "orders.total_amount", Aggregation: "sum", Format: strPtr("currency"), IsActive: true, RateBehavior: RateBehaviorRatioOfSums},
			{ID: "mt-2", ModelID: "m-1", Name: "dead_metric", Expression: "orders.x", Aggregation: "sum", IsActive: false},
		},
		Joins: []Join{
			{ID: "j-1", ModelID: "m-1", Name: "orders_customers", FromTable: "orders", FromColumn: "customer_id", ToTable: "customers", ToColumn: "id", JoinType: "LEFT", Relationship: "many_to_one", IsActive: true},
		},
	}
}

func TestModelFileRoundTrip(t *testing.T) {
	file := NewModelFile(sampleModel())

	assert.Equal(t, ModelFileSchemaVersion, file.SchemaVersion)
	assert.Len(t, file.Dimensions, 2, "inactive dimensions excluded")
	assert.Len(t, file.Metrics, 1, "inactive metrics excluded")
	assert.Equal(t, "created_at", file.Dimensions[0].Name, "dimensions sorted by name")

	out, err := MarshalModelFile(file)
	require.NoError(t, err)

	parsed, err := ParseModelFile(out)
	require.NoError(t, err)
	assert.Equal(t, file, *parsed)

	out2, err := MarshalModelFile(*parsed)
	require.NoError(t, err)
	assert.Equal(t, string(out), string(out2), "marshal is deterministic")

	model := parsed.Model("ds-2")
	assert.Equal(t, "ds-2", model.DatasourceID)
	assert.Empty(t, model.ID)
	assert.True(t, model.IsActive)
	require.Len(t, model.Dimensions, 2)
	assert.Equal(t, "Açık", model.Dimensions[1].EnumValues[0].Label)
	require.Len(t, model.Metrics, 1)
	assert.Equal(t, "currency", *model.Metrics[0].Format)
	assert.Equal(t, RateBehaviorRatioOfSums, model.Metrics[0].RateBehavior, "rate_behavior round-trips")
	require.Len(t, model.Joins, 1)
}

func TestParseModelFileRejectsBadInput(t *testing.T) {
	cases := []struct {
		name string
		in   string
	}{
		{"wrong version", "biqly_semantic_model: v99\nname: x\nbase_table: t\n"},
		{"missing name", "biqly_semantic_model: v1\nbase_table: t\n"},
		{"missing base_table", "biqly_semantic_model: v1\nname: x\n"},
		{"invalid yaml", "biqly_semantic_model: [\n"},
		{"invalid rate_behavior", "biqly_semantic_model: v1\nname: x\nbase_table: t\nmetrics:\n  - name: m\n    expression: t.c\n    aggregation: sum\n    rate_behavior: bogus\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseModelFile([]byte(tc.in))
			assert.Error(t, err)
		})
	}
}

func TestParseModelFileAcceptsJSON(t *testing.T) {
	in := `{"biqly_semantic_model":"v1","name":"orders","base_schema":"public","base_table":"orders"}`
	parsed, err := ParseModelFile([]byte(in))
	require.NoError(t, err)
	assert.Equal(t, "orders", parsed.Name)
}
