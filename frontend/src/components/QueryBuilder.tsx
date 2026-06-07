import '../styles/queryBuilder.css'

import { useEffect, useMemo, useState } from 'react'

import { useApi } from '../hooks/useApi'
import { useArrayState } from '../hooks/useArrayState'
import { useDatasources } from '../hooks/useDatasources'
import { useModelDetail } from '../hooks/useModelDetail'
import { useQueryParam } from '../hooks/useQueryParam'
import { useSemanticModels } from '../hooks/useSemanticModels'
import { useT } from '../i18n'
import type { GenerateSemanticModelResponse } from '../types/semantic'
import { modelListHint, modelListLabel } from '../types/semantic'
import { rowsToChartData } from '../utils/chartData'
import { formatResultCell } from '../utils/resultCellFormat'
import { CteStep } from './queryBuilder/CteStep'
import { FieldsStep } from './queryBuilder/FieldsStep'
import { FilterStep } from './queryBuilder/FilterStep'
import { HavingStep } from './queryBuilder/HavingStep'
import { buildQueryPayload } from './queryBuilder/logicalQuery'
import { NotebookStep } from './queryBuilder/NotebookStep'
import {
  addFilterRow,
  addHavingRow,
  addGroupByRow as appendGroupByRow,
  removeGroupByRow as dropGroupByRow,
  patchFilterRow,
  updateGroupByRow as patchGroupByRow,
  patchHavingRow,
  removeFilterRow,
  removeHavingRow,
} from './queryBuilder/rowState'
import { SortStep } from './queryBuilder/SortStep'
import { SummarizeStep } from './queryBuilder/SummarizeStep'
import type { CTERow, FilterRow, HavingRow, SelectItem, WindowFuncRow } from './queryBuilder/types'
import { newRowId } from './queryBuilder/types'
import {
  dimFieldOptions,
  dimOptionsForGroupRow,
  filterFieldOptions,
  metricFieldOptions,
  orderByFieldOptions,
} from './queryBuilder/utils'
import { WindowFuncStep } from './queryBuilder/WindowFuncStep'
import { ChartContainer } from './ui/ChartContainer'
import { ChartTypeSelector } from './ui/ChartTypeSelector'
import { ErrorAlert } from './ui/ErrorAlert'
import { LoadingScreen } from './ui/LoadingScreen'
import { LockedState } from './ui/LockedState'
import { Select } from './ui/Select'

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
  const { postData, loading, error } = useApi()
  const [dsParam, setDsParam] = useQueryParam('ds')
  const { datasources, loading: dsLoading } = useDatasources()
  const loadedDatasources = !dsLoading
  const [datasourceId, setDatasourceId] = useState(dsParam)
  const [modelId, setModelId] = useState('')
  const { models, loading: modelsLoading, setModels } = useSemanticModels(datasourceId)
  const {
    model: modelDetail,
    loading: modelDetailLoading,
    setModel: setModelDetail,
  } = useModelDetail(modelId)
  const [generatingModel, setGeneratingModel] = useState(false)
  const [generatedModel, setGeneratedModel] = useState<GenerateSemanticModelResponse | null>(null)

  const isLocked = useMemo(() => {
    if (!loadedDatasources) {
      return false
    }
    if (!datasourceId) {
      return false
    }
    return !datasources.some((d) => d.id === datasourceId)
  }, [loadedDatasources, datasourceId, datasources])

  useEffect(() => {
    setDsParam(datasourceId)
  }, [datasourceId, setDsParam])

  // Set default datasourceId
  useEffect(() => {
    if (datasources.length > 0 && !datasourceId) {
      setDatasourceId(datasources[0]!.id)
    }
  }, [datasources, datasourceId])

  // Set default modelId
  useEffect(() => {
    if (models.length > 0) {
      setModelId((prev) => {
        if (prev && models.some((m) => m.id === prev)) {
          return prev
        }
        const published = models.filter((m) => m.status === 'published')
        if (published.length > 0) {
          return published[0]!.id
        }
        return models[0]?.id ?? ''
      })
    } else {
      setModelId('')
    }
  }, [models])

  const selectItemsState = useArrayState<SelectItem>([])
  const filterState = useArrayState<FilterRow>([])
  const groupByState = useArrayState<string>([])
  const { items: selectItems, setItems: setSelectItems } = selectItemsState
  const { items: filters, setItems: setFilters } = filterState
  const { items: groupBy, setItems: setGroupBy } = groupByState
  const [orderBy, setOrderBy] = useState<string>('')
  const [orderDir, setOrderDir] = useState('asc')
  const [limit, setLimit] = useState(100)
  const [offset] = useState(0)
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
  const filterFieldOpts = useMemo(
    () => filterFieldOptions(dimensions, metrics, t),
    [dimensions, metrics, t],
  )

  const orderByOpts = useMemo(() => {
    const fields = orderByFieldOptions(dimensions, metrics, t)
    if (fields.length === 0) {
      return []
    }
    return [{ value: '', label: t('query_builder.order_none'), hint: '' }, ...fields]
  }, [dimensions, metrics, t])

  const metricOptsHaving = useMemo(() => metricFieldOptions(metrics), [metrics])

  const createSemanticModel = async () => {
    if (!datasourceId || generatingModel) {
      return
    }
    setGeneratingModel(true)
    try {
      const res = await postData<GenerateSemanticModelResponse>('/api/semantic/models/generate', {
        datasource_id: datasourceId,
        publish: true,
      })
      if (!res?.model) {
        return
      }
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
        const rawDims = selectItems
          .filter((item) => item.type === 'dimension' && item.name)
          .map((item) => item.name)

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
          if (name) {
            nextSelectItems.push({ id: newRowId(), type: 'dimension', name })
          }
        }
        for (const item of selectItems) {
          if (item.type === 'metric') {
            nextSelectItems.push(item)
          }
        }
        if (nextSelectItems.length === 0) {
          const firstDim = dimensions[0]?.name
          if (firstDim) {
            nextSelectItems.push({ id: newRowId(), type: 'dimension', name: firstDim })
          }
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
    if (!existing) {
      return
    }
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
  const updateGroupByRow = (i: number, value: string) =>
    setGroupBy((prev) => patchGroupByRow(prev, i, value))
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

  const addWindowFunc = () =>
    windowFunctionState.add({ func: 'ROW_NUMBER', field: '', partition_by: '', order_by: '' })
  const updateWindowFunc = (i: number, field: keyof WindowFuncRow, value: string) => {
    const existing = windowFunctions[i]
    windowFunctionState.update(i, {
      func: existing?.func ?? 'ROW_NUMBER',
      field: existing?.field ?? '',
      partition_by: existing?.partition_by ?? '',
      order_by: existing?.order_by ?? '',
      [field]: value,
    })
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

  if (dsLoading || modelsLoading || (modelId ? modelDetailLoading : false)) {
    return <LoadingScreen minHeight="300px" />
  }

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
          <div
            className="toggle-group query-builder-mode-toggle"
            role="group"
            aria-label={t('query_builder.mode_toggle_aria')}
          >
            <button
              type="button"
              className={`toggle-btn ${mode === 'simple' ? 'active' : ''}`}
              onClick={() => setMode('simple')}
            >
              {t('query_builder.mode_simple')}
            </button>
            <button
              type="button"
              className={`toggle-btn ${mode === 'advanced' ? 'active' : ''}`}
              onClick={() => setMode('advanced')}
            >
              {t('query_builder.mode_advanced')}
            </button>
          </div>
        </div>

        {isLocked ? (
          <LockedState
            datasourceId={datasourceId}
            datasourceName={datasources.find((d) => d.id === datasourceId)?.name ?? dsParam}
          />
        ) : (
          <>
            {/* Semantic Model Warning/Setup */}
            {modelId && models.find((m) => m.id === modelId)?.status !== 'published' ? (
              <p
                className="hint-text"
                style={{ marginBottom: '1rem', fontSize: '0.82rem', color: 'var(--text-muted)' }}
              >
                {t('query_builder.draft_model_warning')}
              </p>
            ) : null}

            {datasourceId && models.length === 0 ? (
              <div className="semantic-model-setup" style={{ marginBottom: '1rem' }}>
                <div>
                  <strong>{t('query_builder.model_setup_title')}</strong>
                  <p>{t('query_builder.model_setup_body')}</p>
                </div>
                <button
                  type="button"
                  className="btn btn-sm"
                  onClick={() => {
                    void createSemanticModel()
                  }}
                  disabled={generatingModel}
                >
                  {generatingModel
                    ? t('query_builder.model_setup_generating')
                    : t('query_builder.model_setup_create')}
                </button>
              </div>
            ) : null}

            {generatedModel ? (
              <div
                className={
                  generatedModel.validation?.valid === false
                    ? 'semantic-model-setup semantic-model-setup--error'
                    : 'semantic-model-setup semantic-model-setup--success'
                }
                style={{ marginBottom: '1rem' }}
              >
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
                      {generatedModel.validation.errors.map((msg) => (
                        <li key={msg}>{msg}</li>
                      ))}
                    </ul>
                  ) : null}
                </div>
              </div>
            ) : null}

            {/* Notebook Steps Stack */}
            {modelDetail && (
              <div className="query-builder-notebook">
                {/* Step 1: Data */}
                <NotebookStep label="Data" themeClass="data">
                  <span className="notebook-tag notebook-tag--blue">{modelDetail.base_table}</span>
                </NotebookStep>

                {/* Step 2: Joins (Read-only display of relationships defined on semantic layer) */}
                {modelDetail.joins && modelDetail.joins.length > 0 && (
                  <NotebookStep label="Join data" themeClass="join">
                    {modelDetail.joins.map((j, index) => {
                      const getCardinality = (rel?: string) => {
                        if (!rel) {
                          return '1:1'
                        }
                        switch (rel) {
                          case 'many_to_one':
                            return 'N:1'
                          case 'one_to_many':
                            return '1:N'
                          case 'one_to_one':
                            return '1:1'
                          case 'many_to_many':
                            return 'N:N'
                          default:
                            return rel.replace(/_/g, '-')
                        }
                      }
                      return (
                        <div key={j.id || index} className="notebook-join-flow">
                          <span className="notebook-join-type">{j.join_type}</span>
                          <span className="notebook-tag notebook-tag--table">{j.from_table}</span>
                          <span className="notebook-join-connector">
                            <span className="notebook-join-line"></span>
                            <span className="notebook-join-cardinality">
                              {getCardinality(j.relationship)}
                            </span>
                            <span className="notebook-join-line"></span>
                          </span>
                          <span className="notebook-tag notebook-tag--table">{j.to_table}</span>
                          <span className="notebook-join-on-clause">
                            <span className="notebook-join-on-label">ON</span>
                            <code className="notebook-join-expression">
                              <span className="notebook-join-table-prefix">{j.from_table}</span>.
                              {j.from_column} ={' '}
                              <span className="notebook-join-table-prefix">{j.to_table}</span>.
                              {j.to_column}
                            </code>
                          </span>
                        </div>
                      )
                    })}
                  </NotebookStep>
                )}

                {/* Step 3: Filter (Toggled if filters list is not empty) */}
                <FilterStep
                  filters={filters}
                  filterFieldOpts={filterFieldOpts}
                  updateFilter={updateFilter}
                  removeFilter={removeFilter}
                  addFilter={addFilter}
                  onClear={() => setFilters([])}
                  t={t}
                />

                {/* Step 4: Fields (Shown if NOT summarized) */}
                {!isSummarized && (
                  <FieldsStep
                    selectItems={selectItems}
                    dimensions={dimensions}
                    metrics={metrics}
                    updateSelectItem={updateSelectItem}
                    removeSelectItem={removeSelectItem}
                    addSelectItem={addSelectItem}
                    dimFieldOptions={dimFieldOptions}
                    metricFieldOptions={metricFieldOptions}
                    t={t}
                  />
                )}

                {/* Step 5: Summarize (Aggregations and Group by columns) */}
                <SummarizeStep
                  selectItems={selectItems}
                  groupBy={groupBy}
                  dimensions={dimensions}
                  metrics={metrics}
                  updateSelectItem={updateSelectItem}
                  removeSelectItem={removeSelectItem}
                  addMetricSelectItem={addMetricSelectItem}
                  updateGroupByRow={updateGroupByRow}
                  removeGroupByRow={removeGroupByRow}
                  addGroupByRow={addGroupByRow}
                  onClear={() => {
                    setIsSummarized(false)
                    setGroupBy([])
                    setSelectItems([])
                  }}
                  metricFieldOptions={metricFieldOptions}
                  dimOptionsForGroupRow={dimOptionsForGroupRow}
                  t={t}
                />

                {/* Step 6: Sort (If orderBy is active) */}
                <SortStep
                  orderBy={orderBy}
                  orderDir={orderDir}
                  orderByOpts={orderByOpts}
                  setOrderBy={setOrderBy}
                  setOrderDir={setOrderDir}
                  onClear={() => setOrderBy('')}
                  t={t}
                />

                {/* Step 7: Limit */}
                <NotebookStep
                  label="Row limit"
                  themeClass="limit"
                  onClose={() => setLimit(100)}
                  closeTitle={t('common.cancel')}
                >
                  <input
                    type="number"
                    min={1}
                    inputMode="numeric"
                    value={limit}
                    onChange={(e) => setLimit(Number(e.target.value))}
                    style={{ width: '6rem' }}
                  />
                </NotebookStep>

                {/* Advanced Step: Having (Advanced Mode only) */}
                {mode === 'advanced' && (
                  <HavingStep
                    having={having}
                    metricOptsHaving={metricOptsHaving}
                    updateHaving={updateHaving}
                    removeHaving={removeHaving}
                    addHaving={addHaving}
                    onClear={() => setHaving([])}
                    t={t}
                  />
                )}

                {/* Advanced Step: Window function (Advanced Mode only) */}
                {mode === 'advanced' && (
                  <WindowFuncStep
                    windowFunctions={windowFunctions}
                    updateWindowFunc={updateWindowFunc}
                    removeWindowFunc={removeWindowFunc}
                    addWindowFunc={addWindowFunc}
                    onClear={() => windowFunctionState.setItems([])}
                    t={t}
                  />
                )}

                {/* Advanced Step: CTEs (Advanced Mode only) */}
                {mode === 'advanced' && (
                  <CteStep
                    ctes={ctes}
                    updateCTE={updateCTE}
                    removeCTE={removeCTE}
                    addCTE={addCTE}
                    onClear={() => cteState.setItems([])}
                    t={t}
                  />
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
                      if (firstOpt) {
                        setOrderBy(firstOpt.value)
                      }
                    }
                  }}
                >
                  + Sort
                </button>
                <button type="button" className="toolbar-btn toolbar-btn--limit" onClick={() => {}}>
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
                <button
                  type="button"
                  className="visualize-btn"
                  onClick={() => {
                    void runQuery()
                  }}
                  disabled={loading}
                >
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
              <h2>
                {t('query_builder.results_title', {
                  rows: result.stats?.row_count ?? 0,
                  ms: result.stats?.duration_ms ?? 0,
                })}
              </h2>
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
            <h2>
              {t('query_builder.results_title', {
                rows: result.stats?.row_count ?? 0,
                ms: result.stats?.duration_ms ?? 0,
              })}
            </h2>
          )}

          {chartData.length > 0 && <ChartContainer data={chartData} type={chartType} />}

          {result.columns && result.rows && (
            <div className="results-table-scroll">
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
                        <td key={j}>
                          {formatResultCell(cell, result.columns?.[j]?.name ?? '', {})}
                        </td>
                      ))}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      )}
    </div>
  )
}
