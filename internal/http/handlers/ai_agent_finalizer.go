package handlers

import (
	"context"
	"encoding/json"

	"github.com/bytedance/sonic"

	"github.com/biqly/biqly/internal/agent"
	"github.com/biqly/biqly/internal/ai"
	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/toolcontract"
)

// composeWebAgentFinalResult builds the ai.Response-shaped result payload for
// a web agent run's terminal "final" outcome — the SAME shape the legacy job
// pipeline composes (ai_job_exec.go: finishAIRunResult / enrichAIRunResponse /
// attachAINaturalLanguageAnswer / attachSuggestedFollowUps), so the
// frontend's AssistantMessageCard renders a web agent answer without any
// frontend changes (verified in T11).
//
// state.Terminal.Final carries only the planner's own free-text answer and
// confidence — no structured data. The governed query result (LogicalQuery,
// SQL, rows, chart hints) comes from whichever web tool call last actually
// executed a query (run_question, run_logical_query, or run_skill); see
// lastWebAgentQueryData. That step's Observation payload is the raw JSON
// body the corresponding /api/* endpoint already returned, already enriched
// (query.EnrichResult runs server-side in each of those three handlers
// before the web agent ever sees the observation) — this function only
// derives the visualization hint and anomaly warnings from it, exactly like
// enrichAIRunResponse does; it never re-runs EnrichResult itself.
func (h *AIHandler) composeWebAgentFinalResult(ctx context.Context, req webAgentRequest, resume webAgentResumeInfo, runID string, state agent.RuntimeState) *ai.Response {
	resp := &ai.Response{
		Result: &ai.AIResult{},
		Metadata: &ai.AIMetadata{
			RunID:    runID,
			RunSteps: webAgentRunSteps(state.Steps, state.ClarificationHistory),
		},
	}
	if state.Terminal != nil && state.Terminal.Final != nil {
		resp.Result.Answer = state.Terminal.Final.Answer
		resp.Result.Confidence = state.Terminal.Final.Confidence
	}

	data, ok := lastWebAgentQueryData(state.Steps)
	if !ok {
		// No tool call in this run ever executed a query (e.g. the planner
		// answered purely from list_datasources/list_models) — the planner's
		// own final answer above is all there is to show.
		return resp
	}
	resp.Result.LogicalQuery = data.logicalQuery
	resp.Result.SQL = data.sql
	resp.Result.Warnings = data.warnings
	resp.Result.Result = data.result

	chartType, reason := query.VisualizationHintFromResult(data.result)
	resp.Result.VisualizationHint = &ai.VisualizationHint{ChartType: chartType, Reason: reason}
	if anomalyWarnings := query.AnomalyWarningMessages(data.result); len(anomalyWarnings) > 0 {
		resp.Result.Warnings = append(resp.Result.Warnings, anomalyWarnings...)
	}

	question := webAgentFinalQuestion(req, resume)
	// Same helpers, same gating, as the legacy job pipeline: both are
	// no-ops unless resp.Result.Result is set (true here) and h.service is
	// configured, so this is safe to call unconditionally.
	h.attachAINaturalLanguageAnswer(ctx, resp, question)
	h.attachSuggestedFollowUps(ctx, resp, aiQueryRequest{Question: question, PriorTurns: req.PriorTurns})
	return resp
}

// webAgentFinalQuestion picks the question the business summary and
// follow-up suggestions are generated for. On a resumed run req.Message is
// empty and req.ClarificationAnswer is just the user's disambiguation choice
// (e.g. a datasource name) — using it produced nonsense summaries like
// "The count for 'zlitter' is 1,126". The run's ORIGINAL question
// (resume.OriginalQuestion, persisted at run creation) always wins, exactly
// as it does for the resumed planner prompt (webAgentRunContext).
func webAgentFinalQuestion(req webAgentRequest, resume webAgentResumeInfo) string {
	return firstNonEmpty(resume.OriginalQuestion, req.Message, req.ClarificationAnswer)
}

