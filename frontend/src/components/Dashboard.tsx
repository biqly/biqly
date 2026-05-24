import { useEffect, useState, useMemo, useCallback } from 'react'
import { Bar, BarChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'
import { useApi } from '../hooks/useApi'
import { useT } from '../i18n'
import { chartAxisStroke, chartGridStroke, chartTooltipStyle } from '../utils/chartConfig'
import { getRateColor } from '../utils/formatters'
import { KPICard } from './ui/KPICard'
import { ChartContainer } from './ui/ChartContainer'
import { Select } from './ui/Select'
import { ErrorAlert } from './ui/ErrorAlert'
import { formatResultCell } from '../utils/resultCellFormat'
import { buildQueryPayload } from './queryBuilder/logicalQuery'
import type { ModelStats } from '../types/ai'
import type { Datasource } from '../types/metadata'
import type { SemanticModelDetail, SemanticModelSummary } from '../types/semantic'
import { modelListLabel, modelListHint } from '../types/semantic'

interface AIUsageSummary {
  total_queries: number
  success_rate: number
  failure_rate: number
  avg_retry_count: number
  avg_latency_ms: number
  total_cost: number
  positive_feedback?: number
  negative_feedback?: number
}

interface DayUsage {
  date: string
  total_queries: number
  failure_rate: number
  avg_retry_count: number
  avg_latency_ms: number
  total_cost: number
  total_tokens: number
}

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

export default function Dashboard() {
  const [activeTab, setActiveTab] = useState<'table' | 'analytics'>('table')

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '1.5rem' }}>
      <div className="analytics-tabs-container">
        <button
          type="button"
          className={`analytics-tab-btn ${activeTab === 'table' ? 'active' : ''}`}
          onClick={() => setActiveTab('table')}
        >
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="tab-icon">
            <rect x="3" y="3" width="18" height="18" rx="2" />
            <path d="M3 9h18" />
            <path d="M9 21V9" />
          </svg>
          Table Browser
        </button>
        <button
          type="button"
          className={`analytics-tab-btn ${activeTab === 'analytics' ? 'active' : ''}`}
          onClick={() => setActiveTab('analytics')}
        >
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="tab-icon">
            <line x1="18" y1="20" x2="18" y2="10" />
            <line x1="12" y1="20" x2="12" y2="4" />
            <line x1="6" y1="20" x2="6" y2="14" />
          </svg>
          AI Analytics
        </button>
      </div>

      <div className="analytics-tab-content">
        {activeTab === 'table' ? (
          <TableBrowserSection />
        ) : (
          <div style={{ display: 'flex', flexDirection: 'column', gap: '2rem' }}>
            <AIUsageSection />
            <ModelSuccessRates />
          </div>
        )}
      </div>
    </div>
  )
}

