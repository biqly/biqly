package ai

import (
	"context"
	"encoding/json"
	"fmt"
)

// goldenStubProvider returns canonical LogicalQuery JSON for golden case IDs.
// Used by CI regression gate without calling a live LLM.
type goldenStubProvider struct {
	byQuestion map[string]string
}

// NewGoldenStubProvider builds a stub that always returns the expected
// LogicalQuery for each DefaultGoldenCases question.
func NewGoldenStubProvider() Provider {
	return NewGoldenStubProviderForCases(DefaultGoldenCases())
}

// NewGoldenStubProviderForCases maps each case question to its expected JSON.
func NewGoldenStubProviderForCases(cases []GoldenCase) Provider {
	byQ := make(map[string]string, len(cases))
	for _, c := range cases {
		b, err := json.Marshal(c.Expected)
		if err != nil {
			continue
		}
		byQ[c.Question] = string(b)
	}
	return &goldenStubProvider{byQuestion: byQ}
}

func (p *goldenStubProvider) Generate(_ context.Context, prompt string) (GenerationResult, error) {
	content, err := p.lookup(prompt)
	if err != nil {
		return GenerationResult{}, err
	}
	return GenerationResult{Content: content}, nil
}

func (p *goldenStubProvider) GenerateAt(ctx context.Context, prompt string, _ float64) (GenerationResult, error) {
	return p.Generate(ctx, prompt)
}

func (p *goldenStubProvider) lookup(prompt string) (string, error) {
	for q, body := range p.byQuestion {
		if containsFold(prompt, q) {
			return body, nil
		}
	}
	return "", fmt.Errorf("golden stub: no matching case in prompt")
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
