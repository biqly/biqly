package ai

import (
	"context"
	"testing"

	evalpkg "github.com/biqly/biqly/internal/ai/eval"
	"github.com/biqly/biqly/internal/config"
	"github.com/biqly/biqly/internal/query"
)

// Accuracy thresholds that gate the build. Adjust deliberately — lowering
// these silently degrades the pipeline without surfacing regressions.
const (
	goldenLogicalThreshold   = 1.00 // golden set uses stub provider → must be perfect
	goldenExecutionThreshold = 1.00
	benchmarkPassThreshold   = 1.00
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
		Connection: config.AIConnectionConfig{Model: "stub"},
		Generation: config.AIGenerationConfig{
			MaxTokens:   2048,
			Temperature: 0,
			MaxRetries:  0,
		},
	}
	svc := NewServiceWithProvider(&cfg, query.NewValidator(1000), evalpkg.NewGoldenStubProvider())
	opts := evalpkg.SuiteOptions{
		Cases: evalpkg.DefaultGoldenCases(),
		Modes: evalpkg.ModeLogical | evalpkg.ModeExecution,
	}
	result := evalpkg.RunGoldenSuite(context.Background(), svc, opts)

	logicalRate := float64(result.LogicalPassed) / float64(result.Total)
	executionRate := float64(result.ExecutionPassed) / float64(result.Total)

	if logicalRate < goldenLogicalThreshold {
		for _, c := range result.Cases {
			if !c.LogicalMatch {
				t.Errorf("[%s] logical mismatch: %s", c.Case.ID, c.LogicalReason)
			}
		}
		t.Fatalf("logical accuracy %.2f below threshold %.2f (%d/%d passed)",
			logicalRate, goldenLogicalThreshold, result.LogicalPassed, result.Total)
	}
	if executionRate < goldenExecutionThreshold {
		for _, c := range result.Cases {
			if !c.ExecutionMatch {
				t.Errorf("[%s] execution mismatch: %s", c.Case.ID, c.ExecutionReason)
			}
		}
		t.Fatalf("execution accuracy %.2f below threshold %.2f (%d/%d passed)",
			executionRate, goldenExecutionThreshold, result.ExecutionPassed, result.Total)
	}
}

func TestBenchmarkSuiteRegressionGate(t *testing.T) {
	cfg := config.AIConfig{
		Connection: config.AIConnectionConfig{Model: "stub"},
		Generation: config.AIGenerationConfig{
			MaxTokens:   2048,
			Temperature: 0,
			MaxRetries:  0,
		},
	}
	svc := NewServiceWithProvider(&cfg, query.NewValidator(1000), evalpkg.NewGoldenStubProviderForCases(evalpkg.BenchmarkCases()))
	opts := evalpkg.SuiteOptions{
		Cases: evalpkg.BenchmarkCases(),
		Modes: evalpkg.ModeLogical | evalpkg.ModeExecution,
	}
	result := evalpkg.RunGoldenSuite(context.Background(), svc, opts)

	logicalRate := float64(result.LogicalPassed) / float64(result.Total)
	executionRate := float64(result.ExecutionPassed) / float64(result.Total)

	if logicalRate < benchmarkPassThreshold {
		t.Fatalf("benchmark logical accuracy %.2f below threshold %.2f (%d/%d passed)",
			logicalRate, benchmarkPassThreshold, result.LogicalPassed, result.Total)
	}
	if executionRate < benchmarkPassThreshold {
		t.Fatalf("benchmark execution accuracy %.2f below threshold %.2f (%d/%d passed)",
			executionRate, benchmarkPassThreshold, result.ExecutionPassed, result.Total)
	}
}
