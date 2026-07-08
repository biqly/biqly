import { describe, expect, it } from 'vitest'

import type { AgentChatRequest, AgentStreamEvent } from '../../types/agent'
import { runAgentModeTurn } from './agentModeTurn'

const baseRequest: AgentChatRequest = {
  message: 'How many orders last week?',
  conversation_id: 'conv-1',
  datasource_id: 'ds-1',
}

function fakeStream(events: AgentStreamEvent[]) {
  return async function* () {
    for (const event of events) {
      // Mirrors the real SSE client: each event arrives after network I/O.
      await Promise.resolve()
      yield event
    }
  }
}

// throwingStream builds a stream fixture that throws before emitting any
// event, matching streamAgentChat's behavior on a non-OK/network failure.
// `yield*` over an empty array satisfies the AsyncGenerator contract (and
// require-yield lint) without ever actually yielding a value.
function throwingStream(error: unknown) {
  return () =>
    (async function* (): AsyncGenerator<AgentStreamEvent, void, void> {
      yield* []
      await Promise.resolve()
      throw error
    })()
}

describe('runAgentModeTurn', () => {
  it('routes the request and options straight through to the injected stream', async () => {
    // A mutable holder (rather than a reassigned `let`) sidesteps TS control-flow
    // narrowing quirks across the closure boundary below.
    const received: { request?: AgentChatRequest; token?: string } = {}
    const stream = (request: AgentChatRequest, opts?: { token?: string }) => {
      received.request = request
      received.token = opts?.token
      return (async function* (): AsyncGenerator<AgentStreamEvent, void, void> {
        await Promise.resolve()
        yield { type: 'error', code: 'boom', message: 'nope' }
      })()
    }

    await runAgentModeTurn(baseRequest, {
      token: 'jwt-123',
      clarificationFallback: 'fallback',
      genericErrorMessage: 'generic',
      stream,
    })

    expect(received.request).toEqual(baseRequest)
    expect(received.token).toBe('jwt-123')
  })

  it('ignores run_started/step events and resolves on the terminal result event', async () => {
    const stream = fakeStream([
      { type: 'run_started', run_id: 'run-1' },
      { type: 'step', seq: 1, kind: 'tool_call_started', tool: 'sql_run' },
      {
        type: 'result',
        run_id: 'run-1',
        result: { sql: 'SELECT 1', confidence: 0.8, answer: 'One.' },
      },
    ])

    const outcome = await runAgentModeTurn(baseRequest, {
      clarificationFallback: 'fallback',
      genericErrorMessage: 'generic',
      stream,
    })

    expect(outcome.kind).toBe('result')
    if (outcome.kind === 'result') {
      expect(outcome.response.sql).toBe('SELECT 1')
      expect(outcome.response.answer).toBe('One.')
      expect(outcome.response.run_id).toBe('run-1')
    }
  })

  it('surfaces a structured error event as an error outcome', async () => {
    const stream = fakeStream([{ type: 'error', code: 'run_failed', message: 'provider timeout' }])

    const outcome = await runAgentModeTurn(baseRequest, {
      clarificationFallback: 'fallback',
      genericErrorMessage: 'generic',
      stream,
    })

    expect(outcome).toEqual({ kind: 'error', message: 'provider timeout' })
  })

  it('falls back to a plain message on clarification_required (T11 owns the real card)', async () => {
    const stream = fakeStream([
      {
        type: 'clarification_required',
        run_id: 'run-2',
        question: 'Which datasource did you mean?',
        allow_free_text: true,
      },
    ])

    const outcome = await runAgentModeTurn(baseRequest, {
      clarificationFallback: 'fallback text',
      genericErrorMessage: 'generic',
      stream,
    })

    expect(outcome).toEqual({ kind: 'clarification', message: 'Which datasource did you mean?' })
  })

  it('uses the clarification fallback text when the event has no question', async () => {
    const stream = fakeStream([
      { type: 'clarification_required', run_id: 'run-3', allow_free_text: false },
    ])

    const outcome = await runAgentModeTurn(baseRequest, {
      clarificationFallback: 'fallback text',
      genericErrorMessage: 'generic',
      stream,
    })

    expect(outcome).toEqual({ kind: 'clarification', message: 'fallback text' })
  })

  it('turns a thrown network error into an error outcome using the generic message', async () => {
    const stream = throwingStream(new Error('HTTP 500'))

    const outcome = await runAgentModeTurn(baseRequest, {
      clarificationFallback: 'fallback',
      genericErrorMessage: 'generic',
      stream,
    })

    expect(outcome).toEqual({ kind: 'error', message: 'HTTP 500' })
  })

  it('falls back to the generic error message when a thrown error is not an Error instance', async () => {
    const stream = throwingStream('not an error object')

    const outcome = await runAgentModeTurn(baseRequest, {
      clarificationFallback: 'fallback',
      genericErrorMessage: 'generic error message',
      stream,
    })

    expect(outcome).toEqual({ kind: 'error', message: 'generic error message' })
  })

  it('resolves silently to "none" (no error) when the stream throws after the caller aborted it', async () => {
    const controller = new AbortController()
    controller.abort()
    const abortError = new DOMException('The user aborted a request.', 'AbortError')
    const stream = throwingStream(abortError)

    const outcome = await runAgentModeTurn(baseRequest, {
      signal: controller.signal,
      clarificationFallback: 'fallback',
      genericErrorMessage: 'generic error message',
      stream,
    })

    expect(outcome).toEqual({ kind: 'none' })
  })

  it('still surfaces an error outcome for a thrown failure when the signal was never aborted', async () => {
    const controller = new AbortController()
    const stream = throwingStream(new Error('HTTP 500'))

    const outcome = await runAgentModeTurn(baseRequest, {
      signal: controller.signal,
      clarificationFallback: 'fallback',
      genericErrorMessage: 'generic',
      stream,
    })

    expect(outcome).toEqual({ kind: 'error', message: 'HTTP 500' })
  })

  it('resolves to "none" when the stream ends without a terminal event', async () => {
    const stream = fakeStream([{ type: 'run_started', run_id: 'run-4' }])

    const outcome = await runAgentModeTurn(baseRequest, {
      clarificationFallback: 'fallback',
      genericErrorMessage: 'generic',
      stream,
    })

    expect(outcome).toEqual({ kind: 'none' })
  })
})
