import { describe, expect, it } from 'vitest'

import type { PaginatedListState } from './usePaginatedListLogic'
import {
  errorMessage,
  initialPaginatedListState,
  pageParamValue,
  paginatedListReducer,
  parsePageParam,
} from './usePaginatedListLogic'

describe('initialPaginatedListState', () => {
  it('starts loading with an empty list, like the screens it replaces', () => {
    expect(initialPaginatedListState<string>()).toEqual({
      items: [],
      total: 0,
      loading: true,
      error: null,
    })
  })
})

describe('paginatedListReducer', () => {
  const loaded: PaginatedListState<string> = {
    items: ['a', 'b'],
    total: 12,
    loading: false,
    error: null,
  }

  it('keeps current items visible while reloading (LoadingOverlay behavior)', () => {
    const next = paginatedListReducer(loaded, { type: 'fetch-start' })
    expect(next).toEqual({ ...loaded, loading: true })
  })

  it('replaces items and clears the error on success', () => {
    const errored = { ...loaded, error: 'boom' }
    const next = paginatedListReducer(errored, {
      type: 'fetch-success',
      items: ['c'],
      total: 1,
    })
    expect(next).toEqual({ items: ['c'], total: 1, loading: false, error: null })
  })

  it('keeps stale rows on fetch error, matching current screen behavior', () => {
    const next = paginatedListReducer(loaded, { type: 'fetch-error', error: 'HTTP 500' })
    expect(next).toEqual({ ...loaded, loading: false, error: 'HTTP 500' })
  })

  it('routes mutation errors into the shared channel and lets them be cleared', () => {
    const withError = paginatedListReducer(loaded, { type: 'set-error', error: 'revoke failed' })
    expect(withError.error).toBe('revoke failed')
    expect(withError.loading).toBe(false)
    const cleared = paginatedListReducer(withError, { type: 'set-error', error: null })
    expect(cleared.error).toBeNull()
  })
})

describe('parsePageParam / pageParamValue (syncToUrl)', () => {
  it('parses valid page params and falls back to page 1 otherwise', () => {
    expect(parsePageParam('3')).toBe(3)
    expect(parsePageParam('1')).toBe(1)
    expect(parsePageParam('')).toBe(1)
    expect(parsePageParam(null)).toBe(1)
    expect(parsePageParam(undefined)).toBe(1)
    expect(parsePageParam('0')).toBe(1)
    expect(parsePageParam('-2')).toBe(1)
    expect(parsePageParam('abc')).toBe(1)
  })

  it('keeps page 1 out of the URL and round-trips other pages', () => {
    expect(pageParamValue(1)).toBe('')
    expect(pageParamValue(0)).toBe('')
    expect(pageParamValue(5)).toBe('5')
    expect(parsePageParam(pageParamValue(7))).toBe(7)
    expect(parsePageParam(pageParamValue(1))).toBe(1)
  })
})

describe('errorMessage', () => {
  it('mirrors the inline `e instanceof Error ? e.message : String(e)` pattern', () => {
    expect(errorMessage(new Error('nope'))).toBe('nope')
    expect(errorMessage('raw')).toBe('raw')
    expect(errorMessage(404)).toBe('404')
  })
})
