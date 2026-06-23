import { describe, expect, it } from 'vitest'

import type { CatalogEntry } from '../../hooks/useSemanticCatalog'
import { findActiveMention, score } from './mentionUtils'

const dim = (name: string, label = name): CatalogEntry => ({
  type: 'dimension',
  name,
  label,
  group: 'Dimensions',
})

describe('findActiveMention', () => {
  it('detects an @ at the start of the input', () => {
    expect(findActiveMention('@rev', 4)).toEqual({ at: 0, query: 'rev' })
  })

  it('detects an @ after whitespace mid-string', () => {
    expect(findActiveMention('show @total', 11)).toEqual({ at: 5, query: 'total' })
  })

  it('returns null without any @', () => {
    expect(findActiveMention('top customers', 13)).toBeNull()
  })

  it('does not fire for email-style strings (a@b)', () => {
    expect(findActiveMention('mail user@host', 14)).toBeNull()
  })

  it('closes the mention once a space follows the query', () => {
    expect(findActiveMention('@rev enue', 9)).toBeNull()
  })

  it('matches an empty query right after @', () => {
    expect(findActiveMention('by @', 4)).toEqual({ at: 3, query: '' })
  })
})

describe('score', () => {
  it('scores every entry equally when the query is empty', () => {
    expect(score(dim('revenue'), '')).toBe(1)
  })

  it('ranks an exact name match highest', () => {
    expect(score(dim('total_revenue'), 'total_revenue')).toBe(100)
  })

  it('ranks a prefix match above a substring match', () => {
    expect(score(dim('total_revenue'), 'total')).toBe(80)
    expect(score(dim('monthly_revenue'), 'revenue')).toBe(50)
  })

  it('returns 0 when nothing matches', () => {
    expect(score(dim('revenue'), 'zzz')).toBe(0)
  })
})
