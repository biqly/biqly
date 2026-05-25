import { useEffect, useMemo, useState } from 'react'
import '../styles/queryBuilder.css'
import { useT } from '../i18n'
import { useApi } from '../hooks/useApi'
import { useArrayState } from '../hooks/useArrayState'
import { useQueryParam } from '../hooks/useQueryParam'
import type { Datasource } from '../types/metadata'
import { formatResultCell } from '../utils/resultCellFormat'
import { rowsToChartData } from '../utils/chartData'
import { ChartContainer } from './ui/ChartContainer'
import { ChartTypeSelector } from './ui/ChartTypeSelector'
import { ErrorAlert } from './ui/ErrorAlert'
import { Select } from './ui/Select'
import { LockedState } from './ui/LockedState'
import type {
  GenerateSemanticModelResponse,
  SemanticModelDetail,
  SemanticModelSummary,
} from '../types/semantic'
import { modelListHint, modelListLabel } from '../types/semantic'
import {
  addFilterRow,
  addGroupByRow as appendGroupByRow,
  addHavingRow,
  patchFilterRow,
  patchHavingRow,
  removeFilterRow,
  removeGroupByRow as dropGroupByRow,
  removeHavingRow,
  updateGroupByRow as patchGroupByRow,
} from './queryBuilder/rowState'
import { buildQueryPayload } from './queryBuilder/logicalQuery'
import type { CTERow, FilterRow, HavingRow, SelectItem, WindowFuncRow } from './queryBuilder/types'
import { newRowId, WINDOW_FUNC_OPTIONS } from './queryBuilder/types'
import {
  aggregationDisplayName,
  dimOptionsForGroupRow,
  dimFieldOptions,
  filterFieldOptions,
  metricFieldOptions,
  orderByFieldOptions,
} from './queryBuilder/utils'

interface QueryBuilderResult {
  columns?: { name: string; type?: string }[]
  rows?: unknown[][]
  stats?: {
    row_count?: number
    duration_ms?: number
  }
}

interface QueryExplainResponse {
  compiled_sql?: string
}

