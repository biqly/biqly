package ai

import (
	"context"
	"github.com/bytedance/sonic"
	"os"
	"testing"

	evalpkg "github.com/biqly/biqly/internal/ai/eval"
	"github.com/biqly/biqly/internal/config"
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

// TestGoldenEvalAgainstLiveLLM runs the seed set against the real configured
// LLM. Skipped by default — CI-friendly. Enable by exporting the standard AI
// env vars (BI_AI_API_KEY, BI_AI_BASE_URL, BI_AI_MODEL) and BI_AI_GOLDEN_EVAL=1.
func TestGoldenEvalAgainstLiveLLM(t *testing.T) {
	if os.Getenv("BI_AI_GOLDEN_EVAL") != "1" {
		t.Skip("set BI_AI_GOLDEN_EVAL=1 to run live LLM golden evaluation")
	}
	cfg := config.AIConfig{
		Connection: config.AIConnectionConfig{
			Provider: os.Getenv("BI_AI_PROVIDER"),
			APIKey:   os.Getenv("BI_AI_API_KEY"),
			BaseURL:  os.Getenv("BI_AI_BASE_URL"),
			Model:    os.Getenv("BI_AI_MODEL"),
		},
		Generation: config.AIGenerationConfig{
			MaxTokens:   2048,
			Temperature: 0.0,
			MaxRetries:  1,
		},
	}
	if !cfg.QueryLLMConfigured() {
		t.Skip("BI_AI_MODEL and BI_AI_API_KEY (or BI_AI_BASE_URL for keyless local LLM) are required for live golden eval")
	}
	svc := NewService(&cfg, query.NewValidator(1000))

	cases := evalpkg.DefaultGoldenCases()
	pass := 0
	for _, c := range cases {
		resp, err := svc.ProcessQuestion(context.Background(), c.Question, c.Model)
		if err != nil {
			t.Errorf("[%s] error: %v", c.ID, err)
			continue
		}
		var respLQ *query.LogicalQuery
		if resp != nil && resp.Result != nil {
			respLQ = resp.Result.LogicalQuery
		}
		ok, reason := evalpkg.LogicalQueryEqual(respLQ, &c.Expected)
		if !ok {
			t.Errorf("[%s] mismatch: %s; got=%+v", c.ID, reason, respLQ)
			continue
		}
		pass++
	}
	t.Logf("golden eval: %d / %d passed", pass, len(cases))
}

// marshalLogicalQuery produces the JSON the schema validator expects.
func marshalLogicalQuery(lq query.LogicalQuery) (string, error) {
	b, err := sonic.ConfigStd.Marshal(lq)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
