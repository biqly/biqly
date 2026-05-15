package semantic_test

import (
	"context"
	"testing"

	"github.com/biqly/biqly/internal/semantic"
)

type fakeSemanticCatalog struct {
	columns   []semantic.CatalogColumn
	relations []semantic.CatalogRelation
	policies  []semantic.CatalogPolicy
}

func (f fakeSemanticCatalog) ListSemanticColumns(context.Context, string) ([]semantic.CatalogColumn, error) {
	return f.columns, nil
}

func (f fakeSemanticCatalog) ListSemanticRelations(context.Context, string) ([]semantic.CatalogRelation, error) {
	return f.relations, nil
}

func (f fakeSemanticCatalog) ListSemanticPolicies(context.Context, string) ([]semantic.CatalogPolicy, error) {
	return f.policies, nil
}

func TestValidateContextRejectsMetricUnknownColumn(t *testing.T) {
	model := validPublishModel()
	model.Metrics = append(model.Metrics, semantic.Metric{
		Name:        "bad_revenue",
		Expression:  "orders.missing_amount",
		Aggregation: string(semantic.AggSum),
		IsActive:    true,
	})

	result := semantic.ValidateContext(context.Background(), model, validPublishCatalog())
	if result.Valid {
		t.Fatal("ValidateContext() valid = true, want false")
	}
	if !result.HasError("metric expression references unknown column: orders.missing_amount") {
		t.Fatalf("ValidateContext() errors = %v, want unknown metric column error", result.Errors)
	}
}

func TestValidateContextRejectsJoinNotInMetadata(t *testing.T) {
	model := validPublishModel()
	model.Joins = append(model.Joins, semantic.Join{
		Name:         "orders_to_products",
		FromTable:    "orders",
		FromColumn:   "product_id",
		ToTable:      "products",
		ToColumn:     "id",
		JoinType:     "LEFT",
		Relationship: "many_to_one",
		IsActive:     true,
	})

	result := semantic.ValidateContext(context.Background(), model, validPublishCatalog())
	if result.Valid {
		t.Fatal("ValidateContext() valid = true, want false (unknown columns)")
	}
	if !result.HasWarning("join does not match datasource metadata relation: orders_to_products") {
		t.Fatalf("ValidateContext() warnings = %v, want metadata relation warning", result.Warnings)
	}
}

func TestValidateContextWarnsOnFanoutRelationship(t *testing.T) {
	model := validPublishModel()
	model.Joins[0].Relationship = "one_to_many"

	result := semantic.ValidateContext(context.Background(), model, validPublishCatalog())
	if !result.Valid {
		t.Fatalf("ValidateContext() valid = false, errors = %v", result.Errors)
	}
	if !result.HasWarning("join can fan out aggregations: orders_customers uses one_to_many") {
		t.Fatalf("ValidateContext() warnings = %v, want fanout warning", result.Warnings)
	}
}

func TestValidateContextRejectsPermissionUnknownField(t *testing.T) {
	catalog := validPublishCatalog()
	catalog.policies = []semantic.CatalogPolicy{{
		DeniedFields: []string{"orders.secret_margin"},
		RowFilters:   []semantic.CatalogRowFilter{{Field: "missing_region"}},
	}}

	result := semantic.ValidateContext(context.Background(), validPublishModel(), catalog)
	if result.Valid {
		t.Fatal("ValidateContext() valid = true, want false")
	}
	if !result.HasError("permission policy references unknown field: orders.secret_margin") {
		t.Fatalf("ValidateContext() errors = %v, want denied field error", result.Errors)
	}
	if !result.HasError("permission row filter references unknown field: missing_region") {
		t.Fatalf("ValidateContext() errors = %v, want row filter error", result.Errors)
	}
}

func validPublishModel() semantic.SemanticModel {
	return semantic.SemanticModel{
		ID:           "model-1",
		DatasourceID: "ds1",
		Name:         "orders",
		BaseSchema:   "public",
		BaseTable:    "orders",
		IsActive:     true,
		Dimensions: []semantic.Dimension{
			{Name: "customer_name", ColumnRef: "customers.name", Type: "text", IsActive: true},
		},
		Metrics: []semantic.Metric{
			{Name: "revenue", Expression: "orders.total_amount", Aggregation: string(semantic.AggSum), IsActive: true},
		},
		Joins: []semantic.Join{
			{
				Name:         "orders_customers",
				FromTable:    "orders",
				FromColumn:   "customer_id",
				ToTable:      "customers",
				ToColumn:     "id",
				JoinType:     "LEFT",
				Relationship: "many_to_one",
				IsActive:     true,
			},
		},
	}
}

func validPublishCatalog() fakeSemanticCatalog {
	return fakeSemanticCatalog{
		columns: []semantic.CatalogColumn{
			{SchemaName: "public", TableName: "orders", ColumnName: "id"},
			{SchemaName: "public", TableName: "orders", ColumnName: "customer_id"},
			{SchemaName: "public", TableName: "orders", ColumnName: "total_amount"},
			{SchemaName: "public", TableName: "customers", ColumnName: "id"},
			{SchemaName: "public", TableName: "customers", ColumnName: "name"},
		},
		relations: []semantic.CatalogRelation{
			{
				FromSchema: "public",
				FromTable:  "orders",
				FromColumn: "customer_id",
				ToSchema:   "public",
				ToTable:    "customers",
				ToColumn:   "id",
			},
		},
	}
}
