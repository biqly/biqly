import '../styles/modeling.css'

import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useParams } from 'react-router-dom'

import { useApi } from '../hooks/useApi'
import { useConfirm } from '../hooks/useConfirm'
import { useDatasources } from '../hooks/useDatasources'
import { useModelDetail } from '../hooks/useModelDetail'
import { useQueryParam } from '../hooks/useQueryParam'
import { useSemanticModels } from '../hooks/useSemanticModels'
import { useT } from '../i18n'
import type {
  ColumnRow,
  GenerateSemanticModelResponse,
  SemanticDimension,
  SemanticMetric,
  SemanticJoin,
  SemanticModelDetail,
  SemanticModelSummary,
  TableRow,
} from '../types/semantic'
import { AddMetricModal } from './modeling/AddMetricModal'
import { BaseSwapModal } from './modeling/BaseSwapModal'
import { activeEntities, inactiveEntities } from './modeling/entityActions'
import { EditDimensionModal } from './modeling/EditDimensionModal'
import { EnumValuesModal } from './modeling/EnumValuesModal'
import { JoinEditor } from './modeling/JoinEditor'
import { ModelingCanvas } from './modeling/ModelingCanvas'
import { ModelingPalette } from './modeling/ModelingPalette'
import type { JoinForm, SuggestedJoin, Tab } from './modeling/types'
import { publishModelRequest, suggestedJoinToPayload } from './modeling/types'
import { useEntityActions } from './modeling/useEntityActions'
import { useModelingCanvas } from './modeling/useModelingCanvas'
import {
  buildJoinPayload,
  canSaveJoinForm,
  columnOptions,
  columnRefMatchesTable,
  columnsAreJoinCompatible,
  columnSelectOptions,
  defaultJoinForm,
  findColumn,
  patchJoinForm,
  tableKey,
} from './modeling/utils'
import { ShareButton } from './sharing/ShareButton'
import { ErrorAlert } from './ui/ErrorAlert'
import { LoadingScreen } from './ui/LoadingScreen'
import { LockedState } from './ui/LockedState'
import { Modal } from './ui/Modal'
import { Select } from './ui/Select'

