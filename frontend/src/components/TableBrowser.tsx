import { Fragment, useCallback, useEffect, useMemo, useState } from 'react'
import '../styles/tableBrowser.css'
import { useApi } from '../hooks/useApi'
import { useT } from '../i18n'
import { formatResultCell } from '../utils/resultCellFormat'
import { buildQueryPayload } from './queryBuilder/logicalQuery'
import { ErrorAlert } from './ui/ErrorAlert'
import { Select } from './ui/Select'
import type { Datasource } from '../types/metadata'
import type { SemanticMetric, SemanticModelDetail, SemanticModelSummary } from '../types/semantic'
import { modelListLabel, modelListHint } from '../types/semantic'

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
  if (!metrics?.length) return null
  const byName = metrics.find((m) => m.name === 'row_count' || m.name === 'count')
  if (byName) return byName.name
  const countAgg = metrics.find((m) => m.aggregation === 'count')
  return countAgg?.name ?? null
}

function parseCountValue(rows: unknown[][] | undefined): number | null {
  if (!rows?.length) return null
  const cell = rows[0]?.[0]
  const n = typeof cell === 'number' ? cell : Number(cell)
  return Number.isFinite(n) ? Math.trunc(n) : null
}

export default function TableBrowser() {
  const t = useT()
  const { get, postData, error } = useApi()

  const [datasources, setDatasources] = useState<Datasource[]>([])
  const [datasourceId, setDatasourceId] = useState('')
  const [models, setModels] = useState<SemanticModelSummary[]>([])
  const [modelId, setModelId] = useState('')
  const [modelDetail, setModelDetail] = useState<SemanticModelDetail | null>(null)

  const [filters, setFilters] = useState<TableBrowserFilter[]>([])
  const [result, setResult] = useState<QueryBuilderResult | null>(null)
  const [totalRows, setTotalRows] = useState<number | null>(null)
  const [page, setPage] = useState(0)
  const [pageSize, setPageSize] = useState<number>(DEFAULT_PAGE_SIZE)
  const [fetching, setFetching] = useState(false)
  const [expandedRow, setExpandedRow] = useState<number | null>(null)

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

  useEffect(() => {
    get<Datasource[]>('/api/datasources').then((data) => {
      if (data && data.length > 0) {
        setDatasources(data)
        const first = data[0]
        if (first) setDatasourceId(first.id)
      }
    })
  }, [])

  useEffect(() => {
    if (!datasourceId) {
      setModels([])
      setModelId('')
      setModelDetail(null)
      setResult(null)
      return
    }
    get<SemanticModelSummary[]>(`/api/semantic/models?datasource_id=${encodeURIComponent(datasourceId)}`).then((data) => {
      if (data) {
        setModels(data)
        const published = data.filter((m) => m.status === 'published')
        const firstPub = published[0]
        const firstData = data[0]
        if (firstPub) {
          setModelId(firstPub.id)
        } else if (firstData) {
          setModelId(firstData.id)
        } else {
          setModelId('')
          setModelDetail(null)
          setResult(null)
        }
      }
    })
  }, [datasourceId])

  useEffect(() => {
    if (!modelId) {
      setModelDetail(null)
      setResult(null)
      return
    }
    get<SemanticModelDetail>(`/api/semantic/models/${encodeURIComponent(modelId)}`).then((d) => {
      if (d) {
        setModelDetail(d)
        setFilters([])
        setPage(0)
        setExpandedRow(null)
      }
    })
  }, [modelId])

  useEffect(() => {
    setPage(0)
    setExpandedRow(null)
  }, [filters])

  const dimensions = useMemo(() => modelDetail?.dimensions ?? [], [modelDetail])

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

  const runBrowseQuery = useCallback(async () => {
    if (!datasourceId || !modelId || !modelDetail || dimensions.length === 0) return

    setFetching(true)
    const offset = page * pageSize

    const selectItems = dimensions.map((d) => ({
      id: d.id,
      type: 'dimension' as const,
      name: d.name,
    }))

    const baseState = {
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
    }

    const countPromise =
      countMetricName != null
        ? postData<QueryBuilderResult>(
            '/api/query/run',
            buildQueryPayload({
              ...baseState,
              selectItems: [{ id: 'count', type: 'metric', name: countMetricName }],
              limit: 1,
              offset: 0,
            }),
          )
        : Promise.resolve(null)

    const dataPromise = postData<QueryBuilderResult>(
      '/api/query/run',
      buildQueryPayload({
        ...baseState,
        selectItems,
        limit: pageSize,
        offset,
      }),
    )

    const [countRes, dataRes] = await Promise.all([countPromise, dataPromise])
    if (countRes) {
      setTotalRows(parseCountValue(countRes.rows))
    } else {
      setTotalRows(null)
    }
    if (dataRes) {
      setResult(dataRes)
    }
    setFetching(false)
  }, [
    datasourceId,
    modelId,
    modelDetail,
    dimensions,
    filterPayload,
    page,
    pageSize,
    countMetricName,
    postData,
  ])

  useEffect(() => {
    void runBrowseQuery()
  }, [runBrowseQuery])

  const rowCount = result?.rows?.length ?? 0
  const rangeStart = rowCount > 0 ? page * pageSize + 1 : 0
  const rangeEnd = page * pageSize + rowCount
  const hasNext =
    totalRows != null
      ? rangeEnd < totalRows
      : rowCount === pageSize

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
    if (!popoverField) return

    let finalChips = [...popoverChips]
    const textVal = chipInputText.trim()
    if (textVal && !finalChips.includes(textVal)) {
      finalChips.push(textVal)
    }

    if (finalChips.length === 0) return

    const finalValue = finalChips.length > 1 ? JSON.stringify(finalChips) : (finalChips[0] || '')

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

  const handleOpenAddFilter = (defaultField = '') => {
    setEditingFilterId(null)
    setPopoverField(defaultField || (dimensions[0]?.name ?? ''))
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
    const dim = dimensions.find((d) => d.name === name)
    return dim ? (dim.label || dim.name) : name
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
    return dimensions.map((d) => ({
      value: d.name,
      label: d.label || d.name,
    }))
  }, [dimensions])

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
        ? t('table_browser.range_of_total', { start: rangeStart, end: rangeEnd, total: totalRows })
        : t('table_browser.range_unknown_total', { start: rangeStart, end: rangeEnd })

  return (
    <div className="card card--table-browser">
      <div className="table-browser-header">
        <h3 style={{ margin: 0 }}>{t('table_browser.title')}</h3>
        <div className="table-browser-selectors">
          <Select
            value={datasourceId}
            onChange={setDatasourceId}
            options={datasources.map((d) => ({ value: d.id, label: d.name, hint: d.type }))}
            size="sm"
          />
          {datasourceId && models.length > 0 && (
            <>
              <span className="separator">/</span>
              <Select
                value={modelId}
                onChange={setModelId}
                options={models.map((m) => ({
                  value: m.id,
                  label: modelListLabel(m),
                  hint: modelListHint(m),
                }))}
                size="sm"
              />
            </>
          )}
        </div>
      </div>

      {modelDetail ? (
        <>
          <div className="table-browser-title-row">
            <span className="table-browser-title">
              {modelDetail.base_table}
              <span
                className="table-browser-info-icon"
                title={modelDetail.description || t('table_browser.no_description')}
              >
                i
              </span>
            </span>
          </div>

          <div className="table-browser-filter-bar">
            {filters.map((f) => (
              <span
                key={f.id}
                className="table-browser-filter-tag"
                style={{ cursor: 'pointer' }}
                onClick={() => handleOpenEditFilter(f)}
              >
                {getDimensionLabel(f.field)} {getOperatorLabel(f.operator)} {formatFilterValue(f.value)}
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
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" style={{ width: '0.85rem', height: '0.85rem' }}>
                <line x1="12" y1="5" x2="12" y2="19" />
                <line x1="5" y1="12" x2="19" y2="12" />
              </svg>
              {t('table_browser.filter')}
            </button>

            {popoverOpen && (
              <div className="filter-popover" style={{ width: '18rem' }}>
                <div className="filter-popover-header" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', borderBottom: 'none', paddingBottom: '0.2rem' }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '0.4rem' }}>
                    <button type="button" className="filter-popover-back" onClick={() => setPopoverOpen(false)}>‹</button>
                    <span style={{ fontSize: '0.86rem', fontWeight: 700 }}>{getDimensionLabel(popoverField)}</span>
                  </div>
                  <Select value={popoverOperator} onChange={setPopoverOperator} options={operatorOptions} size="sm" />
                </div>

                {!editingFilterId && (
                  <div className="filter-popover-row" style={{ marginTop: '0.1rem' }}>
                    <label>{t('table_browser.column')}</label>
                    <Select value={popoverField} onChange={setPopoverField} options={filterFieldOpts} size="sm" />
                  </div>
                )}

                <div className="filter-popover-row">
                  <label>{t('table_browser.value')}</label>
                  <div className="chip-input-container" onClick={() => document.getElementById('chip-input-el')?.focus()}>
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
                        } else if (e.key === 'Backspace' && !chipInputText && popoverChips.length > 0) {
                          handleRemoveChip(popoverChips.length - 1)
                        }
                      }}
                      placeholder={popoverChips.length === 0 ? t('table_browser.enter_value') : ''}
                      className="chip-input-field"
                    />
                  </div>
                </div>

                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginTop: '0.2rem' }}>
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
                    {editingFilterId ? t('table_browser.update_filter') : t('table_browser.add_filter')}
                  </button>
                </div>
              </div>
            )}
          </div>

          <ErrorAlert error={error} />

          {fetching && <div className="table-browser-loading">{t('table_browser.loading')}</div>}

          {!fetching && result && result.columns && result.rows && (
            <>
              <div className="table-browser-table-wrap">
                <table className="results-table table-browser-grid">
                  <thead>
                    <tr>
                      <th scope="col" className="table-browser-col-index"></th>
                      {result.columns.map((col) => (
                        <th
                          key={col.name}
                          scope="col"
                          className="th-clickable"
                          onClick={() => handleOpenAddFilter(col.name)}
                          title={t('table_browser.filter_by_column', { column: col.name })}
                        >
                          {getDimensionLabel(col.name)}
                          <span className="th-chevron">▼</span>
                        </th>
                      ))}
                    </tr>
                  </thead>
                  <tbody>
                    {result.rows.map((row, i) => {
                      const isExpanded = expandedRow === i
                      return (
                        <Fragment key={i}>
                          <tr
                            className={`table-browser-data-row${isExpanded ? ' is-expanded' : ''}`}
                            onClick={() => setExpandedRow(isExpanded ? null : i)}
                            aria-expanded={isExpanded}
                          >
                            <td className="table-browser-col-index">
                              <span className="row-index-number">{page * pageSize + i + 1}</span>
                            </td>
                            {row.map((cell, j) => {
                              const colName = result.columns?.[j]?.name ?? ''
                              const display = formatResultCell(cell, colName, {})
                              return (
                                <td key={j} title={display}>
                                  {display}
                                </td>
                              )
                            })}
                          </tr>
                          {isExpanded && (
                            <tr key={`detail-${i}`} className="table-browser-detail-row">
                              <td colSpan={(result.columns?.length ?? 0) + 1}>
                                <div className="table-browser-detail-grid" role="region" aria-label={t('table_browser.row_detail')}>
                                  {result.columns?.map((col, j) => {
                                    const colName = col.name
                                    const display = formatResultCell(row[j], colName, {})
                                    return (
                                      <div key={colName} className="table-browser-detail-item">
                                        <span className="table-browser-detail-label">{getDimensionLabel(colName)}</span>
                                        <span className="table-browser-detail-value">{display}</span>
                                      </div>
                                    )
                                  })}
                                </div>
                              </td>
                            </tr>
                          )}
                        </Fragment>
                      )
                    })}
                  </tbody>
                </table>
              </div>

              <div className="table-browser-pagination">
                <span className="table-browser-range">{rangeLabel}</span>
                <div className="table-browser-pagination-controls">
                  <label className="table-browser-page-size-label">
                    {t('table_browser.rows_per_page')}
                    <Select
                      value={String(pageSize)}
                      onChange={(v) => {
                        setPageSize(Number(v))
                        setPage(0)
                        setExpandedRow(null)
                      }}
                      options={pageSizeOptions}
                      size="sm"
                    />
                  </label>
                  <button
                    type="button"
                    className="table-browser-page-btn"
                    disabled={page === 0 || fetching}
                    onClick={() => {
                      setPage((p) => Math.max(0, p - 1))
                      setExpandedRow(null)
                    }}
                  >
                    {t('table_browser.prev_page')}
                  </button>
                  <span className="table-browser-page-num">
                    {t('table_browser.page_number', { page: page + 1 })}
                  </span>
                  <button
                    type="button"
                    className="table-browser-page-btn"
                    disabled={!hasNext || fetching}
                    onClick={() => {
                      setPage((p) => p + 1)
                      setExpandedRow(null)
                    }}
                  >
                    {t('table_browser.next_page')}
                  </button>
                </div>
              </div>
            </>
          )}
        </>
      ) : (
        <p style={{ color: 'var(--text-muted)', fontSize: '0.85rem' }}>{t('table_browser.select_model')}</p>
      )}
    </div>
  )
}
