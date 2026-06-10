import { describe, expect, it } from 'vitest'

import { deriveClarificationStage, MAX_CLARIFICATION_ROUNDS } from './clarificationStage'

describe('deriveClarificationStage', () => {
  it('hides the indicator and stages on the first response', () => {
    expect(deriveClarificationStage(undefined)).toEqual({
      round: 0,
      displayRound: 0,
      interactiveTier: false,
      capReached: false,
    })
  })

  it('reports normal rounds below the cap', () => {
    const stage = deriveClarificationStage(1)
    expect(stage.displayRound).toBe(1)
    expect(stage.interactiveTier).toBe(false)
    expect(stage.capReached).toBe(false)
  })

  it('flags the Tier-3 interactive pass exactly at the cap', () => {
    const stage = deriveClarificationStage(MAX_CLARIFICATION_ROUNDS)
    expect(stage.interactiveTier).toBe(true)
    expect(stage.capReached).toBe(false)
    expect(stage.displayRound).toBe(MAX_CLARIFICATION_ROUNDS)
  })

  it('flags past-cap and never displays a round above the max', () => {
    const stage = deriveClarificationStage(MAX_CLARIFICATION_ROUNDS + 1)
    expect(stage.interactiveTier).toBe(false)
    expect(stage.capReached).toBe(true)
    expect(stage.displayRound).toBe(MAX_CLARIFICATION_ROUNDS)
  })
})
