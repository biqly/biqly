import { useCallback, useEffect, useMemo, useState } from 'react'

import { useApi } from '../hooks/useApi'
import { useArrayState } from '../hooks/useArrayState'
import { useDatasources } from '../hooks/useDatasources'
import { useModelDetail } from '../hooks/useModelDetail'
import { useQueryParam } from '../hooks/useQueryParam'
import { useSemanticModels } from '../hooks/useSemanticModels'
import { useT } from '../i18n'
import { buttonClass } from '../lib/buttonClasses'
import { cardClass } from '../lib/cardClasses'
import { cn } from '../lib/cn'
import { legacyFeedbackClass } from '../lib/feedbackClasses'
import { legacyLayoutClass } from '../lib/layoutClasses'
import { toggleBtnClass, toggleGroupClass } from '../lib/toggleClasses'
import type { GenerateSemanticModelResponse } from '../types/semantic'
import { modelListHint, modelListLabel } from '../types/semantic'
import { rowsToChartData } from '../utils/chartData'
import { pickPublishedModelId, pickValidIdOrFirst } from '../utils/effectiveSelection'
import { buildQueryPayload } from './queryBuilder/logicalQuery'
import {
  qbCardClass,
  qbHeaderClass,
  qbModeToggleClass,
  qbPickersClass,
  qbSqlCardHeadClass,
} from './queryBuilder/queryBuilderClasses'
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

// eslint-disable-next-line complexity
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
  const [fieldLabelMode, setFieldLabelMode] = useState<'human' | 'technical'>('human')

  const dimensions = useMemo(() => modelDetail?.dimensions ?? [], [modelDetail])
  const metrics = useMemo(() => modelDetail?.metrics ?? [], [modelDetail])
  const filterFieldOpts = useMemo(
    () => filterFieldOptions(dimensions, metrics, t, fieldLabelMode),
    [dimensions, metrics, t, fieldLabelMode],
  )

  const orderByOpts = useMemo(() => {
    const fields = orderByFieldOptions(dimensions, metrics, t, fieldLabelMode)
    if (fields.length === 0) {
      return []
    }
    return [{ value: '', label: t('query_builder.order_none'), hint: '' }, ...fields]
  }, [dimensions, metrics, t, fieldLabelMode])

  const metricOptsHaving = useMemo(
    () => metricFieldOptions(metrics, fieldLabelMode),
    [metrics, fieldLabelMode],
  )

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
    setIsSummarized((prev) => !prev)
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

  // Auto-compile SQL when inputs change and SQL preview is visible
  useEffect(() => {
    if (!sqlVisible || !datasourceId || !modelId) {
      return
    }
    const payload = buildPayload()
    const timer = setTimeout(() => {
      void compileSql(payload)
    }, 300)
    return () => clearTimeout(timer)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [
    sqlVisible,
    datasourceId,
    modelId,
    selectItems,
    filters,
    groupBy,
    orderBy,
    orderDir,
    limit,
    having,
    windowFunctions,
    ctes,
    isSummarized,
    mode,
  ])

  const chartData = useMemo(() => rowsToChartData(result?.rows), [result?.rows])

  if (dsLoading || modelsLoading || (modelId ? modelDetailLoading : false)) {
    return <LoadingScreen minHeight="300px" />
  }

  return (
    <div className={legacyLayoutClass('page-stack')}>
      <div className={cn(cardClass(), qbCardClass)}>
        {/* Header Breadcrumbs and Mode selector */}
        <div className={qbHeaderClass}>
          <div className={qbPickersClass}>
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
            {datasourceId && modelId && (
              <Select
                value={fieldLabelMode}
                onChange={setFieldLabelMode}
                options={[
                  { value: 'human', label: t('query_builder.label_mode_human') || 'Display Names' },
                  {
                    value: 'technical',
                    label: t('query_builder.label_mode_technical') || 'Technical Names',
                  },
                ]}
                size="sm"
              />
            )}
          </div>
          <div
            className={toggleGroupClass(qbModeToggleClass)}
            role="group"
            aria-label={t('query_builder.mode_toggle_aria')}
          >
            <button
              type="button"
              className={toggleBtnClass(mode === 'simple')}
              onClick={() => setMode('simple')}
            >
              {t('query_builder.mode_simple')}
            </button>
            <button
              type="button"
              className={toggleBtnClass(mode === 'advanced')}
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
                fieldLabelMode={fieldLabelMode}
                t={t}
              />
            )}
            <ErrorAlert error={error} />
          </>
        )}
      </div>

      {/* SQL Preview */}
      {sqlVisible && sql && (
        <div className={cardClass()}>
          <div className={qbSqlCardHeadClass}>
            <h2>{t('query_builder.generated_sql')}</h2>
            <button
              type="button"
              className={buttonClass('ghost', { size: 'sm' })}
              onClick={() => setSqlVisible(false)}
            >
              {t('query_builder.hide_sql')}
            </button>
          </div>
          <div className={legacyFeedbackClass('sql-preview')}>{sql}</div>
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
