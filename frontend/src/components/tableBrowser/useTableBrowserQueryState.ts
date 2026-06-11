import { useCallback, useEffect, useMemo, useState } from 'react'

const DEFAULT_PAGE_SIZE = 50

export interface TableRowsResult {
  columns?: { name: string }[]
  rows?: unknown[][]
  total?: number | null
}

export interface TableSort {
  column: string
  dir: 'asc' | 'desc'
}

interface FilterPayloadItem {
  id: string
  field: string
  operator: string
  value: string
  caseSensitive?: boolean
}

export function buildTableRowsUrl(datasourceId: string, schema: string, table: string): string {
  return `/api/datasources/${encodeURIComponent(datasourceId)}/tables/${encodeURIComponent(schema)}/${encodeURIComponent(table)}/rows`
}

export function tableRowsBody(
  filterPayload: FilterPayloadItem[],
  sort: TableSort | null,
  limit: number,
  offset: number,
): Record<string, unknown> {
  return {
    filters: filterPayload.map((f) => ({
      column: f.field,
      operator: f.operator,
      value: f.value,
      case_sensitive: f.caseSensitive ?? false,
    })),
    order_by: sort?.column ?? '',
    order_dir: sort?.dir ?? '',
    limit,
    offset,
    include_total: true,
  }
}

function derivePaging(totalRows: number | null, page: number, pageSize: number, rowCount: number) {
  const rangeStart = rowCount > 0 ? page * pageSize + 1 : 0
  const rangeEnd = page * pageSize + rowCount
  const totalPages =
    totalRows != null && totalRows > 0
      ? Math.ceil(totalRows / pageSize)
      : totalRows === 0
        ? 0
        : null
  const lastPageIndex = totalPages != null && totalPages > 0 ? totalPages - 1 : null
  const hasNext = lastPageIndex != null ? page < lastPageIndex : rowCount === pageSize
  return { rangeStart, rangeEnd, totalPages, lastPageIndex, hasNext }
}

/**
 * Fetches the selected table's own rows (independent of the model's base
 * table) through the metadata rows endpoint, with server-side filtering,
 * sorting and pagination.
 */
export function useTableBrowserQueryState({
  datasourceId,
  schema,
  table,
  filterPayload,
  columnOrder,
  postData,
  onPageReset,
  filtersKey,
}: {
  datasourceId: string
  schema: string
  table: string
  filterPayload: FilterPayloadItem[]
  columnOrder: string[]
  postData: <T>(url: string, body: unknown) => Promise<T | null>
  onPageReset: () => void
  filtersKey: string
}) {
  const [sortState, setSortState] = useState<{ key: string; sort: TableSort | null }>({
    key: '',
    sort: null,
  })
  const tableScopeKey = `${datasourceId}:${schema}.${table}`
  const sort = sortState.key === tableScopeKey ? sortState.sort : null
  const sortKey = sort ? `${sort.column}:${sort.dir}` : ''
  const queryScopeKey = `${tableScopeKey}:${filtersKey}:${sortKey}`

  const [resultState, setResultState] = useState<{
    key: string
    result: TableRowsResult | null
  }>({ key: '', result: null })
  const result = resultState.key === queryScopeKey ? resultState.result : null
  const totalRows = result?.total ?? null
  const [pageState, setPageState] = useState({ key: queryScopeKey, page: 0 })
  const page = pageState.key === queryScopeKey ? pageState.page : 0
  const [pageSize, setPageSize] = useState<number>(DEFAULT_PAGE_SIZE)
  const [fetching, setFetching] = useState(false)

  const displayColumnNames = useMemo(() => {
    const fromResult = result?.columns?.map((c) => c.name) ?? []
    if (fromResult.length === 0) {
      return columnOrder
    }
    const inResult = new Set(fromResult)
    const ordered = columnOrder.filter((n) => inResult.has(n))
    for (const n of fromResult) {
      if (!ordered.includes(n)) {
        ordered.push(n)
      }
    }
    return ordered.length > 0 ? ordered : fromResult
  }, [result?.columns, columnOrder])

  const columnIndexByName = useMemo(() => {
    const m = new Map<string, number>()
    result?.columns?.forEach((c, i) => m.set(c.name, i))
    return m
  }, [result?.columns])

  const runDataQuery = useCallback(async () => {
    if (!datasourceId || !schema || !table) {
      return
    }
    void Promise.resolve().then(() => setFetching(true))
    const dataRes = await postData<TableRowsResult>(
      buildTableRowsUrl(datasourceId, schema, table),
      tableRowsBody(filterPayload, sort, pageSize, page * pageSize),
    )
    if (dataRes) {
      setResultState({ key: queryScopeKey, result: dataRes })
    }
    setFetching(false)
  }, [datasourceId, schema, table, filterPayload, sort, page, pageSize, postData, queryScopeKey])

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void runDataQuery()
  }, [runDataQuery])

  const rowCount = result?.rows?.length ?? 0
  const { rangeStart, rangeEnd, totalPages, lastPageIndex, hasNext } = derivePaging(
    totalRows,
    page,
    pageSize,
    rowCount,
  )

  const setPage = useCallback(
    (next: number) => {
      setPageState({ key: queryScopeKey, page: next })
      onPageReset()
    },
    [onPageReset, queryScopeKey],
  )

  const goToPage = useCallback(
    (next: number) => {
      setPage(next)
    },
    [setPage],
  )

  const toggleSort = useCallback(
    (column: string) => {
      setSortState((prev) => {
        const current = prev.key === tableScopeKey ? prev.sort : null
        let next: TableSort | null
        if (current?.column !== column) {
          next = { column, dir: 'asc' }
        } else if (current.dir === 'asc') {
          next = { column, dir: 'desc' }
        } else {
          next = null
        }
        return { key: tableScopeKey, sort: next }
      })
      setPageState({ key: '', page: 0 })
      onPageReset()
    },
    [onPageReset, tableScopeKey],
  )

  const hasTableData = Boolean(result?.columns?.length)
  const showInitialPlaceholder = fetching && !hasTableData
  const showTablePanel = hasTableData || showInitialPlaceholder

  const resetQueryState = useCallback(() => {
    setResultState({ key: queryScopeKey, result: null })
    setPageState({ key: queryScopeKey, page: 0 })
  }, [queryScopeKey])

  return {
    result,
    totalRows,
    page,
    pageSize,
    setPageSize,
    fetching,
    displayColumnNames,
    columnIndexByName,
    rowCount,
    rangeStart,
    rangeEnd,
    totalPages,
    lastPageIndex,
    hasNext,
    goToPage,
    sort,
    toggleSort,
    showTablePanel,
    showInitialPlaceholder,
    resetQueryState,
    setPage,
  }
}
