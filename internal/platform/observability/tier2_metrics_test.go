package observability

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestAgentMetricsRecord(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)

	m.RecordAgentRunTerminal("completed")
	m.RecordAgentRunTerminal("failed")
	m.RecordAgentTerminalFailure("max_steps_exceeded")
	m.RecordAgentStepDuration("query.execute", 250*time.Millisecond)
	m.RecordAgentPolicyDenial("hidden_column_denied")
	m.RecordAgentClarificationRound(2)
	m.RecordAgentShadowComparison("result_mismatch")
	m.RecordAgentQueueRedelivery()
	m.RecordAgentPlannerTokens(120, 40)

	if got := testutil.ToFloat64(m.agentRunsTotal.WithLabelValues("completed")); got != 1 {
		t.Fatalf("agent runs completed = %v, want 1", got)
	}
	if got := testutil.ToFloat64(m.agentRunsTotal.WithLabelValues("failed")); got != 1 {
		t.Fatalf("agent runs failed = %v, want 1", got)
	}
	if got := testutil.ToFloat64(m.agentTerminalFailures.WithLabelValues("max_steps_exceeded")); got != 1 {
		t.Fatalf("agent terminal failures = %v, want 1", got)
	}
	if got := testutil.ToFloat64(m.agentPolicyDenials.WithLabelValues("hidden_column_denied")); got != 1 {
		t.Fatalf("agent policy denials = %v, want 1", got)
	}
	if got := testutil.ToFloat64(m.agentShadowComparisons.WithLabelValues("result_mismatch")); got != 1 {
		t.Fatalf("agent shadow comparisons = %v, want 1", got)
	}
	if got := testutil.ToFloat64(m.agentQueueRedeliveries); got != 1 {
		t.Fatalf("agent queue redeliveries = %v, want 1", got)
	}
	if got := testutil.ToFloat64(m.agentPlannerTokens.WithLabelValues("prompt")); got != 120 {
		t.Fatalf("agent planner prompt tokens = %v, want 120", got)
	}
	if got := testutil.ToFloat64(m.agentPlannerTokens.WithLabelValues("completion")); got != 40 {
		t.Fatalf("agent planner completion tokens = %v, want 40", got)
	}
	if err := CheckGatheredCardinality(reg); err != nil {
		t.Fatalf("CheckGatheredCardinality: %v", err)
	}
}

func TestAgentMetricsBoundUnknownLabels(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)

	// Every label value below is outside its allowed set — each must land on
	// its documented fallback bucket instead of creating a fresh, unbounded
	// Prometheus series.
	m.RecordAgentRunTerminal("some_new_outcome_nobody_declared")
	m.RecordAgentTerminalFailure("some_new_reason_code")
	m.RecordAgentStepDuration("some.unregistered.tool", time.Second)
	m.RecordAgentPolicyDenial("some_new_denial_reason")
	m.RecordAgentShadowComparison("some_new_category")

	if got := testutil.ToFloat64(m.agentRunsTotal.WithLabelValues("failed")); got != 1 {
		t.Fatalf("unbounded outcome should fall back to failed, got %v", got)
	}
	if got := testutil.ToFloat64(m.agentTerminalFailures.WithLabelValues("tool_error")); got != 1 {
		t.Fatalf("unbounded reason should fall back to tool_error, got %v", got)
	}
	if got := testutil.CollectAndCount(m.agentStepDuration, "biqly_agent_step_duration_seconds"); got != 1 {
		t.Fatalf("unbounded tool name should still record exactly one observation (under the other bucket), got %v", got)
	}
	if got := testutil.ToFloat64(m.agentPolicyDenials.WithLabelValues("other")); got != 1 {
		t.Fatalf("unbounded policy denial reason should fall back to other, got %v", got)
	}
	if got := testutil.ToFloat64(m.agentShadowComparisons.WithLabelValues("other")); got != 1 {
		t.Fatalf("unbounded shadow category should fall back to other, got %v", got)
	}
	if err := CheckGatheredCardinality(reg); err != nil {
		t.Fatalf("CheckGatheredCardinality: %v", err)
	}
}
