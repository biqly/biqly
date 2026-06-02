package ambiguity

import (
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/biqly/biqly/internal/ai/routing"
	"github.com/biqly/biqly/internal/semantic"
)

type synonymTarget struct {
	kind        string
	name        string
	label       string
	description string
	confidence  float64
}

// DetectSynonyms returns question synonyms that map to more than one semantic field.
func DetectSynonyms(question string, model *semantic.SemanticModel) []AmbiguityItem {
	if model == nil {
		return nil
	}

	question = normalizeSynonym(question)
	questionTokens := routing.TokenSet(question)
	bySynonym := make(map[string][]synonymTarget)
	for _, dimension := range model.Dimensions {
		addSynonymTargets(bySynonym, question, questionTokens, "dimension", dimension.Name, dimension.Label, dimension.Description, dimension.Synonyms)
	}
	for _, metric := range model.Metrics {
		addSynonymTargets(bySynonym, question, questionTokens, "metric", metric.Name, metric.Label, metric.Description, metric.Synonyms)
	}

	terms := make([]string, 0, len(bySynonym))
	for term := range bySynonym {
		terms = append(terms, term)
	}
	sort.Strings(terms)

	var ambiguities []AmbiguityItem
	for _, term := range terms {
		interpretations := synonymInterpretations(bySynonym[term])
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

func addSynonymTargets(bySynonym map[string][]synonymTarget, question string, questionTokens map[string]bool, kind, name string, label, description *string, synonyms []string) {
	for _, synonym := range synonyms {
		synonym = normalizeSynonym(synonym)
		confidence := synonymMatchConfidence(question, questionTokens, synonym)
		if confidence == 0 {
			continue
		}
		bySynonym[synonym] = append(bySynonym[synonym], synonymTarget{
			kind:        kind,
			name:        name,
			label:       stringValueOr(label, name),
			description: stringValueOr(description, kind+" "+name),
			confidence:  confidence,
		})
	}
}

func synonymMatchConfidence(question string, questionTokens map[string]bool, synonym string) float64 {
	if synonym == "" {
		return 0
	}
	if strings.Contains(question, synonym) {
		return 1
	}

	synonymTokens := routing.TokenSet(synonym)
	if len(synonymTokens) != 1 {
		return 0
	}
	var synonymToken string
	for synonymToken = range synonymTokens {
	}
	synonymRunes := utf8.RuneCountInString(synonymToken)
	if synonymRunes < 4 {
		return 0
	}

	var best float64
	for questionToken := range questionTokens {
		distance := levenshteinDistance(questionToken, synonymToken)
		if distance != 1 {
			continue
		}
		confidence := 1 - float64(distance)/float64(synonymRunes)
		if confidence > best {
			best = confidence
		}
	}
	return best
}

func levenshteinDistance(left, right string) int {
	leftRunes := []rune(left)
	rightRunes := []rune(right)
	previous := make([]int, len(rightRunes)+1)
	current := make([]int, len(rightRunes)+1)
	for i := range previous {
		previous[i] = i
	}
	for i, leftRune := range leftRunes {
		current[0] = i + 1
		for j, rightRune := range rightRunes {
			cost := 1
			if leftRune == rightRune {
				cost = 0
			}
			current[j+1] = min(
				current[j]+1,
				previous[j+1]+1,
				previous[j]+cost,
			)
		}
		previous, current = current, previous
	}
	return previous[len(rightRunes)]
}

func synonymInterpretations(targets []synonymTarget) []Interpretation {
	seen := make(map[string]struct{})
	var interpretations []Interpretation
	for _, target := range targets {
		key := target.kind + "|" + target.name
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		interpretations = append(interpretations, Interpretation{
			Label:       target.label,
			Description: target.description,
			SemanticMapping: SemanticMapping{
				Type: target.kind,
				Name: target.name,
			},
			Confidence: target.confidence,
		})
	}
	sort.Slice(interpretations, func(i, j int) bool {
		return interpretations[i].Label < interpretations[j].Label
	})
	return interpretations
}

func normalizeSynonym(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func stringValueOr(value *string, fallback string) string {
	if value == nil || strings.TrimSpace(*value) == "" {
		return fallback
	}
	return strings.TrimSpace(*value)
}
