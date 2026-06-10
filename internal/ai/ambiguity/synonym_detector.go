package ambiguity

import (
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/biqly/biqly/internal/ai/routing"
	"github.com/biqly/biqly/internal/i18n"
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
func DetectSynonyms(locale i18n.Locale, question string, model *semantic.SemanticModel) []Item {
	if model == nil {
		return nil
	}

	question = normalizeSynonym(question)
	questionTokens := routing.TokenSet(question)
	bySynonym := make(map[string][]synonymTarget)
	for _, dimension := range model.Dimensions {
		// Date-grain dimensions (TimeGrain set) carry bucketing words ("ay",
		// "month", ...) as synonyms; every timestamp column gets the same set,
		// so collisions among them are noise, not business-term ambiguity.
		if dimension.TimeGrain != "" {
			continue
		}
		addSynonymTargets(locale, bySynonym, question, questionTokens, "dimension", dimension.Name, dimension.Label, dimension.Description, dimension.Synonyms)
	}
	for _, metric := range model.Metrics {
		addSynonymTargets(locale, bySynonym, question, questionTokens, "metric", metric.Name, metric.Label, metric.Description, metric.Synonyms)
	}

	terms := make([]string, 0, len(bySynonym))
	for term := range bySynonym {
		terms = append(terms, term)
	}
	sort.Strings(terms)

	var ambiguities []Item
	for _, term := range terms {
		interpretations := synonymInterpretations(bySynonym[term])
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

func addSynonymTargets(locale i18n.Locale, bySynonym map[string][]synonymTarget, question string, questionTokens map[string]struct{}, kind, name string, label, description *string, synonyms []string) {
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
			description: stringValueOr(description, i18n.Tf(locale, "clarification.synonym_fallback_"+kind, map[string]any{"Name": name})),
			confidence:  confidence,
		})
	}
}

// Synonym token length thresholds. Single-character synonym tokens are too
// generic to anchor a clarification, and fuzzy (single edit) matching is only
// trustworthy on reasonably long tokens — otherwise short, generic time-grain
// words ("ay", "day", "days") produce low-value clarifications.
const (
	minExactSynonymTokenRunes = 2
	minFuzzySynonymTokenRunes = 4
)

func synonymMatchConfidence(question string, questionTokens map[string]struct{}, synonym string) float64 {
	if synonym == "" {
		return 0
	}

	synonymTokens := routing.TokenSet(synonym)
	if len(synonymTokens) == 0 {
		return 0
	}

	// Multi-word synonyms are specific enough to match as a contiguous phrase.
	if len(synonymTokens) > 1 {
		if strings.Contains(question, synonym) {
			return 1
		}
		return 0
	}

	// Single-token synonyms must align with a whole question token. Substring
	// matching (e.g. "ay" inside "kayıt") spuriously flags generic tokens and
	// drives repeated low-value clarifications.
	var synonymToken string
	for synonymToken = range synonymTokens {
	}
	synonymRunes := utf8.RuneCountInString(synonymToken)
	if synonymRunes < minExactSynonymTokenRunes {
		return 0
	}
	if _, ok := questionTokens[synonymToken]; ok {
		return 1
	}

	// Fuzzy single-edit match tolerates typos, but only on longer tokens.
	if synonymRunes < minFuzzySynonymTokenRunes {
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
