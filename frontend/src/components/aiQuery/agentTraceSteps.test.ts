import { describe, expect, it } from 'vitest'

import type { AgentStepEvent } from '../../types/agent'
import type { RunStep } from '../../types/ai'
import { appendAgentClarificationStep, mergeAgentStepEvent } from './agentTraceSteps'

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

describe('appendAgentClarificationStep', () => {
  it('appends a clarification row after the existing steps, keeping array order', () => {
    const steps: RunStep[] = [
      { seq: 1, kind: 'list_skills', status: 'ok', duration_ms: 36 },
      { seq: 2, kind: 'run_question', status: 'ok', duration_ms: 22900 },
    ]
    const result = appendAgentClarificationStep(steps, 'asked: which date? — answered: 2026-07-16')
    expect(result.map((s) => s.kind)).toEqual(['list_skills', 'run_question', 'clarification'])
    expect(result[2]).toMatchObject({
      kind: 'clarification',
      status: 'ok',
      detail: 'asked: which date? — answered: 2026-07-16',
    })
  })

  it('uses a negative seq so a later real step event never collides with it', () => {
    const withOne = appendAgentClarificationStep([], 'a')
    expect(withOne[0]?.seq).toBe(-1)
    // A subsequent real (positive-seq) step appends without replacing the row.
    const withStep = mergeAgentStepEvent(withOne, {
      type: 'step',
      seq: 3,
      kind: 'tool_call_started',
      tool: 'run_question',
    })
    expect(withStep.map((s) => s.kind)).toEqual(['clarification', 'run_question'])
    // A second clarification decrements to -2, staying unique.
    const withTwo = appendAgentClarificationStep(withStep, 'b')
    expect(withTwo[withTwo.length - 1]?.seq).toBe(-2)
  })
})
