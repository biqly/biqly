package handlers

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/bytedance/sonic"

	"github.com/biqly/biqly/internal/agent"
	"github.com/biqly/biqly/internal/ai"
	"github.com/biqly/biqly/internal/query"
)

// webAgentSummaryMaxRunes bounds every per-step summary/args string that
// leaves the backend — over the SSE "step" event and in the persisted
// agent_steps.detail column alike. The frontend defensively truncates again
// at render time (RunTrace.tsx's MAX_DETAIL_CHARS), but the cap here keeps
// the wire payload and the DB row small at the source.
const webAgentSummaryMaxRunes = 200

// webAgentStepSummary derives a compact, human-readable summary of one web
// agent tool step for the run trace (SSE `summary` field + persisted
// RunStep.Detail):
//
//   - denied steps keep the BARE policy reason code (e.g.
//     "tool_not_allowlisted") so RunTrace.tsx's existing reason-code → i18n
//     label mapping still fires;
//   - errored steps keep the error text (free text, truncated);
//   - completed steps get a tool-aware summary built from counts, names and
//     durations ONLY — never raw result rows or cell values (design doc:
//     "Step payloads sent to the browser are summaries, not raw tool I/O").
//
// An in-flight step (no observation yet) or an unrecognized payload shape
// yields "" — the trace row simply renders without a detail line.
func webAgentStepSummary(step agent.RuntimeStep) string {
	switch {
	case step.DeniedReason != "":
		return step.DeniedReason
	case step.Error != "":
		return truncateSummaryRunes(step.Error)
	case step.Observation == nil:
		return ""
	}
	payload := step.Observation.Payload
	var summary string
	switch step.Proposal.Tool {
	case agent.ToolWebListDatasources:
		summary = summarizeNamedList(payload, "datasources")
	case agent.ToolWebListModels:
		summary = summarizeNamedList(payload, "models")
	case agent.ToolWebListPromptTemplates:
		summary = summarizeNamedList(payload, "templates")
	case agent.ToolWebListSkills:
		summary = summarizeNamedList(payload, "skills")
	case agent.ToolWebListKnowledgeFiles:
		summary = summarizeKnowledgeFileList(payload)
	case agent.ToolWebReadKnowledgeFile:
		summary = summarizeKnowledgeFileRead(payload)
	case agent.ToolWebRunQuestion:
		summary = summarizeRunQuestionPayload(payload)
	case agent.ToolWebRunLogicalQuery:
		summary = summarizeQueryResultPayload(payload)
	case agent.ToolWebMetricQuery:
		var wrapped struct {
			Result json.RawMessage `json:"result"`
		}
		if err := sonic.Unmarshal(payload, &wrapped); err == nil && len(wrapped.Result) > 0 {
			summary = summarizeQueryResultPayload(wrapped.Result)
		}
	case agent.ToolWebRunSkill:
		summary = summarizeRunSkillPayload(payload)
	case agent.ToolCatalog, agent.ToolSemantic, agent.ToolQueryCompile, agent.ToolQueryExecute, agent.ToolMemoryRecall,
		agent.ToolWebDryPlan, agent.ToolWebDryRun:
		summary = ""
	}
	return truncateSummaryRunes(summary)
}

// webAgentStepArgs returns the proposal's raw JSON arguments for the live
// trace's expandable step details, truncated to webAgentSummaryMaxRunes.
// Empty/no-op argument payloads ("", "null", "{}") yield "" so the frontend
// renders those rows without a disclosure. Arguments are planner-authored
// (question text, datasource/model/skill ids, LogicalQuery documents) and
// never contain tool RESULTS, so no row/PII redaction is needed here.
func webAgentStepArgs(step agent.RuntimeStep) string {
	raw := strings.TrimSpace(string(step.Proposal.Arguments))
	if raw == "" || raw == "null" || raw == "{}" {
		return ""
	}
	return truncateSummaryRunes(raw)
}

// webAgentClarificationDetail composes the persisted detail line for one
// answered clarification round. Plain English composition (not i18n): the
// detail is persisted server-side into agent_steps.detail where no locale is
// available, and Q&A text is user content anyway. The live client-side trace
// synthesizes the same row through an i18n template instead
// (ai_query.run_trace_clarification_detail).
func webAgentClarificationDetail(exchange agent.ClarificationExchange) string {
	return truncateSummaryRunes(
		fmt.Sprintf("asked: %s — answered: %s", exchange.Question, exchange.Answer))
}