export default function QueryBuilder() {
  const t = useT()
  const { get, postData, loading, error } = useApi()
  const [datasources, setDatasources] = useState<Datasource[]>([])
  const [loadedDatasources, setLoadedDatasources] = useState(false)
  const [dsParam, setDsParam] = useQueryParam('ds')
  const [datasourceId, setDatasourceId] = useState(dsParam)
  const [modelId, setModelId] = useState('')
  const [models, setModels] = useState<SemanticModelSummary[]>([])
  const [modelDetail, setModelDetail] = useState<SemanticModelDetail | null>(null)
  const [generatingModel, setGeneratingModel] = useState(false)
  const [generatedModel, setGeneratedModel] = useState<GenerateSemanticModelResponse | null>(null)

  useEffect(() => {
    get<Datasource[]>('/api/datasources').then((data) => {
      if (!data) return
      setDatasources(data)
      setDatasourceId((prev) => {
        if (prev) return prev
        return data[0]?.id ?? ''
      })
      setLoadedDatasources(true)
    })
  }, [])

  const isLocked = useMemo(() => {
    if (!loadedDatasources) return false
    if (!datasourceId) return false
    return !datasources.some((d) => d.id === datasourceId)
  }, [loadedDatasources, datasourceId, datasources])

  useEffect(() => {
    setDsParam(datasourceId)
  }, [datasourceId, setDsParam])

  useEffect(() => {
    if (!datasourceId) {
      setModels([])
      setGeneratedModel(null)
      return
    }
    let cancelled = false
    void get<SemanticModelSummary[]>(
      `/api/semantic/models?datasource_id=${encodeURIComponent(datasourceId)}`,
    ).then((data) => {
      if (!data || cancelled) return
      setModels(data)
      setGeneratedModel(null)
      setModelId((prev) => {
        if (prev && data.some((m) => m.id === prev)) return prev
        const published = data.filter((m) => m.status === 'published')
        if (published.length > 0) return published[0]!.id
        return data[0]?.id ?? ''
      })
    })
    return () => {
      cancelled = true
    }
  }, [datasourceId])

  useEffect(() => {
    if (!modelId) {
      setModelDetail(null)
      return
    }
    let cancelled = false
    void get<SemanticModelDetail>(`/api/semantic/models/${encodeURIComponent(modelId)}`).then((d) => {
      if (!cancelled) setModelDetail(d ?? null)
    })
    return () => {
      cancelled = true
    }
  }, [modelId])

  const selectItemsState = useArrayState<SelectItem>([])
  const filterState = useArrayState<FilterRow>([])
  const groupByState = useArrayState<string>([])
  const { items: selectItems, setItems: setSelectItems } = selectItemsState
  const { items: filters, setItems: setFilters } = filterState
  const { items: groupBy, setItems: setGroupBy } = groupByState
  const [orderBy, setOrderBy] = useState<string>('')
  const [orderDir, setOrderDir] = useState('asc')
  const [limit, setLimit] = useState(100)
  const [offset, setOffset] = useState(0)
  const [mode, setMode] = useState<'simple' | 'advanced'>('simple')
  const havingState = useArrayState<HavingRow>([])
  const windowFunctionState = useArrayState<WindowFuncRow>([])
  const cteState = useArrayState<CTERow>([])
  const { items: having, setItems: setHaving } = havingState
  const { items: windowFunctions } = windowFunctionState
  const { items: ctes } = cteState
  const [result, setResult] = useState<QueryBuilderResult | null>(null)
  const [sql, setSql] = useState('')
  const [chartType, setChartType] = useState<'bar' | 'line' | 'pie'>('bar')

  // Notebook Summarize Step Toggle State
  const [isSummarized, setIsSummarized] = useState(false)

  const dimensions = useMemo(() => modelDetail?.dimensions ?? [], [modelDetail])
  const metrics = useMemo(() => modelDetail?.metrics ?? [], [modelDetail])
  const filterFieldOpts = useMemo(() => filterFieldOptions(dimensions, metrics, t), [dimensions, metrics, t])
  
  const orderByOpts = useMemo(() => {
    const fields = orderByFieldOptions(dimensions, metrics, t)
    if (fields.length === 0) return []
    return [{ value: '', label: t('query_builder.order_none'), hint: '' }, ...fields]
  }, [dimensions, metrics, t])
  
  const metricOptsHaving = useMemo(() => metricFieldOptions(metrics), [metrics])

  const createSemanticModel = async () => {
    if (!datasourceId || generatingModel) return
    setGeneratingModel(true)
    try {
      const res = await postData<GenerateSemanticModelResponse>('/api/semantic/models/generate', {
        datasource_id: datasourceId,
        publish: true,
      })
      if (!res?.model) return
      setGeneratedModel(res)
      setModels((prev) => {
        const summary = res.model
        const next = prev.filter((m) => m.id !== summary.id)
        return [summary, ...next]
      })
      setModelId(res.model.id)
      setModelDetail(res.model)
    } finally {
      setGeneratingModel(false)
    }
  }

  const toggleSummarize = () => {
    setIsSummarized((prev) => {
      const next = !prev
      if (next) {
        // Transitioning to summarized mode:
        // Find dimensions and metrics in selectItems
        const rawDims = selectItems.filter((item) => item.type === 'dimension' && item.name).map((item) => item.name)
        
        if (rawDims.length > 0) {
          setGroupBy(rawDims)
        } else {
          const firstDim = dimensions[0]?.name
          setGroupBy(firstDim ? [firstDim] : [])
        }
        
        const rawMetrics = selectItems.filter((item) => item.type === 'metric')
        if (rawMetrics.length > 0) {
          setSelectItems(rawMetrics)
        } else {
          const firstMetric = metrics[0]?.name
          setSelectItems(firstMetric ? [{ id: newRowId(), type: 'metric', name: firstMetric }] : [])
        }
      } else {
        // Transitioning to raw mode:
        // Move groupBy dimensions and metrics back to selectItems
        const nextSelectItems: SelectItem[] = []
        for (const name of groupBy) {
          if (name) nextSelectItems.push({ id: newRowId(), type: 'dimension', name })
        }
        for (const item of selectItems) {
          if (item.type === 'metric') nextSelectItems.push(item)
        }
        if (nextSelectItems.length === 0) {
          const firstDim = dimensions[0]?.name
          if (firstDim) nextSelectItems.push({ id: newRowId(), type: 'dimension', name: firstDim })
        }
        setSelectItems(nextSelectItems)
        setGroupBy([])
      }
      return next
    })
  }

  const addSelectItem = () => selectItemsState.add({ id: newRowId(), type: 'dimension', name: '' })
  const addMetricSelectItem = (metricName = '') => {
    selectItemsState.add({ id: newRowId(), type: 'metric', name: metricName })
  }
  const updateSelectItem = (i: number, field: keyof SelectItem, value: string) => {
    const existing = selectItems[i]
    if (!existing) return
    if (field === 'type' && value !== existing.type) {
      selectItemsState.update(i, { ...existing, type: value as 'dimension' | 'metric', name: '' })
    } else {
      selectItemsState.update(i, { ...existing, [field]: value })
    }
  }
  const removeSelectItem = (i: number) => selectItemsState.remove(i)

  const addFilter = () => setFilters((prev) => addFilterRow(prev))
  const updateFilter = (i: number, field: keyof FilterRow, value: string) => {
    setFilters((prev) => {
      const next = [...prev]
      next[i] = patchFilterRow(prev[i], field, value)
      return next
    })
  }
  const removeFilter = (i: number) => setFilters((prev) => removeFilterRow(prev, i))

  const addGroupByRow = () => setGroupBy((prev) => appendGroupByRow(prev))
  const updateGroupByRow = (i: number, value: string) => setGroupBy((prev) => patchGroupByRow(prev, i, value))
  const removeGroupByRow = (i: number) => setGroupBy((prev) => dropGroupByRow(prev, i))

  const addHaving = () => setHaving((prev) => addHavingRow(prev))
  const updateHaving = (i: number, field: keyof HavingRow, value: string) => {
    setHaving((prev) => {
      const next = [...prev]
      next[i] = patchHavingRow(prev[i], field, value)
      return next
    })
  }
  const removeHaving = (i: number) => setHaving((prev) => removeHavingRow(prev, i))

  const addWindowFunc = () => windowFunctionState.add({ func: 'ROW_NUMBER', field: '', partition_by: '', order_by: '' })
  const updateWindowFunc = (i: number, field: keyof WindowFuncRow, value: string) => {
    const existing = windowFunctions[i]
    windowFunctionState.update(i, { func: existing?.func ?? 'ROW_NUMBER', field: existing?.field ?? '', partition_by: existing?.partition_by ?? '', order_by: existing?.order_by ?? '', [field]: value })
  }
  const removeWindowFunc = (i: number) => windowFunctionState.remove(i)

  const addCTE = () => cteState.add({ name: '', query: '' })
  const updateCTE = (i: number, field: keyof CTERow, value: string) => {
    const existing = ctes[i]
    cteState.update(i, { name: existing?.name ?? '', query: existing?.query ?? '', [field]: value })
  }
  const removeCTE = (i: number) => cteState.remove(i)

  const runQuery = async () => {
    const querySelectItems = isSummarized
      ? [
          ...groupBy.filter(Boolean).map((g) => ({
            id: newRowId(),
            type: 'dimension' as const,
            name: g,
          })),
          ...selectItems.filter((item) => item.type === 'metric'),
        ]
      : selectItems

    const payload = buildQueryPayload({
      datasourceId,
      modelId,
      mode,
      selectItems: querySelectItems,
      filters,
      groupBy,
      having,
      orderBy,
      orderDir,
      limit,
      offset,
      windowFunctions,
      ctes,
    })

    const explainRes = await postData<QueryExplainResponse>('/api/query/explain', payload)
    if (explainRes?.compiled_sql) {
      setSql(explainRes.compiled_sql)
    }

    const res = await postData<QueryBuilderResult>('/api/query/run', payload)
    if (res) {
      setResult(res)
    }
  }

  const chartData = useMemo(() => rowsToChartData(result?.rows), [result?.rows])

  return (
    <div className="page-stack">
      <div className="card card--query-builder">
        {/* Header Breadcrumbs and Mode selector */}
        <div className="query-builder-header">
          <div className="query-builder-pickers">
            <Select
              value={datasourceId}
              onChange={setDatasourceId}
              placeholder={t('query_builder.placeholder_pick_datasource')}
              options={datasources.map((d) => ({ value: d.id, label: d.name, hint: d.type }))}
              size="sm"
            />
            {datasourceId && models.length > 0 && (
              <Select
                value={modelId}
                onChange={setModelId}
                placeholder={t('query_builder.placeholder_pick_model')}
                disabled={models.length === 0}
                options={models.map((m) => ({
                  value: m.id,
                  label: modelListLabel(m),
                  hint: modelListHint(m),
                }))}
                size="sm"
              />
            )}
          </div>
          <div className="toggle-group query-builder-mode-toggle" role="group" aria-label={t('query_builder.mode_toggle_aria')}>
            <button type="button" className={`toggle-btn ${mode === 'simple' ? 'active' : ''}`} onClick={() => setMode('simple')}>{t('query_builder.mode_simple')}</button>
            <button type="button" className={`toggle-btn ${mode === 'advanced' ? 'active' : ''}`} onClick={() => setMode('advanced')}>{t('query_builder.mode_advanced')}</button>
          </div>
        </div>

        {isLocked ? (
          <LockedState
            datasourceId={datasourceId}
            datasourceName={datasources.find((d) => d.id === datasourceId)?.name || dsParam}
          />
        ) : (
          <>
            {/* Semantic Model Warning/Setup */}
            {modelId && models.find((m) => m.id === modelId)?.status !== 'published' ? (
              <p className="hint-text" style={{ marginBottom: '1rem', fontSize: '0.82rem', color: 'var(--text-muted)' }}>
                {t('query_builder.draft_model_warning')}
              </p>
            ) : null}

        {datasourceId && models.length === 0 ? (
          <div className="semantic-model-setup" style={{ marginBottom: '1rem' }}>
            <div>
              <strong>{t('query_builder.model_setup_title')}</strong>
              <p>{t('query_builder.model_setup_body')}</p>
            </div>
            <button type="button" className="btn btn-sm" onClick={createSemanticModel} disabled={generatingModel}>
              {generatingModel ? t('query_builder.model_setup_generating') : t('query_builder.model_setup_create')}
            </button>
          </div>
        ) : null}

        {generatedModel ? (
          <div className={generatedModel.validation?.valid === false ? 'semantic-model-setup semantic-model-setup--error' : 'semantic-model-setup semantic-model-setup--success'} style={{ marginBottom: '1rem' }}>
            <div>
              <strong>
                {generatedModel.published
                  ? t('query_builder.model_setup_created_published')
                  : t('query_builder.model_setup_created_draft')}
              </strong>
              <p>
                {t('query_builder.model_setup_summary', {
                  dimensions: generatedModel.model.dimensions?.length ?? 0,
                  metrics: generatedModel.model.metrics?.length ?? 0,
                  joins: generatedModel.model.joins?.length ?? 0,
                })}
              </p>
              {generatedModel.validation?.errors?.length ? (
                <ul>
                  {generatedModel.validation.errors.map((msg) => <li key={msg}>{msg}</li>)}
                </ul>
              ) : null}
            </div>
          </div>
        ) : null}

        {/* Notebook Steps Stack */}
        {modelDetail && (
          <div className="query-builder-notebook">
            {/* Step 1: Data */}
            <div className="notebook-step">
              <div className="notebook-step-label notebook-step-label--data">Data</div>
              <div className="notebook-step-card notebook-step-card--data">
                <span className="notebook-tag notebook-tag--blue">
                  {modelDetail.base_table}
                </span>
              </div>
            </div>

            {/* Step 2: Joins (Read-only display of relationships defined on semantic layer) */}
            {modelDetail.joins && modelDetail.joins.length > 0 && (
              <div className="notebook-step">
                <div className="notebook-step-label notebook-step-label--join">Join data</div>
                <div className="notebook-step-card notebook-step-card--join">
                  {modelDetail.joins.map((j, index) => (
                    <div key={j.id || index} className="notebook-join-flow">
                      <span className="notebook-tag notebook-tag--blue">{modelDetail.base_table}</span>
                      <span className="notebook-join-icon">⟝⟞</span>
                      <span className="notebook-tag notebook-tag--blue">{j.to_table}</span>
                      <span className="notebook-join-on">
                        on {modelDetail.base_table}.{j.from_column} = {j.to_table}.{j.to_column}
                      </span>
                    </div>
                  ))}
                </div>
              </div>
            )}

            {/* Step 3: Filter (Toggled if filters list is not empty) */}
            {filters.length > 0 && (
              <div className="notebook-step">
                <div className="notebook-step-label notebook-step-label--filter">Filter</div>
                <div className="notebook-step-card notebook-step-card--filter">
                  {filters.map((f, i) => (
                    <div key={f.id} className="notebook-tag notebook-tag--purple" style={{ display: 'flex', alignItems: 'center', gap: '0.25rem' }}>
                      <Select
                        value={f.field}
                        onChange={(v) => updateFilter(i, 'field', v)}
                        placeholder={t('query_builder.pick_field_placeholder')}
                        disabled={filterFieldOpts.length === 0}
                        options={filterFieldOpts}
                        size="sm"
                      />
                      <Select
                        value={f.operator}
                        onChange={(v) => updateFilter(i, 'operator', v)}
                        options={[
                          { value: 'eq', label: '=' },
                          { value: 'neq', label: '!=' },
                          { value: 'gt', label: '>' },
                          { value: 'gte', label: '>=' },
                          { value: 'lt', label: '<' },
                          { value: 'lte', label: '<=' },
                          { value: 'contains', label: t('query_builder.op_contains') },
                          { value: 'in', label: t('query_builder.op_in') },
                          { value: 'between', label: t('query_builder.op_between') },
                        ]}
                        size="sm"
                      />
                      <input
                        value={f.value}
                        onChange={(e) => updateFilter(i, 'value', e.target.value)}
                        placeholder={t('query_builder.value_placeholder')}
                        autoComplete="off"
                        style={{ width: '7rem' }}
                      />
                      <button
                        type="button"
                        className="notebook-tag-close"
                        onClick={() => removeFilter(i)}
                        aria-label="Remove Filter"
                      >
                        ×
                      </button>
                    </div>
                  ))}
                  <button type="button" className="notebook-add-btn" onClick={addFilter}>+</button>
                  <button
                    type="button"
                    className="notebook-step-close"
                    onClick={() => setFilters([])}
                    title={t('common.cancel')}
                  >
                    ×
                  </button>
                </div>
              </div>
            )}

            {/* Step 4: Fields (Shown if NOT summarized) */}
            {!isSummarized && (
              <div className="notebook-step">
                <div className="notebook-step-label notebook-step-label--fields">Fields</div>
                <div className="notebook-step-card notebook-step-card--fields">
                  {selectItems.map((item, i) => (
                    <div key={item.id} className="notebook-tag notebook-tag--blue" style={{ display: 'flex', alignItems: 'center', gap: '0.25rem' }}>
                      <Select
                        value={item.type}
                        onChange={(v) => updateSelectItem(i, 'type', v)}
                        options={[
                          { value: 'dimension', label: t('query_builder.dimension') },
                          { value: 'metric', label: t('query_builder.metric') },
                        ]}
                        size="sm"
                      />
                      <Select
                        value={item.name}
                        onChange={(v) => updateSelectItem(i, 'name', v)}
                        placeholder={t('query_builder.pick_field_placeholder')}
                        disabled={item.type === 'dimension' ? dimensions.length === 0 : metrics.length === 0}
                        options={item.type === 'dimension' ? dimFieldOptions(dimensions) : metricFieldOptions(metrics)}
                        size="sm"
                      />
                      <button
                        type="button"
                        className="notebook-tag-close"
                        onClick={() => removeSelectItem(i)}
                        aria-label="Remove Field"
                      >
                        ×
                      </button>
                    </div>
                  ))}
                  <button type="button" className="notebook-add-btn" onClick={addSelectItem}>+</button>
                </div>
              </div>
            )}

            {/* Step 5: Summarize (Aggregations and Group by columns) */}
            {isSummarized && (
              <div className="notebook-step">
                <div className="notebook-step-label notebook-step-label--summarize">Summarize</div>
                <div className="notebook-step-card notebook-step-card--summarize">
                  <div className="notebook-summarize-split">
                    {/* Aggregations */}
                    <div className="notebook-summarize-section">
                      {selectItems.filter((item) => item.type === 'metric').map((item) => {
                        const i = selectItems.indexOf(item)
                        return (
                          <div key={item.id} className="notebook-tag notebook-tag--green" style={{ display: 'flex', alignItems: 'center', gap: '0.25rem' }}>
                            <Select
                              value={item.name}
                              onChange={(v) => updateSelectItem(i, 'name', v)}
                              placeholder={t('query_builder.pick_field_placeholder')}
                              options={metricFieldOptions(metrics)}
                              size="sm"
                            />
                            <button
                              type="button"
                              className="notebook-tag-close"
                              onClick={() => removeSelectItem(i)}
                              aria-label="Remove Aggregation"
                            >
                              ×
                            </button>
                          </div>
                        )
                      })}
                      <button type="button" className="notebook-add-btn" onClick={() => addMetricSelectItem('')}>+</button>
                    </div>

                    <div className="notebook-summarize-divider">by</div>

                    {/* Group by dimensions */}
                    <div className="notebook-summarize-section">
                      {groupBy.map((g, i) => (
                        <div key={i} className="notebook-tag notebook-tag--blue" style={{ display: 'flex', alignItems: 'center', gap: '0.25rem' }}>
                          <Select
                            value={g}
                            onChange={(v) => updateGroupByRow(i, v)}
                            placeholder={t('query_builder.pick_dimension_placeholder')}
                            options={dimOptionsForGroupRow(dimensions, groupBy, i)}
                            size="sm"
                          />
                          <button
                            type="button"
                            className="notebook-tag-close"
                            onClick={() => removeGroupByRow(i)}
                            aria-label="Remove Grouping"
                          >
                            ×
                          </button>
                        </div>
                      ))}
                      <button type="button" className="notebook-add-btn" onClick={addGroupByRow}>+</button>
                    </div>
                  </div>
                  <button
                    type="button"
                    className="notebook-step-close"
                    onClick={() => {
                      setIsSummarized(false)
                      setGroupBy([])
                      setSelectItems([])
                    }}
                    title={t('common.cancel')}
                  >
                    ×
                  </button>
                </div>
              </div>
            )}

            {/* Step 6: Sort (If orderBy is active) */}
            {orderBy && (
              <div className="notebook-step">
                <div className="notebook-step-label notebook-step-label--sort">Sort</div>
                <div className="notebook-step-card notebook-step-card--sort">
                  <div className="notebook-tag notebook-tag--purple" style={{ display: 'flex', alignItems: 'center', gap: '0.25rem' }}>
                    <Select
                      value={orderBy}
                      onChange={setOrderBy}
                      placeholder={t('query_builder.pick_field_placeholder')}
                      options={orderByOpts}
                      size="sm"
                    />
                    <Select
                      value={orderDir}
                      onChange={setOrderDir}
                      options={[
                        { value: 'asc', label: 'ASC' },
                        { value: 'desc', label: 'DESC' },
                      ]}
                      size="sm"
                    />
                  </div>
                  <button
                    type="button"
                    className="notebook-step-close"
                    onClick={() => setOrderBy('')}
                    title={t('common.cancel')}
                  >
                    ×
                  </button>
                </div>
              </div>
            )}

            {/* Step 7: Limit */}
            <div className="notebook-step">
              <div className="notebook-step-label notebook-step-label--limit">Row limit</div>
              <div className="notebook-step-card notebook-step-card--limit">
                <input
                  type="number"
                  min={1}
                  inputMode="numeric"
                  value={limit}
                  onChange={(e) => setLimit(Number(e.target.value))}
                  style={{ width: '6rem' }}
                />
                <button
                  type="button"
                  className="notebook-step-close"
                  onClick={() => setLimit(100)}
                  title={t('common.cancel')}
                >
                  ×
                </button>
              </div>
            </div>

            {/* Advanced Step: Having (Advanced Mode only) */}
            {mode === 'advanced' && having.length > 0 && (
              <div className="notebook-step">
                <div className="notebook-step-label notebook-step-label--advanced">Having</div>
                <div className="notebook-step-card notebook-step-card--advanced">
                  {having.map((h, i) => (
                    <div key={i} className="notebook-tag notebook-tag--purple" style={{ display: 'flex', alignItems: 'center', gap: '0.25rem' }}>
                      <Select
                        value={h.field}
                        onChange={(v) => updateHaving(i, 'field', v)}
                        placeholder={t('query_builder.pick_metric_having')}
                        options={metricOptsHaving}
                        size="sm"
                      />
                      <Select
                        value={h.operator}
                        onChange={(v) => updateHaving(i, 'operator', v)}
                        options={[
                          { value: 'gt', label: '>' },
                          { value: 'gte', label: '>=' },
                          { value: 'lt', label: '<' },
                          { value: 'lte', label: '<=' },
                          { value: 'eq', label: '=' },
                          { value: 'neq', label: '!=' },
                        ]}
                        size="sm"
                      />
                      <input
                        value={h.value}
                        onChange={(e) => updateHaving(i, 'value', e.target.value)}
                        placeholder={t('query_builder.value_placeholder')}
                        autoComplete="off"
                        style={{ width: '6rem' }}
                      />
                      <button
                        type="button"
                        className="notebook-tag-close"
                        onClick={() => removeHaving(i)}
                        aria-label="Remove Having Constraint"
                      >
                        ×
                      </button>
                    </div>
                  ))}
                  <button type="button" className="notebook-add-btn" onClick={addHaving}>+</button>
                  <button
                    type="button"
                    className="notebook-step-close"
                    onClick={() => setHaving([])}
                    title={t('common.cancel')}
                  >
                    ×
                  </button>
                </div>
              </div>
            )}

            {/* Advanced Step: Window function (Advanced Mode only) */}
            {mode === 'advanced' && windowFunctions.length > 0 && (
              <div className="notebook-step">
                <div className="notebook-step-label notebook-step-label--advanced">Window Func</div>
                <div className="notebook-step-card notebook-step-card--advanced">
                  {windowFunctions.map((w, i) => (
                    <div key={i} className="notebook-tag notebook-tag--purple" style={{ display: 'flex', alignItems: 'center', gap: '0.25rem' }}>
                      <Select
                        value={w.func}
                        onChange={(v) => updateWindowFunc(i, 'func', v)}
                        options={WINDOW_FUNC_OPTIONS.map((opt) => ({ value: opt, label: opt }))}
                        size="sm"
                      />
                      <input value={w.field} onChange={(e) => updateWindowFunc(i, 'field', e.target.value)} placeholder="field" style={{ width: '6rem' }} />
                      <input value={w.partition_by} onChange={(e) => updateWindowFunc(i, 'partition_by', e.target.value)} placeholder="partition" style={{ width: '6rem' }} />
                      <input value={w.order_by} onChange={(e) => updateWindowFunc(i, 'order_by', e.target.value)} placeholder="order" style={{ width: '6rem' }} />
                      <button
                        type="button"
                        className="notebook-tag-close"
                        onClick={() => removeWindowFunc(i)}
                        aria-label="Remove Window Function"
                      >
                        ×
                      </button>
                    </div>
                  ))}
                  <button type="button" className="notebook-add-btn" onClick={addWindowFunc}>+</button>
                  <button
                    type="button"
                    className="notebook-step-close"
                    onClick={() => windowFunctionState.setItems([])}
                    title={t('common.cancel')}
                  >
                    ×
                  </button>
                </div>
              </div>
            )}

            {/* Advanced Step: CTEs (Advanced Mode only) */}
            {mode === 'advanced' && ctes.length > 0 && (
              <div className="notebook-step">
                <div className="notebook-step-label notebook-step-label--advanced">CTEs</div>
                <div className="notebook-step-card notebook-step-card--advanced">
                  {ctes.map((c, i) => (
                    <div key={i} className="notebook-tag notebook-tag--purple" style={{ display: 'flex', flexDirection: 'column', gap: '0.25rem', alignItems: 'flex-start' }}>
                      <div style={{ display: 'flex', alignItems: 'center', gap: '0.25rem', width: '100%' }}>
                        <input value={c.name} onChange={(e) => updateCTE(i, 'name', e.target.value)} placeholder="CTE Name" style={{ width: '8rem' }} />
                        <button
                          type="button"
                          className="notebook-tag-close"
                          onClick={() => removeCTE(i)}
                          aria-label="Remove CTE"
                        >
                          ×
                        </button>
                      </div>
                      <textarea
                        value={c.query}
                        onChange={(e) => updateCTE(i, 'query', e.target.value)}
                        placeholder="CTE query JSON"
                        rows={2}
                        style={{ background: 'var(--bg-primary)', border: '1px solid var(--border-strong)', color: 'var(--text-primary)', borderRadius: '0.25rem', width: '12rem', padding: '0.25rem', fontSize: '0.74rem' }}
                      />
                    </div>
                  ))}
                  <button type="button" className="notebook-add-btn" onClick={addCTE}>+</button>
                  <button
                    type="button"
                    className="notebook-step-close"
                    onClick={() => cteState.setItems([])}
                    title={t('common.cancel')}
                  >
                    ×
                  </button>
                </div>
              </div>
            )}
          </div>
        )}

        {/* Notebook Bottom Action Toolbar */}
        {modelDetail && (
          <div className="notebook-toolbar">
            <button
              type="button"
              className={`toolbar-btn toolbar-btn--filter ${filters.length > 0 ? 'active' : ''}`}
              onClick={addFilter}
            >
              + Filter
            </button>
            <button
              type="button"
              className={`toolbar-btn toolbar-btn--summarize ${isSummarized ? 'active' : ''}`}
              onClick={toggleSummarize}
            >
              + Summarize
            </button>
            <button
              type="button"
              className={`toolbar-btn toolbar-btn--sort ${orderBy ? 'active' : ''}`}
              onClick={() => {
                if (!orderBy) {
                  const firstOpt = orderByOpts.find((o) => o.value)
                  if (firstOpt) setOrderBy(firstOpt.value)
                }
              }}
            >
              + Sort
            </button>
            <button
              type="button"
              className="toolbar-btn toolbar-btn--limit"
              onClick={() => {}}
            >
              Limit ({limit})
            </button>
            {mode === 'advanced' && (
              <>
                <button
                  type="button"
                  className={`toolbar-btn toolbar-btn--advanced ${having.length > 0 ? 'active' : ''}`}
                  onClick={addHaving}
                >
                  + Having
                </button>
                <button
                  type="button"
                  className={`toolbar-btn toolbar-btn--advanced ${windowFunctions.length > 0 ? 'active' : ''}`}
                  onClick={addWindowFunc}
                >
                  + Window Func
                </button>
                <button
                  type="button"
                  className={`toolbar-btn toolbar-btn--advanced ${ctes.length > 0 ? 'active' : ''}`}
                  onClick={addCTE}
                >
                  + CTE
                </button>
              </>
            )}
          </div>
        )}

        {/* Footer Actions */}
        {modelDetail && (
          <div className="visualize-btn-container">
            <button type="button" className="visualize-btn" onClick={runQuery} disabled={loading}>
              {loading ? t('query_builder.running') : 'Visualize'}
            </button>
          </div>
        )}

        <ErrorAlert error={error} />
          </>
        )}
      </div>

      {/* SQL Preview */}
      {sql && (
        <div className="card">
          <h2>{t('query_builder.generated_sql')}</h2>
          <div className="sql-preview">{sql}</div>
        </div>
      )}

      {/* Query Results */}
      {result && (
        <div className="card">
          {chartData.length > 0 ? (
            <div className="card-header-row card-header-row--spaced">
              <h2>{t('query_builder.results_title', { rows: result.stats?.row_count || 0, ms: result.stats?.duration_ms || 0 })}</h2>
              <ChartTypeSelector
                value={chartType}
                onChange={setChartType}
                variant="group"
                ariaLabel={t('ai_query.chart_type_aria')}
                labels={{
                  bar: t('ai_query.chart_bar'),
                  line: t('ai_query.chart_line'),
                  pie: t('ai_query.chart_pie'),
                  table: t('ai_query.chart_table'),
                }}
              />
            </div>
          ) : (
            <h2>{t('query_builder.results_title', { rows: result.stats?.row_count || 0, ms: result.stats?.duration_ms || 0 })}</h2>
          )}

          {chartData.length > 0 && (
            <ChartContainer data={chartData} type={chartType} />
          )}

          {result.columns && result.rows && (
            <table className="results-table">
              <thead>
                <tr>
                  {result.columns.map((col) => (
                    <th key={col.name}>{col.name}</th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {result.rows.map((row, i) => (
                  <tr key={i}>
                    {row.map((cell, j) => (
                      <td key={j}>{formatResultCell(cell, result.columns?.[j]?.name ?? '', {})}</td>
                    ))}
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      )}
    </div>
  )
}