// webAgentRunSteps converts a web agent run's recorded steps into the same
// ai.RunStep shape agentStepsFromResponse (ai_agent_run.go) produces for the
// legacy job pipeline's run_steps/run_id trace, so the frontend's
// RunTracePanel renders a web agent run the same way. Detail carries
// webAgentStepSummary's output — the bare reason code / error text for
// denied/failed steps (preserving RunTrace.tsx's reason-code i18n mapping)
// and the tool-aware human summary for completed ones — matching what the
// live SSE trace showed while the run streamed.
//
// Each answered clarification round is interleaved at its true position —
// right after the tool step it followed (ClarificationExchange.AfterSeq) —
// so the trace reads in real temporal order (e.g. run_query -> clarify ->
// run_query) instead of stranding every clarification at the end where a
// re-query after a clarification looks like a pointless duplicate run. This
// mirrors the live client-side trace (appendAgentClarificationStep). The
// clarifications slice is oldest-first with non-decreasing AfterSeq (each
// round pauses after at least as many steps as the previous), and steps are
// already in seq order, so a single forward merge suffices. Output seqs are
// renumbered contiguously in render order to keep the persisted agent_steps
// rows unique per (run, seq) even though a clarification slots between two
// adjacent tool-step seqs.
func webAgentRunSteps(steps []agent.RuntimeStep, clarifications []agent.ClarificationExchange) []ai.RunStep {
	out := make([]ai.RunStep, 0, len(steps)+len(clarifications))
	ci := 0
	emitClarificationsThrough := func(afterSeq int) {
		for ci < len(clarifications) && clarifications[ci].AfterSeq <= afterSeq {
			out = append(out, ai.RunStep{
				Seq:    len(out) + 1,
				Kind:   "clarification",
				Status: ai.RunStepStatusOK,
				Detail: webAgentClarificationDetail(clarifications[ci]),
			})
			ci++
		}
	}
	for _, step := range steps {
		// Clarifications asked before this step (AfterSeq < step.Seq) render
		// ahead of it; AfterSeq == step.Seq means asked right after it, so it
		// waits for the next iteration (or the trailing flush below).
		emitClarificationsThrough(step.Seq - 1)
		status := ai.RunStepStatusOK
		if step.DeniedReason != "" || step.Error != "" {
			status = ai.RunStepStatusFailed
		}
		out = append(out, ai.RunStep{
			Seq:        len(out) + 1,
			Kind:       string(step.Proposal.Tool),
			Status:     status,
			DurationMs: step.DurationMs,
			Detail:     webAgentStepSummary(step),
		})
	}
	// Clarifications asked after the last recorded step (and any whose
	// AfterSeq is out of range) trail at the end.
	for ; ci < len(clarifications); ci++ {
		out = append(out, ai.RunStep{
			Seq:    len(out) + 1,
			Kind:   "clarification",
			Status: ai.RunStepStatusOK,
			Detail: webAgentClarificationDetail(clarifications[ci]),
		})
	}
	return out
}

// extractedQueryData is the common shape lastWebAgentQueryData derives from
// whichever web tool's observation last carried an executed query.Result,
// regardless of that tool's own distinct wire shape (see
// webAgentQueryDataFromStep).
type extractedQueryData struct {
	logicalQuery *query.LogicalQuery
	sql          string
	warnings     []string
	result       *query.Result
}

// lastWebAgentQueryData walks steps most-recent-first and returns the last
// one that actually executed a query and produced a result. A denied or
// errored step never carries a usable Observation and is skipped, so a
// failed retry never shadows an earlier successful result.
func lastWebAgentQueryData(steps []agent.RuntimeStep) (extractedQueryData, bool) {
	for i := len(steps) - 1; i >= 0; i-- {
		step := steps[i]
		if step.Observation == nil || step.DeniedReason != "" || step.Error != "" {
			continue
		}
		if data, ok := webAgentQueryDataFromStep(step); ok {
			return data, true
		}
	}
	return extractedQueryData{}, false
}