export default function Modeling() {
  const { modelId: routeModelId } = useParams<{ modelId: string }>()
  const t = useT()
  const confirm = useConfirm()
  const { get, postData, putData, patchData, deleteData, loading, error } = useApi()
  const [dsParam, setDsParam] = useQueryParam('ds')
  const [modelParam, setModelParam] = useQueryParam('model')
  const { datasources, loading: dsLoading } = useDatasources()
  const loadedDatasources = !dsLoading
  const [datasourceId, setDatasourceId] = useState(dsParam)
  const [tables, setTables] = useState<TableRow[]>([])
  const [columns, setColumns] = useState<ColumnRow[]>([])
  const { models, loading: modelsLoading, setModels } = useSemanticModels(datasourceId)
  const [modelId, setModelId] = useState(routeModelId || modelParam)

  useEffect(() => {
    if (routeModelId) {
      setModelId(routeModelId)
    }
  }, [routeModelId])

  const {
    model,
    loading: modelDetailLoading,
    setModel,
  } = useModelDetail(modelId, { includeInactive: true })
  const [joinForm, setJoinForm] = useState<JoinForm>(() => defaultJoinForm([], [], null))
  const [creatingModel, setCreatingModel] = useState(false)
  const [savingJoin, setSavingJoin] = useState(false)
  const [message, setMessage] = useState<string | null>(null)
  const [activeTab, setActiveTab] = useState<Tab>('joins')
  const [suggestedJoins, setSuggestedJoins] = useState<SuggestedJoin[]>([])
  const [publishing, setPublishing] = useState(false)
  const [highlightJoinId, setHighlightJoinId] = useState<string | null>(null)
  const [paletteOpen, setPaletteOpen] = useState(true)
  const [editorOpen, setEditorOpen] = useState(true)
  const prevDsRef = useRef(datasourceId)

  useEffect(() => {
    if (datasources.length > 0 && !datasourceId) {
      setDatasourceId(datasources[0]!.id)
    }
  }, [datasources, datasourceId])

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

  useEffect(() => {
    setModelParam(modelId)
  }, [modelId, setModelParam])

  useEffect(() => {
    if (!datasourceId) {
      return
    }
    if (loadedDatasources && !datasources.some((d) => d.id === datasourceId)) {
      return
    }
    const dsChanged = Boolean(prevDsRef.current && prevDsRef.current !== datasourceId)
    prevDsRef.current = datasourceId

    setMessage(null)
    if (dsChanged) {
      setModel(null)
      setModelId('')
    }

    get<TableRow[]>(`/api/datasources/${datasourceId}/tables`).then((data) => setTables(data ?? []))
    get<ColumnRow[]>(`/api/datasources/${datasourceId}/columns`).then((data) =>
      setColumns(data ?? []),
    )
  }, [datasourceId, loadedDatasources, datasources])

  useEffect(() => {
    if (models.length > 0) {
      setModelId((prev) => {
        if (prev && models.some((m) => m.id === prev)) {
          return prev
        }
        const published = models.find((m) => m.status === 'published')
        return published?.id ?? models[0]?.id ?? ''
      })
    } else {
      setModelId('')
    }
  }, [models])

  useEffect(() => {
    if (!modelId) {
      setSuggestedJoins([])
      return
    }
    get<SuggestedJoin[]>(`/api/semantic/models/${modelId}/suggested-joins`).then((data) => {
      setSuggestedJoins(data ?? [])
    })
  }, [modelId])

  useEffect(() => {
    if (!model || tables.length === 0 || columns.length === 0) {
      return
    }
    setJoinForm(defaultJoinForm(tables, columns, model))
  }, [model, tables, columns])

  const excludedSchemas = useMemo(() => new Set(model?.excluded_schemas ?? []), [model])

  const includedTables = useMemo(
    () => tables.filter((t) => !excludedSchemas.has(t.schema_name)),
    [tables, excludedSchemas],
  )

  const visibilityStorageKey = modelId ? `modeling.visibility.${modelId}` : ''
  const [manualShown, setManualShown] = useState<Set<string>>(new Set())
  const [manualHidden, setManualHidden] = useState<Set<string>>(new Set())

  useEffect(() => {
    if (!visibilityStorageKey) {
      setManualShown(new Set())
      setManualHidden(new Set())
      return
    }
    try {
      const raw = localStorage.getItem(visibilityStorageKey)
      if (!raw) {
        setManualShown(new Set())
        setManualHidden(new Set())
        return
      }
      const parsed = JSON.parse(raw) as { shown?: string[]; hidden?: string[] }
      setManualShown(new Set(parsed.shown ?? []))
      setManualHidden(new Set(parsed.hidden ?? []))
    } catch {
      setManualShown(new Set())
      setManualHidden(new Set())
    }
  }, [visibilityStorageKey])

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

  const tableCards = useMemo(() => {
    const keys = new Set<string>()
    if (model) {
      keys.add(tableKey(model.base_schema, model.base_table))
    }
    for (const join of (model?.joins ?? []).filter((j) => j.is_active !== false)) {
      keys.add(tableKey(join.from_schema || model?.base_schema || '', join.from_table))
      keys.add(tableKey(join.to_schema || model?.base_schema || '', join.to_table))
    }
    const baseKey = model ? tableKey(model.base_schema, model.base_table) : ''
    const preferred = includedTables.filter((t) => keys.has(tableKey(t.schema_name, t.table_name)))
    const autofill = includedTables
      .filter((t) => !keys.has(tableKey(t.schema_name, t.table_name)))
      .slice(0, Math.max(0, 9 - preferred.length))
    const auto = [...preferred, ...autofill]
    const autoKeys = new Set(auto.map((t) => tableKey(t.schema_name, t.table_name)))
    const filteredAuto = auto.filter((t) => {
      const k = tableKey(t.schema_name, t.table_name)
      if (k === baseKey) {
        return true
      }
      return !manualHidden.has(k)
    })
    const extras = includedTables.filter((t) => {
      const k = tableKey(t.schema_name, t.table_name)
      return manualShown.has(k) && !autoKeys.has(k)
    })
    return [...filteredAuto, ...extras]
  }, [model, includedTables, manualShown, manualHidden])

  const [baseSwapOpen, setBaseSwapOpen] = useState(false)
  const [savingBaseSwap, setSavingBaseSwap] = useState(false)
  const [addMetricOpen, setAddMetricOpen] = useState(false)
  const [enumDimension, setEnumDimension] = useState<SemanticDimension | null>(null)
  const [editingMetric, setEditingMetric] = useState<SemanticMetric | null>(null)
  const [editingDimension, setEditingDimension] = useState<SemanticDimension | null>(null)

  const expressionRefsTable = useCallback(
    (expr: string | undefined | null, schema: string, table: string) => {
      if (!expr) {
        return false
      }
      const e = expr.toLowerCase()
      const tokens = [`${schema}.${table}.`, `"${schema}"."${table}".`]
      const base = model?.base_schema ?? ''
      if (schema === base) {
        tokens.push(`${table}.`, `"${table}".`)
      }
      return tokens.some((tok) => e.includes(tok.toLowerCase()))
    },
    [model],
  )

  const tableImpact = useCallback(
    (schema: string, table: string) => {
      if (!model) {
        return { joins: 0, dims: 0, metrics: 0 }
      }
      const base = model.base_schema
      const joins = (model.joins ?? []).filter((j) => {
        if (j.is_active === false) {
          return false
        }
        const fs = j.from_schema || base
        const ts = j.to_schema || base
        return (fs === schema && j.from_table === table) || (ts === schema && j.to_table === table)
      }).length
      const dims = (model.dimensions ?? []).filter(
        (d) =>
          d.is_active !== false &&
          columnRefMatchesTable(d.column_ref, schema, table, model.base_schema),
      ).length
      const metrics = (model.metrics ?? []).filter(
        (m) => m.is_active !== false && expressionRefsTable(m.expression, schema, table),
      ).length
      return { joins, dims, metrics }
    },
    [model, expressionRefsTable],
  )

  const columnRefMatchesSchema = useCallback(
    (ref: string | undefined | null, schema: string) => {
      if (!ref) {
        return false
      }
      const r = ref.trim()
      if (!r) {
        return false
      }
      const base = model?.base_schema ?? ''
      if (r.startsWith(`${schema}.`)) {
        return true
      }
      if (schema === base && r.split('.').length === 2) {
        return true
      }
      return false
    },
    [model],
  )

  const expressionRefsSchema = useCallback((expr: string | undefined | null, schema: string) => {
    if (!expr) {
      return false
    }
    const e = expr.toLowerCase()
    const tokens = [`${schema}.`, `"${schema}".`]
    return tokens.some((tok) => e.includes(tok.toLowerCase()))
  }, [])

  const schemaImpact = useCallback(
    (schema: string) => {
      if (!model) {
        return { joins: 0, dims: 0, metrics: 0 }
      }
      const base = model.base_schema
      const joins = (model.joins ?? []).filter((j) => {
        if (j.is_active === false) {
          return false
        }
        const fs = j.from_schema || base
        const ts = j.to_schema || base
        return fs === schema || ts === schema
      }).length
      const dims = (model.dimensions ?? []).filter(
        (d) => d.is_active !== false && columnRefMatchesSchema(d.column_ref, schema),
      ).length
      const metrics = (model.metrics ?? []).filter(
        (m) => m.is_active !== false && expressionRefsSchema(m.expression, schema),
      ).length
      return { joins, dims, metrics }
    },
    [model, columnRefMatchesSchema, expressionRefsSchema],
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
      setManualShown(nextShown)
      setManualHidden(nextHidden)
      persistVisibility(nextShown, nextHidden)
    },
    [manualShown, manualHidden, persistVisibility],
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
    [model, putData, t],
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
    [model, putData, postData, t],
  )

  const requestTableRemoval = useCallback(
    async (schema: string, table: string) => {
      if (!model) {
        return
      }
      const isBase = schema === model.base_schema && table === model.base_table
      if (isBase) {
        setBaseSwapOpen(true)
        return
      }
      const impact = tableImpact(schema, table)
      if (impact.joins === 0 && impact.dims === 0 && impact.metrics === 0) {
        toggleTableVisibility(schema, table, false)
        return
      }
      const ok = await confirm({
        title: t('modeling.remove_table_title'),
        message: t('modeling.remove_table_body', {
          table,
          joins: impact.joins,
          dims: impact.dims,
          metrics: impact.metrics,
        }),
        variant: 'warning',
        confirmLabel: t('modeling.remove_table_action'),
      })
      if (!ok) {
        return
      }
      setMessage(null)
      await postData(`/api/semantic/models/${model.id}/tables/remove`, { schema, table })
      await refreshModels(model.id)
      await loadSuggestedJoins()
      setMessage(t('modeling.table_removed'))
    },
    [confirm, model, tableImpact, toggleTableVisibility, postData, t],
  )

  const canvas = useModelingCanvas(modelId, tableCards, columns, model)

  const highlightedColumns = useMemo(() => {
    const out = new Map<string, Set<string>>()
    for (const j of (model?.joins ?? []).filter((jj) => jj.is_active !== false)) {
      const fk = tableKey(j.from_schema || model?.base_schema || '', j.from_table)
      const tk = tableKey(j.to_schema || model?.base_schema || '', j.to_table)
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
    setJoinForm((prev) => patchJoinForm(prev, patch, columns))
  }

  const refreshModels = async (selectedId?: string) => {
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
  }

  const createModel = async () => {
    if (!datasourceId || creatingModel) {
      return
    }
    setCreatingModel(true)
    setMessage(null)
    try {
      const res = await postData<GenerateSemanticModelResponse>(
        '/api/semantic/models/generate',
        { datasource_id: datasourceId, publish: true },
        { timeout: 180_000 },
      )
      if (!res?.model) {
        return
      }
      setModelId(res.model.id)
      setModel(res.model)
      await refreshModels(res.model.id)
      setMessage(res.published ? t('modeling.created_published') : t('modeling.created_draft'))
    } finally {
      setCreatingModel(false)
    }
  }

  const removeModel = async () => {
    if (!model) {
      return
    }
    const name = model.label || model.name
    const ok = await confirm({
      title: t('modeling.confirm_delete_model_title'),
      message: t('modeling.confirm_delete_model_body', { name }),
      variant: 'danger',
      confirmLabel: t('common.delete'),
    })
    if (!ok) {
      return
    }
    setMessage(null)
    await deleteData(`/api/semantic/models/${model.id}`)
    const list = await get<SemanticModelSummary[]>(
      `/api/semantic/models?datasource_id=${encodeURIComponent(datasourceId)}`,
    )
    const next = list ?? []
    setModels(next)
    const nextId = next[0]?.id ?? ''
    setModelId(nextId)
    if (!nextId) {
      setModel(null)
    }
    setMessage(t('modeling.model_deleted'))
  }

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

  const saveJoin = async () => {
    if (!model || !canSaveJoin) {
      return
    }
    setSavingJoin(true)
    setMessage(null)
    try {
      await postData<SemanticJoin>(
        `/api/semantic/models/${model.id}/joins`,
        buildJoinPayload(joinForm),
      )
      await refreshModels(model.id)
      await loadSuggestedJoins()
      setMessage(t('modeling.relationship_added'))
    } finally {
      setSavingJoin(false)
    }
  }

  const addSuggestedJoin = async (suggestion: SuggestedJoin) => {
    if (!model) {
      return
    }
    setMessage(null)
    try {
      await postData<SemanticJoin>(
        `/api/semantic/models/${model.id}/joins`,
        suggestedJoinToPayload(suggestion),
      )
      await refreshModels(model.id)
      await loadSuggestedJoins()
      setMessage(t('modeling.fk_relationship_added'))
    } catch {
      setMessage(t('modeling.relationship_add_failed'))
    }
  }

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

  const requestSchemaToggle = async (schemaName: string, isExcluded: boolean) => {
    if (isExcluded) {
      void toggleSchemaExcluded(schemaName)
      return
    }
    const impact = schemaImpact(schemaName)
    const ok = await confirm({
      title: t('modeling.exclude_schema_title'),
      message: t('modeling.exclude_schema_body', {
        schema: schemaName,
        joins: impact.joins,
        dims: impact.dims,
        metrics: impact.metrics,
      }),
      variant: 'warning',
      confirmLabel: t('modeling.exclude_schema_action'),
    })
    if (!ok || !model) {
      return
    }
    setMessage(null)
    await postData(`/api/semantic/models/${model.id}/schemas/remove`, { schema: schemaName })
    await refreshModels(model.id)
    await loadSuggestedJoins()
    setMessage(t('modeling.schema_excluded'))
  }

  const loadSuggestedJoins = async () => {
    if (!modelId) {
      return
    }
    const data = await get<SuggestedJoin[]>(`/api/semantic/models/${modelId}/suggested-joins`)
    setSuggestedJoins(data ?? [])
  }

  const tableOptions = tables.map((t) => ({
    value: tableKey(t.schema_name, t.table_name),
    label: `${t.schema_name}.${t.table_name}`,
    hint: t.table_type,
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
    renameDimension,
    renameMetric,
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
      const impact = tableImpact(tbl.schema_name, tbl.table_name)
      return impact.joins > 0 || impact.dims > 0 || impact.metrics > 0
    }).length
  }, [baseKey, includedTables, model, tableImpact])

  const highlightedTables = useMemo(() => {
    if (!highlightJoinId) {
      return null
    }
    const join = joins.find((j) => j.id === highlightJoinId)
    if (!join) {
      return null
    }
    return new Set([
      tableKey(join.from_schema || model?.base_schema || '', join.from_table),
      tableKey(join.to_schema || model?.base_schema || '', join.to_table),
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
        tableKey(join.from_schema || model?.base_schema || '', join.from_table) +
        '::' +
        join.from_column,
      to:
        tableKey(join.to_schema || model?.base_schema || '', join.to_table) + '::' + join.to_column,
    }
  }, [highlightJoinId, joins, model])

  if (dsLoading || modelsLoading || (modelId ? modelDetailLoading : false)) {
    return <LoadingScreen minHeight="300px" />
  }

  return (
    <div className="modeling-page">
      <section className="modeling-toolbar">
        <div className="form-group">
          <label htmlFor="modeling-datasource">{t('modeling.datasource_label')}</label>
          <Select
            id="modeling-datasource"
            name="datasource"
            value={datasourceId}
            onChange={setDatasourceId}
            placeholder={t('modeling.datasource_placeholder')}
            header={t('modeling.datasource_header')}
            options={datasources.map((d) => ({ value: d.id, label: d.name, hint: d.type }))}
          />
        </div>
        <div className="form-group">
          <label htmlFor="modeling-model">{t('modeling.model_label')}</label>
          <Select
            id="modeling-model"
            name="model"
            value={modelId}
            onChange={setModelId}
            placeholder={
              models.length === 0 ? t('modeling.no_models') : t('modeling.model_placeholder')
            }
            header={t('modeling.model_header')}
            options={models.map((m) => ({ value: m.id, label: m.label || m.name, hint: m.status }))}
          />
        </div>
        <div className="modeling-toolbar-actions">
          <button
            className="btn btn-primary"
            type="button"
            onClick={createModel}
            disabled={!datasourceId || creatingModel}
          >
            {creatingModel ? t('modeling.creating') : t('modeling.create_from_metadata')}
          </button>
          {model && (
            <button
              className="btn btn-secondary"
              type="button"
              onClick={renameModel}
              title={t('modeling.rename_model_button_title')}
            >
              {t('modeling.rename_model_button')}
            </button>
          )}
          {model && (
            <button
              className="btn btn-secondary"
              type="button"
              onClick={publishModel}
              disabled={publishing || model.status === 'published'}
            >
              {publishing
                ? t('modeling.publishing')
                : model.status === 'published'
                  ? t('modeling.published')
                  : t('modeling.publish')}
            </button>
          )}
          {model && (
            <button
              className="btn btn-danger-outline"
              type="button"
              onClick={removeModel}
              title={t('modeling.delete_model_title')}
            >
              {t('common.delete')}
            </button>
          )}
          {model && <ShareButton resourceType="model" resourceID={model.id} />}
        </div>
      </section>

      {isLocked ? (
        <LockedState
          datasourceId={datasourceId}
          datasourceName={datasources.find((d) => d.id === datasourceId)?.name || dsParam}
        />
      ) : (
        <>
          {error && <ErrorAlert error={error} />}
          {message && (
            <div className="semantic-model-setup semantic-model-setup--success">{message}</div>
          )}

          <section
            className={`modeling-shell ${paletteOpen ? '' : 'modeling-shell--palette-closed'} ${editorOpen ? '' : 'modeling-shell--editor-closed'}`}
          >
            <ModelingPalette
              open={paletteOpen}
              onToggle={() => setPaletteOpen((value) => !value)}
              model={model}
              usedTableCount={usedTableCount}
              joins={joins}
              inactiveJoins={inactiveJoins}
              dims={dims}
              inactiveDims={inactiveDims}
              metrics={metrics}
              inactiveMetrics={inactiveMetrics}
              activeTab={activeTab}
              onTabChange={setActiveTab}
              tables={tables}
              includedTables={includedTables}
              excludedSchemas={excludedSchemas}
              tableCards={tableCards}
              tableImpact={tableImpact}
              suggestedJoins={suggestedJoins}
              highlightJoinId={highlightJoinId}
              onHighlightJoin={setHighlightJoinId}
              onSchemaToggle={requestSchemaToggle}
              onRenameTable={renameTable}
              onMakeBase={requestMakeBase}
              onRemoveTable={requestTableRemoval}
              onToggleTableVisibility={toggleTableVisibility}
              onOpenBaseSwap={() => setBaseSwapOpen(true)}
              onDeleteJoin={deleteJoin}
              onAddSuggestedJoin={addSuggestedJoin}
              onReactivateJoin={reactivateJoin}
              onEditDimension={setEditingDimension}
              onEditDimensionValues={setEnumDimension}
              onDeleteDimension={deleteDimension}
              onReactivateDimension={reactivateDimension}
              onOpenAddMetric={() => setAddMetricOpen(true)}
              onEditMetric={setEditingMetric}
              onDeleteMetric={deleteMetric}
              onReactivateMetric={reactivateMetric}
              t={t}
            />

            <ModelingCanvas
              canvas={canvas}
              tableCards={tableCards}
              joins={joins}
              baseKey={baseKey}
              highlightJoinId={highlightJoinId}
              highlightedTables={highlightedTables}
              highlightedColumns={highlightedColumns}
              highlightedJoinColumns={highlightedJoinColumns}
              t={t}
            />
            <JoinEditor
              open={editorOpen}
              onToggle={() => setEditorOpen((value) => !value)}
              joinForm={joinForm}
              onChange={updateJoinForm}
              tableOptions={tableOptions}
              fromColumns={fromColumns}
              toColumns={toColumns}
              fromColumnOptions={fromColumnOptions}
              toColumnOptions={toColumnOptions}
              fromColumnValue={fromColumnValue}
              toColumnValue={toColumnValue}
              selectedFromColumn={selectedFromColumn}
              canSave={canSaveJoin}
              saving={savingJoin}
              loading={loading}
              onSave={saveJoin}
              t={t}
            />
          </section>
        </>
      )}
      {renameTarget && (
        <Modal
          open
          onClose={closeRename}
          className="modal-card--modeling"
          labelledBy="modeling-rename-title"
          title={renameTarget.title}
          subtitle={renameTarget.subtitle}
        >
          <form
            onSubmit={(event) => {
              event.preventDefault()
              void submitRename()
            }}
          >
            <div className="form-group">
              <label htmlFor="modeling-rename-value">{t('modeling.display_name_label')}</label>
              <input
                id="modeling-rename-value"
                autoFocus
                value={renameValue}
                onChange={(event) => setRenameValue(event.target.value)}
                placeholder={renameTarget.current}
                disabled={savingRename}
              />
            </div>
            <div className="modal-actions">
              <button
                className="btn btn-secondary"
                type="button"
                onClick={closeRename}
                disabled={savingRename}
              >
                {t('common.cancel')}
              </button>
              <button
                className="btn btn-primary"
                type="submit"
                disabled={savingRename || !renameValue.trim()}
              >
                {savingRename ? t('common.saving') : t('common.update')}
              </button>
            </div>
          </form>
        </Modal>
      )}
      {baseSwapOpen && model && (
        <BaseSwapModal
          candidateTables={baseSwapCandidates}
          onCancel={() => setBaseSwapOpen(false)}
          onSubmit={swapBaseAndRemoveOld}
          saving={savingBaseSwap}
          t={t}
        />
      )}
      {(addMetricOpen || editingMetric) && model && (
        <AddMetricModal
          model={model}
          includedTables={includedTables}
          columns={columns}
          metric={editingMetric || undefined}
          onClose={() => {
            setAddMetricOpen(false)
            setEditingMetric(null)
          }}
          onCreated={async () => {
            const isEdit = !!editingMetric
            setAddMetricOpen(false)
            setEditingMetric(null)
            await refreshModels(model.id)
            setMessage(isEdit ? t('modeling.metric_updated') : t('modeling.metric_added'))
          }}
          postData={postData}
          putData={putData}
          t={t}
        />
      )}
      {editingDimension && model && (
        <EditDimensionModal
          model={model}
          includedTables={includedTables}
          columns={columns}
          dimension={editingDimension}
          onClose={() => setEditingDimension(null)}
          onSaved={async () => {
            setEditingDimension(null)
            await refreshModels(model.id)
            setMessage(t('modeling.dimension_updated'))
          }}
          putData={putData}
          t={t}
        />
      )}
      {enumDimension && model && (
        <EnumValuesModal
          modelId={model.id}
          dimension={enumDimension}
          onClose={() => setEnumDimension(null)}
          onSaved={async () => {
            setEnumDimension(null)
            await refreshModels(model.id)
            setMessage(t('modeling.dimension_label_updated'))
          }}
          putData={putData}
          t={t}
        />
      )}
    </div>
  )
}
