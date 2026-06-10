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

// Result AmbiguityResult describes whether a question needs user clarification.
type Result struct {
	IsAmbiguous      bool   `json:"is_ambiguous"`
	Ambiguities      []Item `json:"ambiguities,omitempty"`
	ResolvedQuestion string `json:"resolved_question,omitempty"`
}

// Item AmbiguityItem describes one ambiguous term in the user's question.
type Item struct {
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

// AnalyzeSynonymHomonym runs tier-1 detectors: glossary collisions, synonym/homonym
// matches, and vague temporal phrases — all deterministic and free. Scope detection
// stays out of tier 1 because its heuristics are noisier.
func AnalyzeSynonymHomonym(ctx context.Context, question string, model *semantic.SemanticModel, glossary []prompt.GlossaryEntry, confidenceThreshold float64) Result {
	locale := i18n.FromContext(ctx)
	return analyzeWithDetectors(ctx, []func(context.Context) []Item{
		func(context.Context) []Item { return DetectGlossary(question, glossary, model) },
		func(context.Context) []Item { return DetectSynonyms(locale, question, model) },
		func(context.Context) []Item { return DetectTemporal(locale, question, model) },
	}, confidenceThreshold, ruleBasedAnalysisTimeout)
}

// Analyze runs all rule-based ambiguity detectors before LogicalQuery generation.
func Analyze(ctx context.Context, question string, model *semantic.SemanticModel, glossary []prompt.GlossaryEntry, confidenceThreshold float64) Result {
	locale := i18n.FromContext(ctx)
	return analyzeWithDetectors(ctx, []func(context.Context) []Item{
		func(context.Context) []Item { return DetectGlossary(question, glossary, model) },
		func(context.Context) []Item { return DetectSynonyms(locale, question, model) },
		func(context.Context) []Item { return DetectTemporal(locale, question, model) },
		func(context.Context) []Item { return DetectScope(locale, question, model) },
	}, confidenceThreshold, ruleBasedAnalysisTimeout)
}

func analyzeWithDetectors(ctx context.Context, detectors []func(context.Context) []Item, confidenceThreshold float64, timeout time.Duration) Result {
	if confidenceThreshold <= 0 {
		confidenceThreshold = defaultConfidenceThreshold
	}
	analysisCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	type detectorResult struct {
		index int
		group []Item
	}
	results := make(chan detectorResult, len(detectors))
	for index, detector := range detectors {
		go func() {
			select {
			case <-analysisCtx.Done():
				return
			default:
			}
			result := detectorResult{index: index, group: detector(analysisCtx)}
			select {
			case results <- result:
			case <-analysisCtx.Done():
			}
		}()
	}

	groups := make([][]Item, len(detectors))
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

func ambiguityResult(groups [][]Item, confidenceThreshold float64) Result {
	ambiguities := mergeAmbiguities(groups...)
	ambiguities = filterAmbiguities(ambiguities, confidenceThreshold)
	return Result{
		IsAmbiguous: len(ambiguities) > 0,
		Ambiguities: ambiguities,
	}
}

func mergeAmbiguities(groups ...[]Item) []Item {
	byTerm := make(map[string]*Item)
	for _, group := range groups {
		for _, item := range group {
			term := strings.ToLower(strings.TrimSpace(item.Term))
			key := item.Type + "|" + term
			merged, ok := byTerm[key]
			if !ok {
				merged = &Item{
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

	ambiguities := make([]Item, 0, len(keys))
	for _, key := range keys {
		item := byTerm[key]
		sort.Slice(item.Interpretations, func(i, j int) bool {
			return item.Interpretations[i].Label < item.Interpretations[j].Label
		})
		ambiguities = append(ambiguities, *item)
	}
	return ambiguities
}

func mergeInterpretation(item *Item, interpretation Interpretation) {
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

func filterAmbiguities(ambiguities []Item, threshold float64) []Item {
	filtered := make([]Item, 0, len(ambiguities))
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
