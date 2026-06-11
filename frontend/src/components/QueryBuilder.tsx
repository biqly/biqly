import '../styles/queryBuilder.css'

import { useCallback, useMemo, useState } from 'react'

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
import { pickPublishedModelId, pickValidIdOrFirst } from '../utils/effectiveSelection'
import { buildQueryPayload } from './queryBuilder/logicalQuery'
import {
  QueryBuilderDraftModelWarning,
  QueryBuilderEmptyModelSetup,
  QueryBuilderGeneratedModelBanner,
} from './queryBuilder/QueryBuilderModelSetup'
import { QueryBuilderNotebook } from './queryBuilder/QueryBuilderNotebook'
import { QueryBuilderResults } from './queryBuilder/QueryBuilderResults'
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
import type { CTERow, FilterRow, HavingRow, SelectItem, WindowFuncRow } from './queryBuilder/types'
import { newRowId } from './queryBuilder/types'
import { filterFieldOptions, metricFieldOptions, orderByFieldOptions } from './queryBuilder/utils'
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
  const [selectedDatasourceId, setSelectedDatasourceId] = useState(dsParam)
  const [selectedModelId, setSelectedModelId] = useState('')
  const datasourceId = useMemo(
    () => pickValidIdOrFirst(selectedDatasourceId, datasources),
    [selectedDatasourceId, datasources],
  )
  const setDatasourceId = useCallback(
    (id: string) => {
      setSelectedDatasourceId(id)
      setDsParam(id)
    },
    [setDsParam],
  )
  const { models, loading: modelsLoading, setModels } = useSemanticModels(datasourceId)
  const modelId = useMemo(
    () => pickPublishedModelId(selectedModelId, models),
    [selectedModelId, models],
  )
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
  const [sqlVisible, setSqlVisible] = useState(false)
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
      setSelectedModelId(res.model.id)
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

  const buildPayload = () => {
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

    return buildQueryPayload({
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
  }

  const compileSql = async (payload: ReturnType<typeof buildPayload>) => {
    const explainRes = await postData<QueryExplainResponse>('/api/query/explain', payload)
    if (explainRes?.compiled_sql) {
      setSql(explainRes.compiled_sql)
    }
  }

  const toggleSqlPreview = async () => {
    if (sqlVisible) {
      setSqlVisible(false)
      return
    }
    await compileSql(buildPayload())
    setSqlVisible(true)
  }

  const runQuery = async () => {
    const payload = buildPayload()
    await compileSql(payload)
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
                onChange={setSelectedModelId}
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
            <QueryBuilderDraftModelWarning
              show={Boolean(
                modelId && models.find((m) => m.id === modelId)?.status !== 'published',
              )}
              t={t}
            />
            <QueryBuilderEmptyModelSetup
              show={Boolean(datasourceId && models.length === 0)}
              generatingModel={generatingModel}
              onCreate={() => {
                void createSemanticModel()
              }}
              t={t}
            />
            <QueryBuilderGeneratedModelBanner generatedModel={generatedModel} t={t} />
            {modelDetail && (
              <QueryBuilderNotebook
                modelDetail={modelDetail}
                filters={filters}
                filterFieldOpts={filterFieldOpts}
                updateFilter={updateFilter}
                removeFilter={removeFilter}
                addFilter={addFilter}
                setFilters={setFilters}
                isSummarized={isSummarized}
                selectItems={selectItems}
                dimensions={dimensions}
                metrics={metrics}
                updateSelectItem={updateSelectItem}
                removeSelectItem={removeSelectItem}
                addSelectItem={addSelectItem}
                groupBy={groupBy}
                addMetricSelectItem={addMetricSelectItem}
                updateGroupByRow={updateGroupByRow}
                removeGroupByRow={removeGroupByRow}
                addGroupByRow={addGroupByRow}
                setIsSummarized={setIsSummarized}
                setGroupBy={setGroupBy}
                setSelectItems={setSelectItems}
                orderBy={orderBy}
                orderDir={orderDir}
                orderByOpts={orderByOpts}
                setOrderBy={setOrderBy}
                setOrderDir={setOrderDir}
                limit={limit}
                setLimit={setLimit}
                mode={mode}
                having={having}
                metricOptsHaving={metricOptsHaving}
                updateHaving={updateHaving}
                removeHaving={removeHaving}
                addHaving={addHaving}
                setHaving={setHaving}
                windowFunctions={windowFunctions}
                updateWindowFunc={updateWindowFunc}
                removeWindowFunc={removeWindowFunc}
                addWindowFunc={addWindowFunc}
                clearWindowFunctions={() => windowFunctionState.setItems([])}
                ctes={ctes}
                updateCTE={updateCTE}
                removeCTE={removeCTE}
                addCTE={addCTE}
                clearCtes={() => cteState.setItems([])}
                toggleSummarize={toggleSummarize}
                loading={loading}
                runQuery={runQuery}
                sqlVisible={sqlVisible}
                onToggleSql={toggleSqlPreview}
                t={t}
              />
            )}
            <ErrorAlert error={error} />
          </>
        )}
      </div>

      {/* SQL Preview */}
      {sqlVisible && sql && (
        <div className="card qb-sql-card">
          <div className="qb-sql-card__head">
            <h2>{t('query_builder.generated_sql')}</h2>
            <button
              type="button"
              className="btn btn-sm btn-ghost"
              onClick={() => setSqlVisible(false)}
            >
              {t('query_builder.hide_sql')}
            </button>
          </div>
          <div className="sql-preview">{sql}</div>
        </div>
      )}

      {result && (
        <QueryBuilderResults
          result={result}
          chartData={chartData}
          chartType={chartType}
          setChartType={setChartType}
          t={t}
        />
      )}
    </div>
  )
}
