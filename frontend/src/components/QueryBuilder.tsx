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
  isCrossSchemaJoin,
  joinEdgeLabel,
  metricDisplayName,
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
        if (prev && data.some((d) => d.id === prev)) return prev
        return data[0]?.id ?? ''
      })
    })
  }, [])

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

  const dimensions = useMemo(() => modelDetail?.dimensions ?? [], [modelDetail])
  const metrics = useMemo(() => modelDetail?.metrics ?? [], [modelDetail])
  const filterFieldOpts = useMemo(() => filterFieldOptions(dimensions, metrics, t), [dimensions, metrics, t])
  const orderByOpts = useMemo(() => {
    const fields = orderByFieldOptions(dimensions, metrics, t)
    if (fields.length === 0) return []
    return [{ value: '', label: t('query_builder.order_none'), hint: '' }, ...fields]
  }, [dimensions, metrics, t])
  const metricOptsHaving = useMemo(() => metricFieldOptions(metrics), [metrics])
  const selectedMetricNames = useMemo(
    () => new Set(selectItems.filter((item) => item.type === 'metric' && item.name).map((item) => item.name)),
    [selectItems],
  )

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

  const addSelectItem = () => selectItemsState.add({ id: newRowId(), type: 'dimension', name: '' })
  const addMetricSelectItem = (metricName = '') => {
    if (metricName && selectedMetricNames.has(metricName)) return
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

  // Window function helpers
  const addWindowFunc = () => windowFunctionState.add({ func: 'ROW_NUMBER', field: '', partition_by: '', order_by: '' })
  const updateWindowFunc = (i: number, field: keyof WindowFuncRow, value: string) => {
    const existing = windowFunctions[i]
    windowFunctionState.update(i, { func: existing?.func ?? 'ROW_NUMBER', field: existing?.field ?? '', partition_by: existing?.partition_by ?? '', order_by: existing?.order_by ?? '', [field]: value })
  }
  const removeWindowFunc = (i: number) => windowFunctionState.remove(i)

  // CTE helpers
  const addCTE = () => cteState.add({ name: '', query: '' })
  const updateCTE = (i: number, field: keyof CTERow, value: string) => {
    const existing = ctes[i]
    cteState.update(i, { name: existing?.name ?? '', query: existing?.query ?? '', [field]: value })
  }
  const removeCTE = (i: number) => cteState.remove(i)

  const runQuery = async () => {
    const payload = buildQueryPayload({
      datasourceId,
      modelId,
      mode,
      selectItems,
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

    // Then execute
    const res = await postData<QueryBuilderResult>('/api/query/run', payload)
    if (res) {
      setResult(res)
    }
  }

  const chartData = useMemo(() => rowsToChartData(result?.rows), [result?.rows])

  return (
    <div className="page-stack">
      <div className="card card--query-builder">
        <div className="card-header-row card-header-row--spaced">
          <h2>{t('query_builder.setup_title')}</h2>
          <div className="toggle-group">
            <button type="button" className={`toggle-btn ${mode === 'simple' ? 'active' : ''}`} onClick={() => setMode('simple')}>{t('query_builder.mode_simple')}</button>
            <button type="button" className={`toggle-btn ${mode === 'advanced' ? 'active' : ''}`} onClick={() => setMode('advanced')}>{t('query_builder.mode_advanced')}</button>
          </div>
        </div>
        <div className="query-builder-inline-2">
          <div className="form-group" style={{ flex: 1 }}>
            <label htmlFor="query-datasource">{t('query_builder.datasource_label')}</label>
            <Select
              id="query-datasource"
              name="datasource"
              value={datasourceId}
              onChange={setDatasourceId}
              placeholder={t('query_builder.placeholder_pick_datasource')}
              header={t('query_builder.header_datasources')}
              options={datasources.map((d) => ({ value: d.id, label: d.name, hint: d.type }))}
            />
          </div>
          <div className="form-group" style={{ flex: 1 }}>
            <label htmlFor="query-model">{t('query_builder.semantic_model_label')}</label>
            <Select
              id="query-model"
              name="model_id"
              value={modelId}
              onChange={setModelId}
              placeholder={models.length ? t('query_builder.placeholder_pick_model') : t('query_builder.no_models_for_ds')}
              header={t('query_builder.header_semantic_models')}
              disabled={models.length === 0}
              options={models.map((m) => ({
                value: m.id,
                label: modelListLabel(m),
                hint: modelListHint(m),
              }))}
            />
            {modelId && models.find((m) => m.id === modelId)?.status !== 'published' ? (
              <p className="hint-text" style={{ marginTop: '0.35rem', fontSize: '0.82rem', color: 'var(--text-muted)' }}>
                {t('query_builder.draft_model_warning')}
              </p>
            ) : null}
            {datasourceId && models.length === 0 ? (
              <div className="semantic-model-setup">
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
              <div className={generatedModel.validation?.valid === false ? 'semantic-model-setup semantic-model-setup--error' : 'semantic-model-setup semantic-model-setup--success'}>
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
          </div>
        </div>

        {(modelDetail?.joins?.length ?? 0) > 0 && (
          <details className="semantic-joins-panel">
            <summary>{t('query_builder.join_definitions', { count: modelDetail!.joins!.length })}</summary>
            <ul className="semantic-joins-list">
              {modelDetail!.joins!.map((j) => {
                const cross = isCrossSchemaJoin(j, modelDetail?.base_schema)
                return (
                  <li key={j.id ?? j.name} className={cross ? 'semantic-join--cross-schema' : undefined}>
                    <code>{joinEdgeLabel(j, modelDetail?.base_schema)}</code>
                    {j.relationship ? <span className="semantic-join-rel">{j.relationship}</span> : null}
                    {cross ? <span className="semantic-join-badge">{t('query_builder.cross_schema')}</span> : null}
                  </li>
                )
              })}
            </ul>
          </details>
        )}


        <div className="form-group">
          <span className="form-label">{t('query_builder.select_fields_label')}</span>
          {selectItems.map((item, i) => (
            <div key={item.id} className="query-builder-row">
              <Select
                value={item.type}
                onChange={(v) => updateSelectItem(i, 'type', v)}
                ariaLabel={t('query_builder.field_type_aria', { n: i + 1 })}
                options={[
                  { value: 'dimension', label: t('query_builder.dimension') },
                  { value: 'metric', label: t('query_builder.metric') },
                ]}
              />
              <Select
                value={item.name}
                onChange={(v) => updateSelectItem(i, 'name', v)}
                ariaLabel={t('query_builder.field_name_aria', { n: i + 1 })}
                placeholder={t('query_builder.pick_field_placeholder')}
                header={item.type === 'dimension' ? t('query_builder.dimensions_header') : t('query_builder.metrics_header')}
                disabled={
                  !modelId
                  || (item.type === 'dimension' ? dimensions.length === 0 : metrics.length === 0)
                }
                options={item.type === 'dimension' ? dimFieldOptions(dimensions) : metricFieldOptions(metrics)}
              />
              <button type="button" className="remove-btn remove-btn--compact" onClick={() => removeSelectItem(i)} aria-label={t('query_builder.remove_field_aria', { n: i + 1 })}>×</button>
            </div>
          ))}
          <button type="button" className="add-btn" onClick={addSelectItem}>{t('query_builder.add_field')}</button>
        </div>

        <div className="form-group">
          <span className="form-label">{t('query_builder.filters_label')}</span>
          {filters.map((f, i) => (
            <div key={f.id} className="query-builder-row query-builder-row--filter">
              <Select
                value={f.field}
                onChange={(v) => updateFilter(i, 'field', v)}
                ariaLabel={t('query_builder.filter_field_aria', { n: i + 1 })}
                placeholder={t('query_builder.pick_field_placeholder')}
                header={t('query_builder.dim_or_metric_header')}
                disabled={!modelId || filterFieldOpts.length === 0}
                options={filterFieldOpts}
              />
              <Select
                value={f.operator}
                onChange={(v) => updateFilter(i, 'operator', v)}
                ariaLabel={t('query_builder.filter_operator_aria', { n: i + 1 })}
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
              />
              <input value={f.value} onChange={(e) => updateFilter(i, 'value', e.target.value)} placeholder={t('query_builder.value_placeholder')} aria-label={t('query_builder.filter_value_aria', { n: i + 1 })} autoComplete="off" />
              <button type="button" className="remove-btn remove-btn--compact" onClick={() => removeFilter(i)} aria-label={t('query_builder.remove_filter_aria', { n: i + 1 })}>×</button>
            </div>
          ))}
          <button type="button" className="add-btn" onClick={addFilter}>{t('query_builder.add_filter')}</button>
        </div>

        <div className="form-group">
          <label>{t('query_builder.group_by_label')}</label>
          <p style={{ margin: '0 0 0.4rem', fontSize: '0.82rem', color: 'var(--text-muted)' }}>
            {t('query_builder.group_by_hint')}
          </p>
          {groupBy.map((g, i) => (
            <div key={i} className="query-builder-row query-builder-row--group">
              <Select
                value={g}
                onChange={(v) => updateGroupByRow(i, v)}
                ariaLabel={t('query_builder.group_row_aria', { n: i + 1 })}
                placeholder={t('query_builder.pick_dimension_placeholder')}
                header={t('query_builder.dimensions_header')}
                disabled={!modelId || dimensions.length === 0}
                options={dimOptionsForGroupRow(dimensions, groupBy, i)}
              />
              <button type="button" className="remove-btn remove-btn--compact" onClick={() => removeGroupByRow(i)} aria-label={t('query_builder.remove_group_aria', { n: i + 1 })}>×</button>
            </div>
          ))}
          <button type="button" className="add-btn" onClick={addGroupByRow}>{t('query_builder.add_group_row')}</button>
        </div>

        <div className="form-group query-builder-aggregations">
          <label>{t('query_builder.aggregations_label')}</label>
          <p className="query-builder-section-hint">{t('query_builder.aggregations_hint')}</p>
          {metrics.length === 0 ? (
            <p className="query-builder-empty-note">{t('query_builder.aggregations_empty')}</p>
          ) : (
            <div className="query-builder-aggregation-grid">
              {metrics.map((metric) => {
                const selected = selectedMetricNames.has(metric.name)
                return (
                  <button
                    key={metric.id}
                    type="button"
                    className={`query-builder-aggregation-chip ${selected ? 'query-builder-aggregation-chip--selected' : ''}`}
                    onClick={() => addMetricSelectItem(metric.name)}
                    disabled={selected}
                    aria-label={t('query_builder.add_aggregation_aria', { name: metricDisplayName(metric), aggregation: aggregationDisplayName(metric.aggregation) })}
                  >
                    <span>{aggregationDisplayName(metric.aggregation)}</span>
                    <strong>{metricDisplayName(metric)}</strong>
                  </button>
                )
              })}
            </div>
          )}
        </div>

        <div className="query-builder-inline-2">
          <div className="form-group" style={{ flex: 1 }}>
            <label htmlFor="query-order-by">{t('query_builder.order_by_label')}</label>
            <Select
              id="query-order-by"
              name="order_by"
              value={orderBy}
              onChange={setOrderBy}
              placeholder={t('query_builder.pick_field_placeholder')}
              header={t('query_builder.dim_or_metric_header')}
              disabled={!modelId || orderByOpts.length === 0}
              options={orderByOpts}
            />
          </div>
          <div className="form-group" style={{ flex: 1 }}>
            <label htmlFor="query-order-direction">{t('query_builder.order_direction_label')}</label>
            <Select
              id="query-order-direction"
              name="order_direction"
              value={orderDir}
              onChange={setOrderDir}
              options={[
                { value: 'asc', label: 'ASC' },
                { value: 'desc', label: 'DESC' },
              ]}
            />
          </div>
          <div className="form-group" style={{ flex: 1 }}>
            <label htmlFor="query-limit">{t('query_builder.limit_label')}</label>
            <input id="query-limit" name="limit" type="number" min={1} inputMode="numeric" value={limit} onChange={(e) => setLimit(Number(e.target.value))} />
          </div>
          {mode === 'advanced' && (
            <div className="form-group" style={{ flex: 1 }}>
              <label htmlFor="query-offset">{t('query_builder.offset_label')}</label>
              <input id="query-offset" name="offset" type="number" min={0} inputMode="numeric" value={offset} onChange={(e) => setOffset(Number(e.target.value))} />
            </div>
          )}
        </div>

        {/* ─── Advanced Mode Sections ─────────────────────────── */}
        {mode === 'advanced' && (
          <div className="query-builder-advanced">
            <details className="query-builder-panel">
              <summary>{t('query_builder.having_panel')}</summary>
              <div className="query-builder-panel__body">
                <div className="form-group query-builder-panel__fields">
                  {having.map((h, i) => (
                    <div key={i} className="query-builder-row query-builder-row--filter">
                      <Select
                        value={h.field}
                        onChange={(v) => updateHaving(i, 'field', v)}
                        ariaLabel={t('query_builder.having_field_aria', { n: i + 1 })}
                        placeholder={t('query_builder.pick_metric_having')}
                        header={t('query_builder.metrics_post_header')}
                        disabled={!modelId || metricOptsHaving.length === 0}
                        options={metricOptsHaving}
                      />
                      <Select
                        value={h.operator}
                        onChange={(v) => updateHaving(i, 'operator', v)}
                        ariaLabel={t('query_builder.having_operator_aria', { n: i + 1 })}
                        options={[
                          { value: 'gt', label: '>' },
                          { value: 'gte', label: '>=' },
                          { value: 'lt', label: '<' },
                          { value: 'lte', label: '<=' },
                          { value: 'eq', label: '=' },
                          { value: 'neq', label: '!=' },
                        ]}
                      />
                      <input value={h.value} onChange={(e) => updateHaving(i, 'value', e.target.value)} placeholder={t('query_builder.value_placeholder')} aria-label={t('query_builder.having_value_aria', { n: i + 1 })} autoComplete="off" />
                      <button type="button" className="remove-btn remove-btn--compact" onClick={() => removeHaving(i)} aria-label={t('query_builder.remove_having_aria', { n: i + 1 })}>×</button>
                    </div>
                  ))}
                  <button type="button" className="add-btn" onClick={addHaving}>{t('query_builder.add_having')}</button>
                </div>
              </div>
            </details>

            <details className="query-builder-panel">
              <summary>{t('query_builder.window_panel')}</summary>
              <div className="query-builder-panel__body">
                <div className="form-group query-builder-panel__fields">
                  {windowFunctions.map((w, i) => (
                    <div key={i} className="query-builder-row query-builder-row--wide">
                      <Select
                        value={w.func}
                        onChange={(v) => updateWindowFunc(i, 'func', v)}
                        ariaLabel={t('query_builder.window_type_aria', { n: i + 1 })}
                        options={WINDOW_FUNC_OPTIONS.map((opt) => ({ value: opt, label: opt }))}
                      />
                      <input value={w.field} onChange={(e) => updateWindowFunc(i, 'field', e.target.value)} placeholder={t('query_builder.window_field_placeholder')} aria-label={t('query_builder.window_field_aria', { n: i + 1 })} autoComplete="off" />
                      <input value={w.partition_by} onChange={(e) => updateWindowFunc(i, 'partition_by', e.target.value)} placeholder={t('query_builder.window_partition_placeholder')} aria-label={t('query_builder.window_partition_aria', { n: i + 1 })} autoComplete="off" />
                      <input value={w.order_by} onChange={(e) => updateWindowFunc(i, 'order_by', e.target.value)} placeholder={t('query_builder.window_order_placeholder')} aria-label={t('query_builder.window_order_aria', { n: i + 1 })} autoComplete="off" />
                      <button type="button" className="remove-btn remove-btn--compact" onClick={() => removeWindowFunc(i)} aria-label={t('query_builder.remove_window_aria', { n: i + 1 })}>×</button>
                    </div>
                  ))}
                  <button type="button" className="add-btn" onClick={addWindowFunc}>{t('query_builder.add_window')}</button>
                </div>
              </div>
            </details>

            <details className="query-builder-panel">
              <summary>{t('query_builder.cte_panel')}</summary>
              <div className="query-builder-panel__body">
                <div className="form-group query-builder-panel__fields">
                  {ctes.map((c, i) => (
                    <div key={i} className="query-builder-cte-card">
                      <div className="query-builder-row query-builder-row--cte-head">
                        <input value={c.name} onChange={(e) => updateCTE(i, 'name', e.target.value)} placeholder={t('query_builder.cte_name_placeholder')} aria-label={t('query_builder.cte_name_aria', { n: i + 1 })} autoComplete="off" />
                        <button type="button" className="remove-btn remove-btn--compact" onClick={() => removeCTE(i)} aria-label={t('query_builder.remove_cte_aria', { n: i + 1 })}>×</button>
                      </div>
                      <textarea
                        className="query-builder-cte-textarea"
                        value={c.query}
                        onChange={(e) => updateCTE(i, 'query', e.target.value)}
                        placeholder={t('query_builder.cte_json_placeholder')}
                        rows={3}
                      />
                    </div>
                  ))}
                  <button type="button" className="add-btn" onClick={addCTE}>{t('query_builder.add_cte')}</button>
                </div>
              </div>
            </details>
          </div>
        )}

        <div className="query-builder-footer">
          <button type="button" className="btn btn-sm" onClick={runQuery} disabled={loading}>
            {loading ? t('query_builder.running') : t('query_builder.run_query_btn')}
          </button>
        </div>

        <ErrorAlert error={error} />
      </div>

      {sql && (
        <div className="card">
          <h2>{t('query_builder.generated_sql')}</h2>
          <div className="sql-preview">{sql}</div>
        </div>
      )}

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
