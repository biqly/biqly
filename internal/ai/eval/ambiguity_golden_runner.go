package eval

import (
	"context"
	"fmt"
	"strings"

	ambiguitypkg "github.com/biqly/biqly/internal/ai/ambiguity"
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

// RunAmbiguityGoldenAnalysis asserts rule-based ambiguity detection for cases
// with expected_type clarification or pass.
func RunAmbiguityGoldenAnalysis(ctx context.Context, cases []AmbiguityGoldenCase) AmbiguityGoldenSuiteResult {
	out := AmbiguityGoldenSuiteResult{Cases: make([]AmbiguityGoldenCaseResult, 0, len(cases))}
	for _, c := range cases {
		if c.ClarificationChoice != "" {
			continue
		}
		cr := AmbiguityGoldenCaseResult{Case: c}
		got := ambiguitypkg.Analyze(ctx, c.Question, c.Model, c.Glossary, 0)
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
		resolved, err := ambiguitypkg.Resolve(ctx, c.Question, c.ClarificationChoice, c.Model, c.Glossary)
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

		analysis := ambiguitypkg.Analyze(ctx, source.Question, source.Model, source.Glossary, 0)
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
