import { describe, expect, it } from 'vitest'

import { getRateColor } from './formatters'

describe('getRateColor', () => {
  it('maps high, medium, and low rates to semantic colors', () => {
    expect(getRateColor(80)).toBe('var(--success)')
    expect(getRateColor(50)).toBe('var(--warning)')
    expect(getRateColor(49.9)).toBe('var(--error)')
  })
})
