package dbt

import (
	"testing"

	"github.com/biqly/biqly/internal/semantic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sampleManifest = `{
  "metadata": {"dbt_version": "1.7.0", "project_name": "jaffle"},
  "nodes": {
    "model.jaffle.orders": {
      "unique_id": "model.jaffle.orders",
      "resource_type": "model",
      "name": "orders",
      "package_name": "jaffle",
      "schema": "analytics",
      "alias": "orders",
      "description": "Order facts",
      "columns": {
        "order_id": {"name": "order_id", "description": "PK", "data_type": "integer"},
        "customer_id": {"name": "customer_id", "data_type": "integer"},
        "amount": {"name": "amount", "description": "Order total", "data_type": "numeric"},
        "status": {"name": "status", "data_type": "text"}
      },
      "config": {"enabled": true},
      "tags": ["finance"]
    },
    "model.jaffle.customers": {
      "unique_id": "model.jaffle.customers",
      "resource_type": "model",
      "name": "customers",
      "schema": "analytics",
      "alias": "customers",
      "columns": {
        "customer_id": {"name": "customer_id", "data_type": "integer"},
        "customer_name": {"name": "customer_name", "data_type": "character varying"}
      },
      "config": {}
    },
    "test.jaffle.not_null_orders_order_id": {
      "unique_id": "test.jaffle.not_null_orders_order_id",
      "resource_type": "test",
      "name": "not_null_orders_order_id",
      "attached_node": "model.jaffle.orders",
      "column_name": "order_id",
      "test_metadata": {"name": "not_null", "kwargs": {}}
    },
    "test.jaffle.unique_orders_order_id": {
      "unique_id": "test.jaffle.unique_orders_order_id",
      "resource_type": "test",
      "attached_node": "model.jaffle.orders",
      "column_name": "order_id",
      "test_metadata": {"name": "unique", "kwargs": {}}
    },
    "test.jaffle.accepted_values_orders_status": {
      "unique_id": "test.jaffle.accepted_values_orders_status",
      "resource_type": "test",
      "attached_node": "model.jaffle.orders",
      "column_name": "status",
      "test_metadata": {
        "name": "accepted_values",
        "kwargs": {"values": ["placed", "shipped", "completed"]}
      }
    },
    "test.jaffle.relationships_orders_customer_id": {
      "unique_id": "test.jaffle.relationships_orders_customer_id",
      "resource_type": "test",
      "attached_node": "model.jaffle.orders",
      "column_name": "customer_id",
      "test_metadata": {
        "name": "relationships",
        "kwargs": {"to": "ref('customers')", "field": "customer_id"}
      }
    },
    "model.jaffle.disabled_thing": {
      "unique_id": "model.jaffle.disabled_thing",
      "resource_type": "model",
      "name": "disabled_thing",
      "schema": "analytics",
      "columns": {"id": {"name": "id"}},
      "config": {"enabled": false}
    }
  },
  "sources": {
    "source.jaffle.raw.payments": {
      "unique_id": "source.jaffle.raw.payments",
      "name": "payments",
      "source_name": "raw",
      "schema": "raw",
      "identifier": "payments",
      "columns": {
        "id": {"name": "id", "data_type": "integer"}
      }
    }
  }
}`

const sampleCatalog = `{
  "nodes": {
    "model.jaffle.orders": {
      "unique_id": "model.jaffle.orders",
      "metadata": {"type": "BASE TABLE", "schema": "analytics", "name": "orders"},
      "columns": {
        "order_id": {"type": "integer", "index": 1, "name": "order_id"},
        "customer_id": {"type": "integer", "index": 2, "name": "customer_id"},
        "amount": {"type": "numeric", "index": 3, "name": "amount"},
        "status": {"type": "text", "index": 4, "name": "status"},
        "created_at": {"type": "timestamp without time zone", "index": 5, "name": "created_at"}
      }
    },
    "model.jaffle.customers": {
      "unique_id": "model.jaffle.customers",
      "metadata": {"schema": "analytics", "name": "customers"},
      "columns": {
        "customer_id": {"type": "integer", "index": 1, "name": "customer_id"},
        "customer_name": {"type": "character varying", "index": 2, "name": "customer_name"}
      }
    }
  },
  "sources": {}
}`

func TestParseProject_ExtractsModelsAndTests(t *testing.T) {
	project, err := ParseProject([]byte(sampleManifest), []byte(sampleCatalog))
	require.NoError(t, err)
	assert.Equal(t, "1.7.0", project.DbtVersion)
	assert.Equal(t, "jaffle", project.ProjectName)
	require.Len(t, project.Models, 2) // disabled skipped
	assert.True(t, project.NotNullColumns["model.jaffle.orders"]["order_id"])
	assert.True(t, project.UniqueColumns["model.jaffle.orders"]["order_id"])
	require.Len(t, project.AcceptedValues, 1)
	assert.Equal(t, []string{"placed", "shipped", "completed"}, project.AcceptedValues[0].Values)
	require.Len(t, project.Relationships, 1)
	assert.Equal(t, "name:customers", project.Relationships[0].ToUniqueID)

	// catalog-only column created_at should appear on orders
	var orders Node
	for _, m := range project.Models {
		if m.Name == "orders" {
			orders = m
			break
		}
	}
	require.Contains(t, orders.Columns, "created_at")
	assert.Contains(t, orders.Columns["created_at"].DataType, "timestamp")
}

func TestParseManifest_Empty(t *testing.T) {
	_, err := ParseManifest(nil)
	require.Error(t, err)
}

func TestConvertProject_BuildsDraftModels(t *testing.T) {
	project, err := ParseProject([]byte(sampleManifest), []byte(sampleCatalog))
	require.NoError(t, err)

	result := ConvertProject(project, ConvertOptions{DatasourceID: "ds-1"})
	require.Len(t, result.Models, 2)

	orders := findModel(t, result.Models, "orders")
	assert.Equal(t, "draft", orders.Status)
	assert.Equal(t, "ds-1", orders.DatasourceID)
	assert.Equal(t, "analytics", orders.BaseSchema)
	assert.Equal(t, "orders", orders.BaseTable)
	require.NotNil(t, orders.Description)
	assert.Equal(t, "Order facts", *orders.Description)

	assertOrdersDimensions(t, orders)
	assertOrdersMetrics(t, orders)
	require.NotEmpty(t, orders.Joins)
	assert.Equal(t, "customers", orders.Joins[0].ToTable)
	assert.Equal(t, "customer_id", orders.Joins[0].FromColumn)

	customers := findModel(t, result.Models, "customers")
	for _, d := range customers.Dimensions {
		if d.Name == "customer_name" {
			assert.True(t, d.IsDisplay)
		}
	}
}

func findModel(t *testing.T, models []*semantic.SemanticModel, name string) *semantic.SemanticModel {
	t.Helper()
	for _, m := range models {
		if m.Name == name {
			return m
		}
	}
	t.Fatalf("model %q not found", name)
	return nil
}

func assertOrdersDimensions(t *testing.T, orders *semantic.SemanticModel) {
	t.Helper()
	dimByName := map[string]semantic.Dimension{}
	for _, d := range orders.Dimensions {
		assert.NotEmpty(t, d.ID)
		assert.Equal(t, orders.ID, d.ModelID)
		dimByName[d.Name] = d
	}
	require.Contains(t, dimByName, "amount")
	require.Contains(t, dimByName, "created_at")
	assert.Equal(t, "number", dimByName["amount"].Type)
	assert.Equal(t, "date", dimByName["created_at"].Type)
	assert.Contains(t, dimByName["order_id"].Synonyms, "primary_key")
	require.Len(t, dimByName["status"].EnumValues, 3)
	assert.Equal(t, "placed", dimByName["status"].EnumValues[0].RawValue)
}

func assertOrdersMetrics(t *testing.T, orders *semantic.SemanticModel) {
	t.Helper()
	metricNames := map[string]bool{}
	for _, met := range orders.Metrics {
		metricNames[met.Name] = true
	}
	assert.True(t, metricNames["sum_amount"])
	assert.True(t, metricNames["avg_amount"])
	assert.True(t, metricNames["count_rows"])
}

func TestConvertProject_IncludeSourcesAndNameDedup(t *testing.T) {
	project, err := ParseProject([]byte(sampleManifest), []byte(sampleCatalog))
	require.NoError(t, err)

	result := ConvertProject(project, ConvertOptions{
		DatasourceID:   "ds-1",
		ExistingNames:  []string{"orders"},
		IncludeSources: true,
	})
	names := map[string]bool{}
	for _, m := range result.Models {
		names[m.Name] = true
	}
	assert.True(t, names["orders_2"], "existing name should force suffix")
	assert.True(t, names["raw_payments"])
}

func TestConvertProject_MissingDatasource(t *testing.T) {
	project, err := ParseProject([]byte(sampleManifest), []byte("{}"))
	require.NoError(t, err)
	result := ConvertProject(project, ConvertOptions{})
	assert.Empty(t, result.Models)
	assert.NotEmpty(t, result.Warnings)
}
