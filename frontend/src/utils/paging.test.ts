import { describe, expect, it } from 'vitest'

import { clampPage, getTotalPages, pageRange, pageSizeSelectOptions } from './paging'

describe('getTotalPages', () => {
  it('computes ceil(total / pageSize) for non-empty lists', () => {
    expect(getTotalPages(1, 10)).toBe(1)
    expect(getTotalPages(10, 10)).toBe(1)
    expect(getTotalPages(11, 10)).toBe(2)
    expect(getTotalPages(95, 10)).toBe(10)
  })

  it('returns 1 for an empty list (OQ-1 policy)', () => {
    // Characterization: AuditLogPanel/DatasourceAccessPanel/SharedResourcesList
    // compute a bare Math.ceil today (totalPages = 0 when total = 0), but the
    // Pagination component clamps with Math.max(1, totalPages), so the rendered
    // result is identical to this policy.
    expect(getTotalPages(0, 10)).toBe(1)
  })

  it('guards against invalid page sizes', () => {
    expect(getTotalPages(50, 0)).toBe(1)
    expect(getTotalPages(50, -5)).toBe(1)
  })
})

describe('clampPage', () => {
  it('clamps into [1, totalPages]', () => {
    expect(clampPage(0, 5)).toBe(1)
    expect(clampPage(-3, 5)).toBe(1)
    expect(clampPage(3, 5)).toBe(3)
    expect(clampPage(9, 5)).toBe(5)
  })

  it('treats totalPages below 1 as a single page', () => {
    expect(clampPage(4, 0)).toBe(1)
  })
})

describe('pageRange', () => {
  it('matches the inline math in ui/Pagination.tsx', () => {
    // start = (currentPage - 1) * itemsPerPage + 1; end = min(currentPage * itemsPerPage, totalItems)
    expect(pageRange(1, 10, 95)).toEqual({ start: 1, end: 10 })
    expect(pageRange(5, 10, 95)).toEqual({ start: 41, end: 50 })
    expect(pageRange(10, 10, 95)).toEqual({ start: 91, end: 95 })
  })

  it('yields 0–0 for an empty list', () => {
    expect(pageRange(1, 10, 0)).toEqual({ start: 0, end: 0 })
  })
})

describe('pageSizeSelectOptions', () => {
  it('maps numeric options to string value/label pairs', () => {
    expect(pageSizeSelectOptions([5, 10, 25, 50])).toEqual([
      { value: '5', label: '5' },
      { value: '10', label: '10' },
      { value: '25', label: '25' },
      { value: '50', label: '50' },
    ])
  })

  it('returns an empty array for no options', () => {
    expect(pageSizeSelectOptions([])).toEqual([])
  })
})
