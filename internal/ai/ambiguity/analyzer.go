// Package ambiguity detects semantic ambiguity before LogicalQuery generation.
package ambiguity

import (
	"context"
	"sort"
	"strings"

	"github.com/biqly/biqly/internal/ai/prompt"
	"github.com/biqly/biqly/internal/semantic"
)

const defaultConfidenceThreshold = 0.70

// AmbiguityResult describes whether a question needs user clarification.
type AmbiguityResult struct {
	IsAmbiguous      bool            `json:"is_ambiguous"`
	Ambiguities      []AmbiguityItem `json:"ambiguities,omitempty"`
	ResolvedQuestion string          `json:"resolved_question,omitempty"`
}

// AmbiguityItem describes one ambiguous term in the user's question.
type AmbiguityItem struct {
	Term            string           `json:"term"`
	Type            string           `json:"type"`
	Interpretations []Interpretation `json:"interpretations"`
}

// Interpretation describes one possible meaning for an ambiguous term.
type Interpretation struct {
	Label           string          `json:"label"`
	Description     string          `json:"description,omitempty"`
	SemanticMapping SemanticMapping `json:"semantic_mapping"`
	Confidence      float64         `json:"confidence"`
}

// SemanticMapping identifies the semantic target represented by an interpretation.
type SemanticMapping struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

// Analyze runs rule-based ambiguity detectors before LogicalQuery generation.
func Analyze(_ context.Context, question string, model *semantic.SemanticModel, glossary []prompt.GlossaryEntry) AmbiguityResult {
	ambiguities := mergeAmbiguities(
		DetectGlossary(question, glossary, model),
		DetectSynonyms(question, model),
	)
	ambiguities = filterAmbiguities(ambiguities, defaultConfidenceThreshold)
	return AmbiguityResult{
		IsAmbiguous: len(ambiguities) > 0,
		Ambiguities: ambiguities,
	}
}

func mergeAmbiguities(groups ...[]AmbiguityItem) []AmbiguityItem {
	byTerm := make(map[string]*AmbiguityItem)
	for _, group := range groups {
		for _, item := range group {
			term := strings.ToLower(strings.TrimSpace(item.Term))
			key := item.Type + "|" + term
			merged, ok := byTerm[key]
			if !ok {
				merged = &AmbiguityItem{
					Term: term,
					Type: item.Type,
				}
				byTerm[key] = merged
			}
			for _, interpretation := range item.Interpretations {
				mergeInterpretation(merged, interpretation)
			}
		}
	}

	keys := make([]string, 0, len(byTerm))
	for key := range byTerm {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	ambiguities := make([]AmbiguityItem, 0, len(keys))
	for _, key := range keys {
		item := byTerm[key]
		sort.Slice(item.Interpretations, func(i, j int) bool {
			return item.Interpretations[i].Label < item.Interpretations[j].Label
		})
		ambiguities = append(ambiguities, *item)
	}
	return ambiguities
}

func mergeInterpretation(item *AmbiguityItem, interpretation Interpretation) {
	key := strings.ToLower(interpretation.SemanticMapping.Type) + "|" + strings.ToLower(interpretation.SemanticMapping.Name)
	for i := range item.Interpretations {
		current := item.Interpretations[i]
		currentKey := strings.ToLower(current.SemanticMapping.Type) + "|" + strings.ToLower(current.SemanticMapping.Name)
		if currentKey != key {
			continue
		}
		if interpretation.Confidence > current.Confidence {
			item.Interpretations[i] = interpretation
		}
		return
	}
	item.Interpretations = append(item.Interpretations, interpretation)
}

func filterAmbiguities(ambiguities []AmbiguityItem, threshold float64) []AmbiguityItem {
	filtered := make([]AmbiguityItem, 0, len(ambiguities))
	for _, item := range ambiguities {
		interpretations := make([]Interpretation, 0, len(item.Interpretations))
		for _, interpretation := range item.Interpretations {
			if interpretation.Confidence >= threshold {
				interpretations = append(interpretations, interpretation)
			}
		}
		if len(interpretations) < 2 {
			continue
		}
		item.Interpretations = interpretations
		filtered = append(filtered, item)
	}
	return filtered
}
