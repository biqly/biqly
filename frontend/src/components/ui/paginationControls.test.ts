import { describe, expect, it } from 'vitest'

import type { PageToken } from './paginationTokens'
import { buildStablePageTokens } from './paginationTokens'

describe('buildStablePageTokens', () => {
  it('shows every page when the total fits in the window', () => {
    expect(buildStablePageTokens(1, 1)).toEqual([1])
    expect(buildStablePageTokens(2, 3)).toEqual([1, 2, 3])
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

  it('uses compact width for small counts and a 7-token window for large totals', () => {
    expect(buildStablePageTokens(1, 1)).toHaveLength(1)
    expect(buildStablePageTokens(2, 3)).toHaveLength(3)
    expect(buildStablePageTokens(4, 7)).toHaveLength(7)
    expect(buildStablePageTokens(1, 20)).toHaveLength(7)
  })
})
