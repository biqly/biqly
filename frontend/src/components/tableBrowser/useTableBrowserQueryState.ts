import { useCallback, useEffect, useMemo, useState } from 'react'

import type { SemanticDimension, SemanticMetric } from '../../types/semantic'
import { buildQueryPayload } from '../queryBuilder/logicalQuery'

const DEFAULT_PAGE_SIZE = 50

interface QueryBuilderResult {
  columns?: { name: string; type?: string }[]
  rows?: unknown[][]
}

type PageToken = number | 'gap'

function resolveCountMetric(metrics: SemanticMetric[] | undefined): string | null {
  if (!metrics?.length) {
    return null
  }
  const byName = metrics.find((m) => m.name === 'row_count' || m.name === 'count')
  if (byName) {
    return byName.name
  }
  const countAgg = metrics.find((m) => m.aggregation === 'count')
  return countAgg?.name ?? null
}

function parseCountValue(rows: unknown[][] | undefined): number | null {
  if (!rows?.length) {
    return null
  }
  const cell = rows[0]?.[0]
  const n = typeof cell === 'number' ? cell : Number(cell)
  return Number.isFinite(n) ? Math.trunc(n) : null
}

function buildPageList(currentPage: number, totalPages: number): PageToken[] {
  if (totalPages <= 7) {
    return Array.from({ length: totalPages }, (_, i) => i)
  }
  const pages = new Set<number>([0, totalPages - 1, currentPage])
  if (currentPage > 0) {
    pages.add(currentPage - 1)
  }
  if (currentPage < totalPages - 1) {
    pages.add(currentPage + 1)
  }
  const sorted = [...pages].sort((a, b) => a - b)
  const out: PageToken[] = []
  for (let i = 0; i < sorted.length; i++) {
    const p = sorted[i]!
    if (i > 0 && p - sorted[i - 1]! > 1) {
      out.push('gap')
    }
    out.push(p)
  }
  return out
}

interface FilterPayloadItem {
  id: string
  field: string
  operator: string
  value: string
  caseSensitive?: boolean
}

export function useTableBrowserQueryState({
  datasourceId,
  modelId,
  modelMetrics,
  orderedDimensions,
  filterPayload,
  columnOrder,
  postData,
  onPageReset,
  filtersKey,
}: {
  datasourceId: string
  modelId: string
  modelMetrics: SemanticMetric[] | undefined
  orderedDimensions: SemanticDimension[]
  filterPayload: FilterPayloadItem[]
  columnOrder: string[]
  postData: <T>(url: string, body: unknown) => Promise<T | null>
  onPageReset: () => void
  filtersKey: string
}) {
  const queryScopeKey = `${datasourceId}:${modelId}:${filtersKey}:${columnOrder.join('\0')}`
  const [resultState, setResultState] = useState<{
    key: string
    result: QueryBuilderResult | null
    totalRows: number | null
  }>({ key: '', result: null, totalRows: null })
  const result = resultState.key === queryScopeKey ? resultState.result : null
  const totalRows = resultState.key === queryScopeKey ? resultState.totalRows : null
  const [pageState, setPageState] = useState({ key: queryScopeKey, page: 0 })
  const page = pageState.key === queryScopeKey ? pageState.page : 0
  const [pageSize, setPageSize] = useState<number>(DEFAULT_PAGE_SIZE)
  const [fetching, setFetching] = useState(false)

  const countMetricName = useMemo(() => resolveCountMetric(modelMetrics), [modelMetrics])

  const queryBase = useMemo(() => {
    if (!datasourceId || !modelId || orderedDimensions.length === 0) {
      return null
    }
    return {
      datasourceId,
      modelId,
      mode: 'simple' as const,
      filters: filterPayload,
      groupBy: [] as string[],
      having: [],
      orderBy: '',
      orderDir: 'asc' as const,
      windowFunctions: [],
      ctes: [],
      selectItems: orderedDimensions.map((d) => ({
        id: d.id,
        type: 'dimension' as const,
        name: d.name,
      })),
    }
  }, [datasourceId, modelId, orderedDimensions, filterPayload])

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

  const runCountQuery = useCallback(async () => {
    if (!queryBase || !countMetricName) {
      void Promise.resolve().then(() =>
        setResultState((prev) =>
          prev.key === queryScopeKey ? { ...prev, totalRows: null } : prev,
        ),
      )
      return
    }
    const countRes = await postData<QueryBuilderResult>(
      '/api/query/run',
      buildQueryPayload({
        ...queryBase,
        selectItems: [{ id: 'count', type: 'metric', name: countMetricName }],
        limit: 1,
        offset: 0,
      }),
    )
    if (countRes) {
      setResultState((prev) =>
        prev.key === queryScopeKey ? { ...prev, totalRows: parseCountValue(countRes.rows) } : prev,
      )
    }
  }, [queryBase, countMetricName, postData, queryScopeKey])

  const runDataQuery = useCallback(async () => {
    if (!queryBase) {
      return
    }
    void Promise.resolve().then(() => setFetching(true))
    const dataRes = await postData<QueryBuilderResult>(
      '/api/query/run',
      buildQueryPayload({
        ...queryBase,
        limit: pageSize,
        offset: page * pageSize,
      }),
    )
    if (dataRes) {
      setResultState((prev) => ({
        key: queryScopeKey,
        result: dataRes,
        totalRows: prev.key === queryScopeKey ? prev.totalRows : null,
      }))
    }
    setFetching(false)
  }, [queryBase, page, pageSize, postData, queryScopeKey])

  useEffect(() => {
    void runCountQuery()
  }, [runCountQuery])

  useEffect(() => {
    void runDataQuery()
  }, [runDataQuery])

  const rowCount = result?.rows?.length ?? 0
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

  const pageList = useMemo(() => {
    if (totalPages == null || totalPages <= 1) {
      return null
    }
    return buildPageList(page, totalPages)
  }, [page, totalPages])

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

  const hasTableData = Boolean(result?.columns?.length)
  const showInitialPlaceholder = fetching && !hasTableData
  const showTablePanel = hasTableData || showInitialPlaceholder

  const resetQueryState = useCallback(() => {
    setResultState({ key: queryScopeKey, result: null, totalRows: null })
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
    pageList,
    lastPageIndex,
    hasNext,
    goToPage,
    showTablePanel,
    showInitialPlaceholder,
    resetQueryState,
    setPage,
  }
}