// summarizeKnowledgeFileList reports how many knowledge files were listed and
// their paths — never file contents.
func summarizeKnowledgeFileList(payload json.RawMessage) string {
	var wrapped struct {
		Files []struct {
			Path string `json:"path"`
		} `json:"files"`
	}
	if err := sonic.Unmarshal(payload, &wrapped); err != nil {
		return ""
	}
	paths := make([]string, 0, len(wrapped.Files))
	for _, f := range wrapped.Files {
		paths = append(paths, f.Path)
	}
	return fmt.Sprintf("%d files: %s", len(wrapped.Files), strings.Join(paths, ", "))
}

// summarizeKnowledgeFileRead reports which file was read (path + title), not
// its content.
func summarizeKnowledgeFileRead(payload json.RawMessage) string {
	var wrapped struct {
		File struct {
			Path  string `json:"path"`
			Title string `json:"title"`
		} `json:"file"`
	}
	if err := sonic.Unmarshal(payload, &wrapped); err != nil || wrapped.File.Path == "" {
		return ""
	}
	if wrapped.File.Title != "" {
		return fmt.Sprintf("read %s (%s)", wrapped.File.Path, wrapped.File.Title)
	}
	return "read " + wrapped.File.Path
}

// namedListItem is the minimal element shape shared by the three governed
// list tools' responses (datasources, semantic models, skills): only a
// display name is extracted, never any other field.
type namedListItem struct {
	Name  string `json:"name"`
	Label string `json:"label,omitempty"`
}

// summarizeNamedList renders "N <noun>: A, B, C, …" (first three names) from
// either a bare JSON array or an object wrapping the array under `noun`
// (e.g. the skills endpoint's {"skills": [...]}).
func summarizeNamedList(payload json.RawMessage, noun string) string {
	items, ok := decodeNamedList(payload, noun)
	if !ok {
		return ""
	}
	const maxNames = 3
	names := make([]string, 0, min(len(items), maxNames))
	for _, item := range items {
		if len(names) == maxNames {
			break
		}
		if name := firstNonEmpty(item.Name, item.Label); name != "" {
			names = append(names, name)
		}
	}
	out := fmt.Sprintf("%d %s", len(items), noun)
	if len(names) > 0 {
		out += ": " + strings.Join(names, ", ")
		if len(items) > len(names) {
			out += ", …"
		}
	}
	return out
}

func decodeNamedList(payload json.RawMessage, key string) ([]namedListItem, bool) {
	var items []namedListItem
	if err := sonic.Unmarshal(payload, &items); err == nil {
		return items, true
	}
	var obj map[string]json.RawMessage
	if err := sonic.Unmarshal(payload, &obj); err != nil {
		return nil, false
	}
	raw, ok := obj[key]
	if !ok {
		return nil, false
	}
	if err := sonic.Unmarshal(raw, &items); err != nil {
		return nil, false
	}
	return items, true
}

func summarizeRowStats(stats query.Stats) string {
	return fmt.Sprintf("%d rows in %dms", stats.RowCount, stats.DurationMs)
}

// summarizeRunQuestionPayload reads the row stats from run_question's wire
// shape (a full ai.Response: logical_query/sql/result all under .result —
// see webAgentQueryDataFromStep).
func summarizeRunQuestionPayload(payload json.RawMessage) string {
	var wrapped struct {
		Result *ai.AIResult `json:"result"`
	}
	if err := sonic.Unmarshal(payload, &wrapped); err != nil ||
		wrapped.Result == nil || wrapped.Result.Result == nil {
		return ""
	}
	return summarizeRowStats(wrapped.Result.Result.Stats)
}

// summarizeQueryResultPayload reads the row stats from run_logical_query's
// wire shape (a bare query.Result — see webAgentQueryDataFromStep).
func summarizeQueryResultPayload(payload json.RawMessage) string {
	var result query.Result
	if err := sonic.Unmarshal(payload, &result); err != nil || len(result.Columns) == 0 {
		return ""
	}
	return summarizeRowStats(result.Stats)
}

// summarizeRunSkillPayload reads the skill name and row count from
// run_skill's wire shape ({skill_id, name, sql, result} — see
// webAgentQueryDataFromStep).
func summarizeRunSkillPayload(payload json.RawMessage) string {
	var wrapped struct {
		Name   string        `json:"name"`
		Result *query.Result `json:"result"`
	}
	if err := sonic.Unmarshal(payload, &wrapped); err != nil || wrapped.Result == nil {
		return ""
	}
	if wrapped.Name != "" {
		return fmt.Sprintf("skill %q: %d rows", wrapped.Name, wrapped.Result.Stats.RowCount)
	}
	return summarizeRowStats(wrapped.Result.Stats)
}

func truncateSummaryRunes(s string) string {
	runes := []rune(s)
	if len(runes) <= webAgentSummaryMaxRunes {
		return s
	}
	return string(runes[:webAgentSummaryMaxRunes]) + "…"
}
