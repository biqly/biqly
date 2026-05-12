package semantic

import "fmt"

// ContextBudget caps the size of a published semantic context. Limits exist to
// keep the AI prompt within model context windows and to surface ballooning
// schemas at publish time rather than at runtime.
//
// All zero values mean "unlimited" so callers can pass a partially populated
// budget. Use DefaultContextBudget for a starting point.
type ContextBudget struct {
	// MaxModels caps the number of base models contributing to a single
	// published context. Reserved for composite models; ignored for base
	// semantic models (always 1).
	MaxModels int `json:"max_models,omitempty"`
	// MaxDimensions caps the number of dimensions exposed by the model.
	MaxDimensions int `json:"max_dimensions,omitempty"`
	// MaxMetrics caps the number of metrics exposed by the model.
	MaxMetrics int `json:"max_metrics,omitempty"`
	// MaxJoins caps the number of joins reachable from the base table.
	MaxJoins int `json:"max_joins,omitempty"`
	// MaxPromptChars caps the rune-length of the AI prompt assembled from this
	// context. Soft check: prompt builder still trims dynamically; this guard
	// flags published contexts that will routinely overflow.
	MaxPromptChars int `json:"max_prompt_chars,omitempty"`
}

// DefaultContextBudget returns conservative limits that work for typical BI
// schemas. Callers can override individual fields without re-specifying the
// rest.
func DefaultContextBudget() ContextBudget {
	return ContextBudget{
		MaxModels:      3,
		MaxDimensions:  80,
		MaxMetrics:     40,
		MaxJoins:       30,
		MaxPromptChars: 20000,
	}
}

// EnforceBudget reports limit breaches for the supplied model. Returned strings
// are user-facing warnings (publish-blocking; callers decide whether to lift to
// errors). promptChars is the estimated prompt size; pass 0 to skip that check.
func EnforceBudget(model SemanticModel, budget ContextBudget, promptChars int) []string {
	var warnings []string
	if budget.MaxDimensions > 0 && len(model.Dimensions) > budget.MaxDimensions {
		warnings = append(warnings, fmt.Sprintf(
			"semantic context exceeds max_dimensions budget: %d > %d",
			len(model.Dimensions), budget.MaxDimensions,
		))
	}
	if budget.MaxMetrics > 0 && len(model.Metrics) > budget.MaxMetrics {
		warnings = append(warnings, fmt.Sprintf(
			"semantic context exceeds max_metrics budget: %d > %d",
			len(model.Metrics), budget.MaxMetrics,
		))
	}
	if budget.MaxJoins > 0 && len(model.Joins) > budget.MaxJoins {
		warnings = append(warnings, fmt.Sprintf(
			"semantic context exceeds max_joins budget: %d > %d",
			len(model.Joins), budget.MaxJoins,
		))
	}
	if budget.MaxPromptChars > 0 && promptChars > budget.MaxPromptChars {
		warnings = append(warnings, fmt.Sprintf(
			"semantic context exceeds max_prompt_chars budget: %d > %d (prompt will be auto-trimmed at query time)",
			promptChars, budget.MaxPromptChars,
		))
	}
	return warnings
}