// webAgentQueryDataFromStep decodes one step's Observation payload according
// to its tool's own wire shape:
//   - run_question dispatches to POST /api/ai/query/run, whose body is a full
//     ai.Response — logical_query/sql/warnings/result all live under .result.
//   - run_logical_query dispatches to POST /api/query/run, whose body is a
//     bare query.Result (see (*QueryHandler).Run) — no sql or logical_query
//     is echoed back, so the LogicalQuery is instead recovered from the tool
//     call's own arguments (what the planner asked to run).
//   - run_skill dispatches to POST /api/ai/skills/{id}/run, whose body is
//     {skill_id, name, sql, result} (see skillRunResponse) — sql is present,
//     but the skill's LogicalQuery is never echoed back over that endpoint.
//
// Any other tool (list_datasources, list_models, list_skills) never carries
// a query.Result and is rejected.
func webAgentQueryDataFromStep(step agent.RuntimeStep) (extractedQueryData, bool) {
	switch step.Proposal.Tool {
	case agent.ToolWebListKnowledgeFiles, agent.ToolWebReadKnowledgeFile:
		// Knowledge reads carry markdown, never a query.Result.
		return extractedQueryData{}, false
	case agent.ToolWebRunQuestion:
		var wrapped struct {
			Result *ai.AIResult `json:"result"`
		}
		if err := sonic.Unmarshal(step.Observation.Payload, &wrapped); err != nil ||
			wrapped.Result == nil || wrapped.Result.Result == nil {
			return extractedQueryData{}, false
		}
		return extractedQueryData{
			logicalQuery: wrapped.Result.LogicalQuery,
			sql:          wrapped.Result.SQL,
			warnings:     wrapped.Result.Warnings,
			result:       wrapped.Result.Result,
		}, true
	case agent.ToolWebRunLogicalQuery:
		var result query.Result
		if err := sonic.Unmarshal(step.Observation.Payload, &result); err != nil {
			return extractedQueryData{}, false
		}
		return extractedQueryData{
			logicalQuery: logicalQueryFromRunLogicalQueryArgs(step.Proposal.Arguments),
			result:       &result,
		}, true
	case agent.ToolWebMetricQuery:
		var wrapped struct {
			LogicalQuery *query.LogicalQuery `json:"logical_query"`
			Result       *query.Result       `json:"result"`
		}
		if err := sonic.Unmarshal(step.Observation.Payload, &wrapped); err != nil || wrapped.Result == nil {
			return extractedQueryData{}, false
		}
		return extractedQueryData{logicalQuery: wrapped.LogicalQuery, result: wrapped.Result}, true
	case agent.ToolWebRunSkill:
		var wrapped struct {
			SQL    string        `json:"sql"`
			Result *query.Result `json:"result"`
		}
		if err := sonic.Unmarshal(step.Observation.Payload, &wrapped); err != nil || wrapped.Result == nil {
			return extractedQueryData{}, false
		}
		return extractedQueryData{sql: wrapped.SQL, result: wrapped.Result}, true
	case agent.ToolWebListDatasources, agent.ToolWebListModels, agent.ToolWebListPromptTemplates,
		agent.ToolWebListSkills, agent.ToolCatalog, agent.ToolSemantic, agent.ToolQueryCompile,
		agent.ToolQueryExecute, agent.ToolMemoryRecall,
		agent.ToolWebDryPlan, agent.ToolWebDryRun:
		return extractedQueryData{}, false
	default:
		return extractedQueryData{}, false
	}
}

// logicalQueryFromRunLogicalQueryArgs recovers the LogicalQuery a
// run_logical_query proposal asked to run from its own arguments — the
// dispatch response never echoes it back (see webAgentQueryDataFromStep).
// Best-effort: malformed arguments (which policy would already have denied
// before the tool ever ran) yield a nil LogicalQuery rather than an error.
func logicalQueryFromRunLogicalQueryArgs(raw json.RawMessage) *query.LogicalQuery {
	var in toolcontract.RunLogicalQueryInput
	if err := sonic.Unmarshal(raw, &in); err != nil || in.LogicalQuery == nil {
		return nil
	}
	encoded, err := sonic.Marshal(in.LogicalQuery)
	if err != nil {
		return nil
	}
	var lq query.LogicalQuery
	if err := sonic.Unmarshal(encoded, &lq); err != nil {
		return nil
	}
	return &lq
}
