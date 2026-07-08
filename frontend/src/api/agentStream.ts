import type { AgentChatRequest, AgentStreamEvent } from '../types/agent'
import { buildFetchHeaders } from './apiClient'
import { csrfFetch } from './csrf'

const AGENT_CHAT_PATH = '/api/agent/chat'
const DONE_MARKER = '[DONE]'

export interface AgentStreamOptions {
  /** From an AbortController — abort() cancels the underlying fetch and ends
   * the stream, e.g. a future cancel button (T11). */
  signal?: AbortSignal
  token?: string
}

// createFrameBuffer reassembles "\n\n"-delimited SSE frames from chunks that
// may split a frame at any boundary — a ReadableStream reader only guarantees
// chunk arrival order, not chunk-to-frame alignment.
function createFrameBuffer(): (chunk: string) => string[] {
  let pending = ''
  return (chunk: string) => {
    pending += chunk
    const frames: string[] = []
    let separatorIndex = pending.indexOf('\n\n')
    while (separatorIndex !== -1) {
      frames.push(pending.slice(0, separatorIndex))
      pending = pending.slice(separatorIndex + 2)
      separatorIndex = pending.indexOf('\n\n')
    }
    return frames
  }
}

// extractDataPayload returns the concatenated `data:` line content of an SSE
// frame, or null when the frame carries no data line at all — a bare comment
// frame such as the `: heartbeat` keep-alive (newAgentSSESender,
// ai_agent_chat.go) — which must be silently ignored, never surfaced as an
// event.
function extractDataPayload(frame: string): string | null {
  const dataLines = frame
    .split('\n')
    .filter((line) => line.startsWith('data:'))
    .map((line) => (line.startsWith('data: ') ? line.slice(6) : line.slice(5)))
  return dataLines.length > 0 ? dataLines.join('\n') : null
}

function parseEventPayload(payload: string): AgentStreamEvent {
  const parsed: unknown = JSON.parse(payload)
  return parsed as AgentStreamEvent
}

// parseAgentEventStream turns already-decoded text chunks into parsed agent
// events, stopping (without erroring) at the `[DONE]` terminator frame.
// Exported standalone from streamAgentChat so the frame-splitting/heartbeat/
// [DONE] parsing is unit-testable without a real network response body.
export async function* parseAgentEventStream(
  chunks: AsyncIterable<string>,
): AsyncGenerator<AgentStreamEvent, void, void> {
  const pushChunk = createFrameBuffer()
  for await (const chunk of chunks) {
    for (const frame of pushChunk(chunk)) {
      const payload = extractDataPayload(frame)
      if (payload === null) {
        continue
      }
      if (payload === DONE_MARKER) {
        return
      }
      yield parseEventPayload(payload)
    }
  }
}

async function* readAsText(
  reader: ReadableStreamDefaultReader<Uint8Array>,
): AsyncGenerator<string, void, void> {
  const decoder = new TextDecoder()
  try {
    let result = await reader.read()
    while (!result.done) {
      yield decoder.decode(result.value, { stream: true })
      result = await reader.read()
    }
  } finally {
    reader.releaseLock()
  }
}

// streamAgentChat POSTs to /api/agent/chat (Accept: text/event-stream) and
// yields each parsed SSE event as it arrives.
export async function* streamAgentChat(
  request: AgentChatRequest,
  options: AgentStreamOptions = {},
): AsyncGenerator<AgentStreamEvent, void, void> {
  const body = JSON.stringify(request)
  const headers = buildFetchHeaders({ token: options.token }, body)
  headers.set('Accept', 'text/event-stream')

  const response = await csrfFetch(AGENT_CHAT_PATH, {
    method: 'POST',
    headers,
    body,
    signal: options.signal,
  })

  if (!response.ok || !response.body) {
    const text = await response.text()
    throw new Error(text || `HTTP ${response.status}`)
  }

  yield* parseAgentEventStream(readAsText(response.body.getReader()))
}
