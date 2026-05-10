package ai

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/biqly/biqly/internal/config"
	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/semantic"
)

// goldenSeed is the starter set for the text-to-SQL golden evaluation. New
// cases should be added here (or, eventually, in testdata/ai-golden/*.json
// loaded by a future helper). Seed cases must be intentionally small — they
// document the contract; large benchmarks belong behind a build tag.
func goldenSeed() []GoldenCase {
	ordersModel := &semantic.SemanticModel{
		ID:           "orders",
		DatasourceID: "ds-1",
		Name:         "public.orders",
		BaseSchema:   "public",
		BaseTable:    "orders",
		Dimensions: []semantic.Dimension{
			{Name: "country", Type: "text", ColumnRef: "orders.country"},
			{Name: "status", Type: "text", ColumnRef: "orders.status"},
		},
		Metrics: []semantic.Metric{
			{Name: "row_count", Aggregation: "count", Expression: "*"},
			{Name: "total_amount", Aggregation: "sum", Expression: "orders.amount"},
		},
	}

	return []GoldenCase{
		{
			ID:       "count-orders",
			Question: "kaç sipariş var",
			Model:    ordersModel,
			Expected: query.LogicalQuery{
				Select: []query.SelectItem{{Type: "metric", Name: "row_count"}},
				Limit:  100,
			},
		},
		{
			ID:       "orders-by-country",
			Question: "ülkeye göre sipariş sayısı",
			Model:    ordersModel,
			Expected: query.LogicalQuery{
				Select: []query.SelectItem{
					{Type: "dimension", Name: "country"},
					{Type: "metric", Name: "row_count"},
				},
				GroupBy: []query.GroupBy{{Field: "country"}},
				Limit:   100,
			},
		},
		{
			ID:       "shipped-orders-total",
			Question: "shipped olan siparişlerin toplam tutarı",
			Model:    ordersModel,
			Expected: query.LogicalQuery{
				Select:  []query.SelectItem{{Type: "metric", Name: "total_amount"}},
				Filters: []query.Filter{{Field: "status", Operator: "eq", Value: "shipped"}},
				Limit:   100,
			},
		},
	}
}

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
	if ok, reason := LogicalQueryEqual(&a, &b); !ok {
		t.Errorf("expected equivalence with reordered selects, got mismatch: %s", reason)
	}

	// Different metric → must NOT match.
	c := query.LogicalQuery{
		Select:  []query.SelectItem{{Type: "metric", Name: "total_amount"}},
		GroupBy: []query.GroupBy{{Field: "country"}},
		Limit:   100,
	}
	if ok, _ := LogicalQueryEqual(&a, &c); ok {
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
	if ok, _ := LogicalQueryEqual(&d, &e); ok {
		t.Errorf("expected mismatch on order_by direction, got equivalence")
	}
}

// TestGoldenSeedSelfConsistent guards the seed set: every expected query must
// pass schema + semantic validation, otherwise the dataset itself is broken.
func TestGoldenSeedSelfConsistent(t *testing.T) {
	v := query.NewValidator(1000)
	sv := NewSchemaValidator()
	for _, c := range goldenSeed() {
		// Schema validator works on the raw JSON form, so re-marshal via the
		// service's parse path to mimic real flow.
		raw, err := marshalLogicalQuery(c.Expected)
		if err != nil {
			t.Errorf("[%s] marshal expected: %v", c.ID, err)
			continue
		}
		if _, err := sv.Validate(raw, c.Model); err != nil {
			t.Errorf("[%s] schema validate expected: %v", c.ID, err)
		}
		if err := v.Validate(c.Expected, c.Model); err != nil {
			t.Errorf("[%s] semantic validate expected: %v", c.ID, err)
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
		Provider:    os.Getenv("BI_AI_PROVIDER"),
		APIKey:      os.Getenv("BI_AI_API_KEY"),
		BaseURL:     os.Getenv("BI_AI_BASE_URL"),
		Model:       os.Getenv("BI_AI_MODEL"),
		MaxTokens:   2048,
		Temperature: 0.0,
		MaxRetries:  1,
	}
	if cfg.APIKey == "" || cfg.Model == "" {
		t.Skip("BI_AI_API_KEY and BI_AI_MODEL are required for live golden eval")
	}
	svc := NewService(cfg, query.NewValidator(1000))

	cases := goldenSeed()
	pass := 0
	for _, c := range cases {
		resp, err := svc.ProcessQuestion(context.Background(), c.Question, c.Model)
		if err != nil {
			t.Errorf("[%s] error: %v", c.ID, err)
			continue
		}
		ok, reason := LogicalQueryEqual(resp.LogicalQuery, &c.Expected)
		if !ok {
			t.Errorf("[%s] mismatch: %s; got=%+v", c.ID, reason, resp.LogicalQuery)
			continue
		}
		pass++
	}
	t.Logf("golden eval: %d / %d passed", pass, len(cases))
}

// marshalLogicalQuery produces the JSON the schema validator expects.
func marshalLogicalQuery(lq query.LogicalQuery) (string, error) {
	b, err := json.Marshal(lq)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
