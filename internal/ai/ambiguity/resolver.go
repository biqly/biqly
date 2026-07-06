// Package ambiguity implements the ambiguity resolution service.
package ambiguity

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/biqly/biqly/internal/ai/prompt"
	"github.com/biqly/biqly/internal/semantic"
)

// ErrInvalidClarificationChoice marks a stale or malformed ambiguity selection.
var ErrInvalidClarificationChoice = errors.New("invalid ambiguity clarification choice")

// Resolve analyzes the original question and applies a selected interpretation.
func Resolve(ctx context.Context, question, choice string, model *semantic.SemanticModel, glossary []prompt.GlossaryEntry) (string, error) {
	return ResolveChoice(question, choice, Analyze(ctx, question, model, glossary, 0))
}

// HasRemaining reports whether the question still has unresolved ambiguities after
// partial term resolution.
func HasRemaining(ctx context.Context, question string, model *semantic.SemanticModel, glossary []prompt.GlossaryEntry) bool {
	result := Analyze(ctx, question, model, glossary, 0)
	return result.IsAmbiguous
}

// ResolveChoice rewrites a question using the selected ambiguity interpretation.
func ResolveChoice(question, choice string, result Result) (string, error) {
	ambiguityIndex, interpretationIndex, err := parseClarificationChoice(choice)
	if err != nil {
		return "", err
	}
	if !result.IsAmbiguous || ambiguityIndex >= len(result.Ambiguities) {
		return "", ErrInvalidClarificationChoice
	}
	item := result.Ambiguities[ambiguityIndex]
	if interpretationIndex >= len(item.Interpretations) {
		return "", ErrInvalidClarificationChoice
	}

	rewritten, ok := replaceTermWithInterpretation(question, item.Term, item.Interpretations[interpretationIndex])
	if !ok {
		return "", fmt.Errorf("%w: term %q not found in question", ErrInvalidClarificationChoice, item.Term)
	}
	return rewritten, nil
}

// replaceTermWithInterpretation rewrites every case-insensitive occurrence of
// term in question with the interpretation's label (falling back to the semantic
// mapping name). ok is false when there is nothing to substitute or the term is
// absent, so callers can decide whether that is an error or a no-op.
func replaceTermWithInterpretation(question, term string, interpretation Interpretation) (string, bool) {
	replacement := strings.TrimSpace(interpretation.Label)
	if replacement == "" {
		replacement = strings.TrimSpace(interpretation.SemanticMapping.Name)
	}
	if replacement == "" || strings.TrimSpace(term) == "" {
		return question, false
	}
	pattern := regexp.MustCompile(`(?i)` + regexp.QuoteMeta(term))
	if !pattern.MatchString(question) {
		return question, false
	}
	return pattern.ReplaceAllStringFunc(question, func(string) string {
		return replacement
	}), true
}

func parseClarificationChoice(choice string) (int, int, error) {
	parts := strings.Split(choice, ":")
	if len(parts) != 3 || parts[0] != "ambiguity" {
		return 0, 0, ErrInvalidClarificationChoice
	}
	ambiguityIndex, err := strconv.Atoi(parts[1])
	if err != nil || ambiguityIndex < 0 {
		return 0, 0, ErrInvalidClarificationChoice
	}
	interpretationIndex, err := strconv.Atoi(parts[2])
	if err != nil || interpretationIndex < 0 {
		return 0, 0, ErrInvalidClarificationChoice
	}
	return ambiguityIndex, interpretationIndex, nil
}
