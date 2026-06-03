import '../styles/tableBrowser.css'

import { type DragEvent, useCallback, useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'

import { useApi } from '../hooks/useApi'
import { useDatasources } from '../hooks/useDatasources'
import { useModelDetail } from '../hooks/useModelDetail'
import { useSemanticModels } from '../hooks/useSemanticModels'
import { useLocale, useT } from '../i18n'
import type { SemanticDimension, SemanticMetric, SemanticModelDetail } from '../types/semantic'
import { modelListHint, modelListLabel } from '../types/semantic'
import { localeNumberTag } from '../utils/formatters'
import { formatResultCell } from '../utils/resultCellFormat'
import { columnRefMatchesTable, splitTableKey, tableKey } from './modeling/utils'
import { buildQueryPayload } from './queryBuilder/logicalQuery'
import { ErrorAlert } from './ui/ErrorAlert'
import { LoadingOverlay } from './ui/LoadingOverlay'
import { LoadingScreen } from './ui/LoadingScreen'
import { Modal } from './ui/Modal'
import { Select } from './ui/Select'

const PAGE_SIZE_OPTIONS = [25, 50, 100] as const
const DEFAULT_PAGE_SIZE = 50

interface QueryBuilderResult {
  columns?: { name: string; type?: string }[]
  rows?: unknown[][]
  stats?: {
    row_count?: number
    duration_ms?: number
  }
}

interface TableBrowserFilter {
  id: string
  field: string
  operator: string
  value: string
  caseSensitive?: boolean
}

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

type PageToken = number | 'gap'

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

interface DetailRowState {
  displayIndex: number
  row: unknown[]
}

const PREFERRED_TITLE_COLUMN_PATTERNS = [
  /^(title|name|label|subject|headline|heading|display_name)$/,
  /^(text|body|content|message|description|caption|summary)$/,
  /(_|^)(title|name|label|subject)$/,
  /(_|^)(text|body|content|message|description)$/,
]

const ID_COLUMN_PATTERNS = [/^(id|uuid|pk)$/, /(_|^)id$/]

type TFn = ReturnType<typeof useT>

const VALIDATION_FIELD_REGEX = /unknown (?:dimension|metric|field): ([\w.]+)/g

function ValidationErrorBanner({
  error,
  t,
  onOpenModeling,
}: {
  error: string | null | undefined
  t: TFn
  onOpenModeling: () => void
}) {
  const [expanded, setExpanded] = useState(false)
  if (!error) {
    return null
  }

  const isValidation = /validation failed/i.test(error)
  if (!isValidation) {
    return <ErrorAlert error={error} />
  }

  const matches = Array.from(error.matchAll(VALIDATION_FIELD_REGEX))
  const fields = Array.from(new Set(matches.map((m) => m[1]).filter(Boolean))) as string[]
  const count = fields.length

  return (
    <div className="error validation-error-banner" role="alert">
      <div className="validation-error-banner__row">
        <span className="validation-error-banner__title">
          ⚠ {t('table_browser.validation_error_summary', { count: String(count) })}
        </span>
        <div className="validation-error-banner__actions">
          {count > 0 && (
            <button type="button" className="btn btn-sm" onClick={() => setExpanded((v) => !v)}>
              {expanded
                ? t('table_browser.validation_error_hide')
                : t('table_browser.validation_error_show')}
            </button>
          )}
          <button type="button" className="btn btn-sm btn-primary" onClick={onOpenModeling}>
            {t('table_browser.validation_error_open_modeling')}
          </button>
        </div>
      </div>
      {expanded && count > 0 && (
        <ul className="validation-error-banner__list">
          {fields.map((f) => (
            <li key={f}>
              <code>{f}</code>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}

function singularize(name: string): string {
  const n = name.toLowerCase()
  if (n.endsWith('ies')) {
    return `${n.slice(0, -3)}y`
  }
  if (n.endsWith('s') && !n.endsWith('ss')) {
    return n.slice(0, -1)
  }
  return n
}

function buildRowModalTitle(
  row: unknown[],
  columns: string[],
  fallback: string,
  tableKeyValue?: string | null,
): string {
  const stringValues: { name: string; value: string }[] = []
  for (let i = 0; i < columns.length; i++) {
    const v = row[i]
    if (v == null) {
      continue
    }
    const s = typeof v === 'string' ? v : typeof v === 'number' ? String(v) : ''
    const trimmed = s.trim()
    if (!trimmed) {
      continue
    }
    const colName = columns[i]
    if (!colName) {
      continue
    }
    stringValues.push({ name: colName.toLowerCase(), value: trimmed })
  }
  if (stringValues.length === 0) {
    return fallback
  }

  const truncate = (s: string) => (s.length > 80 ? `${s.slice(0, 77).trimEnd()}…` : s)

  for (const pattern of PREFERRED_TITLE_COLUMN_PATTERNS) {
    const hit = stringValues.find((c) => pattern.test(c.name))
    if (hit) {
      return truncate(hit.value)
    }
  }

  // Prefer the table's own PK column (e.g. order_id for orders) over foreign keys.
  if (tableKeyValue) {
    const lastSegment = tableKeyValue.split('.').pop() ?? tableKeyValue
    const singular = singularize(lastSegment)
    const pkHit = stringValues.find(
      (c) =>
        c.name === `${singular}_id` ||
        c.name === `${lastSegment.toLowerCase()}_id` ||
        c.name === 'id',
    )
    if (pkHit) {
      return truncate(`${pkHit.name} ${pkHit.value}`)
    }
  }

  // Fall back to first id-like column.
  for (const pattern of ID_COLUMN_PATTERNS) {
    const hit = stringValues.find((c) => pattern.test(c.name))
    if (hit) {
      return truncate(`${hit.name} ${hit.value}`)
    }
  }
  return fallback
}

function collectModelTables(model: SemanticModelDetail): { value: string; label: string }[] {
  const seen = new Set<string>()
  const out: { value: string; label: string }[] = []
  const add = (schema: string, table: string) => {
    const key = tableKey(schema, table)
    if (seen.has(key)) {
      return
    }
    seen.add(key)
    out.push({ value: key, label: key })
  }
  add(model.base_schema, model.base_table)
  for (const j of model.joins ?? []) {
    if (j.is_active === false) {
      continue
    }
    add(j.from_schema ?? model.base_schema, j.from_table)
    add(j.to_schema ?? model.base_schema, j.to_table)
  }
  return out.sort((a, b) => a.label.localeCompare(b.label))
}

function reorderColumnNames(order: string[], source: string, target: string): string[] {
  const next = order.filter((n) => n !== source)
  const idx = next.indexOf(target)
  if (idx === -1) {
    next.push(source)
  } else {
    next.splice(idx, 0, source)
  }
  return next
}

export default function TableBrowser() {
  const navigate = useNavigate()
  const t = useT()
  const [locale] = useLocale()
  const localeTag = localeNumberTag(locale)
  const formatInt = useCallback((n: number) => n.toLocaleString(localeTag), [localeTag])
  const { get, postData, error } = useApi()

  const { datasources, loading: dsLoading } = useDatasources()
  const [datasourceId, setDatasourceId] = useState('')
  const { models, loading: modelsLoading } = useSemanticModels(datasourceId)
  const [modelId, setModelId] = useState('')
  const {
    model: modelDetail,
    setModel: setModelDetail,
    loading: modelLoading,
  } = useModelDetail(modelId)
  const [selectedTableKey, setSelectedTableKey] = useState('')
  const [columnOrder, setColumnOrder] = useState<string[]>([])
  const [dragColumn, setDragColumn] = useState<string | null>(null)
  const [dropTargetColumn, setDropTargetColumn] = useState<string | null>(null)

  const [filters, setFilters] = useState<TableBrowserFilter[]>([])
  const [result, setResult] = useState<QueryBuilderResult | null>(null)
  const [totalRows, setTotalRows] = useState<number | null>(null)
  const [page, setPage] = useState(0)
  const [pageSize, setPageSize] = useState<number>(DEFAULT_PAGE_SIZE)
  const [fetching, setFetching] = useState(false)
  const [detailRow, setDetailRow] = useState<DetailRowState | null>(null)

  const [popoverOpen, setPopoverOpen] = useState(false)
  const [popoverField, setPopoverField] = useState('')
  const [popoverOperator, setPopoverOperator] = useState('contains')
  const [popoverChips, setPopoverChips] = useState<string[]>([])
  const [chipInputText, setChipInputText] = useState('')
  const [popoverCaseSensitive, setPopoverCaseSensitive] = useState(false)
  const [editingFilterId, setEditingFilterId] = useState<string | null>(null)

  const countMetricName = useMemo(
    () => resolveCountMetric(modelDetail?.metrics),
    [modelDetail?.metrics],
  )

  // Set default datasourceId
  useEffect(() => {
    if (datasources.length > 0 && !datasourceId) {
      setDatasourceId(datasources[0]!.id)
    }
  }, [datasources, datasourceId])

  // Set default modelId when models change
  useEffect(() => {
    if (models.length > 0) {
      const published = models.filter((m) => m.status === 'published')
      const firstPub = published[0]
      const firstData = models[0]
      if (firstPub) {
        setModelId(firstPub.id)
      } else if (firstData) {
        setModelId(firstData.id)
      }
    } else {
      setModelId('')
      setResult(null)
    }
  }, [models])

  useEffect(() => {
    setResult(null)
    setTotalRows(null)
  }, [modelId])

  useEffect(() => {
    if (modelDetail) {
      setSelectedTableKey(tableKey(modelDetail.base_schema, modelDetail.base_table))
      setFilters([])
      setPage(0)
      setDetailRow(null)
    }
  }, [modelDetail])

  useEffect(() => {
    setPage(0)
    setDetailRow(null)
  }, [filters])

  const tableOptions = useMemo(() => {
    if (!modelDetail) {
      return []
    }
    return collectModelTables(modelDetail)
  }, [modelDetail])

  const activeDimensions = useMemo(() => {
    if (!modelDetail || !selectedTableKey) {
      return []
    }
    const { schema, table } = splitTableKey(selectedTableKey)
    return (modelDetail.dimensions ?? []).filter(
      (d) =>
        d.is_active !== false &&
        columnRefMatchesTable(d.column_ref, schema, table, modelDetail.base_schema),
    )
  }, [modelDetail, selectedTableKey])

  const dimensionNamesKey = useMemo(
    () =>
      activeDimensions
        .map((d) => d.name)
        .sort()
        .join('\0'),
    [activeDimensions],
  )

  useEffect(() => {
    const names = activeDimensions.map((d) => d.name).sort((a, b) => a.localeCompare(b))
    setColumnOrder(names)
    setPage(0)
    setDetailRow(null)
    setFilters((prev) => prev.filter((f) => names.includes(f.field)))
  }, [modelId, selectedTableKey, dimensionNamesKey])

  const orderedDimensions = useMemo(() => {
    const byName = new Map(activeDimensions.map((d) => [d.name, d]))
    const ordered: SemanticDimension[] = []
    for (const name of columnOrder) {
      const d = byName.get(name)
      if (d) {
        ordered.push(d)
      }
    }
    for (const d of activeDimensions) {
      if (!columnOrder.includes(d.name)) {
        ordered.push(d)
      }
    }
    return ordered
  }, [activeDimensions, columnOrder])

  const filterPayload = useMemo(
    () =>
      filters.map((f) => ({
        id: f.id,
        field: f.field,
        operator: f.operator,
        value: f.value,
        caseSensitive: f.caseSensitive,
      })),
    [filters],
  )

  const queryBase = useMemo(() => {
    if (!datasourceId || !modelId || !modelDetail || orderedDimensions.length === 0) {
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
  }, [datasourceId, modelId, modelDetail, orderedDimensions, filterPayload])

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
    if (!queryBase) {
      setTotalRows(null)
      return
    }
    if (!countMetricName) {
      setTotalRows(null)
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
      setTotalRows(parseCountValue(countRes.rows))
    }
  }, [queryBase, countMetricName, postData])

  const runDataQuery = useCallback(async () => {
    if (!queryBase) {
      return
    }

    setFetching(true)
    const dataRes = await postData<QueryBuilderResult>(
      '/api/query/run',
      buildQueryPayload({
        ...queryBase,
        limit: pageSize,
        offset: page * pageSize,
      }),
    )
    if (dataRes) {
      setResult(dataRes)
    }
    setFetching(false)
  }, [queryBase, page, pageSize, postData])

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

  const goToPage = useCallback((next: number) => {
    setPage(next)
    setDetailRow(null)
  }, [])

  const hasTableData = Boolean(result?.columns?.length)
  const showInitialPlaceholder = fetching && !hasTableData
  const showTablePanel = hasTableData || showInitialPlaceholder

  const handleAddChip = (text: string) => {
    const clean = text.trim()
    if (clean && !popoverChips.includes(clean)) {
      setPopoverChips((prev) => [...prev, clean])
    }
    setChipInputText('')
  }

  const handleRemoveChip = (index: number) => {
    setPopoverChips((prev) => prev.filter((_, i) => i !== index))
  }

  const handleSaveFilter = () => {
    if (!popoverField) {
      return
    }

    const finalChips = [...popoverChips]
    const textVal = chipInputText.trim()
    if (textVal && !finalChips.includes(textVal)) {
      finalChips.push(textVal)
    }

    if (finalChips.length === 0) {
      return
    }

    const finalValue = finalChips.length > 1 ? JSON.stringify(finalChips) : finalChips[0] || ''

    if (editingFilterId) {
      setFilters((prev) =>
        prev.map((f) =>
          f.id === editingFilterId
            ? {
                ...f,
                field: popoverField,
                operator: popoverOperator,
                value: finalValue,
                caseSensitive: popoverCaseSensitive,
              }
            : f,
        ),
      )
    } else {
      setFilters((prev) => [
        ...prev,
        {
          id: Math.random().toString(36).substr(2, 9),
          field: popoverField,
          operator: popoverOperator,
          value: finalValue,
          caseSensitive: popoverCaseSensitive,
        },
      ])
    }
    setPopoverOpen(false)
    setEditingFilterId(null)
    setPopoverChips([])
    setChipInputText('')
    setPopoverCaseSensitive(false)
  }

  const handleColumnDragStart = (colName: string) => (e: DragEvent) => {
    setDragColumn(colName)
    e.dataTransfer.effectAllowed = 'move'
    e.dataTransfer.setData('text/plain', colName)
  }

  const handleColumnDragOver = (colName: string) => (e: DragEvent) => {
    e.preventDefault()
    e.dataTransfer.dropEffect = 'move'
    if (dragColumn && dragColumn !== colName) {
      setDropTargetColumn(colName)
    }
  }

  const handleColumnDrop = (colName: string) => (e: DragEvent) => {
    e.preventDefault()
    const source = e.dataTransfer.getData('text/plain') || dragColumn
    if (source && source !== colName) {
      setColumnOrder((prev) =>
        reorderColumnNames(prev.length ? prev : displayColumnNames, source, colName),
      )
    }
    setDragColumn(null)
    setDropTargetColumn(null)
  }

  const handleColumnDragEnd = () => {
    setDragColumn(null)
    setDropTargetColumn(null)
  }

  const handleOpenAddFilter = (defaultField = '') => {
    setEditingFilterId(null)
    setPopoverField(defaultField || (activeDimensions[0]?.name ?? ''))
    setPopoverOperator('contains')
    setPopoverChips([])
    setChipInputText('')
    setPopoverCaseSensitive(false)
    setPopoverOpen(true)
  }

  const handleOpenEditFilter = (filter: TableBrowserFilter) => {
    setEditingFilterId(filter.id)
    setPopoverField(filter.field)
    setPopoverOperator(filter.operator)
    setPopoverCaseSensitive(!!filter.caseSensitive)

    let chips: string[] = []
    if (filter.value.startsWith('[') && filter.value.endsWith(']')) {
      try {
        chips = JSON.parse(filter.value)
      } catch {
        chips = [filter.value]
      }
    } else if (filter.value) {
      chips = [filter.value]
    }
    setPopoverChips(chips)
    setChipInputText('')
    setPopoverOpen(true)
  }

  const handleRemoveFilter = (id: string) => {
    setFilters((prev) => prev.filter((f) => f.id !== id))
  }

  const getDimensionLabel = (name: string) => {
    const dim = activeDimensions.find((d) => d.name === name)
    return dim ? dim.label || dim.name : name
  }

  const formatFilterValue = (value: string) => {
    let raw = value
    if (value.startsWith('[') && value.endsWith(']')) {
      try {
        const arr = JSON.parse(value) as string[]
        if (arr.length > 1) {
          return arr.map((item) => `"${item}"`).join(' or ')
        }
        if (arr.length === 1 && arr[0]) {
          raw = arr[0]
        }
      } catch {
        // ignore
      }
    }
    return `"${raw}"`
  }

  const getOperatorLabel = (op: string) => {
    switch (op) {
      case 'eq':
        return t('table_browser.op_eq')
      case 'neq':
        return t('table_browser.op_neq')
      case 'contains':
        return t('table_browser.op_contains')
      case 'starts_with':
        return t('table_browser.op_starts_with')
      case 'ends_with':
        return t('table_browser.op_ends_with')
      case 'gt':
        return t('table_browser.op_gt')
      case 'lt':
        return t('table_browser.op_lt')
      case 'gte':
        return t('table_browser.op_gte')
      case 'lte':
        return t('table_browser.op_lte')
      default:
        return op
    }
  }

  const operatorOptions = useMemo(
    () => [
      { value: 'contains', label: t('table_browser.op_contains') },
      { value: 'starts_with', label: t('table_browser.op_starts_with') },
      { value: 'ends_with', label: t('table_browser.op_ends_with') },
      { value: 'eq', label: t('table_browser.op_eq') },
      { value: 'neq', label: t('table_browser.op_neq') },
      { value: 'gt', label: t('table_browser.op_gt') },
      { value: 'lt', label: t('table_browser.op_lt') },
      { value: 'gte', label: t('table_browser.op_gte') },
      { value: 'lte', label: t('table_browser.op_lte') },
    ],
    [t],
  )

  const filterFieldOpts = useMemo(() => {
    return activeDimensions.map((d) => ({
      value: d.name,
      label: d.label || d.name,
    }))
  }, [activeDimensions])

  const pageSizeOptions = useMemo(
    () =>
      PAGE_SIZE_OPTIONS.map((n) => ({
        value: String(n),
        label: String(n),
      })),
    [],
  )

  const rangeLabel =
    rowCount === 0
      ? t('table_browser.range_empty')
      : totalRows != null
        ? t('table_browser.range_of_total', {
            start: formatInt(rangeStart),
            end: formatInt(rangeEnd),
            total: formatInt(totalRows),
          })
        : t('table_browser.range_unknown_total', {
            start: formatInt(rangeStart),
            end: formatInt(rangeEnd),
          })

  if (dsLoading || modelsLoading || (modelId ? modelLoading : false)) {
    return <LoadingScreen minHeight="300px" />
  }

  return (
    <div className="card card--table-browser">
      <div className="table-browser-toolbar">
        <div className="table-browser-toolbar-field">
          <label htmlFor="table-browser-datasource" className="table-browser-toolbar-label">
            {t('saved_questions.label_select_datasource')}
          </label>
          <Select
            id="table-browser-datasource"
            value={datasourceId}
            onChange={setDatasourceId}
            options={datasources.map((d) => ({ value: d.id, label: d.name, hint: d.type }))}
          />
        </div>
        <div className="table-browser-toolbar-field">
          <label htmlFor="table-browser-model" className="table-browser-toolbar-label">
            {t('saved_questions.label_select_model')}
          </label>
          <Select
            id="table-browser-model"
            value={modelId}
            onChange={setModelId}
            disabled={!datasourceId || models.length === 0}
            placeholder={t('query_builder.placeholder_pick_model')}
            options={models.map((m) => ({
              value: m.id,
              label: modelListLabel(m),
              hint: modelListHint(m),
            }))}
          />
        </div>
        {modelDetail && tableOptions.length > 0 && (
          <div className="table-browser-toolbar-field">
            <label htmlFor="table-browser-table" className="table-browser-toolbar-label">
              {t('table_browser.label_select_table')}
            </label>
            <Select
              id="table-browser-table"
              value={selectedTableKey}
              onChange={setSelectedTableKey}
              options={tableOptions}
            />
          </div>
        )}
      </div>

      {modelDetail ? (
        activeDimensions.length === 0 ? (
          <p style={{ color: 'var(--text-muted)', fontSize: '0.85rem' }}>
            {t('table_browser.no_columns_for_table')}
          </p>
        ) : (
          <>
            <div className="table-browser-filter-bar">
              {filters.map((f) => (
                <span
                  key={f.id}
                  className="table-browser-filter-tag"
                  style={{ cursor: 'pointer' }}
                  onClick={() => handleOpenEditFilter(f)}
                >
                  {getDimensionLabel(f.field)} {getOperatorLabel(f.operator)}{' '}
                  {formatFilterValue(f.value)}
                  <button
                    type="button"
                    className="table-browser-filter-tag-close"
                    onClick={(e) => {
                      e.stopPropagation()
                      handleRemoveFilter(f.id)
                    }}
                    aria-label={t('table_browser.remove_filter')}
                  >
                    ×
                  </button>
                </span>
              ))}
              <button
                type="button"
                className="table-browser-add-filter-btn"
                onClick={() => handleOpenAddFilter()}
                title={t('table_browser.add_filter')}
              >
                <svg
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2.5"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  style={{ width: '0.85rem', height: '0.85rem' }}
                >
                  <line x1="12" y1="5" x2="12" y2="19" />
                  <line x1="5" y1="12" x2="19" y2="12" />
                </svg>
                {t('table_browser.filter')}
              </button>

              {popoverOpen && (
                <div className="filter-popover" style={{ width: '18rem' }}>
                  <div
                    className="filter-popover-header"
                    style={{
                      display: 'flex',
                      justifyContent: 'space-between',
                      alignItems: 'center',
                      borderBottom: 'none',
                      paddingBottom: '0.2rem',
                    }}
                  >
                    <div style={{ display: 'flex', alignItems: 'center', gap: '0.4rem' }}>
                      <button
                        type="button"
                        className="filter-popover-back"
                        onClick={() => setPopoverOpen(false)}
                      >
                        ‹
                      </button>
                      <span style={{ fontSize: '0.86rem', fontWeight: 700 }}>
                        {getDimensionLabel(popoverField)}
                      </span>
                    </div>
                    <Select
                      value={popoverOperator}
                      onChange={setPopoverOperator}
                      options={operatorOptions}
                      size="sm"
                    />
                  </div>

                  {!editingFilterId && (
                    <div className="filter-popover-row" style={{ marginTop: '0.1rem' }}>
                      <label>{t('table_browser.column')}</label>
                      <Select
                        value={popoverField}
                        onChange={setPopoverField}
                        options={filterFieldOpts}
                        size="sm"
                      />
                    </div>
                  )}

                  <div className="filter-popover-row">
                    <label>{t('table_browser.value')}</label>
                    <div
                      className="chip-input-container"
                      onClick={() => document.getElementById('chip-input-el')?.focus()}
                    >
                      {popoverChips.map((chip, idx) => (
                        <span key={idx} className="chip-tag">
                          {chip}
                          <button
                            type="button"
                            className="chip-tag-close"
                            onClick={(e) => {
                              e.stopPropagation()
                              handleRemoveChip(idx)
                            }}
                          >
                            ×
                          </button>
                        </span>
                      ))}
                      <input
                        id="chip-input-el"
                        type="text"
                        value={chipInputText}
                        onChange={(e) => setChipInputText(e.target.value)}
                        onKeyDown={(e) => {
                          if (e.key === 'Enter' || e.key === ',') {
                            e.preventDefault()
                            handleAddChip(chipInputText)
                          } else if (
                            e.key === 'Backspace' &&
                            !chipInputText &&
                            popoverChips.length > 0
                          ) {
                            handleRemoveChip(popoverChips.length - 1)
                          }
                        }}
                        placeholder={
                          popoverChips.length === 0 ? t('table_browser.enter_value') : ''
                        }
                        className="chip-input-field"
                      />
                    </div>
                  </div>

                  <div
                    style={{
                      display: 'flex',
                      justifyContent: 'space-between',
                      alignItems: 'center',
                      marginTop: '0.2rem',
                    }}
                  >
                    <div className="filter-popover-checkbox-row">
                      <input
                        type="checkbox"
                        id="case-sensitive-cb"
                        checked={popoverCaseSensitive}
                        onChange={(e) => setPopoverCaseSensitive(e.target.checked)}
                      />
                      <label htmlFor="case-sensitive-cb">{t('table_browser.case_sensitive')}</label>
                    </div>
                    <button
                      type="button"
                      className="filter-popover-btn"
                      style={{ width: 'auto', padding: '0.35rem 0.85rem' }}
                      onClick={handleSaveFilter}
                    >
                      {editingFilterId
                        ? t('table_browser.update_filter')
                        : t('table_browser.add_filter')}
                    </button>
                  </div>
                </div>
              )}
            </div>

            <ValidationErrorBanner
              error={error}
              t={t}
              onOpenModeling={() => {
                const modelId = modelDetail?.id
                const dsId = datasourceId
                const params = new URLSearchParams()
                if (dsId) {
                  params.set('ds', dsId)
                }
                if (modelId) {
                  params.set('model', modelId)
                }
                const qs = params.toString()
                navigate(qs ? `/modeling?${qs}` : '/modeling')
              }}
            />

            {showTablePanel && (
              <>
                {showInitialPlaceholder ? (
                  <div
                    className="table-browser-table-placeholder"
                    role="status"
                    aria-live="polite"
                    aria-busy="true"
                  >
                    <span className="loading-overlay-spinner" aria-hidden="true" />
                    <span>{t('table_browser.loading')}</span>
                  </div>
                ) : result?.columns ? (
                  <LoadingOverlay
                    loading={fetching}
                    label={t('table_browser.loading_page')}
                    className="table-browser-table-overlay"
                  >
                    <div className={`table-browser-table-wrap${fetching ? ' is-blurred' : ''}`}>
                      <table className="results-table table-browser-grid">
                        <thead>
                          <tr>
                            <th scope="col" className="table-browser-col-index"></th>
                            {displayColumnNames.map((colName) => (
                              <th
                                key={colName}
                                scope="col"
                                draggable={!fetching}
                                className={`table-browser-th th-clickable${dragColumn === colName ? ' is-dragging' : ''}${dropTargetColumn === colName ? ' is-drop-target' : ''}`}
                                onDragStart={handleColumnDragStart(colName)}
                                onDragOver={handleColumnDragOver(colName)}
                                onDrop={handleColumnDrop(colName)}
                                onDragEnd={handleColumnDragEnd}
                                onClick={() => !fetching && handleOpenAddFilter(colName)}
                                title={t('table_browser.filter_by_column', { column: colName })}
                              >
                                <span className="table-browser-th-inner">
                                  <span
                                    className="table-browser-th-grip"
                                    aria-hidden="true"
                                    title={t('table_browser.drag_column')}
                                    onClick={(e) => e.stopPropagation()}
                                  >
                                    ⋮⋮
                                  </span>
                                  <span className="table-browser-th-label">
                                    {getDimensionLabel(colName)}
                                  </span>
                                  <span className="th-chevron">▼</span>
                                </span>
                              </th>
                            ))}
                          </tr>
                        </thead>
                        <tbody>
                          {(result.rows ?? []).map((row, i) => (
                            <tr
                              key={i}
                              className={`table-browser-data-row${fetching ? ' is-disabled' : ''}`}
                              onClick={() => {
                                if (fetching) {
                                  return
                                }
                                setDetailRow({
                                  displayIndex: page * pageSize + i + 1,
                                  row,
                                })
                              }}
                            >
                              <td className="table-browser-col-index">
                                <span className="row-index-number">{page * pageSize + i + 1}</span>
                              </td>
                              {displayColumnNames.map((colName) => {
                                const j = columnIndexByName.get(colName)
                                const cell = j != null ? row[j] : null
                                const display = formatResultCell(cell, colName, {})
                                return (
                                  <td key={colName} title={display}>
                                    {display}
                                  </td>
                                )
                              })}
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </div>
                  </LoadingOverlay>
                ) : null}

                {hasTableData && (
                  <div className={`table-browser-pagination${fetching ? ' is-loading' : ''}`}>
                    <span className="table-browser-range">{rangeLabel}</span>
                    <div className="table-browser-pagination-controls">
                      <div className="table-browser-page-size">
                        <span className="table-browser-page-size-label">
                          {t('table_browser.rows_per_page')}
                        </span>
                        <Select
                          value={String(pageSize)}
                          onChange={(v) => {
                            setPageSize(Number(v))
                            goToPage(0)
                          }}
                          options={pageSizeOptions}
                          size="sm"
                        />
                      </div>
                      <nav
                        className="table-browser-page-nav"
                        aria-label={t('table_browser.pagination_nav')}
                      >
                        <button
                          type="button"
                          className="table-browser-page-btn table-browser-page-btn--icon"
                          disabled={page === 0 || fetching}
                          onClick={() => goToPage(0)}
                          title={t('table_browser.first_page')}
                          aria-label={t('table_browser.first_page')}
                        >
                          «
                        </button>
                        <button
                          type="button"
                          className="table-browser-page-btn table-browser-page-btn--icon"
                          disabled={page === 0 || fetching}
                          onClick={() => goToPage(page - 1)}
                          title={t('table_browser.prev_page')}
                          aria-label={t('table_browser.prev_page')}
                        >
                          ‹
                        </button>
                        {pageList ? (
                          <div className="table-browser-page-list" role="list">
                            {pageList.map((token, idx) =>
                              token === 'gap' ? (
                                <span
                                  key={`gap-${idx}`}
                                  className="table-browser-page-gap"
                                  aria-hidden="true"
                                >
                                  …
                                </span>
                              ) : (
                                <button
                                  key={token}
                                  type="button"
                                  role="listitem"
                                  className={`table-browser-page-num-btn${token === page ? ' is-active' : ''}`}
                                  disabled={fetching || token === page}
                                  onClick={() => goToPage(token)}
                                  aria-label={t('table_browser.go_to_page', { page: token + 1 })}
                                  aria-current={token === page ? 'page' : undefined}
                                >
                                  {formatInt(token + 1)}
                                </button>
                              ),
                            )}
                          </div>
                        ) : (
                          <span className="table-browser-page-num">
                            {t('table_browser.page_number', { page: formatInt(page + 1) })}
                          </span>
                        )}
                        <button
                          type="button"
                          className="table-browser-page-btn table-browser-page-btn--icon"
                          disabled={!hasNext || fetching}
                          onClick={() => goToPage(page + 1)}
                          title={t('table_browser.next_page')}
                          aria-label={t('table_browser.next_page')}
                        >
                          ›
                        </button>
                        <button
                          type="button"
                          className="table-browser-page-btn table-browser-page-btn--icon"
                          disabled={lastPageIndex == null || page >= lastPageIndex || fetching}
                          onClick={() => lastPageIndex != null && goToPage(lastPageIndex)}
                          title={t('table_browser.last_page')}
                          aria-label={t('table_browser.last_page')}
                        >
                          »
                        </button>
                      </nav>
                    </div>
                  </div>
                )}
              </>
            )}
          </>
        )
      ) : (
        <p style={{ color: 'var(--text-muted)', fontSize: '0.85rem' }}>
          {t('table_browser.select_model')}
        </p>
      )}

      <Modal
        open={detailRow != null && result?.columns != null}
        title={
          detailRow && result?.columns
            ? buildRowModalTitle(
                detailRow.row,
                result.columns.map((c) => c.name),
                t('table_browser.row_detail_title', { n: formatInt(detailRow.displayIndex) }),
                selectedTableKey ?? modelDetail?.base_table,
              )
            : t('table_browser.row_detail')
        }
        subtitle={selectedTableKey || modelDetail?.base_table}
        onClose={() => setDetailRow(null)}
        bodyClassName="table-browser-detail-modal-body"
      >
        {detailRow && result?.columns && (
          <div
            className="table-browser-detail-grid"
            role="region"
            aria-label={t('table_browser.row_detail')}
          >
            {displayColumnNames.map((colName) => {
              const j = columnIndexByName.get(colName)
              const display = formatResultCell(j != null ? detailRow.row[j] : null, colName, {})
              return (
                <div key={colName} className="table-browser-detail-item">
                  <span className="table-browser-detail-label">{getDimensionLabel(colName)}</span>
                  <span className="table-browser-detail-value">{display}</span>
                </div>
              )
            })}
          </div>
        )}
      </Modal>
    </div>
  )
}
