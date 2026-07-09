package query

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStructuredMetricQuery_ToLogicalQuery(t *testing.T) {
	lq, err := StructuredMetricQuery{
		DatasourceID: "ds-1",
		ModelID:      "model-1",
		Measures:     []string{"revenue"},
		Dimensions:   []string{"region"},
		TimeDimension: &TimeDimension{
			Dimension: "order_date",
			Grain:     TimeGrainMonth,
			DateRange: &DateRange{Start: "2024-01-01", End: "2024-12-31"},
		},
		Filters: []StructuredFilter{
			{Field: "status", Operator: OpEq, Value: "completed"},
		},
		Sort:   []StructuredSort{{Field: "revenue", Direction: OrderDesc}},
		Limit:  50,
		Offset: 10,
	}.ToLogicalQuery()
	require.NoError(t, err)

	assert.Equal(t, "ds-1", lq.DatasourceID)
	assert.Equal(t, "model-1", lq.ModelID)
	assert.Equal(t, 50, lq.Limit)
	assert.Equal(t, 10, lq.Offset)

	require.Len(t, lq.Select, 3)
	assert.Equal(t, SelectTypeDimension, lq.Select[0].Type)
	assert.Equal(t, "region", lq.Select[0].Name)
	assert.Equal(t, "order_date", lq.Select[1].Name)
	assert.Equal(t, SelectTypeMetric, lq.Select[2].Type)
	assert.Equal(t, "revenue", lq.Select[2].Name)

	require.Len(t, lq.GroupBy, 2)
	assert.Equal(t, "region", lq.GroupBy[0].Field)
	assert.Equal(t, "order_date", lq.GroupBy[1].Field)
	assert.Equal(t, TimeGrainMonth, lq.GroupBy[1].TimeGrain)

	require.Len(t, lq.Filters, 2)
	assert.Equal(t, OpBetween, lq.Filters[0].Operator)
	assert.Equal(t, "status", lq.Filters[1].Field)

	require.Len(t, lq.OrderBy, 1)
	assert.Equal(t, "revenue", lq.OrderBy[0].Field)
	assert.Equal(t, OrderDesc, lq.OrderBy[0].Direction)
}

func TestStructuredMetricQuery_RequiresIDsAndFields(t *testing.T) {
	_, err := StructuredMetricQuery{ModelID: "m"}.ToLogicalQuery()
	require.Error(t, err)

	_, err = StructuredMetricQuery{DatasourceID: "ds"}.ToLogicalQuery()
	require.Error(t, err)

	_, err = StructuredMetricQuery{DatasourceID: "ds", ModelID: "m"}.ToLogicalQuery()
	require.Error(t, err)
}

func TestStructuredMetricQuery_DefaultLimitAndOpenRange(t *testing.T) {
	lq, err := StructuredMetricQuery{
		DatasourceID: "ds",
		ModelID:      "m",
		Measures:     []string{"count_rows"},
		TimeDimension: &TimeDimension{
			Dimension: "created_at",
			Grain:     TimeGrainDay,
			DateRange: &DateRange{Start: "2024-01-01"},
		},
	}.ToLogicalQuery()
	require.NoError(t, err)
	assert.Equal(t, 1000, lq.Limit)
	require.Len(t, lq.Filters, 1)
	assert.Equal(t, OpGte, lq.Filters[0].Operator)
}

func TestStructuredMetricQuery_InvalidOperatorAndGrain(t *testing.T) {
	_, err := StructuredMetricQuery{
		DatasourceID: "ds",
		ModelID:      "m",
		Measures:     []string{"x"},
		Filters:      []StructuredFilter{{Field: "a", Operator: "like", Value: "%"}},
	}.ToLogicalQuery()
	require.Error(t, err)

	_, err = StructuredMetricQuery{
		DatasourceID:  "ds",
		ModelID:       "m",
		Measures:      []string{"x"},
		TimeDimension: &TimeDimension{Dimension: "d", Grain: "fortnight"},
	}.ToLogicalQuery()
	require.Error(t, err)
}
