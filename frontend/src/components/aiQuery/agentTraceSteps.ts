import type { AgentStepEvent } from '../../types/agent'
import type { RunStep } from '../../types/ai'

// mapAgentStepStatus turns the wire-level AgentStepStatus ('started' |
// 'completed' | 'denied' | 'failed') into RunTracePanel's RunStep status
// vocabulary. 'started' becomes 'running' (a client-only, not-yet-terminal
// state) rather than 'ok', so the panel never has to lie about a step that
// hasn't finished yet.
function mapAgentStepStatus(status: AgentStepEvent['status']): RunStep['status'] {
  if (status === 'denied' || status === 'failed') {
    return 'failed'
  }
  if (status === 'completed') {
    return 'ok'
  }
  return 'running'
}

// mergeAgentStepEvent folds one live "step" SSE event into the accumulated
// RunStep[] trace. The web agent runtime emits exactly two step-hook events
// per tool call sharing the same `seq` — a "started" one (no observation
// yet) and a terminal one (completed/denied/failed) — so a step already
// present for `event.seq` is replaced in place rather than appended again,
// giving one row per tool call that updates live instead of two rows.
//
// `kind` is set from `event.tool` (the ToolName, e.g. "list_models") rather
// than the SSE `kind` field ("tool_call_started"/"tool_call_completed"/...):
// this is the SAME string the backend persists as RunStep.kind for a
// reloaded run (webAgentRunSteps sets Kind to the raw tool name), so a
// single STEP_LABEL_KEYS entry per tool name (RunTrace.tsx) labels both the
// live and the reloaded trace with no further branching needed.
//
// duration_ms arrives only on terminal step events (tool_call_completed /
// denied / failed) — the "started" event fires before dispatch, so it
// carries none and the row shows 0 until the terminal event replaces it.
export function mergeAgentStepEvent(steps: RunStep[], event: AgentStepEvent): RunStep[] {
  const idx = steps.findIndex((s) => s.seq === event.seq)
  const next: RunStep = {
    seq: event.seq,
    kind: event.tool ?? event.kind,
    status: mapAgentStepStatus(event.status),
    duration_ms: event.duration_ms ?? 0,
    detail: event.summary,
    // Defensive: keep the started event's args if a terminal event ever
    // arrives without them (the handler sends them on both today).
    args: event.args ?? (idx === -1 ? undefined : steps[idx]?.args),
  }
  if (idx === -1) {
    return [...steps, next]
  }
  const merged = steps.slice()
  merged[idx] = next
  return merged
}

// appendAgentClarificationStep appends a client-synthesized `clarification`
// row to the live trace at the moment the user answers a pending
// clarification (see AIQuery.tsx), so the clarify round-trip stays visible
// in the run's history exactly where it happened. The synthetic seq is
// NEGATIVE (-1, -2, … per synthesized row): server-emitted step seqs are
// always positive, so a later real step event can neither collide with nor
// replace this row in mergeAgentStepEvent's seq-keyed merge — and rows
// render in array order, so the negative seq never affects placement.
// The persisted counterpart is webAgentRunSteps' appended clarification
// RunSteps (ai_agent_finalizer.go), which land at the END of the reloaded
// trace instead (the backend records no step position for a clarification).
export function appendAgentClarificationStep(steps: RunStep[], detail: string): RunStep[] {
  const syntheticCount = steps.filter((s) => s.seq < 0).length
  return [
    ...steps,
    {
      seq: -(syntheticCount + 1),
      kind: 'clarification',
      status: 'ok',
      duration_ms: 0,
      detail,
    },
  ]
}
