package eval

import (
	"context"
	"errors"

	"github.com/bytedance/sonic"

	providerpkg "github.com/biqly/biqly/internal/ai/provider"
	"github.com/biqly/biqly/internal/query"
)

// NewMemoryRecallStubProvider returns a stub that answers the memory-recall
// golden question correctly only when the prompt includes the recall few-shot
// section; without recall it returns an intentionally wrong LogicalQuery.
func NewMemoryRecallStubProvider() providerpkg.Provider {
	c := MemoryRecallGoldenCase()
	expected, err := sonic.ConfigStd.Marshal(c.Expected)
	if err != nil {
		return &memoryRecallStubProvider{targetQuestion: c.Question}
	}
	wrong, err := sonic.ConfigStd.Marshal(query.LogicalQuery{
		Select: []query.SelectItem{{Type: "metric", Name: "total_amount"}},
		Limit:  100,
	})
	if err != nil {
		return &memoryRecallStubProvider{targetQuestion: c.Question, expectedJSON: string(expected)}
	}
	return &memoryRecallStubProvider{
		targetQuestion: c.Question,
		expectedJSON:   string(expected),
		wrongJSON:      string(wrong),
	}
}

type memoryRecallStubProvider struct {
	targetQuestion string
	expectedJSON   string
	wrongJSON      string
}

func (p *memoryRecallStubProvider) Generate(_ context.Context, prompt string) (providerpkg.GenerationResult, error) {
	if !containsFold(prompt, p.targetQuestion) {
		return providerpkg.GenerationResult{}, errors.New("memory recall stub: target question not in prompt")
	}
	if p.expectedJSON == "" {
		return providerpkg.GenerationResult{}, errors.New("memory recall stub: not initialized")
	}
	if containsFold(prompt, memoryRecallFewShotMarker) {
		return providerpkg.GenerationResult{Content: p.expectedJSON}, nil
	}
	if p.wrongJSON == "" {
		return providerpkg.GenerationResult{}, errors.New("memory recall stub: wrong query not initialized")
	}
	return providerpkg.GenerationResult{Content: p.wrongJSON}, nil
}

func (p *memoryRecallStubProvider) GenerateAt(ctx context.Context, prompt string, _ float64) (providerpkg.GenerationResult, error) {
	return p.Generate(ctx, prompt)
}
