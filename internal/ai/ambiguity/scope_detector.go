package ambiguity

import (
	"sort"
	"strings"

	"github.com/biqly/biqly/internal/ai/routing"
	"github.com/biqly/biqly/internal/semantic"
)

var scopeQualifiers = []string{
	"büyük", "yüksek", "düşük", "az", "çok", "fazla", "küçük",
	"large", "big", "high", "low", "small", "many", "few", "top",
}

// DetectScope flags qualitative qualifiers when it is unclear which metric they modify.
func DetectScope(question string, model *semantic.SemanticModel) []AmbiguityItem {
	if model == nil || len(model.Metrics) < 2 {
		return nil
	}

	qualifier, ok := findScopeQualifier(question)
	if !ok {
		return nil
	}

	questionTokens := routing.TokenSet(strings.ToLower(question))
	matched := matchedMetrics(questionTokens, model.Metrics)
	if len(matched) == 1 {
		return nil
	}

	targets := matched
	if len(targets) == 0 {
		targets = model.Metrics
	}
	if len(targets) > 5 {
		targets = targets[:5]
	}
	if len(targets) < 2 {
		return nil
	}

	interpretations := make([]Interpretation, 0, len(targets))
	for _, metric := range targets {
		label := stringValueOr(metric.Label, metric.Name)
		interpretations = append(interpretations, Interpretation{
			Label:       qualifier + " — " + label,
			Description: "Apply the qualifier to metric " + metric.Name,
			SemanticMapping: SemanticMapping{
				Type: "metric",
				Name: metric.Name,
			},
			Confidence: 1,
		})
	}
	sort.Slice(interpretations, func(i, j int) bool {
		return interpretations[i].Label < interpretations[j].Label
	})

	return []AmbiguityItem{{
		Term:            qualifier,
		Type:            "scope",
		Interpretations: interpretations,
	}}
}

func findScopeQualifier(question string) (string, bool) {
	normalized := strings.ToLower(strings.TrimSpace(question))
	if normalized == "" {
		return "", false
	}
	tokens := routing.TokenSet(normalized)
	for _, qualifier := range scopeQualifiers {
		if tokens[qualifier] || strings.Contains(normalized, qualifier) {
			return qualifier, true
		}
	}
	return "", false
}

func matchedMetrics(questionTokens map[string]bool, metrics []semantic.Metric) []semantic.Metric {
	var matched []semantic.Metric
	for _, metric := range metrics {
		if metricMatchesQuestion(questionTokens, metric) {
			matched = append(matched, metric)
		}
	}
	sort.Slice(matched, func(i, j int) bool {
		return matched[i].Name < matched[j].Name
	})
	return matched
}

func metricMatchesQuestion(questionTokens map[string]bool, metric semantic.Metric) bool {
	name := strings.ToLower(strings.TrimSpace(metric.Name))
	if name != "" && questionTokens[name] {
		return true
	}
	if metric.Label != nil {
		label := strings.ToLower(strings.TrimSpace(*metric.Label))
		if label != "" && questionTokens[label] {
			return true
		}
	}
	for _, synonym := range metric.Synonyms {
		synonym = strings.ToLower(strings.TrimSpace(synonym))
		if synonym != "" && questionTokens[synonym] {
			return true
		}
	}
	return false
}
