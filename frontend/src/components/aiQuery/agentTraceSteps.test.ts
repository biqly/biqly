import { describe, expect, it } from 'vitest'

import type { AgentStepEvent } from '../../types/agent'
import { mergeAgentStepEvent } from './agentTraceSteps'

describe('mergeAgentStepEvent', () => {
  it('appends a new step as "running" for a started event with no prior entry', () => {
    const event: AgentStepEvent = {
      type: 'step',
      seq: 1,
      kind: 'tool_call_started',
      tool: 'list_models',
    }
    const result = mergeAgentStepEvent([], event)
    expect(result).toEqual([
      { seq: 1, kind: 'list_models', status: 'running', duration_ms: 0, detail: undefined },
    ])
  })

  it('replaces the started row in place with the completed row for the same seq', () => {
    const started: AgentStepEvent = {
      type: 'step',
      seq: 1,
      kind: 'tool_call_started',
      tool: 'run_question',
    }
    const completed: AgentStepEvent = {
      type: 'step',
      seq: 1,
      kind: 'tool_call_completed',
      tool: 'run_question',
      status: 'completed',
    }
    const afterStart = mergeAgentStepEvent([], started)
    const afterComplete = mergeAgentStepEvent(afterStart, completed)

    expect(afterComplete).toHaveLength(1)
    expect(afterComplete[0]).toMatchObject({ seq: 1, kind: 'run_question', status: 'ok' })
  })

  it('maps a denied terminal status to "failed"', () => {
    const started: AgentStepEvent = {
      type: 'step',
      seq: 2,
      kind: 'tool_call_started',
      tool: 'run_logical_query',
    }
    const denied: AgentStepEvent = {
      type: 'step',
      seq: 2,
      kind: 'tool_call_denied',
      tool: 'run_logical_query',
      status: 'denied',
    }
    const result = mergeAgentStepEvent(mergeAgentStepEvent([], started), denied)
    expect(result[0]?.status).toBe('failed')
  })

  it('maps a failed terminal status to "failed"', () => {
    const event: AgentStepEvent = {
      type: 'step',
      seq: 3,
      kind: 'tool_call_failed',
      tool: 'list_skills',
      status: 'failed',
    }
    const result = mergeAgentStepEvent([], event)
    expect(result[0]?.status).toBe('failed')
  })

  it('appends distinct seqs as separate rows, preserving arrival order', () => {
    const first: AgentStepEvent = {
      type: 'step',
      seq: 1,
      kind: 'tool_call_started',
      tool: 'list_datasources',
    }
    const second: AgentStepEvent = {
      type: 'step',
      seq: 2,
      kind: 'tool_call_started',
      tool: 'run_skill',
    }
    const result = mergeAgentStepEvent(mergeAgentStepEvent([], first), second)
    expect(result.map((s) => s.seq)).toEqual([1, 2])
  })

  it('falls back to the raw kind when the event has no tool name', () => {
    const event: AgentStepEvent = { type: 'step', seq: 1, kind: 'run_started_marker' }
    const result = mergeAgentStepEvent([], event)
    expect(result[0]?.kind).toBe('run_started_marker')
  })
})
