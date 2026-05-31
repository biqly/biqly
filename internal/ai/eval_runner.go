package ai

import (
	"context"
	"time"

	providerpkg "github.com/biqly/biqly/internal/ai/provider"
	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/semantic"
)

// EvalMode selects which checks RunGoldenSuite performs.
type EvalMode uint8

const (
	EvalModeLogical EvalMode = 1 << iota
	EvalModeExecution
	EvalModeJudge
)

// EvalModeAll runs logical, execution, and LLM judge checks.
const EvalModeAll = EvalModeLogical | EvalModeExecution | EvalModeJudge

// ResultExecutor runs a LogicalQuery and returns tabular results.
type ResultExecutor interface {
	Execute(ctx context.Context, model *semantic.SemanticModel, lq *query.LogicalQuery) (*query.Result, error)
}

// EvalSuiteOptions configures a golden / benchmark eval run.
type EvalSuiteOptions struct {
	Cases    []GoldenCase
	Modes    EvalMode
	Executor ResultExecutor
	Judge    providerpkg.Provider
}

// EvalCaseResult is the outcome for one eval case.
type EvalCaseResult struct {
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
func (r EvalCaseResult) Pass(opts EvalSuiteOptions) bool {
	if r.Err != nil {
		return false
	}
	if opts.Modes&EvalModeLogical != 0 && !r.LogicalMatch {
		return false
	}
	if opts.Modes&EvalModeExecution != 0 && !r.ExecutionMatch {
		return false
	}
	if opts.Modes&EvalModeJudge != 0 && opts.Judge != nil && !r.JudgeMatch {
		return false
	}
	return true
}

// EvalSuiteResult aggregates a full suite run.
type EvalSuiteResult struct {
	Total           int
	LogicalPassed   int
	ExecutionPassed int
	JudgePassed     int
	Passed          int
	Failed          int
	PassRate        float64
	AvgConfidence   float64
	Cases           []EvalCaseResult
}

// RunGoldenSuite runs each case through the AI service and optional checks.
func RunGoldenSuite(ctx context.Context, svc *Service, opts EvalSuiteOptions) *EvalSuiteResult {
	cases := opts.Cases
	if len(cases) == 0 {
		cases = DefaultGoldenCases()
	}
	if opts.Modes == 0 {
		opts.Modes = EvalModeLogical
	}
	if opts.Executor == nil && opts.Modes&EvalModeExecution != 0 {
		opts.Executor = MemoryResultExecutor{}
	}

	out := &EvalSuiteResult{Total: len(cases)}
	var confSum float64
	var confN int

	for _, c := range cases {
		cr := EvalCaseResult{Case: c}
		start := time.Now()

		resp, err := svc.ProcessQuestion(ctx, c.Question, c.Model)
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

		if opts.Modes&EvalModeLogical != 0 {
			cr.LogicalMatch, cr.LogicalReason = LogicalQueryEqual(resp.LogicalQuery, &c.Expected)
			if cr.LogicalMatch {
				out.LogicalPassed++
			}
		} else {
			cr.LogicalMatch = true
		}

		if opts.Modes&EvalModeExecution != 0 && opts.Executor != nil {
			cr.ExecutionMatch, cr.ExecutionReason = compareExecution(ctx, opts.Executor, c.Model, &c.Expected, resp.LogicalQuery)
			if cr.ExecutionMatch {
				out.ExecutionPassed++
			}
		} else {
			cr.ExecutionMatch = true
		}

		if opts.Modes&EvalModeJudge != 0 && opts.Judge != nil {
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
func (r *EvalSuiteResult) ToEvalResultsWithMetrics() []EvalResultWithMetrics {
	out := make([]EvalResultWithMetrics, 0, len(r.Cases))
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
		er := EvalResult{
			Case:   c.Case,
			Got:    got,
			Match:  match,
			Reason: reason,
		}
		if c.Err != nil {
			er.Reason = c.Err.Error()
		}
		out = append(out, EvalResultWithMetrics{
			EvalResult:                  er,
			Confidence:                  c.Confidence,
			LatencyMs:                   c.LatencyMs,
			TokenCount:                  c.TokenCount,
			PromptTemplateVersions:      c.PromptTemplateVersions,
			PromptTemplateBundleVersion: c.PromptTemplateBundleVersion,
		})
	}
	return out
}
