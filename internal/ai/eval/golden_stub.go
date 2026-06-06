package eval

import (
	"context"
	"errors"
	"github.com/bytedance/sonic"

	providerpkg "github.com/biqly/biqly/internal/ai/provider"
)

// goldenStubProvider returns canonical LogicalQuery JSON for golden case IDs.
// Used by CI regression gate without calling a live LLM.
type goldenStubProvider struct {
	byQuestion map[string]string
}

// NewGoldenStubProvider builds a stub that always returns the expected
// LogicalQuery for each DefaultGoldenCases question.
func NewGoldenStubProvider() providerpkg.Provider {
	return NewGoldenStubProviderForCases(DefaultGoldenCases())
}

// NewGoldenStubProviderForCases maps each case question to its expected JSON.
func NewGoldenStubProviderForCases(cases []GoldenCase) providerpkg.Provider {
	byQ := make(map[string]string, len(cases))
	for _, c := range cases {
		b, err := sonic.ConfigStd.Marshal(c.Expected)
		if err != nil {
			continue
		}
		byQ[c.Question] = string(b)
	}
	return &goldenStubProvider{byQuestion: byQ}
}

func (p *goldenStubProvider) Generate(_ context.Context, prompt string) (providerpkg.GenerationResult, error) {
	content, err := p.lookup(prompt)
	if err != nil {
		return providerpkg.GenerationResult{}, err
	}
	return providerpkg.GenerationResult{Content: content}, nil
}

func (p *goldenStubProvider) GenerateAt(ctx context.Context, prompt string, _ float64) (providerpkg.GenerationResult, error) {
	return p.Generate(ctx, prompt)
}

func (p *goldenStubProvider) lookup(prompt string) (string, error) {
	for q, body := range p.byQuestion {
		if containsFold(prompt, q) {
			return body, nil
		}
	}
	return "", errors.New("golden stub: no matching case in prompt")
}

func containsFold(haystack, needle string) bool {
	return len(needle) > 0 && (len(haystack) >= len(needle)) && indexFold(haystack, needle) >= 0
}

func indexFold(s, sub string) int {
	hs := []rune(s)
	ns := []rune(sub)
outer:
	for i := 0; i+len(ns) <= len(hs); i++ {
		for j := range ns {
			a := hs[i+j]
			b := ns[j]
			if a >= 'A' && a <= 'Z' {
				a += 'a' - 'A'
			}
			if b >= 'A' && b <= 'Z' {
				b += 'a' - 'A'
			}
			if a != b {
				continue outer
			}
		}
		return i
	}
	return -1
}