function TableBrowserSection() {
  const t = useT()
  const { get, postData, loading, error } = useApi()

  const [datasources, setDatasources] = useState<Datasource[]>([])
  const [datasourceId, setDatasourceId] = useState('')
  const [models, setModels] = useState<SemanticModelSummary[]>([])
  const [modelId, setModelId] = useState('')
  const [modelDetail, setModelDetail] = useState<SemanticModelDetail | null>(null)
  
  const [filters, setFilters] = useState<TableBrowserFilter[]>([])
  const [result, setResult] = useState<QueryBuilderResult | null>(null)

  // Filter Popover Overlay state
  const [popoverOpen, setPopoverOpen] = useState(false)
  const [popoverField, setPopoverField] = useState('')
  const [popoverOperator, setPopoverOperator] = useState('contains')
  const [popoverChips, setPopoverChips] = useState<string[]>([])
  const [chipInputText, setChipInputText] = useState('')
  const [popoverCaseSensitive, setPopoverCaseSensitive] = useState(false)
  const [editingFilterId, setEditingFilterId] = useState<string | null>(null)

  // Fetch datasources
  useEffect(() => {
    get<Datasource[]>('/api/datasources').then((data) => {
      if (data && data.length > 0) {
        setDatasources(data)
        const first = data[0]
        if (first) setDatasourceId(first.id)
      }
    })
  }, [])

  // Fetch models when datasource changes
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

  // Fetch model details when model changes
  useEffect(() => {
    if (!modelId) {
      setModelDetail(null)
      setResult(null)
      return
    }
    get<SemanticModelDetail>(`/api/semantic/models/${encodeURIComponent(modelId)}`).then((d) => {
      if (d) {
        setModelDetail(d)
        setFilters([]) // reset filters when model changes
      }
    })
  }, [modelId])

  const dimensions = useMemo(() => modelDetail?.dimensions ?? [], [modelDetail])

  // Query raw table data based on current model and filters
  const runBrowseQuery = useCallback(async () => {
    if (!datasourceId || !modelId || !modelDetail || dimensions.length === 0) return

    // select all dimensions to display raw columns
    const selectItems = dimensions.map((d) => ({
      id: d.id,
      type: 'dimension' as const,
      name: d.name,
    }))

    const payload = buildQueryPayload({
      datasourceId,
      modelId,
      mode: 'simple',
      selectItems,
      filters: filters.map((f) => ({
        id: f.id,
        field: f.field,
        operator: f.operator,
        value: f.value,
        caseSensitive: f.caseSensitive,
      })),
      groupBy: [],
      having: [],
      orderBy: '',
      orderDir: 'asc',
      limit: 100,
      offset: 0,
      windowFunctions: [],
      ctes: [],
    })

    const res = await postData<QueryBuilderResult>('/api/query/run', payload)
    if (res) {
      setResult(res)
    }
  }, [datasourceId, modelId, modelDetail, dimensions, filters])

  // Re-run query whenever model detail or filters change
  useEffect(() => {
    void runBrowseQuery()
  }, [modelDetail, filters, runBrowseQuery])

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

  // Handles adding/updating filters from popover
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
      } catch (e) {
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

  const formatFilterValue = (operator: string, value: string) => {
    let raw = value
    if (value.startsWith('[') && value.endsWith(']')) {
      try {
        const arr = JSON.parse(value) as string[]
        if (arr.length > 1) {
          return arr.map(item => `"${item}"`).join(' or ')
        } else if (arr.length === 1 && arr[0]) {
          raw = arr[0]
        }
      } catch (e) {
        // ignore
      }
    }
    return `"${raw}"`
  }

  const getOperatorLabel = (op: string) => {
    switch (op) {
      case 'eq': return 'is'
      case 'neq': return 'is not'
      case 'contains': return 'contains'
      case 'starts_with': return 'starts with'
      case 'ends_with': return 'ends with'
      case 'gt': return '>'
      case 'lt': return '<'
      case 'gte': return '>='
      case 'lte': return '<='
      default: return op
    }
  }

  const filterFieldOpts = useMemo(() => {
    return dimensions.map((d) => ({
      value: d.name,
      label: d.label || d.name,
    }))
  }, [dimensions])

  return (
    <div className="card card--table-browser">
      <div className="table-browser-header">
        <h3 style={{ margin: 0 }}>Table Browser</h3>
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
              <span className="table-browser-info-icon" title={modelDetail.description || 'No description'}>i</span>
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
                {getDimensionLabel(f.field)} {getOperatorLabel(f.operator)} {formatFilterValue(f.operator, f.value)}
                <button
                  type="button"
                  className="table-browser-filter-tag-close"
                  onClick={(e) => {
                    e.stopPropagation()
                    handleRemoveFilter(f.id)
                  }}
                  aria-label="Remove Filter"
                >
                  ×
                </button>
              </span>
            ))}
            <button
              type="button"
              className="table-browser-add-filter-btn"
              onClick={() => handleOpenAddFilter()}
              title="Add Filter"
            >
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" style={{ width: '0.85rem', height: '0.85rem' }}>
                <line x1="12" y1="5" x2="12" y2="19" />
                <line x1="5" y1="12" x2="19" y2="12" />
              </svg>
              Filter
            </button>

            {/* Filter Popover */}
            {popoverOpen && (
              <div className="filter-popover" style={{ width: '18rem' }}>
                <div className="filter-popover-header" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', borderBottom: 'none', paddingBottom: '0.2rem' }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '0.4rem' }}>
                    <button type="button" className="filter-popover-back" onClick={() => setPopoverOpen(false)}>‹</button>
                    <span style={{ fontSize: '0.86rem', fontWeight: 700 }}>{getDimensionLabel(popoverField)}</span>
                  </div>
                  <Select
                    value={popoverOperator}
                    onChange={setPopoverOperator}
                    options={[
                      { value: 'contains', label: 'contains' },
                      { value: 'starts_with', label: 'starts with' },
                      { value: 'ends_with', label: 'ends with' },
                      { value: 'eq', label: 'is' },
                      { value: 'neq', label: 'is not' },
                      { value: 'gt', label: '>' },
                      { value: 'lt', label: '<' },
                      { value: 'gte', label: '>=' },
                      { value: 'lte', label: '<=' },
                    ]}
                    size="sm"
                  />
                </div>

                {!editingFilterId && (
                  <div className="filter-popover-row" style={{ marginTop: '0.1rem' }}>
                    <label>Column</label>
                    <Select
                      value={popoverField}
                      onChange={setPopoverField}
                      options={filterFieldOpts}
                      size="sm"
                    />
                  </div>
                )}

                <div className="filter-popover-row">
                  <label>Value</label>
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
                      placeholder={popoverChips.length === 0 ? "Enter value..." : ""}
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
                    <label htmlFor="case-sensitive-cb">Case sensitive</label>
                  </div>
                  <button
                    type="button"
                    className="filter-popover-btn"
                    style={{ width: 'auto', padding: '0.35rem 0.85rem' }}
                    onClick={handleSaveFilter}
                  >
                    {editingFilterId ? 'Update filter' : 'Add filter'}
                  </button>
                </div>
              </div>
            )}
          </div>

          <ErrorAlert error={error} />

          {loading && <div style={{ color: 'var(--text-muted)', fontSize: '0.85rem' }}>Loading data...</div>}

          {!loading && result && result.columns && result.rows && (
            <div style={{ overflowX: 'auto', maxHeight: '400px', overflowY: 'auto' }}>
              <table className="results-table">
                <thead>
                  <tr>
                    <th scope="col" style={{ width: '3rem' }}></th>
                    {result.columns.map((col) => (
                      <th
                        key={col.name}
                        scope="col"
                        className="th-clickable"
                        onClick={() => handleOpenAddFilter(col.name)}
                        title={`Click to filter by ${col.name}`}
                      >
                        {getDimensionLabel(col.name)}
                        <span className="th-chevron">▼</span>
                      </th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {result.rows.map((row, i) => (
                    <tr key={i}>
                      <td>
                        <span className="row-index-number">{i + 1}</span>
                      </td>
                      {row.map((cell, j) => (
                        <td key={j}>{formatResultCell(cell, result.columns?.[j]?.name ?? '', {})}</td>
                      ))}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </>
      ) : (
        <p style={{ color: 'var(--text-muted)', fontSize: '0.85rem' }}>Select a semantic model to browse tables.</p>
      )}
    </div>
  )
}

function AIUsageSection() {
  const t = useT()
  const { get } = useApi()
  const [summary, setSummary] = useState<AIUsageSummary | null>(null)
  const [daily, setDaily] = useState<DayUsage[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    get<{ summary: AIUsageSummary; daily: DayUsage[] }>('/api/ai/usage').then((data) => {
      if (data) {
        setSummary(data.summary)
        setDaily(data.daily.slice(0, 10).reverse())
      }
      setLoading(false)
    })
  }, [])

  if (loading) return null
  if (!summary) return null

  const trendData = daily.map((d) => ({
    name: d.date.slice(5),
    queries: d.total_queries,
    cost: parseFloat(d.total_cost.toFixed(3)),
  }))

  return (
    <div>
      <h2 style={{ marginBottom: '1rem' }}>{t('dashboard.ai_usage_last_30')}</h2>

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))', gap: '1rem', marginBottom: '1.5rem' }}>
        <KPICard label={t('dashboard.kpi_total_ai_queries')} value={summary.total_queries} color="var(--accent)" />
        <KPICard label={t('dashboard.kpi_success_rate')} value={`${(summary.success_rate * 100).toFixed(0)}%`} color={getRateColor(summary.success_rate * 100)} />
        <KPICard label={t('dashboard.kpi_failure_rate')} value={`${(summary.failure_rate * 100).toFixed(0)}%`} color={getRateColor(100 - summary.failure_rate * 100)} />
        <KPICard label={t('dashboard.kpi_avg_retry')} value={summary.avg_retry_count.toFixed(2)} color="var(--text-muted)" />
        <KPICard label={t('dashboard.kpi_avg_latency')} value={t('evaluation.latency_ms', { ms: Math.round(summary.avg_latency_ms) })} color="var(--warning)" />
        <KPICard label={t('dashboard.kpi_total_cost')} value={`$${summary.total_cost.toFixed(4)}`} color="var(--success)" />
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(400px, 1fr))', gap: '1.5rem' }}>
        <div className="card">
          <h3>{t('dashboard.daily_queries')}</h3>
          {trendData.length > 0 ? (
            <ChartContainer data={trendData} type="line" height={250} dataKey="queries" />
          ) : (
            <p style={{ color: 'var(--text-muted)', textAlign: 'center', paddingTop: '4rem' }}>{t('dashboard.no_ai_queries')}</p>
          )}
        </div>

        <div className="card">
          <h3>{t('dashboard.daily_cost')}</h3>
          {trendData.length > 0 ? (
            <ChartContainer data={trendData} type="bar" height={250} dataKey="cost" fill="#f59e0b" barRadius={[4, 4, 0, 0]} />
          ) : (
            <p style={{ color: 'var(--text-muted)', textAlign: 'center', paddingTop: '4rem' }}>{t('dashboard.no_cost_data')}</p>
          )}
        </div>
      </div>
    </div>
  )
}

