package semantic

import (
	"context"
	"strings"
	"testing"
)

// stubComponentProvider returns pre-seeded published models by ID.
type stubComponentProvider struct {
	models map[string]*SemanticModel
}

func (s stubComponentProvider) GetPublishedFullModel(_ context.Context, id string) (*SemanticModel, error) {
	m, ok := s.models[id]
	if !ok {
		return nil, context.Canceled
	}
	return m, nil
}

func fanoutProvider() stubComponentProvider {
	return stubComponentProvider{models: map[string]*SemanticModel{
		"m-orders": {
			ID: "m-orders", Name: "orders", BaseSchema: "public", BaseTable: "orders",
			Status: ModelStatusPublished,
			Dimensions: []Dimension{
				{Name: "customer_id", ColumnRef: "customer_id", Type: "number"},
				{Name: "order_date", ColumnRef: "created_at", Type: "date"},
			},
			Metrics: []Metric{{Name: "total_amount", Expression: "amount", Aggregation: "sum"}},
		},
		"m-items": {
			ID: "m-items", Name: "items", BaseSchema: "public", BaseTable: "order_items",
			Status: ModelStatusPublished,
			Dimensions: []Dimension{
				{Name: "order_id", ColumnRef: "order_id", Type: "number"},
				{Name: "sku", ColumnRef: "sku", Type: "text"},
			},
			Metrics: []Metric{{Name: "qty", Expression: "quantity", Aggregation: "sum"}},
		},
	}}
}

func baseFanoutComposite() *CompositeModel {
	return &CompositeModel{
		ID: "c-fan", DatasourceID: "ds-1", Name: "orders_items", Status: ModelStatusDraft,
		Components: []ComponentModelRef{
			{Alias: "ord", ModelID: "m-orders", Role: ComponentRolePrimary},
			{Alias: "itm", ModelID: "m-items", Role: ComponentRoleSecondary},
		},
		CanonicalDate: &CanonicalDateRef{ModelAlias: "ord", DimensionName: "order_date"},
	}
}

func hasWarningContaining(warnings []string, substr string) bool {
	for _, w := range warnings {
		if strings.Contains(w, substr) {
			return true
		}
	}
	return false
}

func TestValidateComposite_LowRiskNoFanoutWarning(t *testing.T) {
	composite := baseFanoutComposite()
	composite.CrossModelJoins = []CrossModelJoin{{
		Name: "j1", FromModel: "itm", FromDimension: "order_id",
		ToModel: "ord", ToDimension: "customer_id",
		Relationship: RelationshipManyToOne, IsActive: true,
	}}

	_, result := ValidateComposite(context.Background(), composite, fanoutProvider())
	if !result.Valid {
		t.Fatalf("expected valid composite, errors: %v", result.Errors)
	}
	if hasWarningContaining(result.Warnings, "fan out") || hasWarningContaining(result.Warnings, "fanout") {
		t.Fatalf("many_to_one should not warn about fanout, got %v", result.Warnings)
	}
}

func TestValidateComposite_OneToManyWarnsFanout(t *testing.T) {
	composite := baseFanoutComposite()
	composite.CrossModelJoins = []CrossModelJoin{{
		Name: "j1", FromModel: "ord", FromDimension: "customer_id",
		ToModel: "itm", ToDimension: "order_id",
		Relationship: RelationshipOneToMany, IsActive: true,
	}}

	_, result := ValidateComposite(context.Background(), composite, fanoutProvider())
	if !result.Valid {
		t.Fatalf("expected valid composite, errors: %v", result.Errors)
	}
	if !hasWarningContaining(result.Warnings, "one_to_many") {
		t.Fatalf("expected one_to_many fanout warning, got %v", result.Warnings)
	}
}

func TestValidateComposite_ManyToManyWarnsCritical(t *testing.T) {
	composite := baseFanoutComposite()
	composite.CrossModelJoins = []CrossModelJoin{{
		Name: "j1", FromModel: "ord", FromDimension: "customer_id",
		ToModel: "itm", ToDimension: "order_id",
		Relationship: RelationshipManyToMany, IsActive: true,
	}}

	_, result := ValidateComposite(context.Background(), composite, fanoutProvider())
	if !result.Valid {
		t.Fatalf("expected valid composite, errors: %v", result.Errors)
	}
	if !hasWarningContaining(result.Warnings, "many_to_many") {
		t.Fatalf("expected many_to_many double-count warning, got %v", result.Warnings)
	}
}

func TestEnforceLimits(t *testing.T) {
	composite := baseFanoutComposite()
	composite.CrossModelJoins = []CrossModelJoin{
		{Name: "j1", IsActive: true},
		{Name: "j2", IsActive: true},
		{Name: "j3", IsActive: false},
	}
	resolved := &SemanticModel{
		Dimensions: []Dimension{{Name: "a"}, {Name: "b"}, {Name: "c"}},
		Metrics:    []Metric{{Name: "m1"}, {Name: "m2"}},
	}

	t.Run("within limits passes", func(t *testing.T) {
		r := (&CompositeRepository{}).WithLimits(CompositeLimits{
			MaxComponents: 2, MaxCrossJoins: 2, MaxMergedFields: 5,
		})
		if errs := r.enforceLimits(composite, resolved); len(errs) != 0 {
			t.Fatalf("expected no limit errors, got %v", errs)
		}
	})

	t.Run("zero disables limits", func(t *testing.T) {
		r := &CompositeRepository{}
		if errs := r.enforceLimits(composite, resolved); len(errs) != 0 {
			t.Fatalf("expected no limit errors with zero limits, got %v", errs)
		}
	})

	t.Run("exceeding each limit reports an error", func(t *testing.T) {
		r := (&CompositeRepository{}).WithLimits(CompositeLimits{
			MaxComponents: 1, MaxCrossJoins: 1, MaxMergedFields: 4,
		})
		errs := r.enforceLimits(composite, resolved)
		if len(errs) != 3 {
			t.Fatalf("expected 3 limit errors, got %d: %v", len(errs), errs)
		}
	})

	t.Run("nil resolved skips merged-field check", func(t *testing.T) {
		r := (&CompositeRepository{}).WithLimits(CompositeLimits{MaxMergedFields: 1})
		if errs := r.enforceLimits(composite, nil); len(errs) != 0 {
			t.Fatalf("expected no merged-field error when resolved is nil, got %v", errs)
		}
	})
}
