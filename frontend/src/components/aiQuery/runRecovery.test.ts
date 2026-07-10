import { describe, expect, it } from 'vitest'

import type { RunStep } from '../../types/ai'
import { analyzeRunRecovery, hasRecoveryStory } from './runRecovery'

function step(partial: Partial<RunStep> & Pick<RunStep, 'seq' | 'kind' | 'status'>): RunStep {
  return { duration_ms: 0, ...partial }
}

describe('analyzeRunRecovery', () => {
  it('reports nothing for a clean run', () => {
    const r = analyzeRunRecovery([
      step({ seq: 1, kind: 'planner', status: 'ok' }),
      step({ seq: 2, kind: 'run_logical_query', status: 'ok' }),
      step({ seq: 3, kind: 'final_response', status: 'ok' }),
    ])
    expect(r.hadFailure).toBe(false)
    expect(r.recovered).toBe(false)
    expect(hasRecoveryStory(r)).toBe(false)
  })

  it('detects recovery after a failed step', () => {
    const r = analyzeRunRecovery([
      step({ seq: 1, kind: 'planner', status: 'ok' }),
      step({ seq: 2, kind: 'run_logical_query', status: 'failed', detail: 'timeout' }),
      step({ seq: 3, kind: 'planner', status: 'ok' }),
      step({ seq: 4, kind: 'run_logical_query', status: 'ok', attempt: 2 }),
      step({ seq: 5, kind: 'final_response', status: 'ok' }),
    ])
    expect(r.hadFailure).toBe(true)
    expect(r.recovered).toBe(true)
    expect(r.maxAttempt).toBe(2)
    expect(r.plannerPasses).toBe(2)
    expect(r.failedTerminal).toBe(false)
    expect(hasRecoveryStory(r)).toBe(true)
  })

  it('does not report recovery when the run ends in terminal failure', () => {
    const r = analyzeRunRecovery([
      step({ seq: 1, kind: 'planner', status: 'ok' }),
      step({ seq: 2, kind: 'run_logical_query', status: 'ok' }),
      step({ seq: 3, kind: 'fail', status: 'failed', detail: 'max_steps_exceeded' }),
    ])
    expect(r.hadFailure).toBe(true)
    expect(r.recovered).toBe(false)
    expect(r.failedTerminal).toBe(true)
  })
})
