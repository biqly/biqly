package eval

import (
	_ "embed"
	"errors"
	"fmt"
	"strings"

	"github.com/bytedance/sonic"

	ambiguitypkg "github.com/biqly/biqly/internal/ai/ambiguity"
	"github.com/biqly/biqly/internal/ai/prompt"
	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/semantic"
)

//go:embed testdata/ambiguity_golden.json
var defaultAmbiguityGoldenJSON []byte

const (
	AmbiguityExpectedClarification = "clarification"
	AmbiguityExpectedPass          = "pass"
)

// AmbiguityGoldenCase is one JSON-backed regression case for ambiguity behavior.
//
// Add a case: append an object to internal/ai/eval/testdata/ambiguity_golden.json
// with a unique id, question, model_ref (see ambiguity_golden_models.go), optional
// glossary_ref, expected_type ("clarification" or "pass"), optional expected_detail
// substring, and for post-choice flows clarification_choice plus expected LogicalQuery.
// Run `make eval-regression` to verify.
type AmbiguityGoldenCase struct {
	ID                  string
	Question            string
	Model               *semantic.SemanticModel
	Glossary            []prompt.GlossaryEntry
	ExpectedType        string
	ExpectedDetail      string
	ClarificationChoice string
	Expected            *query.LogicalQuery
}

type rawAmbiguityGoldenCase struct {
	ID                  string              `json:"id"`
	Question            string              `json:"question"`
	ModelRef            string              `json:"model_ref"`
	GlossaryRef         string              `json:"glossary_ref,omitempty"`
	ExpectedType        string              `json:"expected_type"`
	ExpectedDetail      string              `json:"expected_detail,omitempty"`
	ClarificationChoice string              `json:"clarification_choice,omitempty"`
	Expected            *query.LogicalQuery `json:"expected,omitempty"`
}

// LoadDefaultAmbiguityGoldenCases loads the embedded ambiguity golden set.
func LoadDefaultAmbiguityGoldenCases() ([]AmbiguityGoldenCase, error) {
	return LoadAmbiguityGoldenCases(defaultAmbiguityGoldenJSON)
}

// LoadAmbiguityGoldenCases parses ambiguity golden JSON (top-level array).
func LoadAmbiguityGoldenCases(data []byte) ([]AmbiguityGoldenCase, error) {
	var raw []rawAmbiguityGoldenCase
	if err := sonic.ConfigStd.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse ambiguity golden JSON: %w", err)
	}
	models := ambiguityGoldenModels()
	glossaries := ambiguityGoldenGlossaries()

	cases := make([]AmbiguityGoldenCase, 0, len(raw))
	for _, item := range raw {
		c, err := resolveAmbiguityGoldenCase(item, models, glossaries)
		if err != nil {
			return nil, fmt.Errorf("case %q: %w", item.ID, err)
		}
		cases = append(cases, c)
	}
	return cases, nil
}

func resolveAmbiguityGoldenCase(
	item rawAmbiguityGoldenCase,
	models map[string]*semantic.SemanticModel,
	glossaries map[string][]prompt.GlossaryEntry,
) (AmbiguityGoldenCase, error) {
	if item.ID == "" {
		return AmbiguityGoldenCase{}, errors.New("missing id")
	}
	if strings.TrimSpace(item.Question) == "" {
		return AmbiguityGoldenCase{}, errors.New("missing question")
	}
	model, ok := models[item.ModelRef]
	if !ok || model == nil {
		return AmbiguityGoldenCase{}, fmt.Errorf("unknown model_ref %q", item.ModelRef)
	}
	switch item.ExpectedType {
	case AmbiguityExpectedClarification, AmbiguityExpectedPass:
	default:
		return AmbiguityGoldenCase{}, fmt.Errorf("unknown expected_type %q", item.ExpectedType)
	}
	if item.ClarificationChoice != "" && item.Expected == nil {
		return AmbiguityGoldenCase{}, errors.New("clarification_choice requires expected LogicalQuery")
	}
	if item.ClarificationChoice == "" && item.Expected != nil {
		return AmbiguityGoldenCase{}, errors.New("expected LogicalQuery requires clarification_choice")
	}

	var glossary []prompt.GlossaryEntry
	if item.GlossaryRef != "" {
		glossary = glossaries[item.GlossaryRef]
		if glossary == nil {
			return AmbiguityGoldenCase{}, fmt.Errorf("unknown glossary_ref %q", item.GlossaryRef)
		}
	}

	return AmbiguityGoldenCase{
		ID:                  item.ID,
		Question:            item.Question,
		Model:               model,
		Glossary:            glossary,
		ExpectedType:        item.ExpectedType,
		ExpectedDetail:      item.ExpectedDetail,
		ClarificationChoice: item.ClarificationChoice,
		Expected:            item.Expected,
	}, nil
}

func ambiguityDetailText(result ambiguitypkg.Result) string {
	var b strings.Builder
	for _, item := range result.Ambiguities {
		b.WriteString(strings.ToLower(item.Term))
		b.WriteString(strings.ToLower(item.Type))
		for _, interp := range item.Interpretations {
			b.WriteString(strings.ToLower(interp.Label))
			b.WriteString(strings.ToLower(interp.SemanticMapping.Name))
		}
	}
	return b.String()
}
