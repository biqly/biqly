import { streamAgentChat } from '../../api/agentStream'
import type { AgentChatRequest, AgentStepEvent, AgentStreamEvent } from '../../types/agent'
import type { AIQueryResponse } from '../../types/ai'
import { normalizeAgentResultEvent } from '../../utils/normalizeAIQueryResponse'

// AgentTurnOutcome is the terminal shape of a single Agent Mode send —
// runAgentModeTurn's caller (AIQuery.tsx's sendQuery) maps each variant onto
// the existing conversation-persistence / error-surfacing calls (addMessage,
// setJobError) rather than inventing a parallel channel.
export type AgentTurnOutcome =
  | { kind: 'result'; response: AIQueryResponse }
  // T11: the clarification_required event's fields are normalized (question/
  // choices/allowFreeText always populated) so the caller can drive the real
  // ClarificationCard + a resume call without re-deriving fallbacks itself.
  | {
      kind: 'clarification'
      runId: string
      question: string
      choices: { id: string; label: string }[]
      allowFreeText: boolean
    }
  | { kind: 'error'; message: string }
  // The stream ended (e.g. aborted, or closed with [DONE] before any
  // terminal event arrived) without producing a result/error/clarification.
  | { kind: 'none' }

export type AgentEventStream = (
  request: AgentChatRequest,
  options?: { signal?: AbortSignal; token?: string },
) => AsyncGenerator<AgentStreamEvent, void, void>

export interface RunAgentModeTurnOptions {
  token?: string
  signal?: AbortSignal
  /** Fallback question text used when a clarification_required event arrives
   * without one (defensive — the handler always sets it in practice). */
  clarificationFallback: string
  /** Message surfaced via the error outcome when the stream throws (network
   * failure, non-OK response) rather than emitting a structured error event. */
  genericErrorMessage: string
  /** Invoked synchronously for every `step` event as it streams in, so a
   * caller can feed a live trace panel (T11) without waiting for the turn to
   * resolve. Optional so existing/unit-test callers that don't care about
   * the live trace can omit it. */
  onStep?: (event: AgentStepEvent) => void
  /** Injectable for tests; defaults to the real T9 SSE client. */
  stream?: AgentEventStream
}

// runAgentModeTurn drives POST /api/agent/chat (T9's streamAgentChat) for a
// single Agent Mode send (a fresh question, or a resume via
// resume_run_id/clarification_answer) and reduces the event stream down to
// one terminal outcome. `run_started` events are ignored (nothing downstream
// needs the run id before either a step or a terminal event arrives); `step`
// events are forwarded live via onStep rather than accumulated here.
export async function runAgentModeTurn(
  request: AgentChatRequest,
  options: RunAgentModeTurnOptions,
): Promise<AgentTurnOutcome> {
  const stream = options.stream ?? streamAgentChat
  try {
    for await (const event of stream(request, { token: options.token, signal: options.signal })) {
      if (event.type === 'result') {
        const response = normalizeAgentResultEvent(event)
        return response ? { kind: 'result', response } : { kind: 'none' }
      }
      if (event.type === 'error') {
        return { kind: 'error', message: event.message }
      }
      if (event.type === 'step') {
        options.onStep?.(event)
        continue
      }
      if (event.type === 'clarification_required') {
        return {
          kind: 'clarification',
          runId: event.run_id ?? '',
          question: event.question ?? options.clarificationFallback,
          choices: event.choices ?? [],
          allowFreeText: event.allow_free_text,
        }
      }
    }
    return { kind: 'none' }
  } catch (err) {
    // An intentional abort (unmount, or a new turn superseding this one) is
    // not a user-facing error — the caller's AbortController set the signal
    // itself, so `aborted` is the authoritative signal here rather than
    // sniffing `err.name === 'AbortError'` (DOMException isn't reliably
    // `instanceof Error` across environments). Resolve silently as 'none',
    // which AIQuery.tsx's sendQuery already ignores.
    if (options.signal?.aborted) {
      return { kind: 'none' }
    }
    return {
      kind: 'error',
      message: err instanceof Error ? err.message : options.genericErrorMessage,
    }
  }
}
