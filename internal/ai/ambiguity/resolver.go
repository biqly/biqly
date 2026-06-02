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
	return ResolveChoice(question, choice, Analyze(ctx, question, model, glossary))
}

// ResolveChoice rewrites a question using the selected ambiguity interpretation.
func ResolveChoice(question, choice string, result AmbiguityResult) (string, error) {
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

	replacement := strings.TrimSpace(item.Interpretations[interpretationIndex].Label)
	if replacement == "" {
		replacement = strings.TrimSpace(item.Interpretations[interpretationIndex].SemanticMapping.Name)
	}
	if replacement == "" || strings.TrimSpace(item.Term) == "" {
		return "", ErrInvalidClarificationChoice
	}

	pattern := regexp.MustCompile(`(?i)` + regexp.QuoteMeta(item.Term))
	if !pattern.MatchString(question) {
		return "", fmt.Errorf("%w: term %q not found in question", ErrInvalidClarificationChoice, item.Term)
	}
	return pattern.ReplaceAllStringFunc(question, func(string) string {
		return replacement
	}), nil
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
