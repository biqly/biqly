package ambiguity

import (
	"strings"

	"github.com/biqly/biqly/internal/semantic"
)

type vagueTemporal struct {
	phrase          string
	interpretations []Interpretation
}

var vagueTemporalPhrases = []vagueTemporal{
	{
		phrase: "geçen ay",
		interpretations: []Interpretation{
			{Label: "Geçen takvim ayı", Description: "Önceki ayın ilk gününden son gününe kadar"},
			{Label: "Son 30 gün", Description: "Bugünden geriye 30 günlük pencere"},
		},
	},
	{
		phrase: "son zamanlarda",
		interpretations: []Interpretation{
			{Label: "Son 7 gün", Description: "Bugünden geriye bir hafta"},
			{Label: "Son 30 gün", Description: "Bugünden geriye bir ay"},
			{Label: "Son 90 gün", Description: "Bugünden geriye üç ay"},
		},
	},
	{
		phrase: "yakın zamanda",
		interpretations: []Interpretation{
			{Label: "Son 7 gün", Description: "Bugünden geriye bir hafta"},
			{Label: "Son 30 gün", Description: "Bugünden geriye bir ay"},
		},
	},
	{
		phrase: "geçen hafta",
		interpretations: []Interpretation{
			{Label: "Geçen takvim haftası", Description: "Önceki haftanın Pazartesi–Pazar aralığı"},
			{Label: "Son 7 gün", Description: "Bugünden geriye 7 günlük pencere"},
		},
	},
	{
		phrase: "bu yıl",
		interpretations: []Interpretation{
			{Label: "Yıl başından bugüne", Description: "1 Ocak'tan bugüne kadar"},
			{Label: "Son 12 ay", Description: "Bugünden geriye 12 aylık pencere"},
		},
	},
	{
		phrase: "last month",
		interpretations: []Interpretation{
			{Label: "Previous calendar month", Description: "First through last day of the prior month"},
			{Label: "Last 30 days", Description: "Rolling 30-day window ending today"},
		},
	},
	{
		phrase: "recently",
		interpretations: []Interpretation{
			{Label: "Last 7 days", Description: "Rolling one-week window"},
			{Label: "Last 30 days", Description: "Rolling one-month window"},
		},
	},
	{
		phrase: "lately",
		interpretations: []Interpretation{
			{Label: "Last 7 days", Description: "Rolling one-week window"},
			{Label: "Last 30 days", Description: "Rolling one-month window"},
		},
	},
	{
		phrase: "last week",
		interpretations: []Interpretation{
			{Label: "Previous calendar week", Description: "Monday through Sunday of the prior week"},
			{Label: "Last 7 days", Description: "Rolling 7-day window ending today"},
		},
	},
}

// DetectTemporal flags vague relative time phrases that need a concrete window.
func DetectTemporal(question string, model *semantic.SemanticModel) []AmbiguityItem {
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
		interpretations := attachDateMapping(entry.interpretations, dateDim, hasDateDim)
		ambiguities = append(ambiguities, AmbiguityItem{
			Term:            entry.phrase,
			Type:            "temporal",
			Interpretations: interpretations,
		})
	}
	return ambiguities
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

func attachDateMapping(interpretations []Interpretation, dateDim string, hasDateDim bool) []Interpretation {
	out := make([]Interpretation, len(interpretations))
	for i, interpretation := range interpretations {
		out[i] = interpretation
		out[i].Confidence = 1
		if hasDateDim {
			out[i].SemanticMapping = SemanticMapping{
				Type: "dimension",
				Name: dateDim,
			}
		}
	}
	return out
}