function ModelSuccessRates() {
  const t = useT()
  const { get } = useApi()
  const [models, setModels] = useState<ModelStats[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    get<ModelStats[]>('/api/ai/stats/models').then((data) => {
      if (data) setModels(data)
      setLoading(false)
    })
  }, [])

  if (loading) return null
  if (models.length === 0) return null

  return (
    <div>
      <h2 style={{ marginBottom: '1rem' }}>{t('dashboard.model_rates_heading')}</h2>
      <table className="results-table">
        <thead>
          <tr>
            <th>{t('dashboard.col_model')}</th>
            <th style={{ textAlign: 'right' }}>{t('dashboard.tbl_total')}</th>
            <th style={{ textAlign: 'right' }}>{t('dashboard.tbl_success')}</th>
            <th style={{ textAlign: 'right' }}>{t('dashboard.tbl_fail')}</th>
            <th style={{ textAlign: 'right' }}>{t('dashboard.tbl_success_pct')}</th>
            <th style={{ textAlign: 'right' }}>{t('dashboard.tbl_confidence')}</th>
            <th style={{ textAlign: 'right' }}>{t('dashboard.tbl_latency')}</th>
            <th style={{ textAlign: 'right' }}>👍</th>
            <th style={{ textAlign: 'right' }}>👎</th>
          </tr>
        </thead>
        <tbody>
          {models.map((m) => (
            <tr key={m.model_id}>
              <td>{m.model_name || m.model_id}</td>
              <td style={{ textAlign: 'right' }}>{m.total_queries}</td>
              <td style={{ textAlign: 'right', color: 'var(--success)' }}>{m.success_count}</td>
              <td style={{ textAlign: 'right', color: 'var(--error)' }}>{m.failure_count}</td>
              <td style={{ textAlign: 'right' }}>
                <span style={{
                  color: getRateColor(m.success_rate),
                  fontWeight: 700,
                }}>
                  {m.success_rate.toFixed(1)}%
                </span>
              </td>
              <td style={{ textAlign: 'right' }}>{(m.avg_confidence * 100).toFixed(0)}%</td>
              <td style={{ textAlign: 'right' }}>{t('evaluation.latency_ms', { ms: Math.round(m.avg_latency_ms) })}</td>
              <td style={{ textAlign: 'right', color: 'var(--success)' }}>{m.positive_count}</td>
              <td style={{ textAlign: 'right', color: 'var(--error)' }}>{m.negative_count}</td>
            </tr>
          ))}
        </tbody>
      </table>

      <div className="card" style={{ marginTop: '1rem' }}>
        <h3>{t('dashboard.chart_success_compare')}</h3>
        <div style={{ height: 250 }}>
          <ResponsiveContainer width="100%" height="100%">
            <BarChart data={models.map((m) => ({
              name: m.model_name || m.model_id,
              success_rate: m.success_rate,
              confidence: m.avg_confidence * 100,
            }))}>
              <CartesianGrid strokeDasharray="3 3" stroke={chartGridStroke} />
              <XAxis dataKey="name" stroke={chartAxisStroke} tick={{ fontSize: 11 }} />
              <YAxis stroke={chartAxisStroke} domain={[0, 100]} />
              <Tooltip contentStyle={chartTooltipStyle} />
              <Bar dataKey="success_rate" fill="#22c55e" radius={[4, 4, 0, 0]} name={t('dashboard.legend_success_pct')} />
              <Bar dataKey="confidence" fill="#3b82f6" radius={[4, 4, 0, 0]} name={t('dashboard.legend_confidence_pct')} />
            </BarChart>
          </ResponsiveContainer>
        </div>
      </div>
    </div>
  )
}
