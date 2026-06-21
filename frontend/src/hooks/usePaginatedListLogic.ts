/**
 * Pure state transitions for usePaginatedList, extracted for testability
 * (repo pattern: logic-extraction + pure vitest, no component render tests).
 */

export interface PaginatedListState<T> {
  items: T[]
  total: number
  loading: boolean
  error: string | null
}

export type PaginatedListAction<T> =
  | { type: 'fetch-start' }
  | { type: 'fetch-success'; items: T[]; total: number }
  | { type: 'fetch-error'; error: string }
  | { type: 'set-error'; error: string | null }

/** Mirrors the screens being replaced: they all start with loading=true before the first fetch. */
export function initialPaginatedListState<T>(): PaginatedListState<T> {
  return { items: [], total: 0, loading: true, error: null }
}

export function paginatedListReducer<T>(
  state: PaginatedListState<T>,
  action: PaginatedListAction<T>,
): PaginatedListState<T> {
  switch (action.type) {
    case 'fetch-start':
      // Keep current items visible while reloading (LoadingOverlay-on-top behavior).
      return { ...state, loading: true }
    case 'fetch-success':
      return { items: action.items, total: action.total, loading: false, error: null }
    case 'fetch-error':
      // Screens keep stale rows on error today; preserve that.
      return { ...state, loading: false, error: action.error }
    case 'set-error':
      // Mutation handlers (grant/revoke/...) report into the same error channel,
      // and a later successful reload clears it — exactly like the old setError.
      return { ...state, error: action.error }
  }
}

// Canonical implementation moved to ../utils/error. Re-exported here so existing
// `from '../hooks/usePaginatedListLogic'` imports keep working.
export { errorMessage } from '../utils/error'

/** Page number from a URL query param (syncToUrl); anything invalid or < 1 is page 1. */
export function parsePageParam(value: string | null | undefined): number {
  const n = Number.parseInt(value ?? '', 10)
  return Number.isFinite(n) && n >= 1 ? n : 1
}

/** URL representation of a page: page 1 is the default and stays out of the URL. */
export function pageParamValue(page: number): string {
  return page <= 1 ? '' : String(page)
}
