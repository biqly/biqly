package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	ai "github.com/biqly/biqly/internal/ai/eval"
	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/semantic"
	"github.com/google/uuid"
)

type evalTestCaseWire struct {
	ID                   string         `json:"id"`
	Question             string         `json:"question"`
	Status               string         `json:"status"`
	ExpectedLogicalQuery map[string]any `json:"expected_logical_query"`
	GotLogicalQuery      map[string]any `json:"got_logical_query"`
	Confidence           *float64       `json:"confidence,omitempty"`
	ErrorMessage         string         `json:"error_message,omitempty"`
	LogicalMatch         *bool          `json:"logical_match,omitempty"`
	ExecutionMatch       *bool          `json:"execution_match,omitempty"`
	JudgeMatch           *bool          `json:"judge_match,omitempty"`
	JudgeRationale       string         `json:"judge_rationale,omitempty"`
}

type evalRunResponseWire struct {
	Total           int                `json:"total"`
	Passed          int                `json:"passed"`
	Failed          int                `json:"failed"`
	PassRate        float64            `json:"pass_rate"`
	AvgConfidence   float64            `json:"avg_confidence"`
	LogicalPassed   int                `json:"logical_passed"`
	ExecutionPassed int                `json:"execution_passed"`
	JudgePassed     int                `json:"judge_passed"`
	Suite           string             `json:"suite,omitempty"`
	Modes           string             `json:"modes,omitempty"`
	TestCases       []evalTestCaseWire `json:"test_cases"`
	AccuracyTrend   []evalTrendPoint   `json:"accuracy_trend,omitempty"`
}

type evalTrendPoint struct {
	Date     string  `json:"date"`
	PassRate float64 `json:"pass_rate"`
}

func logicalQueryToMap(lq *query.LogicalQuery) map[string]any {
	if lq == nil {
		return map[string]any{}
	}
	b, err := json.Marshal(lq)
	if err != nil {
		return map[string]any{"_marshal_error": err.Error()}
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return map[string]any{}
	}
	return m
}

func (h *AIHandler) evalAIConfigured() error {
	if h.deps.Config.AI.QueryLLMConfigured() {
		return nil
	}
	return errors.New("AI is not configured (set BI_AI_MODEL and BI_AI_API_KEY, or BI_AI_BASE_URL for keyless local LLM)")
}

func (h *AIHandler) evalModesFromRequest(ctx context.Context, r *http.Request) (ai.EvalMode, string, string, []ai.GoldenCase, error) {
	modes := ai.EvalModeLogical | ai.EvalModeExecution
	parts := []string{"logical", "execution"}

	if r.URL.Query().Get("judge") == "1" || strings.Contains(r.URL.Query().Get("modes"), "judge") {
		modes |= ai.EvalModeJudge
		parts = append(parts, "judge")
	}
	if r.URL.Query().Get("logical") == "0" {
		modes &^= ai.EvalModeLogical
		parts = removeMode(parts, "logical")
	}
	if r.URL.Query().Get("execution") == "0" {
		modes &^= ai.EvalModeExecution
		parts = removeMode(parts, "execution")
	}

	var cases []ai.GoldenCase
	var err error
	suiteName := "golden"
	switch strings.TrimSpace(r.URL.Query().Get("suite")) {
	case "benchmark", ai.BenchmarkSuiteName:
		cases = ai.BenchmarkCases()
		suiteName = ai.BenchmarkSuiteName
	case "file:golden":
		cases, err = ai.LoadGoldenCasesFromDir(findGoldenDir())
		if err != nil {
			return 0, "", "", nil, fmt.Errorf("failed to load golden cases: %w", err)
		}
		suiteName = "file:golden"
	default:
		cases = ai.DefaultGoldenCases()
	}

	// Resolve semantic models for loaded cases
	resolvedCases := make([]ai.GoldenCase, len(cases))
	for i, c := range cases {
		modelID := c.Model.ID
		var fullModel *semantic.SemanticModel
		if modelID == "orders" {
			fullModel = ai.OrdersModel()
		} else {
			fullModel, err = h.deps.SemanticRepo.GetFullModel(ctx, modelID)
			if err != nil {
				return 0, "", "", nil, fmt.Errorf("failed to load model %s for case %s: %w", modelID, c.ID, err)
			}
		}
		c.Model = fullModel
		resolvedCases[i] = c
	}

	return modes, suiteName, suiteName + ":" + strings.Join(parts, ","), resolvedCases, nil
}

func removeMode(parts []string, mode string) []string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != mode {
			out = append(out, p)
		}
	}
	return out
}

