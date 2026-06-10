package ambiguity

import (
	"strings"

	"github.com/biqly/biqly/internal/ai/lexicon"
	"github.com/biqly/biqly/internal/i18n"
	"github.com/biqly/biqly/internal/semantic"
)

// MatchTemporalPhrases returns the vague relative-time phrases ("geçen ay",
// "last month", ...) present in the question, in detector order. Callers use
// it to assert that a generated query actually carries a time condition.
// Phrases come from the NL lexicon (ai_nl_lexicon, domain temporal_phrase).
func MatchTemporalPhrases(question string) []string {
	normalized := strings.ToLower(strings.TrimSpace(question))
	if normalized == "" {
		return nil
	}
	var phrases []string
	for _, entry := range lexicon.Active().TemporalPhrases() {
		if strings.Contains(normalized, entry.Phrase) {
			phrases = append(phrases, entry.Phrase)
		}
	}
	return phrases
}

// DetectTemporal flags vague relative time phrases that need a concrete window.
func DetectTemporal(locale i18n.Locale, question string, model *semantic.SemanticModel) []Item {
	normalized := strings.ToLower(strings.TrimSpace(question))
	if normalized == "" {
		return nil
	}

	dateDim, hasDateDim := firstDateDimension(model)
	var ambiguities []Item
	for _, entry := range lexicon.Active().TemporalPhrases() {
		if !strings.Contains(normalized, entry.Phrase) {
			continue
		}
		interpretations := temporalInterpretations(locale, entry.InterpretationKeys, dateDim, hasDateDim)
		ambiguities = append(ambiguities, Item{
			Term:            entry.Phrase,
			Type:            "temporal",
			Interpretations: interpretations,
		})
	}
	return ambiguities
}

func temporalInterpretations(locale i18n.Locale, keys []string, dateDim string, hasDateDim bool) []Interpretation {
	out := make([]Interpretation, len(keys))
	for i, key := range keys {
		out[i] = Interpretation{
			Label:       i18n.T(locale, "clarification.temporal."+key+"_label"),
			Description: i18n.T(locale, "clarification.temporal."+key+"_desc"),
			Confidence:  1,
		}
		if hasDateDim {
			out[i].SemanticMapping = SemanticMapping{
				Type: "dimension",
				Name: dateDim,
			}
		}
	}
	return out
}

func firstDateDimension(model *semantic.SemanticModel) (string, bool) {
	if model == nil {
		return "", false
	}
	for _, dimension := range model.Dimensions {
		if dimension.Type == string(semantic.DimensionTypeDate) {
			return dimension.Name, true
		}
	}
	return "", false
}
