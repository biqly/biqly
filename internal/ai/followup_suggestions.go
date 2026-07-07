package ai

import (
	"slices"
	"strings"
)

// maxSuggestedFollowUps caps the number of follow-up suggestions returned to
// the client so the chat UI never needs to truncate the chip row itself.
const maxSuggestedFollowUps = 3

// validSuggestedFollowUpKinds enumerates the kinds ValidateSuggestedFollowUps
// accepts. Anything else (e.g. a hallucinated kind from an AI rewrite phase)
// is dropped.
var validSuggestedFollowUpKinds = []SuggestedFollowUpKind{
	SuggestedFollowUpBreakdown,
	SuggestedFollowUpComparison,
	SuggestedFollowUpTrend,
	SuggestedFollowUpChart,
	SuggestedFollowUpDrilldown,
	SuggestedFollowUpFilter,
	SuggestedFollowUpExplain,
}

// ValidateSuggestedFollowUps is the final authority for follow-up suggestions
// before they reach a response: every candidate — whether deterministically
// built or AI-rewritten — is re-checked against the fields actually
// available on the current result and against questions the user already
// asked, then capped at maxSuggestedFollowUps. Candidates are kept in their
// input order; invalid or duplicate candidates are dropped rather than
// repaired.
func ValidateSuggestedFollowUps(
	candidates []SuggestedFollowUp,
	availableFields []string,
	priorQuestions []string,
) []SuggestedFollowUp {
	availableFieldSet := make(map[string]struct{}, len(availableFields))
	for _, field := range availableFields {
		availableFieldSet[strings.TrimSpace(field)] = struct{}{}
	}

	seenQuestions := make(map[string]struct{}, len(priorQuestions))
	for _, q := range priorQuestions {
		seenQuestions[normalizeFollowUpQuestion(q)] = struct{}{}
	}

	validated := make([]SuggestedFollowUp, 0, min(len(candidates), maxSuggestedFollowUps))
	for _, candidate := range candidates {
		if len(validated) >= maxSuggestedFollowUps {
			break
		}

		candidate.ID = strings.TrimSpace(candidate.ID)
		candidate.Label = strings.TrimSpace(candidate.Label)
		candidate.Question = strings.TrimSpace(candidate.Question)
		candidate.Reason = strings.TrimSpace(candidate.Reason)
		if candidate.ID == "" || candidate.Label == "" || candidate.Question == "" {
			continue
		}

		if !slices.Contains(validSuggestedFollowUpKinds, candidate.Kind) {
			continue
		}

		requires, ok := trimmedRequiresWithinFieldSet(candidate.Requires, availableFieldSet)
		if !ok {
			continue
		}
		candidate.Requires = requires

		normalizedQuestion := normalizeFollowUpQuestion(candidate.Question)
		if _, duplicate := seenQuestions[normalizedQuestion]; duplicate {
			continue
		}
		seenQuestions[normalizedQuestion] = struct{}{}

		validated = append(validated, candidate)
	}
	return validated
}

// trimmedRequiresWithinFieldSet trims each required field name and reports
// ok=false if any non-empty entry is not in availableFieldSet — the
// candidate must be dropped entirely rather than silently losing a
// dependency the UI relied on to decide the suggestion was safe.
func trimmedRequiresWithinFieldSet(requires []string, availableFieldSet map[string]struct{}) ([]string, bool) {
	trimmed := make([]string, 0, len(requires))
	for _, field := range requires {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		if _, ok := availableFieldSet[field]; !ok {
			return nil, false
		}
		trimmed = append(trimmed, field)
	}
	return trimmed, true
}

func normalizeFollowUpQuestion(question string) string {
	return strings.ToLower(strings.TrimSpace(question))
}
