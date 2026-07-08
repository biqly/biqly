import { streamAgentChat } from '../../api/agentStream'
import type { AgentChatRequest, AgentStreamEvent } from '../../types/agent'
import type { AIQueryResponse } from '../../types/ai'
import { normalizeAgentResultEvent } from '../../utils/normalizeAIQueryResponse'

// AgentTurnOutcome is the terminal shape of a single Agent Mode send —
// runAgentModeTurn's caller (AIQuery.tsx's sendQuery) maps each variant onto
// the existing conversation-persistence / error-surfacing calls (addMessage,
// setJobError) rather than inventing a parallel channel.
export type AgentTurnOutcome =
  | { kind: 'result'; response: AIQueryResponse }
  | { kind: 'clarification'; message: string }
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
  /** Plain-text fallback shown when a clarification_required event arrives —
   * T11 replaces this with the real clarification card; T10 just needs to
   * avoid a silent stall. */
  clarificationFallback: string
  /** Message surfaced via the error outcome when the stream throws (network
   * failure, non-OK response) rather than emitting a structured error event. */
  genericErrorMessage: string
  /** Injectable for tests; defaults to the real T9 SSE client. */
  stream?: AgentEventStream
}

// runAgentModeTurn drives POST /api/agent/chat (T9's streamAgentChat) for a
// single Agent Mode send and reduces the event stream down to one terminal
// outcome. `run_started`/`step` events feed the live trace panel — that's
// T11 scope, so they're consumed and ignored here.
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
      if (event.type === 'clarification_required') {
        return { kind: 'clarification', message: event.question ?? options.clarificationFallback }
      }
    }
    return { kind: 'none' }
  } catch (err) {
    return {
      kind: 'error',
      message: err instanceof Error ? err.message : options.genericErrorMessage,
    }
  }
}
