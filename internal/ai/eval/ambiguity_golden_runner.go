package eval

import (
	"context"
	"errors"
	"fmt"
	"strings"

	ambiguitypkg "github.com/biqly/biqly/internal/ai/ambiguity"
	"github.com/biqly/biqly/internal/i18n"
)

// AmbiguityGoldenCaseResult is the outcome for one ambiguity golden case.
type AmbiguityGoldenCaseResult struct {
	Case   AmbiguityGoldenCase
	Passed bool
	Reason string
}

// AmbiguityGoldenSuiteResult aggregates ambiguity golden regression results.
type AmbiguityGoldenSuiteResult struct {
	Cases  []AmbiguityGoldenCaseResult
	Total  int
	Passed int
}

// AmbiguityGoldenCaseContext returns a context carrying the case locale.
func AmbiguityGoldenCaseContext(ctx context.Context, c AmbiguityGoldenCase) context.Context {
	if strings.TrimSpace(c.Locale) == "" {
		return ctx
	}
	return i18n.WithLocale(ctx, i18n.ParseLocale(c.Locale))
}

// FilterAmbiguityGoldenCasesByLocale returns only cases for the requested locale.
// An empty locale keeps the full suite for CI's default all-locale run.
func FilterAmbiguityGoldenCasesByLocale(cases []AmbiguityGoldenCase, locale string) []AmbiguityGoldenCase {
	locale = strings.TrimSpace(locale)
	if locale == "" {
		return cases
	}

	out := make([]AmbiguityGoldenCase, 0, len(cases))
	for _, c := range cases {
		if strings.EqualFold(c.Locale, locale) {
			out = append(out, c)
		}
	}
	return out
}

// ValidateAmbiguityGoldenLocaleCoverage ensures each locale has at least one
// smoke case for every expected_type. This catches half-added locales before CI.
func ValidateAmbiguityGoldenLocaleCoverage(cases []AmbiguityGoldenCase) error {
	byLocale := make(map[string]map[string]bool)
	for _, c := range cases {
		locale := normalizeAmbiguityGoldenLocale(c.Locale)
		types, ok := byLocale[locale]
		if !ok {
			types = make(map[string]bool, 2)
			byLocale[locale] = types
		}
		types[c.ExpectedType] = true
	}
	if len(byLocale) == 0 {
		return errors.New("no ambiguity golden cases")
	}

	for locale, types := range byLocale {
		for _, expectedType := range []string{AmbiguityExpectedClarification, AmbiguityExpectedPass} {
			if !types[expectedType] {
				return fmt.Errorf("locale %q missing smoke case for expected_type %q", locale, expectedType)
			}
		}
	}
	return nil
}

// RunAmbiguityGoldenAnalysis asserts rule-based ambiguity detection for cases
// with expected_type clarification or pass.
func RunAmbiguityGoldenAnalysis(ctx context.Context, cases []AmbiguityGoldenCase) AmbiguityGoldenSuiteResult {
	out := AmbiguityGoldenSuiteResult{Cases: make([]AmbiguityGoldenCaseResult, 0, len(cases))}
	for _, c := range cases {
		if c.ClarificationChoice != "" {
			continue
		}
		cr := AmbiguityGoldenCaseResult{Case: c}
		caseCtx := AmbiguityGoldenCaseContext(ctx, c)
		got := ambiguitypkg.Analyze(caseCtx, c.Question, c.Model, c.Glossary, 0)
		switch c.ExpectedType {
		case AmbiguityExpectedClarification:
			if !got.IsAmbiguous {
				cr.Reason = "expected ambiguous clarification, analyzer returned pass"
			} else if c.ExpectedDetail != "" && !strings.Contains(ambiguityDetailText(got), strings.ToLower(c.ExpectedDetail)) {
				cr.Reason = fmt.Sprintf("expected_detail %q not found in ambiguity result", c.ExpectedDetail)
			}
		case AmbiguityExpectedPass:
			if got.IsAmbiguous {
				cr.Reason = fmt.Sprintf("expected pass, analyzer flagged ambiguity: %#v", got.Ambiguities)
			}
		default:
			cr.Reason = fmt.Sprintf("unsupported expected_type %q", c.ExpectedType)
		}
		cr.Passed = cr.Reason == ""
		if cr.Passed {
			out.Passed++
		}
		out.Cases = append(out.Cases, cr)
	}
	out.Total = len(out.Cases)
	return out
}

// AmbiguityGoldenChoiceCases builds stub golden cases for post-choice LogicalQuery checks.
func AmbiguityGoldenChoiceCases(ctx context.Context, cases []AmbiguityGoldenCase) ([]GoldenCase, error) {
	var out []GoldenCase
	for _, c := range cases {
		if c.ClarificationChoice == "" || c.Expected == nil {
			continue
		}
		caseCtx := AmbiguityGoldenCaseContext(ctx, c)
		resolved, err := ambiguitypkg.Resolve(caseCtx, c.Question, c.ClarificationChoice, c.Model, c.Glossary)
		if err != nil {
			return nil, fmt.Errorf("[%s] resolve choice: %w", c.ID, err)
		}
		expected := *c.Expected
		out = append(out, GoldenCase{
			ID:       c.ID + "-resolved",
			Question: resolved,
			Model:    c.Model,
			Expected: expected,
		})
	}
	return out, nil
}

// RunAmbiguityGoldenChoiceSuite runs LogicalQuery eval for clarification_choice cases.
func RunAmbiguityGoldenChoiceSuite(ctx context.Context, processor QuestionProcessor, cases []AmbiguityGoldenCase) (AmbiguityGoldenSuiteResult, error) {
	choiceCases, err := AmbiguityGoldenChoiceCases(ctx, cases)
	if err != nil {
		return AmbiguityGoldenSuiteResult{}, err
	}
	byID := make(map[string]AmbiguityGoldenCase, len(cases))
	for _, c := range cases {
		if c.ClarificationChoice != "" {
			byID[c.ID] = c
		}
	}

	out := AmbiguityGoldenSuiteResult{Cases: make([]AmbiguityGoldenCaseResult, 0, len(choiceCases))}
	for _, gc := range choiceCases {
		sourceID := strings.TrimSuffix(gc.ID, "-resolved")
		source := byID[sourceID]
		cr := AmbiguityGoldenCaseResult{Case: source}

		analysisCtx := AmbiguityGoldenCaseContext(ctx, source)
		analysis := ambiguitypkg.Analyze(analysisCtx, source.Question, source.Model, source.Glossary, 0)
		if !analysis.IsAmbiguous {
			cr.Reason = "expected ambiguous question before choice resolution"
			cr.Passed = false
			out.Cases = append(out.Cases, cr)
			continue
		}

		qr, err := processor.EvaluateQuestion(ctx, gc.Question, gc.Model)
		if err != nil {
			cr.Reason = err.Error()
		} else if match, reason := LogicalQueryEqual(qr.LogicalQuery, &gc.Expected); !match {
			cr.Reason = reason
		}
		cr.Passed = cr.Reason == ""
		if cr.Passed {
			out.Passed++
		}
		out.Cases = append(out.Cases, cr)
	}
	out.Total = len(out.Cases)
	return out, nil
}