func (h *AIHandler) executeGoldenCasesWithMetrics(ctx context.Context, r *http.Request) (*evalRunResponseWire, []ai.EvalResultWithMetrics, error) {
	if err := h.evalAIConfigured(); err != nil {
		return nil, nil, err
	}

	modes, suiteName, modesLabel, cases, err := h.evalModesFromRequest(ctx, r)
	if err != nil {
		return nil, nil, err
	}
	opts := ai.EvalSuiteOptions{
		Cases: cases,
		Modes: modes,
	}
	if modes&ai.EvalModeJudge != 0 {
		opts.Judge = h.service.LLMProvider()
	}

	result := ai.RunGoldenSuite(ctx, h.service, opts)
	wire := suiteResultToWire(result, opts, suiteName, modesLabel)
	return wire, result.ToEvalResultsWithMetrics(), nil
}

func suiteResultToWire(result *ai.EvalSuiteResult, opts ai.EvalSuiteOptions, suiteName, modesLabel string) *evalRunResponseWire {
	out := make([]evalTestCaseWire, 0, len(result.Cases))
	for _, c := range result.Cases {
		tc := evalTestCaseWire{
			ID:                   c.Case.ID,
			Question:             c.Case.Question,
			ExpectedLogicalQuery: logicalQueryToMap(&c.Case.Expected),
			GotLogicalQuery:      logicalQueryToMap(c.Got),
			Status:               "fail",
		}
		if c.Err != nil {
			tc.ErrorMessage = c.Err.Error()
		} else if !c.Pass(opts) {
			tc.ErrorMessage = firstNonEmpty(c.LogicalReason, c.ExecutionReason, c.JudgeReason)
		}
		if c.Confidence > 0 {
			tc.Confidence = new(c.Confidence)
		}
		tc.LogicalMatch = new(c.LogicalMatch)
		tc.ExecutionMatch = new(c.ExecutionMatch)
		if opts.Modes&ai.EvalModeJudge != 0 && opts.Judge != nil {
			tc.JudgeMatch = new(c.JudgeMatch)
			tc.JudgeRationale = c.JudgeReason
		}
		if c.Pass(opts) {
			tc.Status = "pass"
		}
		out = append(out, tc)
	}

	return &evalRunResponseWire{
		Total:           result.Total,
		Passed:          result.Passed,
		Failed:          result.Failed,
		PassRate:        result.PassRate,
		AvgConfidence:   result.AvgConfidence,
		LogicalPassed:   result.LogicalPassed,
		ExecutionPassed: result.ExecutionPassed,
		JudgePassed:     result.JudgePassed,
		Suite:           suiteName,
		Modes:           modesLabel,
		TestCases:       out,
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func (h *AIHandler) EvalRun(w http.ResponseWriter, r *http.Request) {
	result, resultsWithMetrics, err := h.executeGoldenCasesWithMetrics(r.Context(), r)
	if err != nil {
		writeInternalError(r.Context(), w, http.StatusServiceUnavailable, "eval run failed", err)
		return
	}

	if h.deps.EvalRepo != nil {
		runID := uuid.New().String()
		ctx := r.Context()
		model := h.deps.Config.AI.Model
		provider := h.deps.Config.AI.Provider
		if provider == "" {
			provider = "openai-compatible"
		}
		if err := h.deps.EvalRepo.SaveRunResults(ctx, runID, provider, model, 0, time.Time{}, resultsWithMetrics); err != nil {
			slog.ErrorContext(ctx, "failed to persist eval results", "run_id", runID, "error", err)
		} else {
			slog.InfoContext(ctx, "eval results persisted", "run_id", runID, "passed", result.Passed, "failed", result.Failed)
		}
	}

	writeJSON(w, http.StatusOK, result)
}

func (h *AIHandler) EvalRunStream(w http.ResponseWriter, r *http.Request) { //nolint:funlen,gocognit
	if err := h.evalAIConfigured(); err != nil {
		writeInternalError(r.Context(), w, http.StatusServiceUnavailable, "AI eval is not configured", err)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}

	ctx := r.Context()
	send := func(line string) {
		// SSE stream (text/event-stream), not HTML: EventSource delivers payloads
		// to JS as strings and never renders them as markup. CR/LF are stripped to
		// keep SSE framing intact, and the global SecurityHeaders middleware sets
		// X-Content-Type-Options: nosniff so the response cannot be MIME-sniffed as
		// HTML. The XSS rule therefore does not apply here.
		safe := strings.ReplaceAll(strings.ReplaceAll(line, "\r", " "), "\n", " ")
		// nosemgrep: go.lang.security.audit.xss.no-fprintf-to-responsewriter.no-fprintf-to-responsewriter
		if _, err := fmt.Fprintf(w, "data: %s\n\n", safe); err != nil {
			slog.ErrorContext(ctx, "eval stream write", "error", err)
			return
		}
		flusher.Flush()
	}

	modes, _, modesLabel, cases, err := h.evalModesFromRequest(ctx, r)
	if err != nil {
		send(fmt.Sprintf("Failed to load suite: %v", err))
		return
	}
	opts := ai.EvalSuiteOptions{Cases: cases, Modes: modes}
	if modes&ai.EvalModeJudge != 0 {
		opts.Judge = h.service.LLMProvider()
	}
	exec := ai.MemoryResultExecutor{}

	send(fmt.Sprintf("Starting eval (%d cases, modes=%s)…", len(cases), modesLabel))

	passed, logicalPassed, execPassed, judgePassed := 0, 0, 0, 0

	for i, c := range cases {
		if ctx.Err() != nil {
			send("Cancelled.")
			return
		}
		send(fmt.Sprintf("[%d/%d] %s — %s", i+1, len(cases), c.ID, c.Question))

		resp, err := h.service.ProcessQuestion(ctx, c.Question, c.Model)
		if err != nil {
			slog.ErrorContext(ctx, "eval case failed", "case_id", c.ID, "error", err)
			send(fmt.Sprintf("[%d/%d] ERROR: processing failed", i+1, len(cases)))
			continue
		}

		var respLQ *query.LogicalQuery
		var confidence float64
		if resp.Result != nil {
			respLQ = resp.Result.LogicalQuery
			confidence = resp.Result.Confidence
		}
		var templateVersions map[string]int
		var bundleVersion int
		if resp.Metadata != nil {
			templateVersions = resp.Metadata.PromptTemplateVersions
			bundleVersion = resp.Metadata.PromptTemplateBundleVersion
		}

		cr := ai.EvalCaseResult{
			Case:                        c,
			Got:                         respLQ,
			Confidence:                  confidence,
			PromptTemplateVersions:      templateVersions,
			PromptTemplateBundleVersion: bundleVersion,
		}
		if resp.Metadata != nil && resp.Metadata.TokenUsage != nil {
			cr.TokenCount = resp.Metadata.TokenUsage.Total
		}
		if modes&ai.EvalModeLogical != 0 {
			cr.LogicalMatch, cr.LogicalReason = ai.LogicalQueryEqual(respLQ, &c.Expected)
			if cr.LogicalMatch {
				logicalPassed++
			}
		} else {
			cr.LogicalMatch = true
			logicalPassed++
		}
		if modes&ai.EvalModeExecution != 0 {
			cr.ExecutionMatch, cr.ExecutionReason = ai.CompareExecutionResults(ctx, exec, c.Model, &c.Expected, respLQ)
			if cr.ExecutionMatch {
				execPassed++
			}
		} else {
			cr.ExecutionMatch = true
			execPassed++
		}
		if modes&ai.EvalModeJudge != 0 && opts.Judge != nil { //nolint:nestif
			ok, rationale, jerr := ai.JudgeLogicalQuery(ctx, opts.Judge, c.Question, c.Model, &c.Expected, respLQ)
			if jerr != nil {
				cr.JudgeMatch = false
				cr.JudgeReason = jerr.Error()
			} else {
				cr.JudgeMatch = ok
				cr.JudgeReason = rationale
				if ok {
					judgePassed++
				}
			}
		} else {
			cr.JudgeMatch = true
			judgePassed++
		}

		if cr.Pass(opts) {
			passed++
			send(fmt.Sprintf("[%d/%d] PASS logical=%v exec=%v judge=%v conf=%.2f",
				i+1, len(cases), cr.LogicalMatch, cr.ExecutionMatch, cr.JudgeMatch, confidence))
		} else {
			send(fmt.Sprintf("[%d/%d] FAIL: %s", i+1, len(cases), firstNonEmpty(cr.LogicalReason, cr.ExecutionReason, cr.JudgeReason)))
		}
	}

	total := len(cases)
	passRate := 0.0
	if total > 0 {
		passRate = float64(passed) / float64(total)
	}
	send(fmt.Sprintf("Summary: %d/%d passed (%.0f%%) logical=%d exec=%d judge=%d",
		passed, total, passRate*100, logicalPassed, execPassed, judgePassed))
	send("[DONE]")
}

func (h *AIHandler) EvalListRuns(w http.ResponseWriter, r *http.Request) {
	if h.deps.EvalRepo == nil {
		writeError(w, http.StatusServiceUnavailable, "eval storage not configured")
		return
	}

	ctx := r.Context()
	runs, err := h.deps.EvalRepo.ListRuns(ctx, h.deps.Config.Query.EvalRunsListLimit)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to list eval runs", err)
		return
	}
	writeJSON(w, http.StatusOK, runs)
}

func (h *AIHandler) EvalGetRun(w http.ResponseWriter, r *http.Request) {
	if h.deps.EvalRepo == nil {
		writeError(w, http.StatusServiceUnavailable, "eval storage not configured")
		return
	}

	runID, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}

	ctx := r.Context()
	summary, err := h.deps.EvalRepo.GetRunSummary(ctx, runID)
	if err != nil {
		writeEntityNotFound(w, "eval run")
		return
	}

	results, err := h.deps.EvalRepo.GetRunResults(ctx, runID)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to get eval results", err, "run_id", runID)
		return
	}

	type testCaseWire struct {
		CaseID     string  `json:"case_id"`
		Question   string  `json:"question"`
		Match      bool    `json:"match"`
		Reason     string  `json:"reason"`
		Confidence float64 `json:"confidence"`
		LatencyMs  int64   `json:"latency_ms"`
	}
	var testCases []testCaseWire
	for _, res := range results {
		testCases = append(testCases, testCaseWire{
			CaseID:     res.CaseID,
			Question:   res.Question,
			Match:      res.Match,
			Reason:     res.Reason,
			Confidence: res.Confidence,
			LatencyMs:  res.LatencyMs,
		})
	}
	if testCases == nil {
		testCases = []testCaseWire{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"summary":    summary,
		"test_cases": testCases,
	})
}

