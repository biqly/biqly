// Package ambiguity detects semantic ambiguity before LogicalQuery generation.
package ambiguity

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/biqly/biqly/internal/ai/prompt"
	"github.com/biqly/biqly/internal/i18n"
	"github.com/biqly/biqly/internal/semantic"
)

const defaultConfidenceThreshold = 0.70
const ruleBasedAnalysisTimeout = 2 * time.Second

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
func Analyze(ctx context.Context, question string, model *semantic.SemanticModel, glossary []prompt.GlossaryEntry, confidenceThreshold float64) AmbiguityResult {
	locale := i18n.FromContext(ctx)
	return analyzeWithDetectors(ctx, []func() []AmbiguityItem{
		func() []AmbiguityItem { return DetectGlossary(question, glossary, model) },
		func() []AmbiguityItem { return DetectSynonyms(locale, question, model) },
		func() []AmbiguityItem { return DetectTemporal(locale, question, model) },
		func() []AmbiguityItem { return DetectScope(locale, question, model) },
	}, confidenceThreshold, ruleBasedAnalysisTimeout)
}

func analyzeWithDetectors(ctx context.Context, detectors []func() []AmbiguityItem, confidenceThreshold float64, timeout time.Duration) AmbiguityResult {
	if confidenceThreshold <= 0 {
		confidenceThreshold = defaultConfidenceThreshold
	}
	analysisCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	type detectorResult struct {
		index int
		group []AmbiguityItem
	}
	results := make(chan detectorResult, len(detectors))
	for index, detector := range detectors {
		go func() {
			result := detectorResult{index: index, group: detector()}
			select {
			case results <- result:
			case <-analysisCtx.Done():
			}
		}()
	}

	groups := make([][]AmbiguityItem, len(detectors))
	completed := 0
	for completed < len(detectors) {
		select {
		case result := <-results:
			groups[result.index] = result.group
			completed++
		case <-analysisCtx.Done():
			return ambiguityResult(groups, confidenceThreshold)
		}
	}
	return ambiguityResult(groups, confidenceThreshold)
}

func ambiguityResult(groups [][]AmbiguityItem, confidenceThreshold float64) AmbiguityResult {
	ambiguities := mergeAmbiguities(groups...)
	ambiguities = filterAmbiguities(ambiguities, confidenceThreshold)
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
	key := interpretationKey(item.Type, interpretation)
	for i := range item.Interpretations {
		current := item.Interpretations[i]
		currentKey := interpretationKey(item.Type, current)
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

func interpretationKey(ambiguityType string, interpretation Interpretation) string {
	key := strings.ToLower(interpretation.SemanticMapping.Type) + "|" + strings.ToLower(interpretation.SemanticMapping.Name)
	if ambiguityType == "temporal" {
		key += "|" + strings.ToLower(interpretation.Label)
	}
	return key
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
