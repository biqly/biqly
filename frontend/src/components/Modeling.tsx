import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import '../styles/modeling.css'
import { useApi } from '../hooks/useApi'
import { useQueryParam } from '../hooks/useQueryParam'
import { useT } from '../i18n'
import type { Datasource } from '../types/metadata'
import type {
  ColumnRow,
  GenerateSemanticModelResponse,
  SemanticDimension,
  SemanticJoin,
  SemanticMetric,
  SemanticModelDetail,
  SemanticModelSummary,
  TableRow,
} from '../types/semantic'
import { AddMetricModal } from './modeling/AddMetricModal'
import { BaseSwapModal } from './modeling/BaseSwapModal'
import { ModelingCanvas } from './modeling/ModelingCanvas'
import { useModelingCanvas } from './modeling/useModelingCanvas'
import type { ConfirmTarget, JoinForm, RenameTarget, SuggestedJoin, Tab } from './modeling/types'
import { publishModelRequest, suggestedJoinToPayload } from './modeling/types'
import {
  buildJoinPayload,
  canSaveJoinForm,
  columnOptions,
  columnSelectOptions,
  defaultJoinForm,
  findColumn,
  columnsAreJoinCompatible,
  formatDataType,
  patchJoinForm,
  tableKey,
} from './modeling/utils'
import { ErrorAlert } from './ui/ErrorAlert'
import { Modal } from './ui/Modal'
import { Select } from './ui/Select'

