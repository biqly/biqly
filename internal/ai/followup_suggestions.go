package ai

import (
	"slices"
	"strings"
	"unicode"
)

// maxSuggestedFollowUps caps the number of follow-up suggestions returned to
// the client so the chat UI never needs to truncate the chip row itself.
const maxSuggestedFollowUps = 3

// similarityMinLength is the minimum length of the shorter of two normalized
// strings before a substring-containment match counts as a near-duplicate.
// Below this length, short strings (e.g. "Trend") would otherwise trigger
// false-positive matches against unrelated longer suggestions.
const similarityMinLength = 16

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
		seenQuestions[normalizeSuggestionText(q)] = struct{}{}
	}
	seenLabels := make(map[string]struct{}, len(candidates))

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

		normalizedQuestion := normalizeSuggestionText(candidate.Question)
		if isNearDuplicate(seenQuestions, normalizedQuestion) {
			continue
		}

		normalizedLabel := normalizeSuggestionText(candidate.Label)
		if isNearDuplicate(seenLabels, normalizedLabel) {
			continue
		}

		seenQuestions[normalizedQuestion] = struct{}{}
		seenLabels[normalizedLabel] = struct{}{}

		validated = append(validated, candidate)
	}
	return validated
}

// isNearDuplicate reports whether normalized is either an exact match of a
// previously seen string, or a near-duplicate under the similarity MVP rule:
// one of the two strings contains the other in full, and the shorter of the
// pair is at least similarityMinLength runes long. The length floor avoids
// treating short, generic strings (e.g. "Trend") as duplicates of unrelated
// longer ones that merely happen to contain the same short substring.
func isNearDuplicate(seen map[string]struct{}, normalized string) bool {
	if _, exact := seen[normalized]; exact {
		return true
	}
	for prior := range seen {
		if isSimilarBySubstring(prior, normalized) {
			return true
		}
	}
	return false
}

// isSimilarBySubstring implements the containment half of the similarity
// MVP: strings.Contains(prior, candidate) or strings.Contains(candidate,
// prior) count as a duplicate once the shorter side reaches
// similarityMinLength.
func isSimilarBySubstring(a, b string) bool {
	shorter, longer := a, b
	if len(b) < len(a) {
		shorter, longer = b, a
	}
	if len(shorter) < similarityMinLength {
		return false
	}
	return strings.Contains(longer, shorter)
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

// normalizeSuggestionText lowercases, trims, collapses internal whitespace
// runs to a single space, and drops simple punctuation, so that suggestions
// which differ only in casing, spacing, or trailing punctuation compare as
// duplicates.
func normalizeSuggestionText(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))

	var b strings.Builder
	b.Grow(len(s))
	pendingSpace := false
	for _, r := range s {
		switch {
		case unicode.IsSpace(r):
			pendingSpace = b.Len() > 0
		case unicode.IsPunct(r):
			// Drop simple punctuation entirely.
		default:
			if pendingSpace {
				b.WriteByte(' ')
				pendingSpace = false
			}
			b.WriteRune(r)
		}
	}
	return b.String()
}