func (h *AIHandler) EvalRegression(w http.ResponseWriter, r *http.Request) {
	if h.deps.EvalRepo == nil {
		writeError(w, http.StatusServiceUnavailable, "eval storage not configured")
		return
	}

	baselineRunID := r.URL.Query().Get("baseline")
	currentRunID := r.URL.Query().Get("current")
	if baselineRunID == "" || currentRunID == "" {
		writeError(w, http.StatusBadRequest, "baseline and current query params are required")
		return
	}

	ctx := r.Context()
	report, err := h.deps.EvalRepo.GenerateRegressionReport(ctx, baselineRunID, currentRunID)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to generate regression report", err, "baseline_run_id", baselineRunID, "current_run_id", currentRunID)
		return
	}

	writeJSON(w, http.StatusOK, report)
}

type evalCaseWire struct {
	ID       string              `json:"id"`
	Question string              `json:"question"`
	ModelID  string              `json:"model_id"`
	Expected query.LogicalQuery `json:"expected"`
}

func (*AIHandler) EvalListCases(w http.ResponseWriter, r *http.Request) {
	cases, err := ai.LoadGoldenCasesFromDir(findGoldenDir())
	if err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "list golden cases failed", err)
		return
	}

	wireList := make([]evalCaseWire, len(cases))
	for i, c := range cases {
		wireList[i] = evalCaseWire{
			ID:       c.ID,
			Question: c.Question,
			ModelID:  c.Model.ID,
			Expected: c.Expected,
		}
	}
	writeJSON(w, http.StatusOK, wireList)
}

func (*AIHandler) EvalCreateCase(w http.ResponseWriter, r *http.Request) {
	var req evalCaseWire
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.ID == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}
	if req.Question == "" {
		writeError(w, http.StatusBadRequest, "question is required")
		return
	}
	if req.ModelID == "" {
		writeError(w, http.StatusBadRequest, "model_id is required")
		return
	}

	err := ai.SaveGoldenCaseToDir(findGoldenDir(), req.ID, req.Question, req.ModelID, req.Expected)
	if err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "save golden case failed", err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"status": "created", "id": req.ID})
}

func (*AIHandler) EvalDeleteCase(w http.ResponseWriter, r *http.Request) {
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}

	err := ai.DeleteGoldenCaseFromDir(findGoldenDir(), id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "delete golden case failed", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"status": "deleted", "id": id})
}

func findGoldenDir() string {
	dir := "testdata/golden"
	if _, err := os.Stat(dir); err == nil {
		return dir
	}
	parent := dir
	for range 4 {
		parent = filepath.Join("..", parent)
		if _, err := os.Stat(parent); err == nil {
			return parent
		}
	}
	return dir
}
