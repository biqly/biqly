package eval

import (
	"context"
	"time"

	providerpkg "github.com/biqly/biqly/internal/ai/provider"
	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/semantic"
)

// Mode EvalMode selects which checks RunGoldenSuite performs.
type Mode uint8

const (
	ModeLogical Mode = 1 << iota
	ModeExecution
	ModeJudge
)

// ResultExecutor runs a LogicalQuery and returns tabular results.
type ResultExecutor interface {
	Execute(ctx context.Context, model *semantic.SemanticModel, lq *query.LogicalQuery) (*query.Result, error)
}

// QuestionProcessor generates a LogicalQuery and the eval metadata needed to
// score one natural-language question.
type QuestionProcessor interface {
	EvaluateQuestion(ctx context.Context, question string, model *semantic.SemanticModel) (*QuestionResult, error)
}

// QuestionResult is the eval-owned projection of an AI generation response.
type QuestionResult struct {
	LogicalQuery                *query.LogicalQuery
	Confidence                  float64
	TokenUsage                  *providerpkg.TokenUsage
	PromptTemplateVersions      map[string]int
	PromptTemplateBundleVersion int
}

// SuiteOptions EvalSuiteOptions configures a golden / benchmark eval run.
type SuiteOptions struct {
	Cases    []GoldenCase
	Modes    Mode
	Executor ResultExecutor
	Judge    providerpkg.Provider
}

// CaseResult EvalCaseResult is the outcome for one eval case.
type CaseResult struct {
	Case                        GoldenCase
	Got                         *query.LogicalQuery
	LogicalMatch                bool
	LogicalReason               string
	ExecutionMatch              bool
	ExecutionReason             string
	JudgeMatch                  bool
	JudgeReason                 string
	Confidence                  float64
	LatencyMs                   int64
	TokenCount                  int
	PromptTemplateVersions      map[string]int
	PromptTemplateBundleVersion int
	Err                         error
}

// Pass returns true when all enabled modes passed for this case.
func (r CaseResult) Pass(opts SuiteOptions) bool {
	if r.Err != nil {
		return false
	}
	if opts.Modes&ModeLogical != 0 && !r.LogicalMatch {
		return false
	}
	if opts.Modes&ModeExecution != 0 && !r.ExecutionMatch {
		return false
	}
	if opts.Modes&ModeJudge != 0 && opts.Judge != nil && !r.JudgeMatch {
		return false
	}
	return true
}

// SuiteResult EvalSuiteResult aggregates a full suite run.
type SuiteResult struct {
	Total           int
	LogicalPassed   int
	ExecutionPassed int
	JudgePassed     int
	Passed          int
	Failed          int
	PassRate        float64
	AvgConfidence   float64
	Cases           []CaseResult
}

