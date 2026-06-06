package eval

import (
	"context"
	"errors"
	"fmt"
	"github.com/bytedance/sonic"
	"strings"

	"github.com/biqly/biqly/internal/ai/jsonextract"
	providerpkg "github.com/biqly/biqly/internal/ai/provider"
	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/semantic"
)

type judgeVerdict struct {
	Pass      bool   `json:"pass"`
	Rationale string `json:"rationale"`
}

// JudgeLogicalQuery asks an LLM whether the generated LogicalQuery correctly
// answers the question relative to the expected reference query.
func JudgeLogicalQuery(ctx context.Context, provider providerpkg.Provider, question string, model *semantic.SemanticModel, expected, got *query.LogicalQuery) (bool, string, error) {
	if provider == nil {
		return false, "", errors.New("judge provider is nil")
	}
	if expected == nil || got == nil {
		return false, "missing logical query", nil
	}
	expJSON, err := sonic.ConfigStd.MarshalIndent(expected, "", "  ")
	if err != nil {
		return false, "", fmt.Errorf("marshal expected: %w", err)
	}
	gotJSON, err := sonic.ConfigStd.MarshalIndent(got, "", "  ")
	if err != nil {
		return false, "", fmt.Errorf("marshal got: %w", err)
	}

	dims := make([]string, 0, len(model.Dimensions))
	for _, d := range model.Dimensions {
		dims = append(dims, d.Name)
	}
	metrics := make([]string, 0, len(model.Metrics))
	for _, m := range model.Metrics {
		metrics = append(metrics, m.Name)
	}

	prompt := fmt.Sprintf(`You are an expert BI query evaluator. Decide whether GENERATED answers the user question as well as REFERENCE for this semantic model.

Semantic model: %s
Dimensions: %s
Metrics: %s

User question: %s

REFERENCE LogicalQuery (canonical):
%s

GENERATED LogicalQuery:
%s

Rules:
- pass=true if GENERATED is semantically equivalent to REFERENCE or a valid alternative that answers the question correctly
- pass=false if wrong metrics, dimensions, filters, grouping, or missing constraints
- Do not require identical JSON; focus on analytical intent

Respond with JSON only:
{"pass": true|false, "rationale": "one short sentence"}`,
		model.Name,
		strings.Join(dims, ", "),
		strings.Join(metrics, ", "),
		question,
		string(expJSON),
		string(gotJSON),
	)

	gen, err := provider.Generate(ctx, prompt)
	if err != nil {
		return false, "", err
	}
	cleaned := jsonextract.CleanAIResponseForJSON(gen.Content)
	var v judgeVerdict
	if err := sonic.ConfigStd.Unmarshal([]byte(cleaned), &v); err != nil {
		return false, "", fmt.Errorf("parse judge response: %w", err)
	}
	return v.Pass, v.Rationale, nil
}
