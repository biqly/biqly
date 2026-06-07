package ai

import (
	"context"
	"github.com/bytedance/sonic"
	"os"
	"strconv"
	"testing"

	evalpkg "github.com/biqly/biqly/internal/ai/eval"
	"github.com/biqly/biqly/internal/query"
)

// TestLogicalQueryEqualBaseline validates the equivalence helper itself with
// hand-crafted positive and negative cases. The runner depends on this being
// correct, so failure here precedes any LLM-driven evaluation.
func TestLogicalQueryEqualBaseline(t *testing.T) {
	a := query.LogicalQuery{
		Select:  []query.SelectItem{{Type: "dimension", Name: "country"}, {Type: "metric", Name: "row_count"}},
		GroupBy: []query.GroupBy{{Field: "country"}},
		Limit:   100,
	}
	// Same as a but with select items reversed; should still match (order-insensitive).
	b := query.LogicalQuery{
		Select:  []query.SelectItem{{Type: "metric", Name: "row_count"}, {Type: "dimension", Name: "country"}},
		GroupBy: []query.GroupBy{{Field: "country"}},
		Limit:   100,
	}
	if ok, reason := evalpkg.LogicalQueryEqual(&a, &b); !ok {
		t.Errorf("expected equivalence with reordered selects, got mismatch: %s", reason)
	}

	// Different metric → must NOT match.
	c := query.LogicalQuery{
		Select:  []query.SelectItem{{Type: "metric", Name: "total_amount"}},
		GroupBy: []query.GroupBy{{Field: "country"}},
		Limit:   100,
	}
	if ok, _ := evalpkg.LogicalQueryEqual(&a, &c); ok {
		t.Errorf("expected mismatch on different metric, got equivalence")
	}

	// order_by direction mismatch → must NOT match.
	d := query.LogicalQuery{
		Select:  a.Select,
		GroupBy: a.GroupBy,
		OrderBy: []query.OrderBy{{Field: "row_count", Direction: "desc"}},
		Limit:   100,
	}
	e := query.LogicalQuery{
		Select:  a.Select,
		GroupBy: a.GroupBy,
		OrderBy: []query.OrderBy{{Field: "row_count", Direction: "asc"}},
		Limit:   100,
	}
	if ok, _ := evalpkg.LogicalQueryEqual(&d, &e); ok {
		t.Errorf("expected mismatch on order_by direction, got equivalence")
	}

	f1 := query.LogicalQuery{
		Select:  []query.SelectItem{{Type: "metric", Name: "row_count"}},
		Filters: []query.Filter{{Field: "status", Operator: "eq", Value: "shipped"}},
		Limit:   100,
	}
	f2 := query.LogicalQuery{
		Select:  []query.SelectItem{{Type: "metric", Name: "row_count"}},
		Filters: []query.Filter{{Field: "status", Operator: "eq", Value: "pending"}},
		Limit:   100,
	}
	if ok, _ := evalpkg.LogicalQueryEqual(&f1, &f2); ok {
		t.Errorf("expected mismatch on filter value, got equivalence")
	}
}

// TestGoldenSeedSelfConsistent guards the seed set: every expected query must
// pass schema + semantic validation, otherwise the dataset itself is broken.
func TestGoldenSeedSelfConsistent(t *testing.T) {
	sv := NewSchemaValidatorWith(query.NewValidator(1000))
	for _, c := range evalpkg.DefaultGoldenCases() {
		raw, err := marshalLogicalQuery(c.Expected)
		if err != nil {
			t.Errorf("[%s] marshal expected: %v", c.ID, err)
			continue
		}
		if _, err := sv.Validate(raw, c.Model); err != nil {
			t.Errorf("[%s] validate expected: %v", c.ID, err)
		}
	}
}

// TestGoldenEvalAgainstLiveLLM runs the nightly suite against the real configured
// LLM. Skipped by default — enable with BI_AI_GOLDEN_EVAL=1 and provider env vars.
func TestGoldenEvalAgainstLiveLLM(t *testing.T) {
	if os.Getenv("BI_AI_GOLDEN_EVAL") != "1" {
		t.Skip("set BI_AI_GOLDEN_EVAL=1 to run live LLM golden evaluation")
	}
	cfg := evalpkg.LiveAIConfigFromEnv()
	if !cfg.QueryLLMConfigured() {
		t.Skip("BI_AI_MODEL and BI_AI_API_KEY (or BI_AI_BASE_URL for keyless local LLM) are required for live golden eval")
	}
	svc := NewService(&cfg, query.NewValidator(1000))
	opts := evalpkg.SuiteOptions{
		Cases: evalpkg.NightlyCases(),
		Modes: evalpkg.ModeLogical | evalpkg.ModeExecution,
	}
	result := evalpkg.RunGoldenSuite(context.Background(), svc, opts)
	minRate := evalpkg.DefaultLiveMinPassRate
	if os.Getenv("BI_AI_GOLDEN_MIN_PASS_RATE") != "" {
		if v, err := strconv.ParseFloat(os.Getenv("BI_AI_GOLDEN_MIN_PASS_RATE"), 64); err == nil {
			minRate = v
		}
	}
	if result.PassRate < minRate {
		for _, c := range result.Cases {
			if !c.Pass(opts) {
				t.Errorf("[%s] failed: logical=%v exec=%v err=%v reason=%s",
					c.Case.ID, c.LogicalMatch, c.ExecutionMatch, c.Err, c.LogicalReason)
			}
		}
		t.Fatalf("live pass rate %.2f below threshold %.2f (%d/%d passed)",
			result.PassRate, minRate, result.Passed, result.Total)
	}
	t.Logf("live eval: %d / %d passed (rate %.2f)", result.Passed, result.Total, result.PassRate)
}

// marshalLogicalQuery produces the JSON the schema validator expects.
func marshalLogicalQuery(lq query.LogicalQuery) (string, error) {
	b, err := sonic.ConfigStd.Marshal(lq)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
