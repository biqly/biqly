package semanticgen

import (
	"strings"
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

func TestAppendMissingDimensions(t *testing.T) {
	model := &semantic.SemanticModel{
		ID:           "m1",
		DatasourceID: "ds1",
		BaseSchema:   "sales",
		BaseTable:    "orders",
		Dimensions: []semantic.Dimension{
			{ID: "d1", ModelID: "m1", Name: "order_date", ColumnRef: "orders.order_date", Type: "date"},
		},
		Joins: []semantic.Join{
			{FromSchema: "sales", FromTable: "orders", FromColumn: "screen", ToSchema: "public", ToTable: "timeline", ToColumn: "screen_name"},
		},
	}
	columns := []metadata.Column{
		{DatasourceID: "ds1", SchemaName: "sales", TableName: "orders", ColumnName: "order_date", DataType: "timestamp"},
		{DatasourceID: "ds1", SchemaName: "public", TableName: "timeline", ColumnName: "screen_name", DataType: "varchar"},
		{DatasourceID: "ds1", SchemaName: "public", TableName: "timeline", ColumnName: "author_name", DataType: "varchar"},
		{DatasourceID: "ds1", SchemaName: "warehouse", TableName: "unrelated", ColumnName: "x", DataType: "varchar"},
	}
	opts := GenerateModelOptions{DatasourceID: "ds1", BaseSchema: "sales", BaseTable: "orders"}

	added := AppendMissingDimensions(model, columns, opts)

	if !containsRef(added, "timeline") {
		t.Fatalf("expected dimensions for the joined timeline table: %#v", added)
	}
	for _, dim := range added {
		if dim.ColumnRef == "orders.order_date" && dim.TimeGrain == "" {
			t.Fatalf("re-added an existing dimension (order_date)")
		}
		if strings.Contains(dim.ColumnRef, "unrelated") {
			t.Fatalf("added a dimension for a table not in the model: %s", dim.ColumnRef)
		}
	}

	// Idempotent: once persisted, a re-run adds nothing.
	model.Dimensions = append(model.Dimensions, added...)
	if again := AppendMissingDimensions(model, columns, opts); len(again) != 0 {
		t.Fatalf("AppendMissingDimensions not idempotent: got %d new dims", len(again))
	}
}

func containsRef(dimensions []semantic.Dimension, substr string) bool {
	for _, dim := range dimensions {
		if strings.Contains(dim.ColumnRef, substr) {
			return true
		}
	}
	return false
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
