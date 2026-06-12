import { describe, expect, it } from 'vitest'

import type { PageToken } from './paginationTokens'
import { buildStablePageTokens } from './paginationTokens'

// Characterization tests (Faz 0.2): lock the current 7-slot stable window
// behavior before any pagination refactor touches its consumers.
describe('buildStablePageTokens', () => {
  it('pads short page counts up to 7 slots', () => {
    expect(buildStablePageTokens(1, 1)).toEqual([1, 'pad', 'pad', 'pad', 'pad', 'pad', 'pad'])
    expect(buildStablePageTokens(2, 3)).toEqual([1, 2, 3, 'pad', 'pad', 'pad', 'pad'])
    expect(buildStablePageTokens(4, 7)).toEqual([1, 2, 3, 4, 5, 6, 7])
  })

  it('shows the leading window with a single trailing gap near the start', () => {
    const expected: PageToken[] = [1, 2, 3, 4, 5, 'gap', 20]
    expect(buildStablePageTokens(1, 20)).toEqual(expected)
    expect(buildStablePageTokens(4, 20)).toEqual(expected)
  })

  it('centers the current page between two gaps in the middle', () => {
    expect(buildStablePageTokens(5, 20)).toEqual([1, 'gap', 4, 5, 6, 'gap', 20])
    expect(buildStablePageTokens(10, 20)).toEqual([1, 'gap', 9, 10, 11, 'gap', 20])
  })

  it('shows the trailing window with a single leading gap near the end', () => {
    const expected: PageToken[] = [1, 'gap', 16, 17, 18, 19, 20]
    expect(buildStablePageTokens(17, 20)).toEqual(expected)
    expect(buildStablePageTokens(20, 20)).toEqual(expected)
  })

  it('always returns exactly 7 tokens so the control width never shifts', () => {
    for (const total of [1, 3, 7, 8, 20, 500]) {
      for (let current = 1; current <= Math.min(total, 30); current++) {
        expect(buildStablePageTokens(current, total)).toHaveLength(7)
      }
    }
  })
})
