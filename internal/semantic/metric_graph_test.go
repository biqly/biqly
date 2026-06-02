package semantic

import (
	"testing"
)

func TestBuildMetricGraph_DetectsDerivesFrom(t *testing.T) {
	resolved := &SemanticModel{
		Metrics: []Metric{
			{Name: "revenue", Expression: "public.orders.amount", Aggregation: "sum"},
			{Name: "cost", Expression: "public.orders.cost", Aggregation: "sum"},
			{Name: "profit", Expression: "revenue - cost", Aggregation: "sum"},
		},
	}
	composite := &CompositeModel{
		Components: []ComponentModelRef{{Alias: "ord", Role: ComponentRolePrimary}},
	}

	graph := BuildMetricGraph(composite, resolved)

	profit := graph.Nodes["profit"]
	deps := map[string]bool{}
	for _, d := range profit.DependsOn {
		deps[d] = true
	}
	if !deps["revenue"] || !deps["cost"] {
		t.Fatalf("profit should depend on revenue and cost, got %v", profit.DependsOn)
	}

	var derives int
	for _, e := range graph.Edges {
		if e.From == "profit" && e.Type == MetricEdgeDerivesFrom {
			derives++
		}
	}
	if derives != 2 {
		t.Fatalf("expected 2 derives_from edges from profit, got %d", derives)
	}
}

func TestDetectCircularDependencies_NoCycle(t *testing.T) {
	resolved := &SemanticModel{
		Metrics: []Metric{
			{Name: "revenue", Expression: "public.orders.amount"},
			{Name: "profit", Expression: "revenue - 10"},
		},
	}
	graph := BuildMetricGraph(&CompositeModel{}, resolved)
	if err := DetectCircularDependencies(graph); err != nil {
		t.Fatalf("expected no cycle, got %v", err)
	}
}

func TestDetectCircularDependencies_FindsCycle(t *testing.T) {
	resolved := &SemanticModel{
		Metrics: []Metric{
			{Name: "alpha", Expression: "beta + 1"},
			{Name: "beta", Expression: "alpha + 1"},
		},
	}
	graph := BuildMetricGraph(&CompositeModel{}, resolved)
	if err := DetectCircularDependencies(graph); err == nil {
		t.Fatal("expected circular dependency error, got nil")
	}
}

func TestBuildMetricGraph_CrossModelDependency(t *testing.T) {
	// A metric from the secondary component referenced by a metric whose
	// expression derives from it still produces a derives_from edge across
	// component boundaries.
	resolved := &SemanticModel{
		Metrics: []Metric{
			{Name: "order_total", Expression: "public.orders.amount", Aggregation: "sum"},
			{Name: "customer_count", Expression: "public.customers.id", Aggregation: "count_distinct"},
			{Name: "avg_order_per_customer", Expression: "order_total / customer_count"},
		},
	}
	composite := &CompositeModel{
		Components: []ComponentModelRef{
			{Alias: "ord", Role: ComponentRolePrimary},
			{Alias: "cust", Role: ComponentRoleSecondary},
		},
	}
	graph := BuildMetricGraph(composite, resolved)

	node := graph.Nodes["avg_order_per_customer"]
	deps := map[string]bool{}
	for _, d := range node.DependsOn {
		deps[d] = true
	}
	if !deps["order_total"] || !deps["customer_count"] {
		t.Fatalf("avg metric should depend on both component metrics, got %v", node.DependsOn)
	}
}
