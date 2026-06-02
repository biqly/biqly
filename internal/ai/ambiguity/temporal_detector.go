package ambiguity

import (
	"strings"

	"github.com/biqly/biqly/internal/i18n"
	"github.com/biqly/biqly/internal/semantic"
)

type vagueTemporal struct {
	phrase             string
	interpretationKeys []string
}

var vagueTemporalPhrases = []vagueTemporal{
	{phrase: "geçen ay", interpretationKeys: []string{"prev_calendar_month", "rolling_30d"}},
	{phrase: "son zamanlarda", interpretationKeys: []string{"last_week", "last_month", "last_quarter"}},
	{phrase: "yakın zamanda", interpretationKeys: []string{"last_week", "last_month"}},
	{phrase: "geçen hafta", interpretationKeys: []string{"prev_calendar_week", "rolling_7d"}},
	{phrase: "bu yıl", interpretationKeys: []string{"ytd", "last_12m"}},
	{phrase: "last month", interpretationKeys: []string{"prev_calendar_month", "rolling_30d"}},
	{phrase: "recently", interpretationKeys: []string{"last_week", "last_month"}},
	{phrase: "lately", interpretationKeys: []string{"last_week", "last_month"}},
	{phrase: "last week", interpretationKeys: []string{"prev_calendar_week", "rolling_7d"}},
}

// DetectTemporal flags vague relative time phrases that need a concrete window.
func DetectTemporal(locale i18n.Locale, question string, model *semantic.SemanticModel) []AmbiguityItem {
	normalized := strings.ToLower(strings.TrimSpace(question))
	if normalized == "" {
		return nil
	}

	dateDim, hasDateDim := firstDateDimension(model)
	var ambiguities []AmbiguityItem
	for _, entry := range vagueTemporalPhrases {
		if !strings.Contains(normalized, entry.phrase) {
			continue
		}
		interpretations := temporalInterpretations(locale, entry.interpretationKeys, dateDim, hasDateDim)
		ambiguities = append(ambiguities, AmbiguityItem{
			Term:            entry.phrase,
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
