package semanticgen

import (
	"testing"

	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/semantic"
)

func TestGenerateModelFromMetadata(t *testing.T) {
	rowEstimate := int64(100)
	tables := []metadata.Table{
		{DatasourceID: "ds1", SchemaName: "sales", TableName: "customers", TableType: "BASE TABLE"},
		{DatasourceID: "ds1", SchemaName: "sales", TableName: "orders", TableType: "BASE TABLE", RowEstimate: &rowEstimate},
	}
	columns := []metadata.Column{
		{DatasourceID: "ds1", SchemaName: "sales", TableName: "orders", ColumnName: "id", DataType: "integer", IsPrimaryKey: true},
		{DatasourceID: "ds1", SchemaName: "sales", TableName: "orders", ColumnName: "customer_id", DataType: "integer", IsForeignKey: true},
		{DatasourceID: "ds1", SchemaName: "sales", TableName: "orders", ColumnName: "order_date", DataType: "timestamp"},
		{DatasourceID: "ds1", SchemaName: "sales", TableName: "orders", ColumnName: "total_amount", DataType: "numeric"},
		{DatasourceID: "ds1", SchemaName: "sales", TableName: "customers", ColumnName: "id", DataType: "integer", IsPrimaryKey: true},
		{DatasourceID: "ds1", SchemaName: "sales", TableName: "customers", ColumnName: "name", DataType: "varchar"},
	}
	relations := []metadata.Relation{
		{
			DatasourceID:     "ds1",
			FromSchema:       "sales",
			FromTable:        "orders",
			FromColumn:       "customer_id",
			ToSchema:         "sales",
			ToTable:          "customers",
			ToColumn:         "id",
			RelationshipType: "many_to_one",
			ConstraintName:   "orders_customer_id_fkey",
		},
	}

	got, err := GenerateModelFromMetadata(tables, columns, relations, GenerateModelOptions{
		DatasourceID: "ds1",
		BaseSchema:   "sales",
		BaseTable:    "orders",
	})
	if err != nil {
		t.Fatalf("GenerateModelFromMetadata() error = %v", err)
	}

	if got.Model.Name != "orders" {
		t.Fatalf("model name = %q, want orders", got.Model.Name)
	}
	if got.Model.BaseSchema != "sales" || got.Model.BaseTable != "orders" {
		t.Fatalf("base = %s.%s, want sales.orders", got.Model.BaseSchema, got.Model.BaseTable)
	}
	if !hasDimension(got.Model.Dimensions, "order_date", "orders.order_date") {
		t.Fatalf("missing order_date dimension: %#v", got.Model.Dimensions)
	}
	if !hasDimensionWithGrain(got.Model.Dimensions, "order_date_year", "orders.order_date", "year") {
		t.Fatalf("missing order_date_year grain dimension: %#v", got.Model.Dimensions)
	}
	if !hasDimensionWithGrain(got.Model.Dimensions, "order_date_month", "orders.order_date", "month") {
		t.Fatalf("missing order_date_month grain dimension: %#v", got.Model.Dimensions)
	}
	if !hasDimension(got.Model.Dimensions, "customers_name", "customers.name") {
		t.Fatalf("missing related customer name dimension: %#v", got.Model.Dimensions)
	}
	if !hasMetric(got.Model.Metrics, "count", "*", string(semantic.AggCount)) {
		t.Fatalf("missing count metric: %#v", got.Model.Metrics)
	}
	if !hasMetric(got.Model.Metrics, "sum_total_amount", "orders.total_amount", string(semantic.AggSum)) {
		t.Fatalf("missing total amount metric: %#v", got.Model.Metrics)
	}
	if len(got.Model.Joins) != 1 {
		t.Fatalf("joins len = %d, want 1", len(got.Model.Joins))
	}
}

func hasDimension(dimensions []semantic.Dimension, name, ref string) bool {
	for _, dim := range dimensions {
		if dim.Name == name && dim.ColumnRef == ref {
			return true
		}
	}
	return false
}

func hasDimensionWithGrain(dimensions []semantic.Dimension, name, ref, grain string) bool {
	for _, dim := range dimensions {
		if dim.Name == name && dim.ColumnRef == ref && dim.TimeGrain == grain {
			return true
		}
	}
	return false
}

func hasMetric(metrics []semantic.Metric, name, expr, aggregation string) bool {
	for _, metric := range metrics {
		if metric.Name == name && metric.Expression == expr && metric.Aggregation == aggregation {
			return true
		}
	}
	return false
}
