package ai

import (
	"context"
	"testing"

	evalpkg "github.com/biqly/biqly/internal/ai/eval"
	"github.com/biqly/biqly/internal/config"
	"github.com/biqly/biqly/internal/query"
)

// TestRepairLoopEvalRegressionGate runs each repair golden scenario through the
// full eval suite with a scripted provider that emits an invalid LogicalQuery
// first and the canonical one on the repair attempt. It proves the structured
// repair loop recovers each case to its expected query (logical + execution).
func TestRepairLoopEvalRegressionGate(t *testing.T) {
	cases := evalpkg.RepairGoldenCases()
	if len(cases) == 0 {
		t.Fatal("no repair golden cases defined")
	}

	for _, rc := range cases {
		t.Run(rc.ID, func(t *testing.T) {
			good, err := marshalLogicalQuery(rc.Expected)
			if err != nil {
				t.Fatalf("marshal expected: %v", err)
			}

			// First reply fails validation; the repair loop must recover to `good`.
			provider := &scriptedProvider{replies: []string{rc.BadFirstResponse, good}}
			cfg := config.AIConfig{
				Connection: config.AIConnectionConfig{Model: "stub"},
				Generation: config.AIGenerationConfig{
					MaxTokens:   2048,
					Temperature: 0,
					MaxRetries:  2,
				},
			}
			svc := NewServiceWithProvider(&cfg, query.NewValidator(1000), provider)

			opts := evalpkg.SuiteOptions{
				Cases: []evalpkg.GoldenCase{rc.GoldenCase},
				Modes: evalpkg.ModeLogical | evalpkg.ModeExecution,
			}
			result := evalpkg.RunGoldenSuite(context.Background(), svc, opts)
			if result.Failed > 0 {
				for _, c := range result.Cases {
					if !c.Pass(opts) {
						t.Fatalf("[%s] repair did not recover: logical=%v (%s) exec=%v (%s)",
							c.Case.ID, c.LogicalMatch, c.LogicalReason, c.ExecutionMatch, c.ExecutionReason)
					}
				}
			}
		})
	}
}
