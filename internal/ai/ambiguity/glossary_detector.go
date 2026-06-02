package ambiguity

import (
	"sort"
	"strings"

	"github.com/biqly/biqly/internal/ai/prompt"
	"github.com/biqly/biqly/internal/ai/routing"
	"github.com/biqly/biqly/internal/semantic"
)

// DetectGlossary returns glossary terms that map to more than one semantic target.
func DetectGlossary(question string, entries []prompt.GlossaryEntry, model *semantic.SemanticModel) []AmbiguityItem {
	selected := prompt.SelectGlossaryForQuestion(question, entries, model)
	questionTokens := routing.TokenSet(question)
	byTerm := make(map[string][]prompt.GlossaryEntry)
	for _, entry := range selected {
		term := strings.ToLower(strings.TrimSpace(entry.Term))
		if term == "" || !glossaryTermMatches(questionTokens, term) {
			continue
		}
		byTerm[term] = append(byTerm[term], entry)
	}

	terms := make([]string, 0, len(byTerm))
	for term := range byTerm {
		terms = append(terms, term)
	}
	sort.Strings(terms)

	var ambiguities []AmbiguityItem
	for _, term := range terms {
		interpretations := glossaryInterpretations(byTerm[term])
		if len(interpretations) < 2 {
			continue
		}
		ambiguities = append(ambiguities, AmbiguityItem{
			Term:            term,
			Type:            "semantic",
			Interpretations: interpretations,
		})
	}
	return ambiguities
}

func glossaryTermMatches(questionTokens map[string]bool, term string) bool {
	for token := range routing.TokenSet(term) {
		if questionTokens[token] {
			return true
		}
	}
	return false
}

func glossaryInterpretations(entries []prompt.GlossaryEntry) []Interpretation {
	seen := make(map[string]struct{})
	var interpretations []Interpretation
	for _, entry := range entries {
		key := strings.ToLower(entry.MapsToType) + "|" + strings.ToLower(entry.MapsToName)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		interpretations = append(interpretations, Interpretation{
			Label:       entry.Definition,
			Description: entry.Definition,
			SemanticMapping: SemanticMapping{
				Type: entry.MapsToType,
				Name: entry.MapsToName,
			},
			Confidence: 1,
		})
	}
	sort.Slice(interpretations, func(i, j int) bool {
		return interpretations[i].Label < interpretations[j].Label
	})
	return interpretations
}
