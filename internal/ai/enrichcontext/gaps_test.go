package enrichcontext

import (
	"testing"

	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/semantic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectGaps_MissingDescriptionsAndEnum(t *testing.T) {
	desc := "has text"
	model := &semantic.SemanticModel{
		ID:           "m-1",
		DatasourceID: "ds-1",
		Name:         "orders",
		BaseSchema:   "public",
		BaseTable:    "orders",
		Dimensions: []semantic.Dimension{
			{
				ID:        "d-1",
				Name:      "status",
				ColumnRef: "orders.status",
				EnumValues: []semantic.EnumMapping{
					{RawValue: "1", Label: ""},
				},
			},
			{
				ID:          "d-2",
				Name:        "country",
				ColumnRef:   "orders.country",
				Description: &desc,
			},
		},
		Metrics: []semantic.Metric{
			{ID: "met-1", Name: "revenue", Expression: "orders.total"},
		},
	}
	columns := []metadata.Column{
		{ID: "c-1", SchemaName: "", TableName: "orders", ColumnName: "status"},
		{ID: "c-2", SchemaName: "", TableName: "orders", ColumnName: "country", Description: &desc},
	}
	glossary := []metadata.BusinessGlossaryRow{
		{ID: "g-1", Term: "ciro", MapsToType: "metric", MapsToName: "revenue"},
	}

	gaps := detectGaps(model, glossary, columns)
	kinds := make(map[GapKind]int)
	for _, g := range gaps {
		kinds[g.Kind]++
	}
	assert.GreaterOrEqual(t, kinds[GapDimensionMissingDescription], 1)
	assert.GreaterOrEqual(t, kinds[GapColumnMissingDescription], 1)
	assert.GreaterOrEqual(t, kinds[GapMetricMissingDescription], 1)
	assert.GreaterOrEqual(t, kinds[GapGlossaryMissingDefinition], 1)
	assert.GreaterOrEqual(t, kinds[GapEnumMissingLabel], 1)
}

func TestDetectGlossaryCollisions(t *testing.T) {
	rows := []metadata.BusinessGlossaryRow{
		{Term: "sales", MapsToType: "metric", MapsToName: "revenue"},
		{Term: "revenue alias", Aliases: []string{"sales"}, MapsToType: "dimension", MapsToName: "region"},
	}
	gaps := detectGlossaryCollisions(rows)
	require.Len(t, gaps, 1)
	assert.Equal(t, GapSynonymCollision, gaps[0].Kind)
	assert.False(t, gaps[0].Applyable)
}

func TestDetectModelSynonymCollisions(t *testing.T) {
	model := &semantic.SemanticModel{
		Dimensions: []semantic.Dimension{{ID: "d1", Name: "amount", Synonyms: []string{"total"}}},
		Metrics:    []semantic.Metric{{ID: "m1", Name: "total", Expression: "sum(x)"}},
	}
	gaps := detectModelSynonymCollisions(model)
	require.Len(t, gaps, 1)
	assert.Equal(t, "collision:model:total", gaps[0].ID)
}
