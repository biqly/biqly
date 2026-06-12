import { describe, expect, it } from 'vitest'

import type { SortState } from './sorting'
import { ariaSortFor, compareValues, sortArrowFor, sortRows, toggleSort } from './sorting'

describe('toggleSort', () => {
  it('cycles asc → desc → none on the same key (resultTable semantics)', () => {
    const first = toggleSort(null, 'name')
    expect(first).toEqual({ key: 'name', dir: 'asc' })
    const second = toggleSort(first, 'name')
    expect(second).toEqual({ key: 'name', dir: 'desc' })
    expect(toggleSort(second, 'name')).toBeNull()
  })

  it('switching to another key always starts ascending', () => {
    const current: SortState = { key: 'name', dir: 'desc' }
    expect(toggleSort(current, 'date')).toEqual({ key: 'date', dir: 'asc' })
  })
})

describe('compareValues', () => {
  it('sorts numerically when both values are numeric', () => {
    expect(compareValues(2, 10, 1)).toBeLessThan(0)
    expect(compareValues('2', '10', 1)).toBeLessThan(0)
  })

  it('falls back to locale string compare otherwise', () => {
    expect(compareValues('apple', 'banana', 1)).toBeLessThan(0)
    expect(compareValues('apple', 'banana', -1)).toBeGreaterThan(0)
  })

  it('pushes nulls to the end regardless of direction', () => {
    expect(compareValues(null, 'a', 1)).toBe(1)
    expect(compareValues('a', null, 1)).toBe(-1)
    expect(compareValues(null, null, 1)).toBe(0)
  })
})

describe('sortRows', () => {
  const rows = [{ n: 3 }, { n: 1 }, { n: 2 }]

  it('returns a sorted copy without mutating the input', () => {
    const sorted = sortRows(rows, { key: 'n', dir: 'asc' }, (r) => r.n)
    expect(sorted.map((r) => r.n)).toEqual([1, 2, 3])
    expect(rows.map((r) => r.n)).toEqual([3, 1, 2])
  })

  it('returns the same reference when no sort is active', () => {
    expect(sortRows(rows, null, (r) => r.n)).toBe(rows)
  })
})

describe('aria/arrow helpers', () => {
  it('reflects the active column only', () => {
    const sort: SortState = { key: 'name', dir: 'desc' }
    expect(ariaSortFor(sort, 'name')).toBe('descending')
    expect(ariaSortFor(sort, 'date')).toBe('none')
    expect(ariaSortFor(null, 'name')).toBe('none')
    expect(sortArrowFor(sort, 'name')).toBe('↓')
    expect(sortArrowFor(sort, 'date')).toBe('')
  })
})