export default function Modeling() {
  const t = useT()
  const { get, postData, putData, patchData, deleteData, loading, error } = useApi()
  const [dsParam, setDsParam] = useQueryParam('ds')
  const [modelParam, setModelParam] = useQueryParam('model')
  const [datasources, setDatasources] = useState<Datasource[]>([])
  const [datasourceId, setDatasourceId] = useState(dsParam)
  const [tables, setTables] = useState<TableRow[]>([])
  const [columns, setColumns] = useState<ColumnRow[]>([])
  const [models, setModels] = useState<SemanticModelSummary[]>([])
  const [modelId, setModelId] = useState(modelParam)
  const [model, setModel] = useState<SemanticModelDetail | null>(null)
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
  const [renameTarget, setRenameTarget] = useState<RenameTarget | null>(null)
  const [renameValue, setRenameValue] = useState('')
  const [savingRename, setSavingRename] = useState(false)
  const [confirmTarget, setConfirmTarget] = useState<ConfirmTarget | null>(null)
  const [savingConfirm, setSavingConfirm] = useState(false)

  const prevDsRef = useRef(datasourceId)

  useEffect(() => {
    get<Datasource[]>('/api/datasources').then((data) => {
      if (!data) return
      setDatasources(data)
      setDatasourceId((prev) => (prev && data.some((d) => d.id === prev) ? prev : (data[0]?.id ?? '')))
    })
  }, [])

  useEffect(() => {
    setDsParam(datasourceId)
  }, [datasourceId, setDsParam])

  useEffect(() => {
    setModelParam(modelId)
  }, [modelId, setModelParam])

  useEffect(() => {
    if (!datasourceId) return
    const dsChanged = Boolean(prevDsRef.current && prevDsRef.current !== datasourceId)
    prevDsRef.current = datasourceId

    setMessage(null)
    if (dsChanged) {
      setModel(null)
      setModelId('')
    }

    get<TableRow[]>(`/api/datasources/${datasourceId}/tables`).then((data) => setTables(data ?? []))
    get<ColumnRow[]>(`/api/datasources/${datasourceId}/columns`).then((data) => setColumns(data ?? []))
    get<SemanticModelSummary[]>(`/api/semantic/models?datasource_id=${encodeURIComponent(datasourceId)}`).then((data) => {
      const next = data ?? []
      setModels(next)
      setModelId((prev) => {
        if (prev && next.some((m) => m.id === prev)) return prev
        const published = next.find((m) => m.status === 'published')
        return published?.id ?? next[0]?.id ?? ''
      })
    })
  }, [datasourceId])

  useEffect(() => {
    if (!modelId) {
      setModel(null)
      return
    }
    get<SemanticModelDetail>(`/api/semantic/models/${modelId}?include_inactive=true`).then((data) => {
      if (!data) return
      setModel(data)
    })
    get<SuggestedJoin[]>(`/api/semantic/models/${modelId}/suggested-joins`).then((data) => {
      setSuggestedJoins(data ?? [])
    })
  }, [modelId])

  useEffect(() => {
    if (!model || tables.length === 0 || columns.length === 0) return
    setJoinForm(defaultJoinForm(tables, columns, model))
  }, [model, tables, columns])

  const excludedSchemas = useMemo(
    () => new Set(((model as SemanticModelSummary | null)?.excluded_schemas) ?? []),
    [model],
  )

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
      if (!visibilityStorageKey) return
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
    if (model) keys.add(tableKey(model.base_schema, model.base_table))
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
      if (k === baseKey) return true
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

  const columnRefMatchesTable = useCallback(
    (ref: string | undefined | null, schema: string, table: string) => {
      if (!ref) return false
      const r = ref.trim()
      if (!r) return false
      const base = model?.base_schema ?? ''
      if (r.startsWith(`${schema}.${table}.`)) return true
      if (schema === base && r.startsWith(`${table}.`)) return true
      return false
    },
    [model],
  )

  const expressionRefsTable = useCallback(
    (expr: string | undefined | null, schema: string, table: string) => {
      if (!expr) return false
      const e = expr.toLowerCase()
      const tokens = [`${schema}.${table}.`, `"${schema}"."${table}".`]
      const base = model?.base_schema ?? ''
      if (schema === base) tokens.push(`${table}.`, `"${table}".`)
      return tokens.some((tok) => e.includes(tok.toLowerCase()))
    },
    [model],
  )

  const tableImpact = useCallback(
    (schema: string, table: string) => {
      if (!model) return { joins: 0, dims: 0, metrics: 0 }
      const base = model.base_schema
      const joins = (model.joins ?? []).filter((j) => {
        if (j.is_active === false) return false
        const fs = j.from_schema || base
        const ts = j.to_schema || base
        return (fs === schema && j.from_table === table) || (ts === schema && j.to_table === table)
      }).length
      const dims = (model.dimensions ?? []).filter((d) => d.is_active !== false && columnRefMatchesTable(d.column_ref, schema, table)).length
      const metrics = (model.metrics ?? []).filter((m) => m.is_active !== false && expressionRefsTable(m.expression, schema, table)).length
      return { joins, dims, metrics }
    },
    [model, columnRefMatchesTable, expressionRefsTable],
  )

  const columnRefMatchesSchema = useCallback(
    (ref: string | undefined | null, schema: string) => {
      if (!ref) return false
      const r = ref.trim()
      if (!r) return false
      const base = model?.base_schema ?? ''
      if (r.startsWith(`${schema}.`)) return true
      if (schema === base && r.split('.').length === 2) return true
      return false
    },
    [model],
  )

  const expressionRefsSchema = useCallback(
    (expr: string | undefined | null, schema: string) => {
      if (!expr) return false
      const e = expr.toLowerCase()
      const tokens = [`${schema}.`, `"${schema}".`]
      return tokens.some((tok) => e.includes(tok.toLowerCase()))
    },
    [],
  )

  const schemaImpact = useCallback(
    (schema: string) => {
      if (!model) return { joins: 0, dims: 0, metrics: 0 }
      const base = model.base_schema
      const joins = (model.joins ?? []).filter((j) => {
        if (j.is_active === false) return false
        const fs = j.from_schema || base
        const ts = j.to_schema || base
        return fs === schema || ts === schema
      }).length
      const dims = (model.dimensions ?? []).filter((d) => d.is_active !== false && columnRefMatchesSchema(d.column_ref, schema)).length
      const metrics = (model.metrics ?? []).filter((m) => m.is_active !== false && expressionRefsSchema(m.expression, schema)).length
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
      if (!model) return
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
      if (!model) return
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
    (schema: string, table: string) => {
      if (!model) return
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
      setConfirmTarget({
        kind: 'table',
        schema,
        table,
        title: t('modeling.remove_table_title'),
        body: t('modeling.remove_table_body', {
          table,
          joins: impact.joins,
          dims: impact.dims,
          metrics: impact.metrics,
        }),
        action: t('modeling.remove_table_action'),
      })
    },
    [model, tableImpact, toggleTableVisibility, t],
  )

  const canvas = useModelingCanvas(modelId, tableCards, columns, model)

  const highlightedColumns = useMemo(() => {
    const out = new Map<string, Set<string>>()
    for (const j of (model?.joins ?? []).filter((jj) => jj.is_active !== false)) {
      const fk = tableKey(j.from_schema || model?.base_schema || '', j.from_table)
      const tk = tableKey(j.to_schema || model?.base_schema || '', j.to_table)
      if (!out.has(fk)) out.set(fk, new Set())
      if (!out.has(tk)) out.set(tk, new Set())
      out.get(fk)!.add(j.from_column)
      out.get(tk)!.add(j.to_column)
    }
    return out
  }, [model])

  const fromColumns = useMemo(() => columnOptions(columns, joinForm.fromTable), [columns, joinForm.fromTable])
  const allToColumns = useMemo(() => columnOptions(columns, joinForm.toTable), [columns, joinForm.toTable])
  const selectedFromColumn = useMemo(
    () => findColumn(columns, joinForm.fromTable, joinForm.fromColumn),
    [columns, joinForm.fromTable, joinForm.fromColumn],
  )
  const selectedToColumn = useMemo(
    () => findColumn(columns, joinForm.toTable, joinForm.toColumn),
    [columns, joinForm.toTable, joinForm.toColumn],
  )
  const toColumns = useMemo(
    () => (selectedFromColumn ? allToColumns.filter((column) => columnsAreJoinCompatible(selectedFromColumn, column)) : allToColumns),
    [allToColumns, selectedFromColumn],
  )
  const fromColumnOptions = useMemo(() => columnSelectOptions(fromColumns, t), [fromColumns, t])
  const toColumnOptions = useMemo(() => columnSelectOptions(toColumns, t), [toColumns, t])
  const fromColumnValue = fromColumns.some((c) => c.column_name === joinForm.fromColumn) ? joinForm.fromColumn : ''
  const toColumnValue = toColumns.some((c) => c.column_name === joinForm.toColumn) ? joinForm.toColumn : ''
  const canSaveJoin = canSaveJoinForm(model, joinForm, columns)

  const updateJoinForm = (patch: Partial<JoinForm>) => {
    setJoinForm((prev) => patchJoinForm(prev, patch, columns))
  }

  const refreshModels = async (selectedId?: string) => {
    const list = await get<SemanticModelSummary[]>(`/api/semantic/models?datasource_id=${encodeURIComponent(datasourceId)}`)
    if (list) setModels(list)
    const id = selectedId ?? modelId
    if (id) {
      const full = await get<SemanticModelDetail>(`/api/semantic/models/${id}?include_inactive=true`)
      if (full) setModel(full)
    }
  }

  const createModel = async () => {
    if (!datasourceId || creatingModel) return
    setCreatingModel(true)
    setMessage(null)
    try {
      const res = await postData<GenerateSemanticModelResponse>(
        '/api/semantic/models/generate',
        { datasource_id: datasourceId, publish: true },
        { timeout: 180_000 },
      )
      if (!res?.model) return
      setModelId(res.model.id)
      setModel(res.model)
      await refreshModels(res.model.id)
      setMessage(res.published ? t('modeling.created_published') : t('modeling.created_draft'))
    } finally {
      setCreatingModel(false)
    }
  }

  const openRename = (target: RenameTarget) => {
    setRenameTarget(target)
    setRenameValue(target.current)
    setMessage(null)
  }

  const closeRename = () => {
    if (savingRename) return
    setRenameTarget(null)
    setRenameValue('')
  }

  const removeModel = async () => {
    if (!model) return
    const name = model.label || model.name
    setConfirmTarget({
      kind: 'model',
      modelId: model.id,
      title: t('modeling.confirm_delete_model_title'),
      body: t('modeling.confirm_delete_model_body', { name }),
      action: t('common.delete'),
    })
  }

  const renameModel = async () => {
    if (!model) return
    const current = model.label || model.name
    openRename({
      kind: 'model',
      current,
      title: t('modeling.rename_model_title'),
      subtitle: model.name,
    })
  }

  const publishModel = async () => {
    if (!model || publishing) return
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
    if (!model || !canSaveJoin) return
    setSavingJoin(true)
    setMessage(null)
    try {
      await postData<SemanticJoin>(`/api/semantic/models/${model.id}/joins`, buildJoinPayload(joinForm))
      await refreshModels(model.id)
      await loadSuggestedJoins()
      setMessage(t('modeling.relationship_added'))
    } finally {
      setSavingJoin(false)
    }
  }

  const addSuggestedJoin = async (suggestion: SuggestedJoin) => {
    if (!model) return
    setMessage(null)
    try {
      await postData<SemanticJoin>(`/api/semantic/models/${model.id}/joins`, suggestedJoinToPayload(suggestion))
      await refreshModels(model.id)
      await loadSuggestedJoins()
      setMessage(t('modeling.fk_relationship_added'))
    } catch {
      setMessage(t('modeling.relationship_add_failed'))
    }
  }

  const deleteJoin = async (joinId: string) => {
    const join = joins.find((item) => item.id === joinId)
    setConfirmTarget({
      kind: 'join',
      joinId,
      title: t('modeling.delete_join_title'),
      body: join
        ? t('modeling.delete_join_body_named', { name: join.name })
        : t('modeling.delete_join_body_generic'),
      action: t('common.delete'),
    })
  }

  const deleteDimension = async (dimId: string) => {
    const dim = dims.find((item) => item.id === dimId)
    setConfirmTarget({
      kind: 'dimension',
      dimId,
      title: t('modeling.confirm_delete_dimension_title'),
      body: dim
        ? t('modeling.confirm_delete_dimension_body_named', { name: dim.label || dim.name })
        : t('modeling.confirm_delete_dimension_body_generic'),
      action: t('common.delete'),
    })
  }

  const deleteMetric = async (metricId: string) => {
    const metric = metrics.find((item) => item.id === metricId)
    setConfirmTarget({
      kind: 'metric',
      metricId,
      title: t('modeling.confirm_delete_metric_title'),
      body: metric
        ? t('modeling.confirm_delete_metric_body_named', { name: metric.label || metric.name })
        : t('modeling.confirm_delete_metric_body_generic'),
      action: t('common.delete'),
    })
  }

  const reactivateJoin = async (join: SemanticJoin) => {
    if (!model) return
    await putData(`/api/semantic/models/${model.id}/joins/${join.id}`, {
      name: join.name,
      from_schema: join.from_schema ?? '',
      from_table: join.from_table,
      from_column: join.from_column,
      to_schema: join.to_schema ?? '',
      to_table: join.to_table,
      to_column: join.to_column,
      join_type: join.join_type,
      relationship: join.relationship,
      is_active: true,
    })
    await refreshModels(model.id)
    setMessage(t('modeling.reactivate_relationship'))
  }

  const reactivateDimension = async (dim: SemanticDimension) => {
    if (!model) return
    await putData(`/api/semantic/models/${model.id}/dimensions/${dim.id}`, {
      name: dim.name,
      label: dim.label ?? '',
      column_ref: dim.column_ref,
      type: dim.type,
      synonyms: dim.synonyms ?? [],
      description: dim.description ?? '',
      is_active: true,
    })
    await refreshModels(model.id)
    setMessage(t('modeling.reactivate_dimension'))
  }

  const renameTable = async (tbl: TableRow) => {
    const current = tbl.label || tbl.table_name
    openRename({
      kind: 'table',
      current,
      table: tbl,
      title: t('modeling.rename_table_title'),
      subtitle: `${tbl.schema_name}.${tbl.table_name}`,
    })
  }

  const renameDimension = async (dim: SemanticDimension) => {
    if (!model) return
    const current = dim.label || dim.name
    openRename({
      kind: 'dimension',
      current,
      dimension: dim,
      title: t('modeling.rename_dimension_title'),
      subtitle: dim.column_ref,
    })
  }

  const renameMetric = async (metric: SemanticMetric) => {
    if (!model) return
    const current = metric.label || metric.name
    openRename({
      kind: 'metric',
      current,
      metric,
      title: t('modeling.rename_metric_title'),
      subtitle: `${metric.aggregation}(${metric.expression})`,
    })
  }

  const toggleSchemaExcluded = async (schemaName: string) => {
    if (!model) return
    const current = new Set((model as SemanticModelSummary).excluded_schemas ?? [])
    if (current.has(schemaName)) current.delete(schemaName)
    else current.add(schemaName)
    const next = Array.from(current)
    await putData(`/api/semantic/models/${model.id}`, {
      base_schema: model.base_schema,
      base_table: model.base_table,
      excluded_schemas: next,
    })
    await refreshModels(model.id)
    setMessage(t('modeling.schema_filter_updated'))
  }

  const requestSchemaToggle = (schemaName: string, isExcluded: boolean) => {
    if (isExcluded) {
      void toggleSchemaExcluded(schemaName)
      return
    }
    const impact = schemaImpact(schemaName)
    setConfirmTarget({
      kind: 'schema',
      schemaName,
      title: t('modeling.exclude_schema_title'),
      body: t('modeling.exclude_schema_body', {
        schema: schemaName,
        joins: impact.joins,
        dims: impact.dims,
        metrics: impact.metrics,
      }),
      action: t('modeling.exclude_schema_action'),
    })
  }

  const submitRename = async () => {
    if (!renameTarget || savingRename) return
    const trimmed = renameValue.trim()
    if (!trimmed || trimmed === renameTarget.current) {
      closeRename()
      return
    }
    setSavingRename(true)
    try {
      if (renameTarget.kind === 'table') {
        const updated = await patchData<TableRow>(`/api/metadata/tables/${renameTarget.table.id}`, { label: trimmed })
        if (updated) {
          setTables((prev) => prev.map((p) => (p.id === renameTarget.table.id ? updated : p)))
          setMessage(t('modeling.table_label_updated'))
        }
      } else if (renameTarget.kind === 'model' && model) {
        await putData(`/api/semantic/models/${model.id}`, {
          label: trimmed,
          base_schema: model.base_schema,
          base_table: model.base_table,
        })
        await refreshModels(model.id)
        setMessage(t('modeling.model_renamed'))
      } else if (renameTarget.kind === 'dimension' && model) {
        const dim = renameTarget.dimension
        await putData(`/api/semantic/models/${model.id}/dimensions/${dim.id}`, {
          name: dim.name,
          label: trimmed,
          column_ref: dim.column_ref,
          type: dim.type,
          synonyms: dim.synonyms ?? [],
          description: dim.description ?? '',
          is_active: dim.is_active,
        })
        await refreshModels(model.id)
        setMessage(t('modeling.dimension_label_updated'))
      } else if (renameTarget.kind === 'metric' && model) {
        const metric = renameTarget.metric
        await putData(`/api/semantic/models/${model.id}/metrics/${metric.id}`, {
          name: metric.name,
          label: trimmed,
          expression: metric.expression,
          aggregation: metric.aggregation,
          format: metric.format ?? '',
          synonyms: metric.synonyms ?? [],
          description: metric.description ?? '',
          is_active: metric.is_active,
        })
        await refreshModels(model.id)
        setMessage(t('modeling.metric_label_updated'))
      }
      setRenameTarget(null)
      setRenameValue('')
    } finally {
      setSavingRename(false)
    }
  }

  const closeConfirm = () => {
    if (savingConfirm) return
    setConfirmTarget(null)
  }

  const submitConfirm = async () => {
    if (!confirmTarget || savingConfirm) return
    setSavingConfirm(true)
    setMessage(null)
    try {
      if (confirmTarget.kind === 'model') {
        await deleteData(`/api/semantic/models/${confirmTarget.modelId}`)
        const list = await get<SemanticModelSummary[]>(`/api/semantic/models?datasource_id=${encodeURIComponent(datasourceId)}`)
        const next = list ?? []
        setModels(next)
        const nextId = next[0]?.id ?? ''
        setModelId(nextId)
        if (!nextId) setModel(null)
        setMessage(t('modeling.model_deleted'))
      } else if (confirmTarget.kind === 'schema' && model) {
        await postData(`/api/semantic/models/${model.id}/schemas/remove`, {
          schema: confirmTarget.schemaName,
        })
        await refreshModels(model.id)
        await loadSuggestedJoins()
        setMessage(t('modeling.schema_excluded'))
      } else if (model && confirmTarget.kind === 'join') {
        await deleteData(`/api/semantic/models/${model.id}/joins/${confirmTarget.joinId}`)
        await refreshModels(model.id)
        await loadSuggestedJoins()
        setMessage(t('modeling.relationship_deleted'))
      } else if (model && confirmTarget.kind === 'dimension') {
        await deleteData(`/api/semantic/models/${model.id}/dimensions/${confirmTarget.dimId}`)
        await refreshModels(model.id)
        setMessage(t('modeling.dimension_deleted'))
      } else if (model && confirmTarget.kind === 'metric') {
        await deleteData(`/api/semantic/models/${model.id}/metrics/${confirmTarget.metricId}`)
        await refreshModels(model.id)
        setMessage(t('modeling.metric_deleted'))
      } else if (model && confirmTarget.kind === 'table') {
        await postData(`/api/semantic/models/${model.id}/tables/remove`, {
          schema: confirmTarget.schema,
          table: confirmTarget.table,
        })
        await refreshModels(model.id)
        await loadSuggestedJoins()
        setMessage(t('modeling.table_removed'))
      }
      setConfirmTarget(null)
    } finally {
      setSavingConfirm(false)
    }
  }

  const reactivateMetric = async (metric: SemanticMetric) => {
    if (!model) return
    await putData(`/api/semantic/models/${model.id}/metrics/${metric.id}`, {
      name: metric.name,
      label: metric.label ?? '',
      expression: metric.expression,
      aggregation: metric.aggregation,
      format: metric.format ?? '',
      synonyms: metric.synonyms ?? [],
      description: metric.description ?? '',
      is_active: true,
    })
    await refreshModels(model.id)
    setMessage(t('modeling.reactivate_metric'))
  }

  const loadSuggestedJoins = async () => {
    if (!modelId) return
    const data = await get<SuggestedJoin[]>(`/api/semantic/models/${modelId}/suggested-joins`)
    setSuggestedJoins(data ?? [])
  }

  const tableOptions = tables.map((t) => ({
    value: tableKey(t.schema_name, t.table_name),
    label: `${t.schema_name}.${t.table_name}`,
    hint: t.table_type,
  }))

  const allJoins = model?.joins ?? []
  const joins = allJoins.filter((j) => j.is_active !== false)
  const inactiveJoins = allJoins.filter((j) => j.is_active === false)
  const allDims = model?.dimensions ?? []
  const dims = allDims.filter((d) => d.is_active !== false)
  const inactiveDims = allDims.filter((d) => d.is_active === false)
  const allMetrics = model?.metrics ?? []
  const metrics = allMetrics.filter((m) => m.is_active !== false)
  const inactiveMetrics = allMetrics.filter((m) => m.is_active === false)

  const baseKey = model ? tableKey(model.base_schema, model.base_table) : null
  const baseSwapCandidates = useMemo(() => {
    if (!model) return []
    return includedTables.filter(
      (tbl) => !(tbl.schema_name === model.base_schema && tbl.table_name === model.base_table),
    )
  }, [includedTables, model])
  const usedTableCount = useMemo(() => {
    if (!model) return 0
    return includedTables.filter((tbl) => {
      const key = tableKey(tbl.schema_name, tbl.table_name)
      if (key === baseKey) return true
      const impact = tableImpact(tbl.schema_name, tbl.table_name)
      return impact.joins > 0 || impact.dims > 0 || impact.metrics > 0
    }).length
  }, [baseKey, includedTables, model, tableImpact])

  const highlightedTables = useMemo(() => {
    if (!highlightJoinId) return null
    const join = joins.find((j) => j.id === highlightJoinId)
    if (!join) return null
    return new Set([
      tableKey(join.from_schema || model?.base_schema || '', join.from_table),
      tableKey(join.to_schema || model?.base_schema || '', join.to_table),
    ])
  }, [highlightJoinId, joins, model])

  const highlightedJoinColumns = useMemo(() => {
    if (!highlightJoinId) return null
    const join = joins.find((j) => j.id === highlightJoinId)
    if (!join) return null
    return {
      from: tableKey(join.from_schema || model?.base_schema || '', join.from_table) + '::' + join.from_column,
      to: tableKey(join.to_schema || model?.base_schema || '', join.to_table) + '::' + join.to_column,
    }
  }, [highlightJoinId, joins, model])

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
            placeholder={models.length === 0 ? t('modeling.no_models') : t('modeling.model_placeholder')}
            header={t('modeling.model_header')}
            options={models.map((m) => ({ value: m.id, label: m.label || m.name, hint: m.status }))}
          />
        </div>
        <div className="modeling-toolbar-actions">
          <button className="btn btn-primary" type="button" onClick={createModel} disabled={!datasourceId || creatingModel}>
            {creatingModel ? t('modeling.creating') : t('modeling.create_from_metadata')}
          </button>
          {model && (
            <button className="btn btn-secondary" type="button" onClick={renameModel} title={t('modeling.rename_model_button_title')}>
              {t('modeling.rename_model_button')}
            </button>
          )}
          {model && (
            <button className="btn btn-secondary" type="button" onClick={publishModel} disabled={publishing || model.status === 'published'}>
              {publishing ? t('modeling.publishing') : model.status === 'published' ? t('modeling.published') : t('modeling.publish')}
            </button>
          )}
          {model && (
            <button className="btn btn-danger-outline" type="button" onClick={removeModel} title={t('modeling.delete_model_title')}>
              {t('common.delete')}
            </button>
          )}
        </div>
      </section>

      {error && <ErrorAlert error={error} />}
      {message && <div className="semantic-model-setup semantic-model-setup--success">{message}</div>}

      <section
        className={`modeling-shell ${paletteOpen ? '' : 'modeling-shell--palette-closed'} ${editorOpen ? '' : 'modeling-shell--editor-closed'}`}
      >
        <aside className={`modeling-palette ${paletteOpen ? '' : 'modeling-side--collapsed'}`} aria-label={t('modeling.model_summary_aria')}>
          <button
            type="button"
            className="modeling-side-toggle modeling-side-toggle--left"
            onClick={() => setPaletteOpen((v) => !v)}
            title={paletteOpen ? t('modeling.collapse_panel') : t('modeling.expand_panel')}
          >
            {paletteOpen ? '‹' : '›'}
          </button>
          <div className="modeling-side-body">
          <div>
            <span className="modeling-kicker">{t('modeling.semantic_layer')}</span>
            <h2>{model?.label || model?.name || t('modeling.no_model_selected')}</h2>
            <p>{t('modeling.semantic_description')}</p>
          </div>
          <div className="modeling-stat-grid">
            <div>
              <strong>{usedTableCount}</strong>
              <span>{t('modeling.tab_short_tables')}</span>
            </div>
            <div>
              <strong>{joins.length}</strong>
              <span>{t('modeling.tab_short_rel')}</span>
            </div>
            <div>
              <strong>{dims.length}</strong>
              <span>{t('modeling.tab_short_dim')}</span>
            </div>
            <div>
              <strong>{metrics.length}</strong>
              <span>{t('modeling.tab_short_metric')}</span>
            </div>
          </div>

          <div className="modeling-tabs">
            <button className={`modeling-tab ${activeTab === 'tables' ? 'modeling-tab--active' : ''}`} onClick={() => setActiveTab('tables')} title={t('modeling.tables_tab')}>
              {t('modeling.tab_short_tables')}
            </button>
            <button className={`modeling-tab ${activeTab === 'joins' ? 'modeling-tab--active' : ''}`} onClick={() => setActiveTab('joins')} title={t('modeling.joins_tab')}>
              {t('modeling.tab_short_rel')}
            </button>
            <button className={`modeling-tab ${activeTab === 'dimensions' ? 'modeling-tab--active' : ''}`} onClick={() => setActiveTab('dimensions')} title={t('modeling.dimensions_tab')}>
              {t('modeling.tab_short_dim')}
            </button>
            <button className={`modeling-tab ${activeTab === 'metrics' ? 'modeling-tab--active' : ''}`} onClick={() => setActiveTab('metrics')} title={t('modeling.metrics_tab')}>
              {t('modeling.tab_short_metric')}
            </button>
          </div>

          <div className="modeling-tab-content">
            {activeTab === 'tables' && (
              <div className="modeling-join-list">
                <h3>{t('modeling.schemas_heading')}</h3>
                {Array.from(new Set(tables.map((tbl) => tbl.schema_name))).sort().map((schemaName) => {
                  const isExcluded = excludedSchemas.has(schemaName)
                  return (
                    <div className={`modeling-join-pill ${isExcluded ? '' : 'modeling-join-pill--active'}`} key={`schema-${schemaName}`}>
                      <div className="modeling-join-pill-header">
                        <strong>{schemaName}</strong>
                        <button
                          className={isExcluded ? 'modeling-add-btn' : 'modeling-delete-btn'}
                          onClick={() => requestSchemaToggle(schemaName, isExcluded)}
                          title={isExcluded ? t('modeling.include_schema_again_title') : t('modeling.exclude_schema_title_short')}
                        >
                          {isExcluded ? '+' : '×'}
                        </button>
                      </div>
                      <span className="modeling-join-meta">
                        {isExcluded ? t('modeling.schema_excluded_status') : t('modeling.schema_included_status')}
                      </span>
                    </div>
                  )
                })}

                <h3>{t('modeling.datasource_tables_heading')}</h3>
                {tables.length === 0 ? (
                  <p className="modeling-empty">{t('modeling.no_tables_sync')}</p>
                ) : (
                  includedTables
                    .map((tbl) => {
                      const key = tableKey(tbl.schema_name, tbl.table_name)
                      const isOnCanvas = tableCards.some((tc) => tableKey(tc.schema_name, tc.table_name) === key)
                      const isBase = model ? key === tableKey(model.base_schema, model.base_table) : false
                      const impact = model ? tableImpact(tbl.schema_name, tbl.table_name) : { joins: 0, dims: 0, metrics: 0 }
                      const inModel = isBase || impact.joins > 0 || impact.dims > 0 || impact.metrics > 0
                      return (
                        <div className={`modeling-join-pill ${isOnCanvas ? 'modeling-join-pill--active' : ''}`} key={tbl.id}>
                          <div className="modeling-join-pill-header">
                            <strong>
                              {tbl.label || tbl.table_name}
                              {isBase && <span className="modeling-base-badge" title={t('modeling.base_table_label')}> ★</span>}
                            </strong>
                            <span className="modeling-pill-actions">
                              <button className="modeling-rename-btn" onClick={() => renameTable(tbl)} title={t('modeling.edit_display_name_title')}>✎</button>
                              {!isBase && inModel && (
                                <button
                                  className="modeling-rename-btn"
                                  onClick={() => requestMakeBase(tbl.schema_name, tbl.table_name)}
                                  title={t('modeling.make_base_title')}
                                >★</button>
                              )}
                              {isOnCanvas && !isBase && (
                                <button
                                  className="modeling-delete-btn"
                                  onClick={() => requestTableRemoval(tbl.schema_name, tbl.table_name)}
                                  title={inModel ? t('modeling.remove_from_model_title') : t('modeling.hide_from_canvas_title')}
                                >×</button>
                              )}
                              {!isOnCanvas && (
                                <button
                                  className="modeling-add-btn"
                                  onClick={() => toggleTableVisibility(tbl.schema_name, tbl.table_name, true)}
                                  title={t('modeling.show_on_canvas_title')}
                                >+</button>
                              )}
                              {isBase && (
                                <button
                                  className="modeling-delete-btn"
                                  onClick={() => setBaseSwapOpen(true)}
                                  title={t('modeling.change_base_title')}
                                >×</button>
                              )}
                            </span>
                          </div>
                          <span className="modeling-join-meta">{tbl.schema_name}.{tbl.table_name}</span>
                          <span className="modeling-join-meta">{isOnCanvas ? t('modeling.on_canvas') : t('modeling.not_visible')}</span>
                        </div>
                      )
                    })
                )}
              </div>
            )}
            {activeTab === 'joins' && (
              <div className="modeling-join-list">
                <h3>{t('modeling.active_relationships')}</h3>
                {joins.length === 0 ? (
                  <p className="modeling-empty">{t('modeling.no_relationships')}</p>
                ) : (
                  joins.map((join) => {
                    const isActive = highlightJoinId === join.id
                    return (
                      <div
                        className={`modeling-join-pill ${isActive ? 'modeling-join-pill--active' : ''}`}
                        key={join.id}
                        onMouseEnter={() => setHighlightJoinId(join.id)}
                        onMouseLeave={() => setHighlightJoinId(null)}
                      >
                        <div className="modeling-join-pill-header">
                          <strong>{join.name}</strong>
                          <button className="modeling-delete-btn" onClick={() => deleteJoin(join.id)} title={t('modeling.delete_relationship_title')}>×</button>
                        </div>
                        <span>{join.from_table}.{join.from_column} → {join.to_table}.{join.to_column}</span>
                        <span className="modeling-join-meta">{join.join_type} · {join.relationship}</span>
                      </div>
                    )
                  })
                )}
                {suggestedJoins.length > 0 && (
                  <>
                    <h3>{t('modeling.suggested_fk_relationships')}</h3>
                    {suggestedJoins.map((s, idx) => (
                      <div className="modeling-join-pill modeling-join-pill--suggested" key={idx}>
                        <div className="modeling-join-pill-header">
                          <strong>{s.from_table}.{s.from_column} → {s.to_table}.{s.to_column}</strong>
                          <button className="modeling-add-btn" onClick={() => addSuggestedJoin(s)} title={t('common.add')}>+</button>
                        </div>
                      </div>
                    ))}
                  </>
                )}
                {inactiveJoins.length > 0 && (
                  <>
                    <h3>{t('modeling.inactive_joins_heading')}</h3>
                    {inactiveJoins.map((join) => (
                      <div className="modeling-join-pill modeling-join-pill--suggested" key={join.id}>
                        <div className="modeling-join-pill-header">
                          <strong>{join.name}</strong>
                          <button className="modeling-add-btn" onClick={() => reactivateJoin(join)} title={t('modeling.reactivate_title')}>+</button>
                        </div>
                        <span>{join.from_table}.{join.from_column} → {join.to_table}.{join.to_column}</span>
                      </div>
                    ))}
                  </>
                )}
              </div>
            )}

            {activeTab === 'dimensions' && (
              <div className="modeling-join-list">
                <h3>{t('modeling.dimensions_tab')}</h3>
                {dims.length === 0 ? (
                  <p className="modeling-empty">{t('modeling.no_dimensions')}</p>
                ) : (
                  dims.map((dim) => (
                    <div className="modeling-join-pill" key={dim.id}>
                      <div className="modeling-join-pill-header">
                        <strong>{dim.label || dim.name}</strong>
                        <span className="modeling-pill-actions">
                          <button className="modeling-rename-btn" onClick={() => renameDimension(dim)} title={t('modeling.edit_display_name_title')}>✎</button>
                          <button className="modeling-delete-btn" onClick={() => deleteDimension(dim.id)} title={t('modeling.delete_dimension_title')}>×</button>
                        </span>
                      </div>
                      <span>{dim.column_ref}</span>
                      <span className="modeling-join-meta">{dim.type}</span>
                    </div>
                  ))
                )}
                {inactiveDims.length > 0 && (
                  <>
                    <h3>{t('modeling.inactive_dimensions_heading')}</h3>
                    {inactiveDims.map((dim) => (
                      <div className="modeling-join-pill modeling-join-pill--suggested" key={dim.id}>
                        <div className="modeling-join-pill-header">
                          <strong>{dim.label || dim.name}</strong>
                          <button className="modeling-add-btn" onClick={() => reactivateDimension(dim)} title={t('modeling.reactivate_title')}>+</button>
                        </div>
                        <span>{dim.column_ref}</span>
                      </div>
                    ))}
                  </>
                )}
              </div>
            )}

            {activeTab === 'metrics' && (
              <div className="modeling-join-list">
                <div className="modeling-section-header" style={{ justifyContent: 'center' }}>
                  <button className="btn btn-sm btn-primary" type="button" onClick={() => setAddMetricOpen(true)} disabled={!model} style={{ width: '100%' }}>
                    {t('modeling.add_metric_btn')}
                  </button>
                </div>
                {metrics.length === 0 ? (
                  <p className="modeling-empty">{t('modeling.no_metrics')}</p>
                ) : (
                  metrics.map((metric) => (
                    <div className="modeling-join-pill" key={metric.id}>
                      <div className="modeling-join-pill-header">
                        <strong>{metric.label || metric.name}</strong>
                        <span className="modeling-pill-actions">
                          <button className="modeling-rename-btn" onClick={() => renameMetric(metric)} title={t('modeling.edit_display_name_title')}>✎</button>
                          <button className="modeling-delete-btn" onClick={() => deleteMetric(metric.id)} title={t('modeling.delete_metric_title')}>×</button>
                        </span>
                      </div>
                      <span>{metric.expression}</span>
                      <span className="modeling-join-meta">{metric.aggregation}</span>
                    </div>
                  ))
                )}
                {inactiveMetrics.length > 0 && (
                  <>
                    <h3>{t('modeling.inactive_metrics_heading')}</h3>
                    {inactiveMetrics.map((metric) => (
                      <div className="modeling-join-pill modeling-join-pill--suggested" key={metric.id}>
                        <div className="modeling-join-pill-header">
                          <strong>{metric.label || metric.name}</strong>
                          <button className="modeling-add-btn" onClick={() => reactivateMetric(metric)} title={t('modeling.reactivate_title')}>+</button>
                        </div>
                        <span>{metric.expression}</span>
                      </div>
                    ))}
                  </>
                )}
              </div>
            )}
          </div>
          </div>
        </aside>

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
        <aside className={`modeling-editor ${editorOpen ? '' : 'modeling-side--collapsed'}`} aria-label={t('modeling.relationship_editor_aria')}>
          <button
            type="button"
            className="modeling-side-toggle modeling-side-toggle--right"
            onClick={() => setEditorOpen((v) => !v)}
            title={editorOpen ? t('modeling.collapse_panel') : t('modeling.expand_panel')}
          >
            {editorOpen ? '›' : '‹'}
          </button>
          <div className="modeling-side-body">
          <div>
            <span className="modeling-kicker">{t('modeling.manual_relationship')}</span>
            <h2>{t('modeling.manual_title')}</h2>
            <p>{t('modeling.manual_desc')}</p>
          </div>
          <div className="form-group">
            <label>{t('modeling.source_table')}</label>
            <Select name="fromTable" value={joinForm.fromTable} onChange={(value) => updateJoinForm({ fromTable: value })} placeholder={t('modeling.table_placeholder')} header={t('modeling.source_table')} options={tableOptions} />
          </div>
          <div className="form-group">
            <label>{t('modeling.source_column')}</label>
            <Select
              name="fromColumn"
              value={fromColumnValue}
              onChange={(value) => updateJoinForm({ fromColumn: value })}
              placeholder={fromColumns.length === 0 ? t('modeling.no_columns') : t('modeling.column_placeholder')}
              header={t('modeling.source_column')}
              options={fromColumnOptions}
              disabled={!joinForm.fromTable || fromColumns.length === 0}
            />
          </div>
          <div className="form-group">
            <label>{t('modeling.target_table')}</label>
            <Select name="toTable" value={joinForm.toTable} onChange={(value) => updateJoinForm({ toTable: value })} placeholder={t('modeling.table_placeholder')} header={t('modeling.target_table')} options={tableOptions} />
          </div>
          <div className="form-group">
            <label>{t('modeling.target_column')}</label>
            <Select
              name="toColumn"
              value={toColumnValue}
              onChange={(value) => updateJoinForm({ toColumn: value })}
              placeholder={toColumns.length === 0 ? t('modeling.no_compatible_columns') : t('modeling.column_placeholder')}
              header={t('modeling.target_column')}
              options={toColumnOptions}
              disabled={!joinForm.toTable || toColumns.length === 0}
            />
            {selectedFromColumn && (
              <small className="modeling-type-hint">
                {t('modeling.compatible_columns_hint', { type: formatDataType(t, selectedFromColumn.data_type) })}
              </small>
            )}
          </div>
          <div className="modeling-editor-grid">
            <div className="form-group">
              <label>{t('modeling.join_type_label')}</label>
              <select value={joinForm.joinType} onChange={(event) => updateJoinForm({ joinType: event.target.value as JoinForm['joinType'] })}>
                <option value="LEFT">LEFT</option>
                <option value="INNER">INNER</option>
                <option value="RIGHT">RIGHT</option>
              </select>
            </div>
            <div className="form-group">
              <label>{t('modeling.cardinality')}</label>
              <select value={joinForm.relationship} onChange={(event) => updateJoinForm({ relationship: event.target.value as JoinForm['relationship'] })}>
                <option value="many_to_one">many_to_one</option>
                <option value="one_to_many">one_to_many</option>
                <option value="one_to_one">one_to_one</option>
                <option value="many_to_many">many_to_many</option>
              </select>
            </div>
          </div>
          <button className="btn btn-primary" type="button" onClick={saveJoin} disabled={!canSaveJoin || savingJoin || loading}>
            {savingJoin ? t('common.saving') : t('modeling.add_relationship')}
          </button>
          </div>
        </aside>
      </section>
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
              <button className="btn btn-secondary" type="button" onClick={closeRename} disabled={savingRename}>
                {t('common.cancel')}
              </button>
              <button className="btn btn-primary" type="submit" disabled={savingRename || !renameValue.trim()}>
                {savingRename ? t('common.saving') : t('common.update')}
              </button>
            </div>
          </form>
        </Modal>
      )}
      {confirmTarget && (
        <Modal
          open
          onClose={closeConfirm}
          className="modal-card--modeling"
          labelledBy="modeling-confirm-title"
          title={confirmTarget.title}
        >
          <p className="modeling-dialog-copy">{confirmTarget.body}</p>
          <div className="modal-actions">
            <button className="btn btn-secondary" type="button" onClick={closeConfirm} disabled={savingConfirm}>
              {t('common.cancel')}
            </button>
            <button className="btn btn-danger" type="button" onClick={() => void submitConfirm()} disabled={savingConfirm}>
              {savingConfirm ? t('common.saving') : confirmTarget.action}
            </button>
          </div>
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
      {addMetricOpen && model && (
        <AddMetricModal
          model={model}
          includedTables={includedTables}
          columns={columns}
          onClose={() => setAddMetricOpen(false)}
          onCreated={async () => {
            setAddMetricOpen(false)
            await refreshModels(model.id)
            setMessage(t('modeling.metric_added'))
          }}
          postData={postData}
          t={t}
        />
      )}
    </div>
  )
}
