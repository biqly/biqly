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
import { formControlClass } from '../lib/formClasses'
import { legacyLayoutClass } from '../lib/layoutClasses'
import { toggleBtnClass, toggleGroupClass } from '../lib/toggleClasses'
import type {
  ColumnRow,
  GenerateSemanticModelResponse,
  SemanticJoin,
  SemanticModelDetail,
  TableRow,
} from '../types/semantic'
import { modelListHint, modelListLabel } from '../types/semantic'
import { rowsToChartData } from '../utils/chartData'
import { pickPublishedModelId, pickValidIdOrFirst } from '../utils/effectiveSelection'
import { buildQueryRequestPayload, initializeSummarizeGroupBy } from './queryBuilder/logicalQuery'
import {
  buildMetadataModel,
  type ColumnsByTable,
  metadataJoinTableKeys,
  metadataModelId,
  metadataTableOptions,
  newMetadataJoin,
  normalizeMetadataJoin,
  splitMetadataTableKey,
} from './queryBuilder/metadataModel'
import {
  qbCardClass,
  qbHeaderClass,
  qbHeaderControlsClass,
  qbHeaderGroupClass,
  qbHeaderLabelClass,
  qbHeaderRowClass,
  qbModeToggleClass,
  qbSavedDraftActionsClass,
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
import {
  deleteSavedQueryDraft,
  newSavedQueryDraftId,
  readSavedQueryDrafts,
  type SavedQueryBuilderDraft,
  upsertSavedQueryDraft,
  writeSavedQueryDrafts,
} from './queryBuilder/savedQueries'
import type {
  CTERow,
  FilterRow,
  HavingRow,
  QueryBuilderSourceMode,
  SelectItem,
  WindowFuncRow,
} from './queryBuilder/types'
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
    total_count?: number
  }
}

interface QueryExplainResponse {
  compiled_sql?: string
}

