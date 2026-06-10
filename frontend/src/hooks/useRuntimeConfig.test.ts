import { describe, expect, it } from 'vitest'

import type { AIAdminRuntimeConfig } from '../api/aiAdmin'
import { buildRuntimeConfigUpdate, clampNumber, draftFromConfig } from './useRuntimeConfig'

const config: AIAdminRuntimeConfig = {
  ambiguity: {
    check_enabled: true,
    confidence_threshold: 0.7,
    max_options: 5,
    tiered_enabled: true,
    max_llm_tier_per_question: 1,
    db_override: true,
    source: 'database',
    sources: { tiered_enabled: 'database' },
  },
  pii: {
    enabled: true,
    detection_threshold: 0.6,
    db_override: false,
    source: 'environment',
  },
  memory: {
    recall_enabled: true,
    recall_limit: 5,
    db_override: false,
    source: 'environment',
  },
}

describe('draftFromConfig', () => {
  it('copies only the editable knobs', () => {
    expect(draftFromConfig(config)).toEqual({
      ambiguity: {
        check_enabled: true,
        confidence_threshold: 0.7,
        max_options: 5,
        tiered_enabled: true,
        max_llm_tier_per_question: 1,
      },
      pii: { detection_threshold: 0.6 },
      memory: { recall_enabled: true, recall_limit: 5 },
    })
  })
})

describe('buildRuntimeConfigUpdate', () => {
  it('round-trips an in-range draft unchanged', () => {
    const draft = draftFromConfig(config)
    expect(buildRuntimeConfigUpdate(draft)).toEqual({
      ambiguity: draft.ambiguity,
      pii: draft.pii,
      memory: draft.memory,
    })
  })

  it('clamps out-of-range values into the backend-accepted ranges', () => {
    const draft = draftFromConfig(config)
    draft.ambiguity.confidence_threshold = 1.5
    draft.ambiguity.max_options = 0
    draft.ambiguity.max_llm_tier_per_question = 99
    draft.pii.detection_threshold = 0
    draft.memory.recall_limit = -3

    const update = buildRuntimeConfigUpdate(draft)
    expect(update.ambiguity?.confidence_threshold).toBe(1)
    expect(update.ambiguity?.max_options).toBe(1)
    expect(update.ambiguity?.max_llm_tier_per_question).toBe(10)
    expect(update.pii?.detection_threshold).toBe(0.05)
    expect(update.memory?.recall_limit).toBe(1)
  })
})

describe('clampNumber', () => {
  it('clamps and treats NaN as the minimum', () => {
    expect(clampNumber(5, 0, 10)).toBe(5)
    expect(clampNumber(-1, 0, 10)).toBe(0)
    expect(clampNumber(11, 0, 10)).toBe(10)
    expect(clampNumber(Number.NaN, 0, 10)).toBe(0)
  })
})
