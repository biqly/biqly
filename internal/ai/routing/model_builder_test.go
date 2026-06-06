package routing

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/biqly/biqly/internal/metadata"
)

func TestBuildSemanticModel(t *testing.T) {
	// Setup test inputs
	datasourceID := "ds1"

	table1 := metadata.Table{
		ID:         "t1",
		SchemaName: "public",
		TableName:  "users",
	}

	selected := []tableBundle{
		{
			table: table1,
			score: 1.0,
		},
	}

	columnsByTable := map[string][]metadata.Column{
		"public.users": {
			{
				ID:           "c1",
				ColumnName:   "id",
				DataType:     "integer",
				IsPrimaryKey: true,
			},
			{
				ID:         "c2",
				ColumnName: "name",
				DataType:   "varchar",
			},
		},
	}

	relations := []metadata.Relation{}
	limits := Limits{}
	timeGrains := []metadata.TimeGrain{}

	model := buildSemanticModel(datasourceID, selected, columnsByTable, relations, limits, timeGrains)

	assert.NotNil(t, model)
	assert.Equal(t, "auto:public.users", model.Name)
	assert.Equal(t, datasourceID, model.DatasourceID)
	assert.Equal(t, "public", model.BaseSchema)
	assert.Equal(t, "users", model.BaseTable)
	assert.True(t, model.IsActive)

	// Check dimensions are built
	assert.NotEmpty(t, model.Dimensions)
	// Check metrics are built
	assert.NotEmpty(t, model.Metrics)
}

func TestBuildSemanticModel_MultiTable(t *testing.T) {
	orders := metadata.Table{ID: "t1", SchemaName: "sales", TableName: "orders"}
	customers := metadata.Table{ID: "t2", SchemaName: "sales", TableName: "customers"}
	selected := []tableBundle{
		{table: orders, score: 2.0},
		{table: customers, score: 1.0},
	}
	columnsByTable := map[string][]metadata.Column{
		"sales.orders": {
			{ID: "c1", ColumnName: "id", DataType: "integer", IsPrimaryKey: true},
			{ID: "c2", ColumnName: "total_amount", DataType: "numeric"},
			{ID: "c3", ColumnName: "order_date", DataType: "date"},
			{ID: "c4", ColumnName: "customer_id", DataType: "integer", IsForeignKey: true},
		},
		"sales.customers": {
			{ID: "c5", ColumnName: "id", DataType: "integer", IsPrimaryKey: true},
			{ID: "c6", ColumnName: "name", DataType: "varchar"},
			{ID: "c7", ColumnName: "country", DataType: "varchar"},
		},
	}
	relations := []metadata.Relation{
		{
			FromSchema: "sales", FromTable: "orders", FromColumn: "customer_id",
			ToSchema: "sales", ToTable: "customers", ToColumn: "id",
		},
	}
	limits := Limits{}
	timeGrains := []metadata.TimeGrain{}

	model := buildSemanticModel("ds1", selected, columnsByTable, relations, limits, timeGrains)

	require.NotNil(t, model)

	// Base table is the first selected table
	assert.Equal(t, "sales", model.BaseSchema)
	assert.Equal(t, "orders", model.BaseTable)

	// Model should have dimensions from both tables
	dimNames := make(map[string]bool)
	for _, d := range model.Dimensions {
		dimNames[d.Name] = true
	}
	// PK and FK columns from orders should be dimensions
	assert.True(t, dimNames["id"] || dimNames["orders_id"], "expected orders.id dimension")
	assert.True(t, dimNames["customer_id"], "expected customer_id dimension")
	// name column from customers should be a dimension
	assert.True(t, dimNames["name"], "expected customers.name dimension")

	// Numeric column total_amount should produce a metric
	metricNames := make(map[string]bool)
	for _, m := range model.Metrics {
		metricNames[m.Name] = true
	}
	assert.True(t, metricNames["sum_total_amount"] || metricNames["total_amount"], "expected total_amount metric")

	// Joins should be generated from relations
	assert.NotEmpty(t, model.Joins, "expected at least one join from the relation")
}

func TestBuildSemanticModel_DateGrains(t *testing.T) {
	table := metadata.Table{ID: "t1", SchemaName: "public", TableName: "events"}
	selected := []tableBundle{{table: table, score: 1.0}}
	columnsByTable := map[string][]metadata.Column{
		"public.events": {
			{ID: "c1", ColumnName: "id", DataType: "integer", IsPrimaryKey: true},
			{ID: "c2", ColumnName: "event_date", DataType: "date"},
			{ID: "c3", ColumnName: "category", DataType: "varchar"},
		},
	}

	timeGrains := []metadata.TimeGrain{
		{Grain: "year", Suffix: "_year", RequiresTime: false},
		{Grain: "month", Suffix: "_month", RequiresTime: false},
	}

	model := buildSemanticModel("ds1", selected, columnsByTable, nil, Limits{}, timeGrains)
	require.NotNil(t, model)

	// event_date should generate the base dimension plus grain dimensions
	found := 0
	for _, d := range model.Dimensions {
		if d.ColumnRef == "events.event_date" {
			found++
		}
	}
	// At least: event_date, event_date_year, event_date_month
	assert.GreaterOrEqual(t, found, 3, "expected base date dimension plus at least 2 grain dimensions")
}

func TestBuildSemanticModel_DimensionLimits(t *testing.T) {
	table := metadata.Table{ID: "t1", SchemaName: "public", TableName: "wide_table"}
	selected := []tableBundle{{table: table, score: 1.0}}

	// Create many columns
	columns := make([]metadata.Column, 200)
	for i := range 200 {
		columns[i] = metadata.Column{
			ID:         "c" + string(rune('0'+i%10)),
			ColumnName: "col_" + string(rune('a'+i%26)) + string(rune('a'+i/26)),
			DataType:   "varchar",
		}
	}
	columnsByTable := map[string][]metadata.Column{"public.wide_table": columns}

	limits := Limits{MaxDimensions: 10}
	model := buildSemanticModel("ds1", selected, columnsByTable, nil, limits, nil)
	require.NotNil(t, model)

	assert.LessOrEqual(t, len(model.Dimensions), 10, "dimensions should be capped by MaxDimensions")
}
