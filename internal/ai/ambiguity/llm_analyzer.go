package ambiguity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/biqly/biqly/internal/ai/jsonextract"
	"github.com/biqly/biqly/internal/ai/prompt"
	"github.com/biqly/biqly/internal/ai/provider"
	"github.com/biqly/biqly/internal/i18n"
	"github.com/biqly/biqly/internal/semantic"
)

// LLMAnalyzer detects ambiguities that are not covered by deterministic rules.
type LLMAnalyzer struct {
	client  provider.Provider
	builder *prompt.Builder
}

// NewLLMAnalyzer creates an analyzer backed by the configured LLM provider.
func NewLLMAnalyzer(client provider.Provider) *LLMAnalyzer {
	return &LLMAnalyzer{
		client:  client,
		builder: &prompt.Builder{},
	}
}

type llmAmbiguityResponse struct {
	IsAmbiguous         bool               `json:"is_ambiguous"`
	ClarificationNeeded bool               `json:"clarification_needed"`
	Ambiguities         []llmAmbiguityItem `json:"ambiguities"`
	AmbiguousTerms      []llmAmbiguityItem `json:"ambiguous_terms"`
}

type llmAmbiguityItem struct {
	Term                     string   `json:"term"`
	PossibleMeanings         []string `json:"possible_meanings"`
	RecommendedClarification string   `json:"recommended_clarification"`
}

// Analyze asks the LLM for structured ambiguity candidates and converts them
// into the shared result shape used by clarification responses.
func (a *LLMAnalyzer) Analyze(ctx context.Context, locale i18n.Locale, question string, model *semantic.SemanticModel, glossary []prompt.GlossaryEntry) (Result, error) {
	if a == nil || a.client == nil {
		return Result{}, errors.New("LLM ambiguity analyzer requires a provider")
	}

	analysisPrompt := a.builder.BuildAmbiguityAnalysis(ctx, locale, question, model, glossary)
	gen, err := a.client.Generate(ctx, analysisPrompt)
	if err != nil {
		return Result{}, fmt.Errorf("generate ambiguity analysis: %w", err)
	}

	var response llmAmbiguityResponse
	if err := json.Unmarshal([]byte(jsonextract.TrimToJSONObject(gen.Content)), &response); err != nil {
		return Result{}, fmt.Errorf("parse ambiguity analysis: %w", err)
	}
	if !response.IsAmbiguous && !response.ClarificationNeeded {
		return Result{}, nil
	}

	items := response.Ambiguities
	if len(items) == 0 {
		items = response.AmbiguousTerms
	}
	ambiguities := make([]Item, 0, len(items))
	for _, item := range items {
		interpretations := llmInterpretations(item)
		if strings.TrimSpace(item.Term) == "" || len(interpretations) < 2 {
			continue
		}
		ambiguities = append(ambiguities, Item{
			Term:            strings.TrimSpace(item.Term),
			Type:            "llm",
			Interpretations: interpretations,
		})
	}

	return Result{
		IsAmbiguous: len(ambiguities) > 0,
		Ambiguities: ambiguities,
	}, nil
}

func llmInterpretations(item llmAmbiguityItem) []Interpretation {
	seen := make(map[string]struct{}, len(item.PossibleMeanings))
	interpretations := make([]Interpretation, 0, len(item.PossibleMeanings))
	for _, meaning := range item.PossibleMeanings {
		meaning = strings.TrimSpace(meaning)
		key := strings.ToLower(meaning)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		interpretations = append(interpretations, Interpretation{
			Label:       meaning,
			Description: item.RecommendedClarification,
			SemanticMapping: SemanticMapping{
				Type: "llm",
				Name: meaning,
			},
			Confidence: 1,
		})
	}
	return interpretations
}
