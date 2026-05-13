package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/biqly/biqly/internal/ai"
	"github.com/biqly/biqly/internal/query"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// --- Wire types (match frontend Evaluation.tsx) ---

type evalTestCaseWire struct {
	ID                   string                 `json:"id"`
	Question             string                 `json:"question"`
	Status               string                 `json:"status"`
	ExpectedLogicalQuery map[string]interface{} `json:"expected_logical_query"`
	GotLogicalQuery      map[string]interface{} `json:"got_logical_query"`
	Confidence           *float64               `json:"confidence,omitempty"`
	ErrorMessage         string                 `json:"error_message,omitempty"`
}

type evalRunResponseWire struct {
	Total         int                `json:"total"`
	Passed        int                `json:"passed"`
	Failed        int                `json:"failed"`
	PassRate      float64            `json:"pass_rate"`
	AvgConfidence float64            `json:"avg_confidence"`
	TestCases     []evalTestCaseWire `json:"test_cases"`
	AccuracyTrend []evalTrendPoint   `json:"accuracy_trend,omitempty"`
}

type evalTrendPoint struct {
	Date     string  `json:"date"`
	PassRate float64 `json:"pass_rate"`
}

func logicalQueryToMap(lq *query.LogicalQuery) map[string]interface{} {
	if lq == nil {
		return map[string]interface{}{}
	}
	b, err := json.Marshal(lq)
	if err != nil {
		return map[string]interface{}{"_marshal_error": err.Error()}
	}
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		return map[string]interface{}{}
	}
	return m
}

func (h *AIHandler) evalAIConfigured() error {
	cfg := h.deps.Config.AI
	if strings.TrimSpace(cfg.APIKey) == "" || strings.TrimSpace(cfg.Model) == "" {
		return fmt.Errorf("AI is not configured (set BI_AI_API_KEY and BI_AI_MODEL)")
	}
	return nil
}

// executeGoldenCases runs each golden case once and returns aggregated results.
func (h *AIHandler) executeGoldenCases(ctx context.Context) (*evalRunResponseWire, error) {
	result, _, err := h.executeGoldenCasesWithMetrics(ctx)
	return result, err
}

// executeGoldenCasesWithMetrics runs each golden case once and returns both the
// wire response and per-case metrics for persistence.
func (h *AIHandler) executeGoldenCasesWithMetrics(ctx context.Context) (*evalRunResponseWire, []ai.EvalResultWithMetrics, error) {
	if err := h.evalAIConfigured(); err != nil {
		return nil, nil, err
	}

	cases := ai.DefaultGoldenCases()
	out := make([]evalTestCaseWire, 0, len(cases))
	metrics := make([]ai.EvalResultWithMetrics, 0, len(cases))
	var confSum float64
	var confN int
	passed := 0

	for _, c := range cases {
		tc := evalTestCaseWire{
			ID:                   c.ID,
			Question:             c.Question,
			ExpectedLogicalQuery: logicalQueryToMap(&c.Expected),
			GotLogicalQuery:      map[string]interface{}{},
			Status:               "fail",
		}

		start := time.Now()
		resp, err := h.service.ProcessQuestion(ctx, c.Question, c.Model)
		latencyMs := time.Since(start).Milliseconds()

		if err != nil {
			tc.ErrorMessage = err.Error()
			out = append(out, tc)
			metrics = append(metrics, ai.EvalResultWithMetrics{
				EvalResult: ai.EvalResult{Case: c, Got: nil, Match: false, Reason: err.Error()},
				LatencyMs:  latencyMs,
			})
			continue
		}
		if resp.LogicalQuery != nil {
			tc.GotLogicalQuery = logicalQueryToMap(resp.LogicalQuery)
		}
		if resp.Confidence > 0 {
			cf := resp.Confidence
			tc.Confidence = &cf
			confSum += resp.Confidence
			confN++
		}

		ok, reason := ai.LogicalQueryEqual(resp.LogicalQuery, &c.Expected)
		tokenCount := 0 // would need LLM client to provide this
		if ok {
			tc.Status = "pass"
			passed++
			metrics = append(metrics, ai.EvalResultWithMetrics{
				EvalResult: ai.EvalResult{Case: c, Got: resp.LogicalQuery, Match: true},
				Confidence: resp.Confidence,
				LatencyMs:  latencyMs,
				TokenCount: tokenCount,
			})
		} else {
			tc.ErrorMessage = reason
			metrics = append(metrics, ai.EvalResultWithMetrics{
				EvalResult: ai.EvalResult{Case: c, Got: resp.LogicalQuery, Match: false, Reason: reason},
				Confidence: resp.Confidence,
				LatencyMs:  latencyMs,
				TokenCount: tokenCount,
			})
		}
		out = append(out, tc)
	}

	total := len(out)
	failed := total - passed
	passRate := 0.0
	if total > 0 {
		passRate = float64(passed) / float64(total)
	}
	avgConf := 0.0
	if confN > 0 {
		avgConf = confSum / float64(confN)
	}

	return &evalRunResponseWire{
		Total:         total,
		Passed:        passed,
		Failed:        failed,
		PassRate:      passRate,
		AvgConfidence: avgConf,
		TestCases:     out,
	}, metrics, nil
}

