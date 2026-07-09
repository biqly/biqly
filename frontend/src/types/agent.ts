import type {
  LogicalQuery,
  PriorTurn,
  QueryResultPayload,
  SuggestedFollowUp,
  VisualizationHint,
} from './ai'

// AgentChatRequest is the POST /api/agent/chat request body (webAgentRequest,
// internal/http/handlers/ai_agent_chat.go). message may be omitted only when
// resuming a paused run with clarification_answer set — the handler requires
// at least one of the two.
export interface AgentChatRequest {
  message?: string
  conversation_id?: string
  datasource_id: string
  prior_turns?: PriorTurn[]
  resume_run_id?: string
  clarification_answer?: string
}

export interface AgentRunStartedEvent {
  type: 'run_started'
  run_id: string
}

export type AgentStepStatus = 'started' | 'completed' | 'denied' | 'failed'

// AgentStepEvent mirrors webAgentStepEvent (ai_agent_chat.go): kind is always
// "tool_call_<status>" today. summary and duration_ms arrive only on
// terminal events (completed/denied/failed); args rides on every event from
// "started" onward.
export interface AgentStepEvent {
  type: 'step'
  seq: number
  kind: string
  tool?: string
  status?: AgentStepStatus
  /** Compact human summary of a terminal step — counts/names/durations for
   * completed steps, the bare reason code / error text for denied/failed
   * ones (webAgentStepSummary). The same string is persisted server-side as
   * the step's agent_steps.detail. */
  summary?: string
  /** The proposal's raw JSON arguments, truncated server-side (~200 runes).
   * Live-trace only: the persisted agent_steps row carries no arguments, so
   * a reloaded trace renders without an expandable args section. */
  args?: string
  duration_ms?: number
}

export interface AgentClarificationChoice {
  id: string
  label: string
}

export interface AgentClarificationRequiredEvent {
  type: 'clarification_required'
  run_id?: string
  question?: string
  choices?: AgentClarificationChoice[]
  allow_free_text: boolean
}

// AgentResultPayload mirrors ai.AIResult's JSON shape (internal/ai/schema.go),
// the `result` field of the "result" SSE event.
export interface AgentResultPayload {
  logical_query?: LogicalQuery
  sql?: string
  warnings?: string[]
  result?: QueryResultPayload
  confidence: number
  visualization_hint?: VisualizationHint
  answer?: string
  caveat?: string
  suggested_followups?: SuggestedFollowUp[]
}

export interface AgentResultEvent {
  type: 'result'
  run_id: string
  answer?: string
  confidence?: number
  result?: AgentResultPayload
  // metadata mirrors ai.AIMetadata's JSON shape (run_steps, model_used, ...);
  // left loosely typed until a consumer (T11) needs specific fields.
  metadata?: Record<string, unknown>
}

export interface AgentErrorEvent {
  type: 'error'
  code: string
  message: string
}

export type AgentStreamEvent =
  | AgentRunStartedEvent
  | AgentStepEvent
  | AgentClarificationRequiredEvent
  | AgentResultEvent
  | AgentErrorEvent

// PendingAgentClarification is the normalized (always-populated) shape a UI
// consumer works with once a clarification_required event pauses a run —
// AgentClarificationRequiredEvent's fields are optional on the wire, but by
// the time runAgentModeTurn resolves a 'clarification' outcome, question/
// choices/allowFreeText have already been defaulted (see agentModeTurn.ts).
export interface PendingAgentClarification {
  runId: string
  question: string
  choices: AgentClarificationChoice[]
  allowFreeText: boolean
}
