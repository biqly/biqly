import { describe, expect, it } from 'vitest'

import {
  formatDateOnly,
  formatDateTime,
  formatDurationMs,
  formatTimeOnly,
  getRateColor,
} from './formatters'

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

describe('formatDateTime / formatDateOnly', () => {
  const iso = '2026-06-12T14:30:00Z'

  it('formats with the requested language tag (en vs tr differ)', () => {
    const en = formatDateTime(iso, 'en-US')
    const tr = formatDateTime(iso, 'tr-TR')
    expect(en).toBe(new Date(iso).toLocaleString('en-US'))
    expect(tr).toBe(new Date(iso).toLocaleString('tr-TR'))
    expect(en).not.toBe(tr)
  })

  it('formats date-only values per locale', () => {
    expect(formatDateOnly(iso, 'en-US')).toBe(new Date(iso).toLocaleDateString('en-US'))
    expect(formatDateOnly(iso, 'tr-TR')).toBe(new Date(iso).toLocaleDateString('tr-TR'))
  })

  it('accepts Date instances', () => {
    const d = new Date(iso)
    expect(formatDateTime(d, 'en-US')).toBe(d.toLocaleString('en-US'))
    expect(formatDateOnly(d, 'tr-TR')).toBe(d.toLocaleDateString('tr-TR'))
  })

  it('passes unparseable strings through as-is (AuditLog convention)', () => {
    expect(formatDateTime('not-a-date', 'en-US')).toBe('not-a-date')
    expect(formatDateOnly('', 'tr-TR')).toBe('')
  })
})

describe('formatTimeOnly', () => {
  it('formats compact time values per locale', () => {
    const iso = '2026-06-12T14:30:00Z'
    expect(formatTimeOnly(iso, 'en-US')).toBe(
      new Date(iso).toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit' }),
    )
    expect(formatTimeOnly(iso, 'tr-TR')).toBe(
      new Date(iso).toLocaleTimeString('tr-TR', { hour: '2-digit', minute: '2-digit' }),
    )
  })
})

describe('getRateColor', () => {
  it('maps high, medium, and low rates to semantic colors', () => {
    expect(getRateColor(80)).toBe('var(--success)')
    expect(getRateColor(50)).toBe('var(--warning)')
    expect(getRateColor(49.9)).toBe('var(--error)')
  })
})