// EvalRun executes the built-in golden text-to-SQL cases against the live
// configured LLM. Requires BI_AI_API_KEY and BI_AI_MODEL.
func (h *AIHandler) EvalRun(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdminKey(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	result, resultsWithMetrics, err := h.executeGoldenCasesWithMetrics(r.Context())
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}

	// Persist results if eval repo is available
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

// EvalRunStream runs the same golden set and streams one-line SSE progress
// events, then sends data: [DONE] so clients can close the stream.
func (h *AIHandler) EvalRunStream(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdminKey(w, r) {
		return
	}
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if err := h.evalAIConfigured(); err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
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

	send := func(line string) {
		safe := strings.ReplaceAll(strings.ReplaceAll(line, "\r", " "), "\n", " ")
		_, err := fmt.Fprintf(w, "data: %s\n\n", safe)
		if err != nil {
			slog.Error("eval stream write", "error", err)
			return
		}
		flusher.Flush()
	}

	ctx := r.Context()
	cases := ai.DefaultGoldenCases()
	send(fmt.Sprintf("Starting golden eval (%d cases)…", len(cases)))

	passed := 0

	for i, c := range cases {
		send(fmt.Sprintf("[%d/%d] %s — %s", i+1, len(cases), c.ID, c.Question))

		resp, err := h.service.ProcessQuestion(ctx, c.Question, c.Model)
		if err != nil {
			send(fmt.Sprintf("[%d/%d] ERROR: %s", i+1, len(cases), err.Error()))
			continue
		}

		ok, reason := ai.LogicalQueryEqual(resp.LogicalQuery, &c.Expected)
		if ok {
			passed++
			send(fmt.Sprintf("[%d/%d] PASS confidence=%.2f", i+1, len(cases), resp.Confidence))
		} else {
			send(fmt.Sprintf("[%d/%d] FAIL: %s", i+1, len(cases), reason))
		}
	}

	total := len(cases)
	passRate := 0.0
	if total > 0 {
		passRate = float64(passed) / float64(total)
	}
	send(fmt.Sprintf("Summary: %d/%d passed (%.0f%%)", passed, total, passRate*100))
	send("[DONE]")
}

func (h *AIHandler) requireAdminKey(w http.ResponseWriter, r *http.Request) bool {
	adminKey := h.deps.Config.Security.AdminAPIKey
	if adminKey == "" {
		writeError(w, http.StatusForbidden, "eval endpoints require BI_ADMIN_API_KEY to be configured")
		return false
	}
	authHeader := r.Header.Get("Authorization")
	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token != adminKey {
		writeError(w, http.StatusUnauthorized, "invalid or missing admin API key")
		return false
	}
	return true
}

// EvalListRuns returns a list of past eval runs.
func (h *AIHandler) EvalListRuns(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdminKey(w, r) {
		return
	}
	if h.deps.EvalRepo == nil {
		writeError(w, http.StatusServiceUnavailable, "eval storage not configured")
		return
	}

	ctx := r.Context()
	runs, err := h.deps.EvalRepo.ListRuns(ctx, h.deps.Config.Query.EvalRunsListLimit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list eval runs")
		return
	}
	writeJSON(w, http.StatusOK, runs)
}

// EvalGetRun returns results for a specific eval run.
func (h *AIHandler) EvalGetRun(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdminKey(w, r) {
		return
	}
	if h.deps.EvalRepo == nil {
		writeError(w, http.StatusServiceUnavailable, "eval storage not configured")
		return
	}

	runID := chi.URLParam(r, "id")
	if runID == "" {
		writeError(w, http.StatusBadRequest, "run id is required")
		return
	}

	ctx := r.Context()
	summary, err := h.deps.EvalRepo.GetRunSummary(ctx, runID)
	if err != nil {
		writeError(w, http.StatusNotFound, "eval run not found")
		return
	}

	results, err := h.deps.EvalRepo.GetRunResults(ctx, runID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get eval results")
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

// EvalRegression generates a regression report between two eval runs.
func (h *AIHandler) EvalRegression(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdminKey(w, r) {
		return
	}
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
		writeError(w, http.StatusInternalServerError, "failed to generate regression report")
		return
	}

	writeJSON(w, http.StatusOK, report)
}
