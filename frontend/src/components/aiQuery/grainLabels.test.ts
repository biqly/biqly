import { describe, expect, test } from 'vitest'

import { formatGrainValue } from './grainLabels'

describe('formatGrainValue', () => {
  test('month_of_year → localized full month name', () => {
    expect(formatGrainValue('month_of_year', 5, 'en-US')).toBe('May')
    expect(formatGrainValue('month_of_year', 5, 'tr-TR')).toBe('Mayıs')
    expect(formatGrainValue('month_of_year', 1, 'en-US')).toBe('January')
    expect(formatGrainValue('month_of_year', 12, 'tr-TR')).toBe('Aralık')
    // String ordinals from JSON are accepted.
    expect(formatGrainValue('month_of_year', '3', 'en-US')).toBe('March')
  })

  test('quarter → localized Q label', () => {
    expect(formatGrainValue('quarter', 2, 'en-US')).toBe('Q2')
    expect(formatGrainValue('quarter', 2, 'tr-TR')).toBe('2. Çeyrek')
    expect(formatGrainValue('quarter', 1, 'en-US')).toBe('Q1')
    expect(formatGrainValue('quarter', 4, 'tr-TR')).toBe('4. Çeyrek')
  })

  test('out-of-range values → null', () => {
    expect(formatGrainValue('month_of_year', 0, 'en-US')).toBeNull()
    expect(formatGrainValue('month_of_year', 13, 'en-US')).toBeNull()
    expect(formatGrainValue('quarter', 0, 'en-US')).toBeNull()
    expect(formatGrainValue('quarter', 5, 'en-US')).toBeNull()
  })

  test('non-numeric or non-integer values → null', () => {
    expect(formatGrainValue('month_of_year', 'abc', 'en-US')).toBeNull()
    expect(formatGrainValue('month_of_year', null, 'en-US')).toBeNull()
    expect(formatGrainValue('month_of_year', undefined, 'en-US')).toBeNull()
    expect(formatGrainValue('quarter', 2.5, 'en-US')).toBeNull()
  })

  test('non-grain formats → null (caller uses raw value)', () => {
    expect(formatGrainValue('number', 5, 'en-US')).toBeNull()
    expect(formatGrainValue('date', 5, 'en-US')).toBeNull()
    expect(formatGrainValue(undefined, 5, 'en-US')).toBeNull()
  })
})
