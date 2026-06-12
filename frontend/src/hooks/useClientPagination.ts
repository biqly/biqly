import { useMemo, useState } from 'react'

import { clampPage, getTotalPages, sliceClientPage } from '../utils/paging'

export interface ClientPagination<T> {
  /** Current page, already clamped into range (rows can shrink under the page). */
  page: number
  setPage: (page: number) => void
  pageSize: number
  setPageSize: (size: number) => void
  totalPages: number
  total: number
  /** The rows of the current page. */
  pageRows: T[]
}

/**
 * Client-side slice pagination over an in-memory list (Faz 1.6,
 * tasks/frontend-table-pagination-standardization.md). The caller keeps
 * owning the rows (fetching, mutating); this hook only owns page state.
 */
export function useClientPagination<T>(rows: T[], initialPageSize: number): ClientPagination<T> {
  const [rawPage, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(initialPageSize)

  const totalPages = getTotalPages(rows.length, pageSize)
  const page = clampPage(rawPage, totalPages)
  const pageRows = useMemo(() => sliceClientPage(rows, page, pageSize), [rows, page, pageSize])

  return { page, setPage, pageSize, setPageSize, totalPages, total: rows.length, pageRows }
}