// RunGoldenSuite runs each case through the AI service and optional checks.
//
//nolint:gocognit
func RunGoldenSuite(ctx context.Context, processor QuestionProcessor, opts SuiteOptions) *SuiteResult {
	cases := opts.Cases
	if len(cases) == 0 {
		cases = DefaultGoldenCases()
	}
	if opts.Modes == 0 {
		opts.Modes = ModeLogical
	}
	if opts.Executor == nil && opts.Modes&ModeExecution != 0 {
		opts.Executor = MemoryResultExecutor{}
	}

	out := &SuiteResult{Total: len(cases)}
	var confSum float64
	var confN int

	for _, c := range cases {
		cr := CaseResult{Case: c}
		start := time.Now()

		resp, err := processor.EvaluateQuestion(ctx, c.Question, c.Model)
		cr.LatencyMs = time.Since(start).Milliseconds()
		if err != nil {
			cr.Err = err
			cr.LogicalReason = err.Error()
			out.Cases = append(out.Cases, cr)
			out.Failed++
			continue
		}
		cr.Got = resp.LogicalQuery
		if resp.Confidence > 0 {
			cr.Confidence = resp.Confidence
			confSum += resp.Confidence
			confN++
		}
		if resp.TokenUsage != nil {
			cr.TokenCount = resp.TokenUsage.Total
		}
		cr.PromptTemplateVersions = resp.PromptTemplateVersions
		cr.PromptTemplateBundleVersion = resp.PromptTemplateBundleVersion

		if opts.Modes&ModeLogical != 0 {
			cr.LogicalMatch, cr.LogicalReason = LogicalQueryEqual(resp.LogicalQuery, &c.Expected)
			if cr.LogicalMatch {
				out.LogicalPassed++
			}
		} else {
			cr.LogicalMatch = true
		}

		if opts.Modes&ModeExecution != 0 && opts.Executor != nil {
			cr.ExecutionMatch, cr.ExecutionReason = compareExecution(ctx, opts.Executor, c.Model, &c.Expected, resp.LogicalQuery)
			if cr.ExecutionMatch {
				out.ExecutionPassed++
			}
		} else {
			cr.ExecutionMatch = true
		}

		if opts.Modes&ModeJudge != 0 && opts.Judge != nil { //nolint:nestif
			ok, rationale, jerr := JudgeLogicalQuery(ctx, opts.Judge, c.Question, c.Model, &c.Expected, resp.LogicalQuery)
			if jerr != nil {
				cr.JudgeMatch = false
				cr.JudgeReason = jerr.Error()
			} else {
				cr.JudgeMatch = ok
				cr.JudgeReason = rationale
				if ok {
					out.JudgePassed++
				}
			}
		} else {
			cr.JudgeMatch = true
		}

		if cr.Pass(opts) {
			out.Passed++
		} else {
			out.Failed++
		}
		out.Cases = append(out.Cases, cr)
	}

	if out.Total > 0 {
		out.PassRate = float64(out.Passed) / float64(out.Total)
	}
	if confN > 0 {
		out.AvgConfidence = confSum / float64(confN)
	}
	return out
}

// CompareExecutionResults runs expected and got LogicalQueries and compares result sets.
func CompareExecutionResults(ctx context.Context, exec ResultExecutor, model *semantic.SemanticModel, expected, got *query.LogicalQuery) (bool, string) {
	return compareExecution(ctx, exec, model, expected, got)
}

func compareExecution(ctx context.Context, exec ResultExecutor, model *semantic.SemanticModel, expected, got *query.LogicalQuery) (bool, string) {
	if expected == nil || got == nil {
		return false, "expected or got logical query is nil"
	}
	expRes, err := exec.Execute(ctx, model, expected)
	if err != nil {
		return false, "execute expected: " + err.Error()
	}
	gotRes, err := exec.Execute(ctx, model, got)
	if err != nil {
		return false, "execute got: " + err.Error()
	}
	return ResultSetEqual(expRes, gotRes)
}

// ToEvalResultsWithMetrics converts suite results for persistence.
func (r *SuiteResult) ToEvalResultsWithMetrics() []ResultWithMetrics {
	out := make([]ResultWithMetrics, 0, len(r.Cases))
	for _, c := range r.Cases {
		reason := c.LogicalReason
		if reason == "" && !c.ExecutionMatch {
			reason = c.ExecutionReason
		}
		if reason == "" && !c.JudgeMatch {
			reason = c.JudgeReason
		}
		match := c.Err == nil && c.LogicalMatch && c.ExecutionMatch && c.JudgeMatch
		var got *query.LogicalQuery
		if c.Got != nil {
			got = c.Got
		}
		er := Result{
			Case:   c.Case,
			Got:    got,
			Match:  match,
			Reason: reason,
		}
		if c.Err != nil {
			er.Reason = c.Err.Error()
		}
		out = append(out, ResultWithMetrics{
			Result:                      er,
			Confidence:                  c.Confidence,
			LatencyMs:                   c.LatencyMs,
			TokenCount:                  c.TokenCount,
			PromptTemplateVersions:      c.PromptTemplateVersions,
			PromptTemplateBundleVersion: c.PromptTemplateBundleVersion,
		})
	}
	return out
}
