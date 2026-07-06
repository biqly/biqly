package handlers

import (
	"testing"

	pkgsemantic "github.com/biqly/biqly/pkg/semantic"
)

func TestMetricFromRequestRateBehavior(t *testing.T) {
	req := createMetricRequest{
		Name:         "conversion_rate",
		Expression:   "orders.rate",
		Aggregation:  "avg",
		RateBehavior: pkgsemantic.RateBehaviorRatioOfSums,
	}
	m, err := metricFromRequest("id-1", "model-1", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.RateBehavior != pkgsemantic.RateBehaviorRatioOfSums {
		t.Fatalf("expected rate behavior to be threaded, got %q", m.RateBehavior)
	}

	req.RateBehavior = "bogus"
	if _, err := metricFromRequest("id-1", "model-1", req); err == nil {
		t.Fatal("expected invalid rate_behavior to be rejected")
	}
}
