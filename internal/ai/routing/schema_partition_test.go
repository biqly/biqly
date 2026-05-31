package routing

import (
	"context"
	"strings"
	"testing"

	"github.com/biqly/biqly/internal/metadata"
)

func TestFilterTablesBySchemaCluster_PrefersMatchingSchema(t *testing.T) {
	tables := []metadata.Table{
		{DatasourceID: "ds1", SchemaName: "sales", TableName: "orders", TableType: "BASE TABLE"},
		{DatasourceID: "ds1", SchemaName: "sales", TableName: "customers", TableType: "BASE TABLE"},
		{DatasourceID: "ds1", SchemaName: "hr", TableName: "employees", TableType: "BASE TABLE"},
		{DatasourceID: "ds1", SchemaName: "hr", TableName: "departments", TableType: "BASE TABLE"},
	}
	columnsByTable := map[string][]metadata.Column{
		"sales.orders": {
			{SchemaName: "sales", TableName: "orders", ColumnName: "order_id", DataType: "int", IsPrimaryKey: true},
			{SchemaName: "sales", TableName: "orders", ColumnName: "total_amount", DataType: "numeric"},
			{SchemaName: "sales", TableName: "orders", ColumnName: "order_date", DataType: "date"},
		},
		"sales.customers": {
			{SchemaName: "sales", TableName: "customers", ColumnName: "customer_id", DataType: "int", IsPrimaryKey: true},
			{SchemaName: "sales", TableName: "customers", ColumnName: "name", DataType: "text"},
		},
		"hr.employees": {
			{SchemaName: "hr", TableName: "employees", ColumnName: "employee_id", DataType: "int", IsPrimaryKey: true},
			{SchemaName: "hr", TableName: "employees", ColumnName: "salary", DataType: "numeric"},
		},
		"hr.departments": {
			{SchemaName: "hr", TableName: "departments", ColumnName: "department_id", DataType: "int", IsPrimaryKey: true},
			{SchemaName: "hr", TableName: "departments", ColumnName: "department_name", DataType: "text"},
		},
	}

	filtered, active := filterTablesBySchemaCluster(tables, columnsByTable, nil, "monthly order revenue by customer", nil)
	if len(active) == 0 || active[0] != "sales" {
		t.Fatalf("active schemas = %v, want sales first", active)
	}
	for _, tbl := range filtered {
		if strings.HasPrefix(tableLabel(tbl), "hr.") {
			t.Fatalf("filtered tables should exclude hr schema; got %s", tableLabel(tbl))
		}
	}
}

func TestExpandSchemaPartitionWithFK_IncludesLinkedSchema(t *testing.T) {
	active := map[string]bool{"sales": true}
	relations := []metadata.Relation{
		{
			FromSchema: "sales",
			FromTable:  "customer",
			ToSchema:   "person",
			ToTable:    "person",
		},
	}
	expandSchemaPartitionWithFK(active, relations)
	if !active["person"] {
		t.Fatalf("FK expansion should include person schema; active=%v", active)
	}
}

func TestTableRouter_SchemaPartitionAutoRouting(t *testing.T) {
	reader := testMetadataReader()
	reader.tables = append(reader.tables,
		metadata.Table{DatasourceID: "ds1", SchemaName: "hr", TableName: "employees", TableType: "BASE TABLE"},
	)
	reader.columns = append(reader.columns,
		metadata.Column{DatasourceID: "ds1", SchemaName: "hr", TableName: "employees", ColumnName: "employee_id", DataType: "uuid", IsPrimaryKey: true},
		metadata.Column{DatasourceID: "ds1", SchemaName: "hr", TableName: "employees", ColumnName: "salary", DataType: "numeric"},
	)

	router := NewTableRouter(reader)
	_, routing, err := router.Route(context.Background(), "ds1", "total order revenue by customer name", nil, true, true)
	if err != nil {
		t.Fatalf("Route() error = %v", err)
	}
	if routing.Debug == nil || len(routing.Debug.SchemaPartitions) == 0 {
		t.Fatalf("expected schema_partitions debug, got %+v", routing.Debug)
	}
	if routing.Debug.SchemaPartitions[0] != "public" {
		t.Fatalf("schema_partitions = %v, want public first", routing.Debug.SchemaPartitions)
	}
	for _, label := range routing.SelectedTables {
		if strings.HasPrefix(label, "hr.") {
			t.Fatalf("auto routing should not select hr tables for order question; selected=%v", routing.SelectedTables)
		}
	}
}
