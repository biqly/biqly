package ambiguity

import (
	"sort"
	"strings"

	"github.com/biqly/biqly/internal/ai/prompt"
	"github.com/biqly/biqly/internal/ai/routing"
	"github.com/biqly/biqly/internal/semantic"
)

// DetectGlossary returns glossary terms that map to more than one semantic target.
func DetectGlossary(question string, entries []prompt.GlossaryEntry, model *semantic.SemanticModel) []Item {
	selected := prompt.SelectGlossaryForQuestion(question, entries, model)
	questionTokens := routing.TokenSet(question)
	byTerm := make(map[string][]prompt.GlossaryEntry)
	for _, entry := range selected {
		term := strings.ToLower(strings.TrimSpace(entry.Term))
		if term == "" || !glossaryTermMatches(questionTokens, term) {
			continue
		}
		// A "model" entry maps a term to the whole dataset/entity (the query
		// subject), not a substitutable field. Treating it as a competing
		// interpretation produces false ambiguities (e.g. the entity word
		// "tweet" colliding with a "tweet_id" dimension), so skip it.
		if strings.EqualFold(strings.TrimSpace(entry.MapsToType), "model") {
			continue
		}
		byTerm[term] = append(byTerm[term], entry)
	}

	terms := make([]string, 0, len(byTerm))
	for term := range byTerm {
		terms = append(terms, term)
	}
	sort.Strings(terms)

	var ambiguities []Item
	for _, term := range terms {
		interpretations := glossaryInterpretations(byTerm[term])
		if len(interpretations) < 2 {
			continue
		}
		ambiguities = append(ambiguities, Item{
			Term:            term,
			Type:            "semantic",
			Interpretations: interpretations,
		})
	}
	return ambiguities
}

func glossaryTermMatches(questionTokens map[string]struct{}, term string) bool {
	for token := range routing.TokenSet(term) {
		if _, ok := questionTokens[token]; !ok {
			return false
		}
	}
	return true
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
