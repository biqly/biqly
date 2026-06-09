package ai

import (
	"context"
	"testing"

	evalpkg "github.com/biqly/biqly/internal/ai/eval"
	"github.com/biqly/biqly/internal/ai/prompt"
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
	runStubSuiteRegression(t, evalpkg.BenchmarkCases(), benchmarkPassThreshold)
}

func TestNightlySuiteSelfConsistent(t *testing.T) {
	sv := NewSchemaValidatorWith(query.NewValidator(1000))
	for _, c := range evalpkg.NightlyCases() {
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

func TestNightlySuiteRegressionGate(t *testing.T) {
	runStubSuiteRegression(t, evalpkg.NightlyCases(), benchmarkPassThreshold)
}

func TestAmbiguityGoldenRegressionGate(t *testing.T) {
	cases, err := evalpkg.LoadDefaultAmbiguityGoldenCases()
	if err != nil {
		t.Fatalf("load ambiguity golden cases: %v", err)
	}

	ctx := context.Background()
	analysis := evalpkg.RunAmbiguityGoldenAnalysis(ctx, cases)
	if analysis.Passed != analysis.Total {
		for _, cr := range analysis.Cases {
			if !cr.Passed {
				t.Errorf("[%s] analysis: %s", cr.Case.ID, cr.Reason)
			}
		}
		t.Fatalf("ambiguity analysis %d/%d passed", analysis.Passed, analysis.Total)
	}

	cfg := config.AIConfig{
		Connection: config.AIConnectionConfig{Model: "stub"},
		Generation: config.AIGenerationConfig{
			MaxTokens:   2048,
			Temperature: 0,
			MaxRetries:  0,
		},
	}

	choiceCases, err := evalpkg.AmbiguityGoldenChoiceCases(ctx, cases)
	if err != nil {
		t.Fatalf("build choice cases: %v", err)
	}
	if len(choiceCases) > 0 {
		svc := NewServiceWithProvider(&cfg, query.NewValidator(1000), evalpkg.NewGoldenStubProviderForCases(choiceCases))
		choiceResult, err := evalpkg.RunAmbiguityGoldenChoiceSuite(ctx, svc, cases)
		if err != nil {
			t.Fatalf("choice suite: %v", err)
		}
		if choiceResult.Passed != choiceResult.Total {
			for _, cr := range choiceResult.Cases {
				if !cr.Passed {
					t.Errorf("[%s] choice: %s", cr.Case.ID, cr.Reason)
				}
			}
			t.Fatalf("ambiguity choice %d/%d passed", choiceResult.Passed, choiceResult.Total)
		}
	}

	clarifySvc := NewServiceWithProvider(&cfg, query.NewValidator(1000), evalpkg.NewGoldenStubProvider())
	for _, c := range cases {
		if c.ExpectedType != evalpkg.AmbiguityExpectedClarification || c.ClarificationChoice != "" {
			continue
		}
		resp, err := clarifySvc.ProcessQuestion(ctx, c.Question, c.Model,
			WithAmbiguityCheck(true),
			WithAmbiguityGlossary(c.Glossary),
		)
		if err != nil {
			t.Fatalf("[%s] ProcessQuestion: %v", c.ID, err)
		}
		if resp.Clarification == nil || !resp.Clarification.NeedsClarification {
			t.Fatalf("[%s] expected NeedsClarification from ProcessQuestion", c.ID)
		}
	}
}

func TestMemoryRecallRegressionGate(t *testing.T) {
	cfg := config.AIConfig{
		Connection: config.AIConnectionConfig{Model: "stub"},
		Generation: config.AIGenerationConfig{
			MaxTokens:   2048,
			Temperature: 0,
			MaxRetries:  0,
		},
	}
	c := evalpkg.MemoryRecallGoldenCase()
	ctx := context.Background()

	withoutRecall := NewServiceWithProvider(&cfg, query.NewValidator(1000), evalpkg.NewMemoryRecallStubProvider())
	respNoRecall, err := withoutRecall.ProcessQuestion(ctx, c.Question, c.Model)
	if err != nil {
		t.Fatalf("ProcessQuestion without recall: %v", err)
	}
	if respNoRecall == nil || respNoRecall.Result == nil || respNoRecall.Result.LogicalQuery == nil {
		t.Fatal("expected LogicalQuery without recall few-shot")
	}
	match, reason := evalpkg.LogicalQueryEqual(&c.Expected, respNoRecall.Result.LogicalQuery)
	if match {
		t.Fatalf("without recall few-shot should not match golden: %s", reason)
	}

	withRecall := NewServiceWithProvider(&cfg, query.NewValidator(1000), evalpkg.NewMemoryRecallStubProvider())
	respRecall, err := withRecall.ProcessQuestion(ctx, c.Question, c.Model,
		WithFewShotExamples([]prompt.FewShotExample{evalpkg.MemoryRecallFewShotExample()}),
	)
	if err != nil {
		t.Fatalf("ProcessQuestion with recall: %v", err)
	}
	if respRecall == nil || respRecall.Result == nil || respRecall.Result.LogicalQuery == nil {
		t.Fatal("expected LogicalQuery with recall few-shot")
	}
	match, reason = evalpkg.LogicalQueryEqual(&c.Expected, respRecall.Result.LogicalQuery)
	if !match {
		t.Fatalf("with recall few-shot should match golden: %s", reason)
	}
}

func runStubSuiteRegression(t *testing.T, cases []evalpkg.GoldenCase, threshold float64) {
	t.Helper()
	cfg := config.AIConfig{
		Connection: config.AIConnectionConfig{Model: "stub"},
		Generation: config.AIGenerationConfig{
			MaxTokens:   2048,
			Temperature: 0,
			MaxRetries:  0,
		},
	}
	svc := NewServiceWithProvider(&cfg, query.NewValidator(1000), evalpkg.NewGoldenStubProviderForCases(cases))
	opts := evalpkg.SuiteOptions{
		Cases: cases,
		Modes: evalpkg.ModeLogical | evalpkg.ModeExecution,
	}
	result := evalpkg.RunGoldenSuite(context.Background(), svc, opts)

	logicalRate := float64(result.LogicalPassed) / float64(result.Total)
	executionRate := float64(result.ExecutionPassed) / float64(result.Total)

	if logicalRate < threshold {
		t.Fatalf("logical accuracy %.2f below threshold %.2f (%d/%d passed)",
			logicalRate, threshold, result.LogicalPassed, result.Total)
	}
	if executionRate < threshold {
		t.Fatalf("execution accuracy %.2f below threshold %.2f (%d/%d passed)",
			executionRate, threshold, result.ExecutionPassed, result.Total)
	}
}
