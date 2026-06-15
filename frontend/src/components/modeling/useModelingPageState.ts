import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useParams } from 'react-router-dom'

import { useApi } from '../../hooks/useApi'
import { useConfirm } from '../../hooks/useConfirm'
import { useDatasources } from '../../hooks/useDatasources'
import { useModelDetail } from '../../hooks/useModelDetail'
import { useQueryParam } from '../../hooks/useQueryParam'
import { useSemanticModels } from '../../hooks/useSemanticModels'
import { useT } from '../../i18n'
import type {
  ColumnRow,
  SemanticDimension,
  SemanticMetric,
  SemanticModelDetail,
  SemanticModelSummary,
  TableRow,
} from '../../types/semantic'
import { pickPublishedModelId, pickValidIdOrFirst } from '../../utils/effectiveSelection'
import { activeEntities, inactiveEntities } from './entityActions'
import { tableImpact } from './modelingImpact'
import {
  runAddSuggestedJoin,
  runCreateModel,
  runRemoveModel,
  runRequestSchemaToggle,
  runRequestTableRemoval,
  runSaveJoin,
  runSyncDimensions,
} from './modelingModelActions'
import { buildModelingTableCards } from './modelingTableCards'
import type { JoinForm, SuggestedJoin, Tab } from './types'
import { publishModelRequest } from './types'
import { useEntityActions } from './useEntityActions'
import { useModelingCanvas } from './useModelingCanvas'
import {
  canSaveJoinForm,
  columnOptions,
  columnsAreJoinCompatible,
  columnSelectOptions,
  defaultJoinForm,
  findColumn,
  patchJoinForm,
  tableKey,
} from './utils'

function readVisibility(storageKey: string): { shown: Set<string>; hidden: Set<string> } {
  if (!storageKey) {
    return { shown: new Set(), hidden: new Set() }
  }
  try {
    const raw = localStorage.getItem(storageKey)
    if (!raw) {
      return { shown: new Set(), hidden: new Set() }
    }
    const parsed = JSON.parse(raw) as { shown?: string[]; hidden?: string[] }
    return {
      shown: new Set(parsed.shown ?? []),
      hidden: new Set(parsed.hidden ?? []),
    }
  } catch {
    return { shown: new Set(), hidden: new Set() }
  }
}

