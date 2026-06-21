import { describe, expect, it } from 'vitest'

import { buildQueryString } from './query'

describe('buildQueryString', () => {
  it('returns empty string when given empty params', () => {
    expect(buildQueryString({})).toBe('')
  })

  it('excludes null, undefined, and empty string values', () => {
    expect(
      buildQueryString({
        a: 1,
        b: null,
        c: undefined,
        d: '',
        e: 'hello',
      }),
    ).toBe('?a=1&e=hello')
  })

  it('correctly formats and encodes valid params', () => {
    expect(
      buildQueryString({
        search: 'john doe',
        active: true,
        limit: 10,
      }),
    ).toBe('?search=john+doe&active=true&limit=10')
  })
})
