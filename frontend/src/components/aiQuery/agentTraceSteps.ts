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
  const next: RunStep = {
    seq: event.seq,
    kind: event.tool ?? event.kind,
    status: mapAgentStepStatus(event.status),
    duration_ms: event.duration_ms ?? 0,
    detail: event.summary,
  }
  const idx = steps.findIndex((s) => s.seq === event.seq)
  if (idx === -1) {
    return [...steps, next]
  }
  const merged = steps.slice()
  merged[idx] = next
  return merged
}