// eslint-disable-next-line complexity
export function useModelingPageState() {
  const { modelId: routeModelId } = useParams<{ modelId: string }>()
  const t = useT()
  const confirm = useConfirm()
  const { get, postData, putData, patchData, deleteData, loading, error } = useApi()
  const [dsParam, setDsParam] = useQueryParam('ds')
  const [modelParam, setModelParam] = useQueryParam('model')
  const { datasources, loading: dsLoading } = useDatasources()
  const loadedDatasources = !dsLoading
  const [selectedDatasourceId, setSelectedDatasourceId] = useState(dsParam)
  const datasourceId = useMemo(
    () => pickValidIdOrFirst(selectedDatasourceId, datasources),
    [selectedDatasourceId, datasources],
  )
  const [tables, setTables] = useState<TableRow[]>([])
  const [columns, setColumns] = useState<ColumnRow[]>([])
  const { models, loading: modelsLoading, setModels } = useSemanticModels(datasourceId)
  const [selectedModelId, setSelectedModelId] = useState(routeModelId ?? modelParam)
  const modelId = useMemo(
    () => routeModelId ?? pickPublishedModelId(selectedModelId, models),
    [routeModelId, selectedModelId, models],
  )

  const setDatasourceId = useCallback(
    (id: string) => {
      setSelectedDatasourceId(id)
      setDsParam(id)
      setSelectedModelId('')
    },
    [setDsParam],
  )

  const setModelId = useCallback(
    (id: string) => {
      setSelectedModelId(id)
      setModelParam(id)
    },
    [setModelParam],
  )

  const {
    model,
    loading: modelDetailLoading,
    setModel,
  } = useModelDetail(modelId, { includeInactive: true })
  const defaultJoinFormValue = useMemo(
    () =>
      model && tables.length > 0 && columns.length > 0
        ? defaultJoinForm(tables, columns, model)
        : null,
    [model, tables, columns],
  )
  const [joinFormOverride, setJoinFormOverride] = useState<JoinForm | null>(null)
  const joinForm = joinFormOverride ?? defaultJoinFormValue ?? defaultJoinForm([], [], null)
  const [creatingModel, setCreatingModel] = useState(false)
  const [savingJoin, setSavingJoin] = useState(false)
  const [message, setMessage] = useState<string | null>(null)
  const [activeTab, setActiveTab] = useState<Tab>('joins')
  const [suggestedJoins, setSuggestedJoins] = useState<SuggestedJoin[]>([])
  const [publishing, setPublishing] = useState(false)
  const [highlightJoinId, setHighlightJoinId] = useState<string | null>(null)
  const [paletteOpen, setPaletteOpen] = useState(
    () => typeof window === 'undefined' || !window.matchMedia('(max-width: 1180px)').matches,
  )
  const [editorOpen, setEditorOpen] = useState(false)
  const prevDsRef = useRef(datasourceId)

  const isLocked = useMemo(() => {
    if (!loadedDatasources) {
      return false
    }
    if (!datasourceId) {
      return false
    }
    return !datasources.some((d) => d.id === datasourceId)
  }, [loadedDatasources, datasourceId, datasources])

  const refreshModels = useCallback(
    async (selectedId?: string) => {
      const list = await get<SemanticModelSummary[]>(
        `/api/semantic/models?datasource_id=${encodeURIComponent(datasourceId)}`,
      )
      if (list) {
        setModels(list)
      }
      const id = selectedId ?? modelId
      if (id) {
        const full = await get<SemanticModelDetail>(
          `/api/semantic/models/${id}?include_inactive=true`,
        )
        if (full) {
          setModel(full)
        }
      }
    },
    [datasourceId, get, modelId, setModel, setModels],
  )

  const loadSuggestedJoins = useCallback(async () => {
    if (!modelId) {
      return
    }
    const data = await get<SuggestedJoin[]>(`/api/semantic/models/${modelId}/suggested-joins`)
    setSuggestedJoins(data ?? [])
  }, [get, modelId])

  useEffect(() => {
    if (!datasourceId) {
      return
    }
    if (loadedDatasources && !datasources.some((d) => d.id === datasourceId)) {
      return
    }
    const dsChanged = Boolean(prevDsRef.current && prevDsRef.current !== datasourceId)
    prevDsRef.current = datasourceId

    if (dsChanged) {
      void Promise.resolve().then(() => {
        setMessage(null)
        setModel(null)
      })
    }

    void get<TableRow[]>(`/api/datasources/${datasourceId}/tables`).then((data) =>
      setTables(data ?? []),
    )
    void get<ColumnRow[]>(`/api/datasources/${datasourceId}/columns`).then((data) =>
      setColumns(data ?? []),
    )
  }, [datasourceId, datasources, get, loadedDatasources, setModel])

  useEffect(() => {
    if (!modelId) {
      void Promise.resolve().then(() => setSuggestedJoins([]))
      return
    }
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void loadSuggestedJoins()
  }, [loadSuggestedJoins, modelId])

  const excludedSchemas = useMemo(() => new Set(model?.excluded_schemas ?? []), [model])

  const includedTables = useMemo(
    () => tables.filter((tbl) => !excludedSchemas.has(tbl.schema_name)),
    [tables, excludedSchemas],
  )

  const visibilityStorageKey = modelId ? `modeling.visibility.${modelId}` : ''
  const baseVisibility = useMemo(() => readVisibility(visibilityStorageKey), [visibilityStorageKey])
  const [visibilityOverride, setVisibilityOverride] = useState<{
    key: string
    shown: Set<string>
    hidden: Set<string>
  } | null>(null)
  const manualShown =
    visibilityOverride?.key === visibilityStorageKey
      ? visibilityOverride.shown
      : baseVisibility.shown
  const manualHidden =
    visibilityOverride?.key === visibilityStorageKey
      ? visibilityOverride.hidden
      : baseVisibility.hidden

  const persistVisibility = useCallback(
    (shown: Set<string>, hidden: Set<string>) => {
      if (!visibilityStorageKey) {
        return
      }
      try {
        localStorage.setItem(
          visibilityStorageKey,
          JSON.stringify({ shown: Array.from(shown), hidden: Array.from(hidden) }),
        )
      } catch {
        // ignore quota errors
      }
    },
    [visibilityStorageKey],
  )

  const tableCards = useMemo(
    () => buildModelingTableCards(model, includedTables, manualShown, manualHidden),
    [model, includedTables, manualShown, manualHidden],
  )

  const [baseSwapOpen, setBaseSwapOpen] = useState(false)
  const [savingBaseSwap, setSavingBaseSwap] = useState(false)
  const [addMetricOpen, setAddMetricOpen] = useState(false)
  const [enumDimension, setEnumDimension] = useState<SemanticDimension | null>(null)
  const [editingMetric, setEditingMetric] = useState<SemanticMetric | null>(null)
  const [editingDimension, setEditingDimension] = useState<SemanticDimension | null>(null)

  const getTableImpact = useCallback(
    (schema: string, table: string) =>
      model ? tableImpact(model, schema, table) : { joins: 0, dims: 0, metrics: 0 },
    [model],
  )

  const toggleTableVisibility = useCallback(
    (schema: string, table: string, makeVisible: boolean) => {
      const k = tableKey(schema, table)
      const nextShown = new Set(manualShown)
      const nextHidden = new Set(manualHidden)
      if (makeVisible) {
        nextShown.add(k)
        nextHidden.delete(k)
      } else {
        nextHidden.add(k)
        nextShown.delete(k)
      }
      setVisibilityOverride({
        key: visibilityStorageKey,
        shown: nextShown,
        hidden: nextHidden,
      })
      persistVisibility(nextShown, nextHidden)
    },
    [manualShown, manualHidden, persistVisibility, visibilityStorageKey],
  )

  const requestMakeBase = useCallback(
    async (schema: string, table: string) => {
      if (!model) {
        return
      }
      await putData(`/api/semantic/models/${model.id}`, {
        base_schema: schema,
        base_table: table,
      })
      await refreshModels(model.id)
      setMessage(t('modeling.base_changed'))
    },
    [model, putData, refreshModels, t],
  )

  const swapBaseAndRemoveOld = useCallback(
    async (newSchema: string, newTable: string) => {
      if (!model) {
        return
      }
      const oldSchema = model.base_schema
      const oldTable = model.base_table
      setSavingBaseSwap(true)
      try {
        await putData(`/api/semantic/models/${model.id}`, {
          base_schema: newSchema,
          base_table: newTable,
        })
        await postData(`/api/semantic/models/${model.id}/tables/remove`, {
          schema: oldSchema,
          table: oldTable,
        })
        await refreshModels(model.id)
        await loadSuggestedJoins()
        setMessage(t('modeling.table_removed'))
        setBaseSwapOpen(false)
      } finally {
        setSavingBaseSwap(false)
      }
    },
    [loadSuggestedJoins, model, postData, putData, refreshModels, t],
  )

  const requestTableRemoval = useCallback(
    (schema: string, table: string) =>
      runRequestTableRemoval(
        {
          model,
          confirm,
          postData,
          refreshModels,
          loadSuggestedJoins,
          setMessage,
          setBaseSwapOpen,
          toggleTableVisibility,
          t,
        },
        schema,
        table,
      ),
    [confirm, loadSuggestedJoins, model, postData, refreshModels, t, toggleTableVisibility],
  )

  const syncDimensions = useCallback(
    () => runSyncDimensions({ model, postData, refreshModels, setMessage, t }),
    [model, postData, refreshModels, t],
  )

  const canvas = useModelingCanvas(modelId, tableCards, columns, model)

  const highlightedColumns = useMemo(() => {
    const out = new Map<string, Set<string>>()
    for (const j of (model?.joins ?? []).filter((jj) => jj.is_active !== false)) {
      const fk = tableKey(j.from_schema ?? model?.base_schema ?? '', j.from_table)
      const tk = tableKey(j.to_schema ?? model?.base_schema ?? '', j.to_table)
      if (!out.has(fk)) {
        out.set(fk, new Set())
      }
      if (!out.has(tk)) {
        out.set(tk, new Set())
      }
      out.get(fk)!.add(j.from_column)
      out.get(tk)!.add(j.to_column)
    }
    return out
  }, [model])

  const fromColumns = useMemo(
    () => columnOptions(columns, joinForm.fromTable),
    [columns, joinForm.fromTable],
  )
  const allToColumns = useMemo(
    () => columnOptions(columns, joinForm.toTable),
    [columns, joinForm.toTable],
  )
  const selectedFromColumn = useMemo(
    () => findColumn(columns, joinForm.fromTable, joinForm.fromColumn),
    [columns, joinForm.fromTable, joinForm.fromColumn],
  )
  const toColumns = useMemo(
    () =>
      selectedFromColumn
        ? allToColumns.filter((column) => columnsAreJoinCompatible(selectedFromColumn, column))
        : allToColumns,
    [allToColumns, selectedFromColumn],
  )
  const fromColumnOptions = useMemo(() => columnSelectOptions(fromColumns, t), [fromColumns, t])
  const toColumnOptions = useMemo(() => columnSelectOptions(toColumns, t), [toColumns, t])
  const fromColumnValue = fromColumns.some((c) => c.column_name === joinForm.fromColumn)
    ? joinForm.fromColumn
    : ''
  const toColumnValue = toColumns.some((c) => c.column_name === joinForm.toColumn)
    ? joinForm.toColumn
    : ''
  const canSaveJoin = canSaveJoinForm(model, joinForm, columns)

  const updateJoinForm = (patch: Partial<JoinForm>) => {
    setJoinFormOverride((prev) => patchJoinForm(prev ?? joinForm, patch, columns))
  }

  const createModel = () =>
    runCreateModel({
      datasourceId,
      creatingModel,
      postData,
      setCreatingModel,
      setMessage,
      setModelId,
      setModel,
      refreshModels,
      t,
    })

  const removeModel = () =>
    runRemoveModel({
      model,
      confirm,
      deleteData,
      get,
      datasourceId,
      setModels,
      setModelId,
      setModel,
      setMessage,
      t,
    })

  const publishModel = async () => {
    if (!model || publishing) {
      return
    }
    setPublishing(true)
    setMessage(null)
    try {
      const { url, body } = publishModelRequest(model.id)
      await postData(url, body)
      await refreshModels(model.id)
      setMessage(t('modeling.model_published'))
    } finally {
      setPublishing(false)
    }
  }

  const saveJoin = () =>
    runSaveJoin({
      model,
      joinForm,
      columns,
      postData,
      refreshModels,
      loadSuggestedJoins,
      setSavingJoin,
      setMessage,
      t,
    })

  const addSuggestedJoin = (suggestion: Parameters<typeof runAddSuggestedJoin>[1]) =>
    runAddSuggestedJoin(
      { model, postData, refreshModels, loadSuggestedJoins, setMessage, t },
      suggestion,
    )

  const toggleSchemaExcluded = async (schemaName: string) => {
    if (!model) {
      return
    }
    const current = new Set((model as SemanticModelSummary).excluded_schemas ?? [])
    if (current.has(schemaName)) {
      current.delete(schemaName)
    } else {
      current.add(schemaName)
    }
    const next = Array.from(current)
    await putData(`/api/semantic/models/${model.id}`, {
      base_schema: model.base_schema,
      base_table: model.base_table,
      excluded_schemas: next,
    })
    await refreshModels(model.id)
    setMessage(t('modeling.schema_filter_updated'))
  }

  const requestSchemaToggle = (schemaName: string, isExcluded: boolean) =>
    runRequestSchemaToggle(
      {
        model,
        confirm,
        postData,
        refreshModels,
        loadSuggestedJoins,
        setMessage,
        toggleSchemaExcluded,
        t,
      },
      schemaName,
      isExcluded,
    )

  const tableOptions = tables.map((tbl) => ({
    value: tableKey(tbl.schema_name, tbl.table_name),
    label: `${tbl.schema_name}.${tbl.table_name}`,
    hint: tbl.table_type,
  }))

  const allJoins = model?.joins ?? []
  const joins = activeEntities(allJoins)
  const inactiveJoins = inactiveEntities(allJoins)
  const allDims = model?.dimensions ?? []
  const dims = activeEntities(allDims)
  const inactiveDims = inactiveEntities(allDims)
  const allMetrics = model?.metrics ?? []
  const metrics = activeEntities(allMetrics)
  const inactiveMetrics = inactiveEntities(allMetrics)
  const {
    renameTarget,
    renameValue,
    savingRename,
    setRenameValue,
    closeRename,
    renameModel,
    renameTable,
    deleteJoin,
    deleteDimension,
    deleteMetric,
    reactivateJoin,
    reactivateDimension,
    reactivateMetric,
    submitRename,
  } = useEntityActions({
    model,
    joins,
    dims,
    metrics,
    confirm,
    deleteData,
    putData,
    patchData,
    refreshModels,
    loadSuggestedJoins,
    setTables,
    setMessage,
    t,
  })

  const baseKey = model ? tableKey(model.base_schema, model.base_table) : null
  const baseSwapCandidates = useMemo(() => {
    if (!model) {
      return []
    }
    return includedTables.filter(
      (tbl) => !(tbl.schema_name === model.base_schema && tbl.table_name === model.base_table),
    )
  }, [includedTables, model])
  const usedTableCount = useMemo(() => {
    if (!model) {
      return 0
    }
    return includedTables.filter((tbl) => {
      const key = tableKey(tbl.schema_name, tbl.table_name)
      if (key === baseKey) {
        return true
      }
      const impact = getTableImpact(tbl.schema_name, tbl.table_name)
      return impact.joins > 0 || impact.dims > 0 || impact.metrics > 0
    }).length
  }, [baseKey, includedTables, model, getTableImpact])

  const highlightedTables = useMemo(() => {
    if (!highlightJoinId) {
      return null
    }
    const join = joins.find((j) => j.id === highlightJoinId)
    if (!join) {
      return null
    }
    return new Set([
      tableKey(join.from_schema ?? model?.base_schema ?? '', join.from_table),
      tableKey(join.to_schema ?? model?.base_schema ?? '', join.to_table),
    ])
  }, [highlightJoinId, joins, model])

  const highlightedJoinColumns = useMemo(() => {
    if (!highlightJoinId) {
      return null
    }
    const join = joins.find((j) => j.id === highlightJoinId)
    if (!join) {
      return null
    }
    return {
      from:
        tableKey(join.from_schema ?? model?.base_schema ?? '', join.from_table) +
        '::' +
        join.from_column,
      to:
        tableKey(join.to_schema ?? model?.base_schema ?? '', join.to_table) + '::' + join.to_column,
    }
  }, [highlightJoinId, joins, model])

  const pageLoading = dsLoading || modelsLoading || (modelId ? modelDetailLoading : false)

  const togglePalette = useCallback(() => {
    setPaletteOpen((prev) => {
      const next = !prev
      if (next && window.matchMedia('(max-width: 1180px)').matches) {
        setEditorOpen(false)
      }
      return next
    })
  }, [])

  const toggleEditor = useCallback(() => {
    setEditorOpen((prev) => {
      const next = !prev
      if (next && window.matchMedia('(max-width: 1180px)').matches) {
        setPaletteOpen(false)
      }
      return next
    })
  }, [])

  const closeMobilePanels = useCallback(() => {
    if (!window.matchMedia('(max-width: 1180px)').matches) {
      return
    }
    setPaletteOpen(false)
    setEditorOpen(false)
  }, [])

  return {
    t,
    dsParam,
    datasourceId,
    setDatasourceId,
    datasources,
    modelId,
    setModelId,
    models,
    model,
    error,
    message,
    isLocked,
    pageLoading,
    creatingModel,
    publishing,
    paletteOpen,
    setPaletteOpen,
    togglePalette,
    editorOpen,
    setEditorOpen,
    toggleEditor,
    closeMobilePanels,
    activeTab,
    setActiveTab,
    joins,
    inactiveJoins,
    dims,
    inactiveDims,
    metrics,
    inactiveMetrics,
    tables,
    includedTables,
    excludedSchemas,
    tableCards,
    usedTableCount,
    suggestedJoins,
    highlightJoinId,
    setHighlightJoinId,
    getTableImpact,
    requestSchemaToggle,
    renameTable,
    requestMakeBase,
    requestTableRemoval,
    toggleTableVisibility,
    setBaseSwapOpen,
    deleteJoin,
    addSuggestedJoin,
    reactivateJoin,
    setEditingDimension,
    setEnumDimension,
    deleteDimension,
    reactivateDimension,
    setAddMetricOpen,
    setEditingMetric,
    deleteMetric,
    reactivateMetric,
    canvas,
    baseKey,
    highlightedTables,
    highlightedColumns,
    highlightedJoinColumns,
    joinForm,
    updateJoinForm,
    tableOptions,
    fromColumns,
    toColumns,
    fromColumnOptions,
    toColumnOptions,
    fromColumnValue,
    toColumnValue,
    selectedFromColumn,
    canSaveJoin,
    savingJoin,
    loading,
    saveJoin,
    createModel,
    removeModel,
    renameModel,
    publishModel,
    syncDimensions,
    renameTarget,
    renameValue,
    savingRename,
    setRenameValue,
    closeRename,
    submitRename,
    baseSwapOpen,
    baseSwapCandidates,
    savingBaseSwap,
    swapBaseAndRemoveOld,
    addMetricOpen,
    editingMetric,
    editingDimension,
    enumDimension,
    setMessage,
    columns,
    refreshModels,
    postData,
    putData,
  }
}
