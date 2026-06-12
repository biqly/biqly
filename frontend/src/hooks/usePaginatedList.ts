import { useCallback, useEffect, useReducer, useRef, useState } from 'react'

import type { PagedResult, PageQuery } from '../types/pagination'
import { getTotalPages } from '../utils/paging'
import type { PaginatedListAction, PaginatedListState } from './usePaginatedListLogic'
import {
  errorMessage,
  initialPaginatedListState,
  pageParamValue,
  paginatedListReducer,
  parsePageParam,
} from './usePaginatedListLogic'
import { useQueryParam } from './useQueryParam'

export interface UsePaginatedListOptions<T> {
  /**
   * Loads one page. Wrap the existing api/ function and map its
   * `{ <key>: T[], total }` shape into PagedResult — the request itself
   * (URL, params) must stay byte-identical to the screen's previous code.
   * The signal may be ignored; staleness is also guarded hook-side.
   */
  fetcher: (query: PageQuery, signal: AbortSignal) => Promise<PagedResult<T>>
  initialPageSize: number
  /** When false, nothing is fetched and loading stays as-is (e.g. missing auth token). */
  enabled?: boolean
  /** Refetch (keeping the current page) when this primitive changes — compose with template strings if needed. */
  fetchKey?: string | number | boolean | null
  /** Reset to page 1 and refetch when this primitive changes (filter semantics). */
  resetPageKey?: string | number | boolean | null
  /**
   * URL query param name to mirror the page into (OQ-2, Faz 7.6). The URL
   * becomes the source of truth: deep links and refresh restore the page,
   * back/forward navigation works via useQueryParam, page 1 stays out of the
   * URL. Pick a name unique within the screen (e.g. 'auditPage').
   */
  syncToUrl?: string
}

export interface PaginatedList<T> extends PaginatedListState<T> {
  page: number
  setPage: (page: number) => void
  pageSize: number
  setPageSize: (size: number) => void
  totalPages: number
  /** Imperative refetch of the current page (after grant/revoke/delete...). */
  reload: () => void
  /** Surface a mutation error into the shared error channel (cleared by the next successful load). */
  setError: (error: string | null) => void
}

/**
 * Standard server-side list pagination state (Faz 1.1,
 * tasks/frontend-table-pagination-standardization.md). Replaces the hand-rolled
 * currentPage/totalItems/loading/error blocks in the paginated screens.
 */
export function usePaginatedList<T>(options: UsePaginatedListOptions<T>): PaginatedList<T> {
  const {
    fetcher,
    initialPageSize,
    enabled = true,
    fetchKey = null,
    resetPageKey = null,
    syncToUrl,
  } = options

  const [internalPage, setInternalPage] = useState(1)
  // Unconditional hook call; with no syncToUrl the '' param reads as '' and is never written.
  const [pageParam, setPageParam] = useQueryParam(syncToUrl ?? '')
  const page = syncToUrl ? parsePageParam(pageParam) : internalPage
  const setPage = useCallback(
    (next: number) => {
      if (syncToUrl) {
        setPageParam(pageParamValue(next))
      } else {
        setInternalPage(next)
      }
    },
    [syncToUrl, setPageParam],
  )
  const [pageSize, setPageSize] = useState(initialPageSize)
  const [state, dispatch] = useReducer(
    paginatedListReducer as (
      s: PaginatedListState<T>,
      a: PaginatedListAction<T>,
    ) => PaginatedListState<T>,
    undefined,
    initialPaginatedListState<T>,
  )

  // Latest-ref so a new fetcher identity (e.g. filter values in its closure)
  // does not itself trigger a fetch — page changes use the latest values,
  // matching the filter-on-submit screens (AuditLogPanel) exactly.
  const fetcherRef = useRef(fetcher)
  useEffect(() => {
    fetcherRef.current = fetcher
  })

  const [version, setVersion] = useState(0)
  const reload = useCallback(() => setVersion((v) => v + 1), [])
  const setError = useCallback((error: string | null) => dispatch({ type: 'set-error', error }), [])

  const prevResetKeyRef = useRef(resetPageKey)

  useEffect(() => {
    if (!enabled) {
      return
    }
    if (prevResetKeyRef.current !== resetPageKey) {
      prevResetKeyRef.current = resetPageKey
      if (page !== 1) {
        // Filter changed while past page 1: jump back and let the re-run fetch.
        // eslint-disable-next-line react-hooks/set-state-in-effect -- single shared disable replacing one per screen
        setPage(1)
        return
      }
    }
    const controller = new AbortController()
    dispatch({ type: 'fetch-start' })
    void (async () => {
      try {
        const res = await fetcherRef.current({ page, pageSize }, controller.signal)
        if (controller.signal.aborted) {
          return
        }
        dispatch({ type: 'fetch-success', items: res.items, total: res.total })
      } catch (e) {
        if (controller.signal.aborted) {
          return
        }
        dispatch({ type: 'fetch-error', error: errorMessage(e) })
      }
    })()
    return () => controller.abort()
  }, [enabled, page, pageSize, fetchKey, resetPageKey, version, setPage])

  return {
    ...state,
    page,
    setPage,
    pageSize,
    setPageSize,
    totalPages: getTotalPages(state.total, pageSize),
    reload,
    setError,
  }
}