// eslint-disable-next-line complexity
export default function QueryBuilder() {
  const t = useT()
  const { get, postData, loading, error } = useApi()
  const [dsParam, setDsParam] = useQueryParam('ds')
  const { datasources, loading: dsLoading } = useDatasources()
  const loadedDatasources = !dsLoading
  const [selectedDatasourceId, setSelectedDatasourceId] = useState(dsParam)
  const [selectedModelId, setSelectedModelId] = useState('')
  const [querySource, setQuerySource] = useState<QueryBuilderSourceMode>('semantic')
  const [metadataTables, setMetadataTables] = useState<TableRow[]>([])
  const [metadataTablesLoading, setMetadataTablesLoading] = useState(false)
  const [metadataBaseTableKey, setMetadataBaseTableKey] = useState('')
  const [metadataJoins, setMetadataJoins] = useState<SemanticJoin[]>([])
  const [columnsByTable, setColumnsByTable] = useState<ColumnsByTable>({})
  const [metadataColumnsLoading, setMetadataColumnsLoading] = useState(false)
  const datasourceId = useMemo(
    () => pickValidIdOrFirst(selectedDatasourceId, datasources),
    [selectedDatasourceId, datasources],
  )
  const setDatasourceId = useCallback(
    (id: string) => {
      setSelectedDatasourceId(id)
      setDsParam(id)
      setMetadataTables([])
      setMetadataBaseTableKey('')
      setMetadataJoins([])
      setColumnsByTable({})
    },
    [setColumnsByTable, setDsParam, setMetadataBaseTableKey, setMetadataJoins, setMetadataTables],
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
  const { items: selectItems, setItems: _setSelectItems } = selectItemsState
  const { items: filters, setItems: setFilters } = filterState
  const { items: groupBy, setItems: setGroupBy } = groupByState
  const [orderBy, setOrderBy] = useState<string>('')
  const [orderDir, setOrderDir] = useState('asc')
  const [limit, setLimit] = useState(100)
  const [page, setPage] = useState(1)
  const offset = (page - 1) * limit
  const [mode, setMode] = useState<'simple' | 'advanced'>('simple')
  const havingState = useArrayState<HavingRow>([])
  const windowFunctionState = useArrayState<WindowFuncRow>([])
  const cteState = useArrayState<CTERow>([])
  const { items: having, setItems: setHaving } = havingState
  const { items: windowFunctions, setItems: setWindowFunctions } = windowFunctionState
  const { items: ctes, setItems: setCtes } = cteState
  const [result, setResult] = useState<QueryBuilderResult | null>(null)
  const [sql, setSql] = useState('')
  const [sqlVisible, setSqlVisible] = useState(false)
  const [chartType, setChartType] = useState<'bar' | 'line' | 'pie'>('bar')

  // Notebook Summarize Step Toggle State
  const [isSummarized, setIsSummarized] = useState(true)
  const [fieldLabelMode, setFieldLabelMode] = useState<'human' | 'technical'>('human')
  const [savedDrafts, setSavedDrafts] = useState<SavedQueryBuilderDraft[]>(() =>
    readSavedQueryDrafts(),
  )
  const [selectedSavedDraftId, setSelectedSavedDraftId] = useState('')
  const [savedDraftName, setSavedDraftName] = useState('')
  const [savedDraftNotice, setSavedDraftNotice] = useState('')

  const metadataTableOpts = useMemo(() => metadataTableOptions(metadataTables), [metadataTables])
  const resolvedMetadataBaseTableKey = useMemo(() => {
    if (
      metadataBaseTableKey &&
      metadataTableOpts.some((option) => option.value === metadataBaseTableKey)
    ) {
      return metadataBaseTableKey
    }
    return metadataTableOpts[0]?.value ?? ''
  }, [metadataBaseTableKey, metadataTableOpts])
  const completeMetadataJoins = useMemo(
    () =>
      metadataJoins
        .filter((join) => join.from_table && join.from_column && join.to_table && join.to_column)
        .map(normalizeMetadataJoin),
    [metadataJoins],
  )
  const metadataModel = useMemo(
    () =>
      buildMetadataModel({
        datasourceId,
        baseTableKey: resolvedMetadataBaseTableKey,
        tables: metadataTables,
        columnsByTable,
        joins: metadataJoins,
      }),
    [columnsByTable, datasourceId, metadataJoins, metadataTables, resolvedMetadataBaseTableKey],
  )
  const metadataRunnableModel = useMemo(() => {
    if (!metadataModel) {
      return null
    }
    return { ...metadataModel, joins: completeMetadataJoins }
  }, [completeMetadataJoins, metadataModel])
  const activeModelDetail: SemanticModelDetail | null =
    querySource === 'metadata' ? metadataModel : modelDetail
  const activeModelId =
    querySource === 'metadata'
      ? resolvedMetadataBaseTableKey
        ? metadataModelId(datasourceId, resolvedMetadataBaseTableKey)
        : ''
      : modelId
  const dimensions = useMemo(() => activeModelDetail?.dimensions ?? [], [activeModelDetail])
  const metrics = useMemo(() => activeModelDetail?.metrics ?? [], [activeModelDetail])
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
  const savedDraftOptions = useMemo(
    () => savedDrafts.map((draft) => ({ value: draft.id, label: draft.name })),
    [savedDrafts],
  )
  const defaultSavedDraftName = useMemo(() => {
    if (querySource === 'metadata') {
      return resolvedMetadataBaseTableKey
        ? resolvedMetadataBaseTableKey
        : t('query_builder.saved_query_default_name')
    }
    const modelName = models.find((model) => model.id === modelId)
    return (
      modelName?.label ??
      modelName?.name ??
      modelDetail?.label ??
      modelDetail?.name ??
      t('query_builder.saved_query_default_name')
    )
  }, [modelDetail, modelId, models, querySource, resolvedMetadataBaseTableKey, t])

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

  const resetQueryDraft = useCallback(() => {
    _setSelectItems([])
    setFilters([])
    setGroupBy([])
    setOrderBy('')
    setOrderDir('asc')
    setPage(1)
    setHaving([])
    setWindowFunctions([])
    setCtes([])
    setResult(null)
    setSql('')
    setSqlVisible(false)
  }, [_setSelectItems, setCtes, setFilters, setGroupBy, setHaving, setWindowFunctions])

  const clearQueryOutputs = useCallback(() => {
    setPage(1)
    setResult(null)
    setSql('')
    setSqlVisible(false)
  }, [])

  const setQuerySourceMode = useCallback(
    (source: QueryBuilderSourceMode) => {
      setQuerySource((prev) => {
        if (prev !== source) {
          resetQueryDraft()
        }
        return source
      })
    },
    [resetQueryDraft, setQuerySource],
  )

  const setMetadataBaseTable = useCallback(
    (key: string) => {
      setMetadataBaseTableKey(key)
      setMetadataJoins([])
      resetQueryDraft()
    },
    [resetQueryDraft, setMetadataBaseTableKey, setMetadataJoins],
  )

  useEffect(() => {
    if (!datasourceId || querySource !== 'metadata') {
      return
    }
    let cancelled = false
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setMetadataTablesLoading(true)
    void get<TableRow[]>(`/api/datasources/${datasourceId}/tables`)
      .then((data) => {
        if (cancelled) {
          return
        }
        setMetadataTables(data ?? [])
      })
      .finally(() => {
        if (!cancelled) {
          setMetadataTablesLoading(false)
        }
      })
    return () => {
      cancelled = true
    }
  }, [datasourceId, get, querySource])

  useEffect(() => {
    if (!resolvedMetadataBaseTableKey || metadataBaseTableKey === resolvedMetadataBaseTableKey) {
      return
    }
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setMetadataBaseTableKey(resolvedMetadataBaseTableKey)
  }, [metadataBaseTableKey, resolvedMetadataBaseTableKey])

  const metadataIncludedTableKeys = useMemo(
    () => metadataJoinTableKeys(resolvedMetadataBaseTableKey, metadataJoins),
    [metadataJoins, resolvedMetadataBaseTableKey],
  )

  useEffect(() => {
    if (querySource !== 'metadata' || !datasourceId || metadataIncludedTableKeys.length === 0) {
      return
    }
    const missingKeys = metadataIncludedTableKeys.filter((key) => !columnsByTable[key])
    if (missingKeys.length === 0) {
      return
    }
    let cancelled = false
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setMetadataColumnsLoading(true)
    void Promise.all(
      missingKeys.map(async (key) => {
        const { schema, table } = splitMetadataTableKey(key)
        const columns = await get<ColumnRow[]>(
          `/api/datasources/${datasourceId}/columns?schema=${encodeURIComponent(schema)}&table=${encodeURIComponent(table)}`,
        )
        return { key, columns: columns ?? [] }
      }),
    )
      .then((entries) => {
        if (cancelled) {
          return
        }
        setColumnsByTable((prev) => {
          const next = { ...prev }
          for (const entry of entries) {
            next[entry.key] = entry.columns
          }
          return next
        })
      })
      .finally(() => {
        if (!cancelled) {
          setMetadataColumnsLoading(false)
        }
      })
    return () => {
      cancelled = true
    }
  }, [columnsByTable, datasourceId, get, metadataIncludedTableKeys, querySource])

  const metadataColumnOptionsByTable = useMemo(
    () =>
      Object.fromEntries(
        Object.entries(columnsByTable).map(([key, columns]) => [
          key,
          columns.map((column) => ({
            value: column.column_name,
            label: column.column_name,
            hint: column.data_type,
          })),
        ]),
      ),
    [columnsByTable],
  )

  const includedTableOptions = useMemo(
    () => metadataTableOpts.filter((option) => metadataIncludedTableKeys.includes(option.value)),
    [metadataIncludedTableKeys, metadataTableOpts],
  )

  const addMetadataJoin = useCallback(() => {
    if (!resolvedMetadataBaseTableKey) {
      return
    }
    setMetadataJoins((prev) => [...prev, newMetadataJoin(resolvedMetadataBaseTableKey)])
  }, [resolvedMetadataBaseTableKey, setMetadataJoins])

  const updateMetadataJoin = useCallback(
    (index: number, join: SemanticJoin) => {
      setMetadataJoins((prev) => prev.map((item, i) => (i === index ? join : item)))
    },
    [setMetadataJoins],
  )

  const removeMetadataJoin = useCallback(
    (index: number) => {
      setMetadataJoins((prev) => prev.filter((_, i) => i !== index))
    },
    [setMetadataJoins],
  )

  const buildSavedDraft = (name: string, id = newSavedQueryDraftId()): SavedQueryBuilderDraft => ({
    id,
    name,
    datasourceId,
    source: querySource,
    modelId,
    metadataBaseTableKey: resolvedMetadataBaseTableKey,
    metadataJoins,
    fieldLabelMode,
    mode,
    selectItems,
    filters,
    groupBy,
    having,
    orderBy,
    orderDir,
    limit,
    isSummarized,
    windowFunctions,
    ctes,
    updatedAt: new Date().toISOString(),
  })

  const saveCurrentDraft = () => {
    if (!datasourceId) {
      return
    }
    const existing = savedDrafts.find((draft) => draft.id === selectedSavedDraftId)
    const typedName = savedDraftName.trim()
    const name = typedName ? typedName : (existing?.name ?? defaultSavedDraftName).trim()
    if (!name) {
      return
    }
    const draft = buildSavedDraft(name, existing?.id)
    const next = upsertSavedQueryDraft(savedDrafts, draft)
    setSavedDrafts(next)
    writeSavedQueryDrafts(next)
    setSelectedSavedDraftId(draft.id)
    setSavedDraftName(draft.name)
    setSavedDraftNotice(t('query_builder.saved_query_saved'))
  }

  const openSavedDraft = (id: string) => {
    const draft = savedDrafts.find((item) => item.id === id)
    if (!draft) {
      setSelectedSavedDraftId('')
      setSavedDraftName('')
      return
    }
    setSelectedSavedDraftId(id)
    setSavedDraftName(draft.name)
    setDatasourceId(draft.datasourceId)
    setQuerySource(draft.source)
    setSelectedModelId(draft.modelId)
    setFieldLabelMode(draft.fieldLabelMode)
    setMode(draft.mode)
    _setSelectItems(draft.selectItems)
    setFilters(draft.filters)
    setGroupBy(draft.groupBy)
    setHaving(draft.having)
    setOrderBy(draft.orderBy)
    setOrderDir(draft.orderDir)
    setLimit(draft.limit)
    setIsSummarized(draft.isSummarized)
    setWindowFunctions(draft.windowFunctions)
    setCtes(draft.ctes)
    setMetadataBaseTableKey(draft.metadataBaseTableKey)
    setMetadataJoins(draft.metadataJoins)
    clearQueryOutputs()
    setSavedDraftNotice(t('query_builder.saved_query_opened'))
  }

  const deleteSelectedDraft = () => {
    if (!selectedSavedDraftId) {
      return
    }
    const next = deleteSavedQueryDraft(savedDrafts, selectedSavedDraftId)
    setSavedDrafts(next)
    writeSavedQueryDrafts(next)
    setSelectedSavedDraftId('')
    setSavedDraftName('')
    setSavedDraftNotice(t('query_builder.saved_query_deleted'))
  }

  const toggleSummarize = () => {
    if (!isSummarized) {
      setGroupBy((current) => initializeSummarizeGroupBy(selectItems, current))
    }
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

  const buildPayload = (overrides?: { offset?: number }) => {
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

    return buildQueryRequestPayload({
      datasourceId,
      modelId: activeModelId,
      inlineModel: querySource === 'metadata' ? (metadataRunnableModel ?? undefined) : undefined,
      mode,
      selectItems: querySelectItems,
      filters,
      groupBy,
      having,
      orderBy,
      orderDir,
      limit,
      offset: overrides?.offset ?? offset,
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
    setPage(1)
    const payload = buildPayload()
    await compileSql(payload)
    const res = await postData<QueryBuilderResult>('/api/query/run', payload)
    if (res) {
      setResult(res)
    }
  }

  const goToPage = async (newPage: number) => {
    const newOffset = (newPage - 1) * limit
    const payload = buildPayload({ offset: newOffset })
    await compileSql(payload)
    const res = await postData<QueryBuilderResult>('/api/query/run', payload)
    if (res) {
      setResult(res)
      setPage(newPage)
    }
  }

  // Auto-compile SQL when inputs change and SQL preview is visible
  useEffect(() => {
    if (!sqlVisible || !datasourceId || !activeModelId) {
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
    activeModelId,
    metadataRunnableModel,
    querySource,
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

  if (
    dsLoading ||
    modelsLoading ||
    (querySource === 'semantic' && modelId ? modelDetailLoading : false) ||
    (querySource === 'metadata' && metadataTablesLoading)
  ) {
    return <LoadingScreen minHeight="300px" />
  }

  return (
    <div className={legacyLayoutClass('page-stack')}>
      <div className={cn(cardClass(), qbCardClass)}>
        {/* Header — labeled groups split across two rows: data/saved, then options */}
        <div className={qbHeaderClass}>
          {/* Row 1 — data source + saved queries */}
          <div className={qbHeaderRowClass}>
            <div className={qbHeaderGroupClass}>
              <span className={qbHeaderLabelClass}>{t('query_builder.group_data_source')}</span>
              <div className={qbHeaderControlsClass}>
                <Select
                  value={datasourceId}
                  onChange={setDatasourceId}
                  placeholder={t('query_builder.placeholder_pick_datasource')}
                  options={datasources.map((d) => ({ value: d.id, label: d.name, hint: d.type }))}
                  size="sm"
                />
                {datasourceId && querySource === 'semantic' && models.length > 0 && (
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
            </div>
            {datasourceId && (
              <div className={qbHeaderGroupClass}>
                <span className={qbHeaderLabelClass}>{t('query_builder.group_saved_queries')}</span>
                <div className={qbSavedDraftActionsClass} aria-live="polite">
                  <Select
                    value={selectedSavedDraftId}
                    onChange={openSavedDraft}
                    placeholder={t('query_builder.saved_query_placeholder')}
                    options={savedDraftOptions}
                    disabled={savedDraftOptions.length === 0}
                    size="sm"
                  />
                  <input
                    className={cn(
                      formControlClass,
                      'min-h-[1.85rem] w-44 max-w-56 rounded-[0.4rem] px-3 py-[0.3rem] text-[0.76rem]',
                    )}
                    value={savedDraftName}
                    onChange={(event) => setSavedDraftName(event.target.value)}
                    aria-label={t('query_builder.saved_query_name_aria')}
                    placeholder={t('query_builder.saved_query_name_placeholder')}
                  />
                  <button
                    type="button"
                    className={buttonClass('secondary', { size: 'sm', autoWidth: true })}
                    onClick={saveCurrentDraft}
                  >
                    {t('query_builder.saved_query_save')}
                  </button>
                  {selectedSavedDraftId && (
                    <button
                      type="button"
                      className={buttonClass('ghost', { size: 'sm', autoWidth: true })}
                      onClick={deleteSelectedDraft}
                    >
                      {t('query_builder.saved_query_delete')}
                    </button>
                  )}
                  {savedDraftNotice && (
                    <span className="text-foreground-muted ml-auto text-xs">
                      {savedDraftNotice}
                    </span>
                  )}
                </div>
              </div>
            )}
          </div>

          {/* Row 2 — options: source / mode / field labels */}
          {(datasourceId || activeModelDetail) && (
            <div className={qbHeaderRowClass}>
              {datasourceId && (
                <div className={qbHeaderGroupClass}>
                  <span className={qbHeaderLabelClass}>
                    {t('query_builder.source_toggle_aria')}
                  </span>
                  <div
                    className={toggleGroupClass(qbModeToggleClass)}
                    role="group"
                    aria-label={t('query_builder.source_toggle_aria')}
                  >
                    <button
                      type="button"
                      className={toggleBtnClass(querySource === 'semantic')}
                      onClick={() => setQuerySourceMode('semantic')}
                    >
                      {t('query_builder.source_semantic')}
                    </button>
                    <button
                      type="button"
                      className={toggleBtnClass(querySource === 'metadata')}
                      onClick={() => setQuerySourceMode('metadata')}
                    >
                      {t('query_builder.source_metadata')}
                    </button>
                  </div>
                </div>
              )}
              <div className={qbHeaderGroupClass}>
                <span className={qbHeaderLabelClass}>{t('query_builder.mode_toggle_aria')}</span>
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
              {datasourceId && activeModelDetail && (
                <div className={qbHeaderGroupClass}>
                  <span className={qbHeaderLabelClass}>{t('query_builder.label_mode_group')}</span>
                  <Select
                    value={fieldLabelMode}
                    onChange={setFieldLabelMode}
                    options={[
                      {
                        value: 'human',
                        label: t('query_builder.label_mode_human') || 'Display Names',
                      },
                      {
                        value: 'technical',
                        label: t('query_builder.label_mode_technical') || 'Technical Names',
                      },
                    ]}
                    size="sm"
                  />
                </div>
              )}
            </div>
          )}
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
                querySource === 'semantic' &&
                modelId &&
                models.find((m) => m.id === modelId)?.status !== 'published',
              )}
              t={t}
            />
            <QueryBuilderEmptyModelSetup
              show={Boolean(querySource === 'semantic' && datasourceId && models.length === 0)}
              generatingModel={generatingModel}
              onCreate={() => {
                void createSemanticModel()
              }}
              t={t}
            />
            <QueryBuilderGeneratedModelBanner generatedModel={generatedModel} t={t} />
            {querySource === 'metadata' && datasourceId && !activeModelDetail && (
              <div className="text-foreground-muted border-border rounded-lg border border-dashed px-4 py-3 text-sm">
                {metadataColumnsLoading
                  ? t('query_builder.metadata_loading_columns')
                  : t('query_builder.metadata_pick_base_table')}
              </div>
            )}
            {activeModelDetail && (
              <QueryBuilderNotebook
                modelDetail={activeModelDetail}
                sourceMode={querySource}
                baseTableKey={resolvedMetadataBaseTableKey}
                tableOptions={metadataTableOpts}
                includedTableOptions={includedTableOptions}
                columnOptionsByTable={metadataColumnOptionsByTable}
                metadataJoinsEditable={querySource === 'metadata'}
                onBaseTableChange={setMetadataBaseTable}
                onAddMetadataJoin={addMetadataJoin}
                onUpdateMetadataJoin={updateMetadataJoin}
                onRemoveMetadataJoin={removeMetadataJoin}
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
          page={page}
          pageSize={limit}
          onPageChange={(p: number) => {
            void goToPage(p)
          }}
          loading={loading}
          t={t}
        />
      )}
    </div>
  )
}
