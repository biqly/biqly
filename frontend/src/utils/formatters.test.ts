import { describe, expect, it } from 'vitest'

import { formatDurationMs, getRateColor } from './formatters'

describe('formatDurationMs', () => {
  it('formats sub-second, second, minute, and hour ranges', () => {
    expect(formatDurationMs(0)).toBe('—')
    expect(formatDurationMs(450)).toBe('450 ms')
    expect(formatDurationMs(22564)).toBe('23 s')
    expect(formatDurationMs(133871)).toBe('2 min 14 s')
    expect(formatDurationMs(355355)).toBe('5 min 55 s')
    expect(formatDurationMs(3_600_000)).toBe('1 h')
    expect(formatDurationMs(3_900_000)).toBe('1 h 5 min')
  })
})

describe('getRateColor', () => {
  it('maps high, medium, and low rates to semantic colors', () => {
    expect(getRateColor(80)).toBe('var(--success)')
    expect(getRateColor(50)).toBe('var(--warning)')
    expect(getRateColor(49.9)).toBe('var(--error)')
  })
})
