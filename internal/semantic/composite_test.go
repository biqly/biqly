package semantic

import (
	"testing"
)

// twoComponentFixture builds a primary "orders" model and a secondary
// "customers" model plus a composite that joins them on customer_id.
func twoComponentFixture() (*CompositeModel, map[string]*SemanticModel) {
	orders := &SemanticModel{
		ID:         "m-orders",
		Name:       "orders",
		BaseSchema: "public",
		BaseTable:  "orders",
		Status:     ModelStatusPublished,
		Dimensions: []Dimension{
			{Name: "order_id", ColumnRef: "id", Type: "number"},
			{Name: "customer_id", ColumnRef: "customer_id", Type: "number"},
			{Name: "order_date", ColumnRef: "created_at", Type: "date"},
			{Name: "region", ColumnRef: "region", Type: "text"},
		},
		Metrics: []Metric{
			{Name: "total_amount", Expression: "amount", Aggregation: "sum"},
		},
	}
	customers := &SemanticModel{
		ID:         "m-customers",
		Name:       "customers",
		BaseSchema: "public",
		BaseTable:  "customers",
		Status:     ModelStatusPublished,
		Dimensions: []Dimension{
			{Name: "id", ColumnRef: "id", Type: "number"},
			{Name: "region", ColumnRef: "region", Type: "text"},
			{Name: "signup_date", ColumnRef: "signup_at", Type: "date"},
		},
		Metrics: []Metric{
			{Name: "customer_count", Expression: "id", Aggregation: "count_distinct"},
		},
	}

	composite := &CompositeModel{
		ID:           "c-1",
		DatasourceID: "ds-1",
		Name:         "sales_with_customers",
		Status:       ModelStatusDraft,
		Components: []ComponentModelRef{
			{Alias: "ord", ModelID: "m-orders", Role: ComponentRolePrimary},
			{Alias: "cust", ModelID: "m-customers", Role: ComponentRoleSecondary},
		},
		CrossModelJoins: []CrossModelJoin{
			{
				Name:          "orders_customers",
				FromModel:     "ord",
				FromDimension: "customer_id",
				ToModel:       "cust",
				ToDimension:   "id",
				JoinType:      "LEFT",
				Relationship:  RelationshipManyToOne,
				IsActive:      true,
			},
		},
		CanonicalDate: &CanonicalDateRef{ModelAlias: "ord", DimensionName: "order_date"},
	}

	return composite, map[string]*SemanticModel{"ord": orders, "cust": customers}
}

func TestCompositeResolver_MergesTwoModels(t *testing.T) {
	composite, components := twoComponentFixture()
	resolved, err := NewCompositeResolver().Resolve(composite, components)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if resolved.BaseTable != "orders" || resolved.BaseSchema != "public" {
		t.Fatalf("base table = %s.%s, want public.orders", resolved.BaseSchema, resolved.BaseTable)
	}

	// Column refs must be fully qualified to schema.table.column.
	for _, d := range resolved.Dimensions {
		if d.Name == "order_id" && d.ColumnRef != "public.orders.id" {
			t.Fatalf("order_id ColumnRef = %q, want public.orders.id", d.ColumnRef)
		}
		if d.Name == "id" && d.ColumnRef != "public.customers.id" {
			t.Fatalf("customers id ColumnRef = %q, want public.customers.id", d.ColumnRef)
		}
	}

	// Both metrics present.
	metricNames := map[string]bool{}
	for _, m := range resolved.Metrics {
		metricNames[m.Name] = true
	}
	if !metricNames["total_amount"] || !metricNames["customer_count"] {
		t.Fatalf("missing merged metrics: %v", metricNames)
	}
}

func TestCompositeResolver_ResolvesDuplicateDimensionByAliasPrefix(t *testing.T) {
	composite, components := twoComponentFixture()
	resolved, err := NewCompositeResolver().Resolve(composite, components)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	// "region" exists in both components. Primary keeps "region"; secondary
	// is disambiguated by alias prefix.
	names := map[string]int{}
	for _, d := range resolved.Dimensions {
		names[d.Name]++
	}
	if names["region"] != 1 {
		t.Fatalf("expected single primary 'region', got %d", names["region"])
	}
	if names["cust_region"] != 1 {
		t.Fatalf("expected disambiguated 'cust_region', got %d", names["cust_region"])
	}
}

func TestCompositeResolver_UsePrimaryConflictResolutionDropsSecondary(t *testing.T) {
	composite, components := twoComponentFixture()
	composite.ConflictResolutions = []DimensionConflictResolution{
		{DimensionName: "region", Resolution: ConflictResolutionUsePrimary},
	}
	resolved, err := NewCompositeResolver().Resolve(composite, components)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	for _, d := range resolved.Dimensions {
		if d.Name == "cust_region" {
			t.Fatal("use_primary should drop secondary region, but cust_region present")
		}
	}
}

func TestCompositeResolver_FlattensCrossModelJoin(t *testing.T) {
	composite, components := twoComponentFixture()
	resolved, err := NewCompositeResolver().Resolve(composite, components)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	var found bool
	for _, j := range resolved.Joins {
		if j.FromTable == "orders" && j.FromColumn == "customer_id" &&
			j.ToTable == "customers" && j.ToColumn == "id" {
			found = true
			if j.ToSchema != "public" {
				t.Fatalf("cross join ToSchema = %q, want public", j.ToSchema)
			}
		}
	}
	if !found {
		t.Fatalf("cross-model join orders->customers not flattened: %+v", resolved.Joins)
	}
}

func TestCompositeResolver_CanonicalDateDimensionPresent(t *testing.T) {
	composite, components := twoComponentFixture()
	resolved, err := NewCompositeResolver().Resolve(composite, components)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	var has bool
	for _, d := range resolved.Dimensions {
		if d.Name == "order_date" {
			has = true
		}
	}
	if !has {
		t.Fatal("canonical date dimension order_date missing from merged model")
	}
}

func TestCompositeResolver_NilAndEmptyErrors(t *testing.T) {
	r := NewCompositeResolver()
	if _, err := r.Resolve(nil, nil); err == nil {
		t.Fatal("expected error for nil composite")
	}
	if _, err := r.Resolve(&CompositeModel{Name: "x"}, nil); err == nil {
		t.Fatal("expected error for composite with no components")
	}
}
