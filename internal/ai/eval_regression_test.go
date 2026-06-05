package ai

import (
	"context"
	"testing"

	evalpkg "github.com/biqly/biqly/internal/ai/eval"
	"github.com/biqly/biqly/internal/config"
	"github.com/biqly/biqly/internal/query"
)

func TestResultSetEqualBaseline(t *testing.T) {
	a := &query.Result{
		Columns: []query.ResultColumn{{Name: "country"}, {Name: "row_count"}},
		Rows: [][]any{
			{"DE", float64(2)},
			{"TR", float64(2)},
		},
	}
	b := &query.Result{
		Columns: []query.ResultColumn{{Name: "row_count"}, {Name: "country"}},
		Rows: [][]any{
			{float64(2), "TR"},
			{float64(2), "DE"},
		},
	}
	ok, reason := evalpkg.ResultSetEqual(a, b)
	if !ok {
		t.Fatalf("expected equivalent result sets, got: %s", reason)
	}
}

func TestExecutionAccuracyGolden(t *testing.T) {
	exec := evalpkg.MemoryResultExecutor{}
	ctx := context.Background()
	for _, c := range evalpkg.DefaultGoldenCases() {
		expRes, err := exec.Execute(ctx, c.Model, &c.Expected)
		if err != nil {
			t.Fatalf("[%s] execute expected: %v", c.ID, err)
		}
		ok, reason := evalpkg.ResultSetEqual(expRes, expRes)
		if !ok {
			t.Fatalf("[%s] self-compare: %s", c.ID, reason)
		}
	}
}

func TestBenchmarkSuiteSelfConsistent(t *testing.T) {
	sv := NewSchemaValidatorWith(query.NewValidator(1000))
	for _, c := range evalpkg.BenchmarkCases() {
		raw, err := marshalLogicalQuery(c.Expected)
		if err != nil {
			t.Errorf("[%s] marshal: %v", c.ID, err)
			continue
		}
		if _, err := sv.Validate(raw, c.Model); err != nil {
			t.Errorf("[%s] validate: %v", c.ID, err)
		}
	}
}

func TestEvalRegressionGate(t *testing.T) {
	cfg := config.AIConfig{
		Model:       "stub",
		MaxTokens:   2048,
		Temperature: 0,
		MaxRetries:  0,
	}
	svc := NewServiceWithProvider(&cfg, query.NewValidator(1000), evalpkg.NewGoldenStubProvider())
	opts := evalpkg.EvalSuiteOptions{
		Cases: evalpkg.DefaultGoldenCases(),
		Modes: evalpkg.EvalModeLogical | evalpkg.EvalModeExecution,
	}
	result := evalpkg.RunGoldenSuite(context.Background(), svc, opts)
	if result.Failed > 0 {
		for _, c := range result.Cases {
			if !c.Pass(opts) {
				t.Errorf("[%s] failed: logical=%v (%s) exec=%v (%s)",
					c.Case.ID, c.LogicalMatch, c.LogicalReason, c.ExecutionMatch, c.ExecutionReason)
			}
		}
		t.Fatalf("regression gate: %d/%d failed", result.Failed, result.Total)
	}
}

func TestBenchmarkSuiteRegressionGate(t *testing.T) {
	cfg := config.AIConfig{
		Model:       "stub",
		MaxTokens:   2048,
		Temperature: 0,
		MaxRetries:  0,
	}
	svc := NewServiceWithProvider(&cfg, query.NewValidator(1000), evalpkg.NewGoldenStubProviderForCases(evalpkg.BenchmarkCases()))
	opts := evalpkg.EvalSuiteOptions{
		Cases: evalpkg.BenchmarkCases(),
		Modes: evalpkg.EvalModeLogical | evalpkg.EvalModeExecution,
	}
	result := evalpkg.RunGoldenSuite(context.Background(), svc, opts)
	if result.PassRate < 1.0 {
		t.Fatalf("benchmark gate: pass_rate=%.2f (%d/%d)", result.PassRate, result.Passed, result.Total)
	}
}
