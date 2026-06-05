package ambiguity

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/biqly/biqly/internal/ai/prompt"
	"github.com/biqly/biqly/internal/ai/provider"
	"github.com/biqly/biqly/internal/i18n"
	"github.com/biqly/biqly/internal/semantic"
)

type fakeLLMAmbiguityProvider struct {
	result provider.GenerationResult
	err    error
	prompt string
}

func (p *fakeLLMAmbiguityProvider) Generate(_ context.Context, promptStr string) (provider.GenerationResult, error) {
	p.prompt = promptStr
	return p.result, p.err
}

func (*fakeLLMAmbiguityProvider) GenerateAt(_ context.Context, _ string, _ float64) (provider.GenerationResult, error) {
	return provider.GenerationResult{}, errors.New("unexpected GenerateAt call")
}

func TestLLMAnalyzerAnalyze(t *testing.T) {
	client := &fakeLLMAmbiguityProvider{
		result: provider.GenerationResult{Content: "```json\n" + `{
			"is_ambiguous": true,
			"ambiguities": [{
				"term": "active customers",
				"possible_meanings": ["Customers with an enabled account", "Customers who ordered recently"],
				"recommended_clarification": "Which active customer definition should be used?"
			}]
		}` + "\n```"},
	}
	analyzer := NewLLMAnalyzer(client)
	model := &semantic.SemanticModel{
		Name:       "sales",
		Dimensions: []semantic.Dimension{{Name: "customer_status"}},
		Metrics:    []semantic.Metric{{Name: "revenue"}},
	}
	glossary := []prompt.GlossaryEntry{{Term: "active", Definition: "Enabled account"}}

	got, err := analyzer.Analyze(context.Background(), i18n.LocaleEN, "Show active customers", model, glossary)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if !got.IsAmbiguous || len(got.Ambiguities) != 1 {
		t.Fatalf("Analyze() = %#v, want one ambiguity", got)
	}
	if got.Ambiguities[0].Type != "llm" || len(got.Ambiguities[0].Interpretations) != 2 {
		t.Fatalf("Analyze() ambiguity = %#v, want two LLM interpretations", got.Ambiguities[0])
	}
	for _, want := range []string{"Show active customers", "sales", "customer_status", "revenue", "active: Enabled account"} {
		if !strings.Contains(client.prompt, want) {
			t.Fatalf("prompt = %q, want substring %q", client.prompt, want)
		}
	}
}

func TestLLMAnalyzerAnalyzeInvalidJSON(t *testing.T) {
	client := &fakeLLMAmbiguityProvider{
		result: provider.GenerationResult{Content: "not json"},
	}

	_, err := NewLLMAnalyzer(client).Analyze(context.Background(), i18n.LocaleTR, "Aktif müşteriler", nil, nil)
	if err == nil {
		t.Fatal("Analyze() error = nil, want invalid JSON error")
	}
}
