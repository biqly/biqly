import { describe, expect, it } from 'vitest'

import {
  canEditVariants,
  canStartExperiment,
  isExperimentActive,
  validateTrafficSplits,
} from './abExperimentPanelLogic'

describe('abExperimentPanelLogic', () => {
  it('validates that traffic splits sum to 100', () => {
    expect(validateTrafficSplits([50, 50])).toBe(true)
    expect(validateTrafficSplits([30, 30, 40])).toBe(true)
    expect(validateTrafficSplits([50, 40])).toBe(false)
    expect(validateTrafficSplits([])).toBe(false)
  })

  it('determines if an experiment can start', () => {
    expect(canStartExperiment(100, 1)).toBe(true)
    expect(canStartExperiment(90, 1)).toBe(false)
    expect(canStartExperiment(100, 2)).toBe(false)
    expect(canStartExperiment(100, 0)).toBe(false)
  })

  it('allows editing variants only in draft status', () => {
    expect(canEditVariants('draft')).toBe(true)
    expect(canEditVariants('running')).toBe(false)
    expect(canEditVariants('paused')).toBe(false)
    expect(canEditVariants('completed')).toBe(false)
    expect(canEditVariants(undefined)).toBe(false)
  })

  it('identifies if an experiment is active', () => {
    expect(isExperimentActive('running')).toBe(true)
    expect(isExperimentActive('paused')).toBe(true)
    expect(isExperimentActive('draft')).toBe(false)
    expect(isExperimentActive('completed')).toBe(false)
    expect(isExperimentActive(undefined)).toBe(false)
  })
})
