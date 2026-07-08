import { describe, expect, it } from 'vitest'

import type { AgentStreamEvent } from '../types/agent'
import { parseAgentEventStream } from './agentStream'

// Each part is yielded after a microtask tick, mirroring how real
// reader.read() chunks arrive asynchronously rather than all at once.
async function* chunks(...parts: string[]): AsyncGenerator<string, void, void> {
  for (const part of parts) {
    await Promise.resolve()
    yield part
  }
}

async function collect(source: AsyncIterable<string>): Promise<AgentStreamEvent[]> {
  const events: AgentStreamEvent[] = []
  for await (const event of parseAgentEventStream(source)) {
    events.push(event)
  }
  return events
}

describe('parseAgentEventStream', () => {
  it('parses a normal event frame', async () => {
    const events = await collect(chunks('data: {"type":"run_started","run_id":"r1"}\n\n'))
    expect(events).toEqual([{ type: 'run_started', run_id: 'r1' }])
  })

  it('reassembles a frame split across multiple reader.read() chunks', async () => {
    const events = await collect(
      chunks(
        'data: {"type":"step","seq":1,',
        '"kind":"tool_call_started","tool":"list_models"}\n\n',
      ),
    )
    expect(events).toEqual([
      { type: 'step', seq: 1, kind: 'tool_call_started', tool: 'list_models' },
    ])
  })

  it('silently ignores heartbeat comment lines', async () => {
    const events = await collect(
      chunks(
        ': heartbeat\n\n',
        'data: {"type":"run_started","run_id":"r1"}\n\n',
        ': heartbeat\n\n',
      ),
    )
    expect(events).toEqual([{ type: 'run_started', run_id: 'r1' }])
  })

  it('stops at the [DONE] terminator without yielding it or anything after', async () => {
    const events = await collect(
      chunks(
        'data: {"type":"run_started","run_id":"r1"}\n\n',
        'data: [DONE]\n\n',
        'data: {"type":"error","code":"unreachable","message":"should not be yielded"}\n\n',
      ),
    )
    expect(events).toEqual([{ type: 'run_started', run_id: 'r1' }])
  })

  it('parses an error-type event', async () => {
    const events = await collect(
      chunks('data: {"type":"error","code":"bad_request","message":"message is required"}\n\n'),
    )
    expect(events).toEqual([{ type: 'error', code: 'bad_request', message: 'message is required' }])
  })
})
