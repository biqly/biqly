import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react'
import { useApi } from '../hooks/useApi'
import { useT, type TranslationKey } from '../i18n'
import type { Datasource } from '../types/metadata'
import { ErrorAlert } from './ui/ErrorAlert'
import { Select } from './ui/Select'

interface TableRow {
  id: string
  schema_name: string
  table_name: string
  table_type: string
  description: string | null
  label: string | null
}

interface ColumnRow {
  id: string
  schema_name: string
  table_name: string
  column_name: string
  data_type: string
  nullable: boolean
  description: string | null
  is_primary_key: boolean
  is_foreign_key: boolean
  referenced_schema?: string | null
  referenced_table: string | null
  referenced_column: string | null
}

interface SemanticModelSummary {
  id: string
  datasource_id: string
  name: string
  label?: string | null
  base_schema: string
  base_table: string
  status: string
  excluded_schemas?: string[]
}

interface SemanticDimension {
  id: string
  name: string
  label?: string | null
  column_ref: string
  type: string
  synonyms?: string[]
  description?: string | null
  is_active: boolean
}

interface SemanticMetric {
  id: string
  name: string
  label?: string | null
  expression: string
  aggregation: string
  format?: string | null
  synonyms?: string[]
  description?: string | null
  is_active: boolean
}

interface SemanticJoin {
  id: string
  name: string
  from_schema?: string
  from_table: string
  from_column: string
  to_schema?: string
  to_table: string
  to_column: string
  join_type: string
  relationship: string
  is_active?: boolean
}

interface SemanticModelDetail extends SemanticModelSummary {
  dimensions?: SemanticDimension[]
  metrics?: SemanticMetric[]
  joins?: SemanticJoin[]
}

interface GenerateSemanticModelResponse {
  model: SemanticModelDetail
  published: boolean
}

interface SuggestedJoin {
  from_schema: string
  from_table: string
  from_column: string
  to_schema: string
  to_table: string
  to_column: string
  name: string
}

interface JoinForm {
  fromTable: string
  fromColumn: string
  toTable: string
  toColumn: string
  joinType: 'LEFT' | 'INNER' | 'RIGHT'
  relationship: 'many_to_one' | 'one_to_many' | 'one_to_one' | 'many_to_many'
}

const CARD_WIDTH = 280
const HEADER_HEIGHT = 61.8
const ROW_HEIGHT = 25.8
const CARD_PAD_Y = 5.6
const COL_LIMIT = 10
const GRID_X = 340
const GRID_Y = 72
const ORIGIN_X = 40
const ORIGIN_Y = 40
const MIN_SCALE = 0.3
const MAX_SCALE = 2.5
const LAYOUT_COLS = 4

const cardHeight = (count: number) => HEADER_HEIGHT + count * ROW_HEIGHT + CARD_PAD_Y * 2
const rowCenterY = (idx: number) => HEADER_HEIGHT + CARD_PAD_Y + idx * ROW_HEIGHT + ROW_HEIGHT / 2

function tableKey(schema: string, table: string) {
  return `${schema}.${table}`
}

function splitTableKey(key: string) {
  const idx = key.indexOf('.')
  if (idx === -1) return { schema: '', table: key }
  return { schema: key.slice(0, idx), table: key.slice(idx + 1) }
}

function compareColumns(a: ColumnRow, b: ColumnRow, linked?: Set<string>) {
  const pk = Number(b.is_primary_key) - Number(a.is_primary_key)
  if (pk !== 0) return pk
  const fk = Number(b.is_foreign_key) - Number(a.is_foreign_key)
  if (fk !== 0) return fk
  if (linked) {
    const joinLinked = Number(linked.has(b.column_name)) - Number(linked.has(a.column_name))
    if (joinLinked !== 0) return joinLinked
  }
  return a.column_name.localeCompare(b.column_name)
}

function columnOptions(columns: ColumnRow[], tableRef: string) {
  const { schema, table } = splitTableKey(tableRef)
  return columns
    .filter((c) => c.schema_name === schema && c.table_name === table)
    .sort((a, b) => compareColumns(a, b))
}

const DATA_TYPE_LABEL_KEYS: Record<string, TranslationKey> = {
  'timestamp with time zone': 'modeling.data_type_timestamptz',
  text: 'modeling.data_type_text',
  uuid: 'modeling.data_type_uuid',
  'user-defined': 'modeling.data_type_user_defined',
}

function formatDataType(t: (key: TranslationKey) => string, dataType: string) {
  const key = DATA_TYPE_LABEL_KEYS[dataType.toLowerCase().trim()]
  return key ? t(key) : dataType
}

function columnSelectHint(column: ColumnRow, t: (key: TranslationKey) => string) {
  const parts: string[] = []
  if (column.is_primary_key) parts.push(t('modeling.pk_badge'))
  if (column.is_foreign_key) parts.push(t('modeling.fk_badge'))
  parts.push(formatDataType(t, column.data_type))
  return parts.join(' · ')
}

function columnSelectOptions(cols: ColumnRow[], t: (key: TranslationKey) => string) {
  return cols.map((column) => ({
    value: column.column_name,
    label: column.column_name,
    hint: columnSelectHint(column, t),
  }))
}

function normalizeJoinDataType(dataType: string) {
  const type = dataType.toLowerCase().replace(/\(.+\)/, '').replace(/\s+/g, ' ').trim()
  if (['smallint', 'int2', 'integer', 'int', 'int4', 'bigint', 'int8', 'serial', 'serial4', 'bigserial', 'serial8'].includes(type)) {
    return 'integer'
  }
  if (['text', 'character varying', 'varchar', 'character', 'char', 'citext', 'nvarchar', 'nchar', 'string'].includes(type)) {
    return 'text'
  }
  if (['boolean', 'bool'].includes(type)) return 'boolean'
  if (['timestamp', 'timestamp without time zone', 'timestamp with time zone', 'timestamptz', 'datetime'].includes(type)) return 'timestamp'
  if (['date'].includes(type)) return 'date'
  if (['numeric', 'decimal', 'double precision', 'float', 'float4', 'float8', 'real', 'money'].includes(type)) return 'decimal'
  if (['json', 'jsonb'].includes(type)) return 'json'
  return type
}

function columnsAreJoinCompatible(left: ColumnRow | null | undefined, right: ColumnRow | null | undefined) {
  if (!left || !right) return false
  return normalizeJoinDataType(left.data_type) === normalizeJoinDataType(right.data_type)
}

function findColumn(columns: ColumnRow[], tableRef: string, columnName: string) {
  return columnOptions(columns, tableRef).find((column) => column.column_name === columnName) ?? null
}

function firstCompatibleColumnName(columns: ColumnRow[], tableRef: string, sourceColumn: ColumnRow | null) {
  const options = columnOptions(columns, tableRef)
  if (!sourceColumn) return options[0]?.column_name ?? ''
  return options.find((column) => columnsAreJoinCompatible(sourceColumn, column))?.column_name ?? ''
}

function defaultJoinForm(tables: TableRow[], columns: ColumnRow[], model: SemanticModelDetail | null): JoinForm {
  const base = model
    ? tableKey(model.base_schema, model.base_table)
    : tables[0]
      ? tableKey(tables[0].schema_name, tables[0].table_name)
      : ''
  const target = tables.find((t) => tableKey(t.schema_name, t.table_name) !== base)
  const toTable = target ? tableKey(target.schema_name, target.table_name) : base
  const fromColumn = columnOptions(columns, base)[0]?.column_name ?? ''
  const sourceColumn = findColumn(columns, base, fromColumn)
  return {
    fromTable: base,
    fromColumn,
    toTable,
    toColumn: firstCompatibleColumnName(columns, toTable, sourceColumn),
    joinType: 'LEFT',
    relationship: 'many_to_one',
  }
}

function joinName(form: JoinForm) {
  const from = splitTableKey(form.fromTable)
  const to = splitTableKey(form.toTable)
  return `${from.table}_${form.fromColumn}_to_${to.table}_${form.toColumn}`
    .replace(/[^a-zA-Z0-9_]+/g, '_')
    .toLowerCase()
}

type Tab = 'joins' | 'dimensions' | 'metrics' | 'tables'

type RenameTarget =
  | { kind: 'model'; current: string; title: string; subtitle: string }
  | { kind: 'table'; current: string; table: TableRow; title: string; subtitle: string }
  | { kind: 'dimension'; current: string; dimension: SemanticDimension; title: string; subtitle: string }
  | { kind: 'metric'; current: string; metric: SemanticMetric; title: string; subtitle: string }

type ConfirmTarget =
  | { kind: 'model'; modelId: string; title: string; body: string; action: string }
  | { kind: 'schema'; schemaName: string; title: string; body: string; action: string }
  | { kind: 'join'; joinId: string; title: string; body: string; action: string }
  | { kind: 'dimension'; dimId: string; title: string; body: string; action: string }
  | { kind: 'metric'; metricId: string; title: string; body: string; action: string }

interface Pt {
  x: number
  y: number
}

interface Viewport {
  scale: number
  tx: number
  ty: number
}

export default function Modeling() {
  const t = useT()
  const { get, postData, putData, patchData, deleteData, loading, error } = useApi()
  const [datasources, setDatasources] = useState<Datasource[]>([])
  const [datasourceId, setDatasourceId] = useState('')
  const [tables, setTables] = useState<TableRow[]>([])
  const [columns, setColumns] = useState<ColumnRow[]>([])
  const [models, setModels] = useState<SemanticModelSummary[]>([])
  const [modelId, setModelId] = useState('')
  const [model, setModel] = useState<SemanticModelDetail | null>(null)
  const [joinForm, setJoinForm] = useState<JoinForm>(() => defaultJoinForm([], [], null))
  const [creatingModel, setCreatingModel] = useState(false)
  const [savingJoin, setSavingJoin] = useState(false)
  const [message, setMessage] = useState<string | null>(null)
  const [activeTab, setActiveTab] = useState<Tab>('joins')
  const [suggestedJoins, setSuggestedJoins] = useState<SuggestedJoin[]>([])
  const [publishing, setPublishing] = useState(false)
  const [positions, setPositions] = useState<Record<string, Pt>>({})
  const [viewport, setViewport] = useState<Viewport>({ scale: 1, tx: 0, ty: 0 })
  const [highlightJoinId, setHighlightJoinId] = useState<string | null>(null)
  const [paletteOpen, setPaletteOpen] = useState(true)
  const [editorOpen, setEditorOpen] = useState(true)
  const [renameTarget, setRenameTarget] = useState<RenameTarget | null>(null)
  const [renameValue, setRenameValue] = useState('')
  const [savingRename, setSavingRename] = useState(false)
  const [confirmTarget, setConfirmTarget] = useState<ConfirmTarget | null>(null)
  const [savingConfirm, setSavingConfirm] = useState(false)

  const viewportRef = useRef(viewport)
  viewportRef.current = viewport
  const wrapRef = useRef<HTMLDivElement | null>(null)

  useEffect(() => {
    get<Datasource[]>('/api/datasources').then((data) => {
      if (!data) return
      setDatasources(data)
      setDatasourceId((prev) => (prev && data.some((d) => d.id === prev) ? prev : (data[0]?.id ?? '')))
    })
  }, [])

  useEffect(() => {
    if (!datasourceId) return
    setMessage(null)
    setModel(null)
    setModelId('')
    get<TableRow[]>(`/api/datasources/${datasourceId}/tables`).then((data) => setTables(data ?? []))
    get<ColumnRow[]>(`/api/datasources/${datasourceId}/columns`).then((data) => setColumns(data ?? []))
    get<SemanticModelSummary[]>(`/api/semantic/models?datasource_id=${encodeURIComponent(datasourceId)}`).then((data) => {
      const next = data ?? []
      setModels(next)
      const published = next.find((m) => m.status === 'published')
      setModelId(published?.id ?? next[0]?.id ?? '')
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
      setJoinForm(defaultJoinForm(tables, columns, data))
    })
    get<SuggestedJoin[]>(`/api/semantic/models/${modelId}/suggested-joins`).then((data) => {
      setSuggestedJoins(data ?? [])
    })
  }, [modelId, tables.length, columns.length])

  const excludedSchemas = useMemo(
    () => new Set(((model as SemanticModelSummary | null)?.excluded_schemas) ?? []),
    [model],
  )

  const includedTables = useMemo(
    () => tables.filter((t) => !excludedSchemas.has(t.schema_name)),
    [tables, excludedSchemas],
  )

  const tableCards = useMemo(() => {
    const keys = new Set<string>()
    if (model) keys.add(tableKey(model.base_schema, model.base_table))
    for (const join of (model?.joins ?? []).filter((j) => j.is_active !== false)) {
      keys.add(tableKey(join.from_schema || model?.base_schema || '', join.from_table))
      keys.add(tableKey(join.to_schema || model?.base_schema || '', join.to_table))
    }
    const preferred = includedTables.filter((t) => keys.has(tableKey(t.schema_name, t.table_name)))
    const rest = includedTables.filter((t) => !keys.has(tableKey(t.schema_name, t.table_name))).slice(0, Math.max(0, 9 - preferred.length))
    return [...preferred, ...rest]
  }, [model, includedTables])

  const cardLayouts = useMemo(() => {
    const joinColumns = new Map<string, Set<string>>()
    for (const join of (model?.joins ?? []).filter((j) => j.is_active !== false)) {
      const fromKey = tableKey(join.from_schema || model?.base_schema || '', join.from_table)
      const toKey = tableKey(join.to_schema || model?.base_schema || '', join.to_table)
      if (!joinColumns.has(fromKey)) joinColumns.set(fromKey, new Set())
      if (!joinColumns.has(toKey)) joinColumns.set(toKey, new Set())
      joinColumns.get(fromKey)!.add(join.from_column)
      joinColumns.get(toKey)!.add(join.to_column)
    }
    const out = new Map<
      string,
      {
        columnsShown: ColumnRow[]
        columnIndex: Map<string, number>
        height: number
        hiddenCount: number
      }
    >()
    for (const t of tableCards) {
      const key = tableKey(t.schema_name, t.table_name)
      const linked = joinColumns.get(key) ?? new Set<string>()
      const allCols = columnOptions(columns, key)
      const cols = [...allCols].sort((a, b) => compareColumns(a, b, linked)).slice(0, COL_LIMIT)
      const idx = new Map<string, number>()
      cols.forEach((c, i) => idx.set(c.column_name, i))
      const hidden = Math.max(0, allCols.length - cols.length)
      const rowCount = cols.length + (hidden > 0 ? 1 : 0)
      out.set(key, { columnsShown: cols, columnIndex: idx, height: cardHeight(rowCount), hiddenCount: hidden })
    }
    return out
  }, [tableCards, columns, model])

  useEffect(() => {
    setPositions({})
    setViewport({ scale: 1, tx: 0, ty: 0 })
  }, [modelId])

  useEffect(() => {
    setPositions(() => {
      const next: Record<string, Pt> = {}
      const colCursors: number[] = Array.from({ length: LAYOUT_COLS }, () => ORIGIN_Y)
      tableCards.forEach((t, i) => {
        const key = tableKey(t.schema_name, t.table_name)
        const layout = cardLayouts.get(key)
        const h = layout?.height ?? HEADER_HEIGHT + 4 * ROW_HEIGHT
        const col = i % LAYOUT_COLS
        const cursorY = colCursors[col] ?? ORIGIN_Y
        next[key] = { x: ORIGIN_X + col * GRID_X, y: cursorY }
        colCursors[col] = Math.max(cursorY, next[key]!.y + h + GRID_Y)
      })
      return next
    })
  }, [tableCards, cardLayouts])

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

  const canvasBounds = useMemo(() => {
    let maxX = 1, maxY = 1
    for (const t of tableCards) {
      const key = tableKey(t.schema_name, t.table_name)
      const pos = positions[key]
      const layout = cardLayouts.get(key)
      if (!pos || !layout) continue
      maxX = Math.max(maxX, pos.x + CARD_WIDTH)
      maxY = Math.max(maxY, pos.y + layout.height)
    }
    return { width: maxX + ORIGIN_X * 2, height: maxY + ORIGIN_Y * 2 }
  }, [tableCards, positions, cardLayouts])

  const onCardDragStart = useCallback(
    (key: string) => (event: React.MouseEvent) => {
      if (event.button !== 0) return
      const target = event.target as HTMLElement
      if (target.closest('button')) return
      event.preventDefault()
      event.stopPropagation()
      const startX = event.clientX
      const startY = event.clientY
      const startPos = positions[key] ?? { x: 0, y: 0 }
      const onMove = (ev: MouseEvent) => {
        const scale = viewportRef.current.scale
        const dx = (ev.clientX - startX) / scale
        const dy = (ev.clientY - startY) / scale
        setPositions((prev) => ({
          ...prev,
          [key]: { x: Math.max(0, startPos.x + dx), y: Math.max(0, startPos.y + dy) },
        }))
      }
      const onUp = () => {
        window.removeEventListener('mousemove', onMove)
        window.removeEventListener('mouseup', onUp)
        document.body.classList.remove('modeling-grabbing')
      }
      document.body.classList.add('modeling-grabbing')
      window.addEventListener('mousemove', onMove)
      window.addEventListener('mouseup', onUp)
    },
    [positions],
  )

  const onCanvasMouseDown = useCallback(
    (event: React.MouseEvent) => {
      const target = event.target as HTMLElement
      if (target.closest('.modeling-table-card')) return
      if (event.button !== 0) return
      event.preventDefault()
      const startX = event.clientX
      const startY = event.clientY
      const startVP = viewportRef.current
      const onMove = (ev: MouseEvent) => {
        setViewport({ ...startVP, tx: startVP.tx + (ev.clientX - startX), ty: startVP.ty + (ev.clientY - startY) })
      }
      const onUp = () => {
        window.removeEventListener('mousemove', onMove)
        window.removeEventListener('mouseup', onUp)
        document.body.classList.remove('modeling-panning')
      }
      document.body.classList.add('modeling-panning')
      window.addEventListener('mousemove', onMove)
      window.addEventListener('mouseup', onUp)
    },
    [],
  )

  useLayoutEffect(() => {
    const node = wrapRef.current
    if (!node) return
    const onWheel = (ev: WheelEvent) => {
      if (!ev.ctrlKey && !ev.metaKey && Math.abs(ev.deltaX) > Math.abs(ev.deltaY)) return
      ev.preventDefault()
      const rect = node.getBoundingClientRect()
      const cx = ev.clientX - rect.left
      const cy = ev.clientY - rect.top
      setViewport((vp) => {
        const factor = ev.deltaY < 0 ? 1.12 : 1 / 1.12
        const newScale = Math.min(MAX_SCALE, Math.max(MIN_SCALE, vp.scale * factor))
        const k = newScale / vp.scale
        return { scale: newScale, tx: cx - k * (cx - vp.tx), ty: cy - k * (cy - vp.ty) }
      })
    }
    node.addEventListener('wheel', onWheel, { passive: false })
    return () => node.removeEventListener('wheel', onWheel)
  }, [])

  const zoomBy = (factor: number) => {
    const node = wrapRef.current
    setViewport((vp) => {
      const newScale = Math.min(MAX_SCALE, Math.max(MIN_SCALE, vp.scale * factor))
      if (!node) return { ...vp, scale: newScale }
      const rect = node.getBoundingClientRect()
      const cx = rect.width / 2
      const cy = rect.height / 2
      const k = newScale / vp.scale
      return { scale: newScale, tx: cx - k * (cx - vp.tx), ty: cy - k * (cy - vp.ty) }
    })
  }

  const resetView = () => setViewport({ scale: 1, tx: 0, ty: 0 })

  const fitView = () => {
    const node = wrapRef.current
    if (!node) return
    const rect = node.getBoundingClientRect()
    const padding = 40
    const scaleX = (rect.width - padding * 2) / Math.max(1, canvasBounds.width)
    const scaleY = (rect.height - padding * 2) / Math.max(1, canvasBounds.height)
    const scale = Math.min(MAX_SCALE, Math.max(MIN_SCALE, Math.min(scaleX, scaleY, 1)))
    setViewport({ scale, tx: padding, ty: padding })
  }

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
  const hasCompatibleJoinColumns = Boolean(selectedFromColumn && selectedToColumn && columnsAreJoinCompatible(selectedFromColumn, selectedToColumn))
  const canSaveJoin = Boolean(model && joinForm.fromTable && joinForm.fromColumn && joinForm.toTable && joinForm.toColumn && hasCompatibleJoinColumns)

  const updateJoinForm = (patch: Partial<JoinForm>) => {
    setJoinForm((prev) => {
      const next = { ...prev, ...patch }
      if (patch.fromTable) {
        next.fromColumn = columnOptions(columns, patch.fromTable)[0]?.column_name ?? ''
      }
      const sourceColumn = findColumn(columns, next.fromTable, next.fromColumn)
      if (patch.fromTable || patch.fromColumn || patch.toTable) {
        const currentTarget = findColumn(columns, next.toTable, next.toColumn)
        if (!columnsAreJoinCompatible(sourceColumn, currentTarget)) {
          next.toColumn = firstCompatibleColumnName(columns, next.toTable, sourceColumn)
        }
      }
      return next
    })
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
      await postData(`/api/semantic/models/${model.id}/publish`, { published_by: 'modeling-ui' })
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
      const from = splitTableKey(joinForm.fromTable)
      const to = splitTableKey(joinForm.toTable)
      await postData<SemanticJoin>(`/api/semantic/models/${model.id}/joins`, {
        name: joinName(joinForm),
        from_schema: from.schema,
        from_table: from.table,
        from_column: joinForm.fromColumn,
        to_schema: to.schema,
        to_table: to.table,
        to_column: joinForm.toColumn,
        join_type: joinForm.joinType,
        relationship: joinForm.relationship,
      })
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
      await postData<SemanticJoin>(`/api/semantic/models/${model.id}/joins`, {
        name: suggestion.name,
        from_schema: suggestion.from_schema,
        from_table: suggestion.from_table,
        from_column: suggestion.from_column,
        to_schema: suggestion.to_schema,
        to_table: suggestion.to_table,
        to_column: suggestion.to_column,
        join_type: 'LEFT',
        relationship: 'many_to_one',
      })
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
    setConfirmTarget({
      kind: 'schema',
      schemaName,
      title: t('modeling.exclude_schema_title'),
      body: t('modeling.exclude_schema_body', { schema: schemaName }),
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
      } else if (confirmTarget.kind === 'schema') {
        await toggleSchemaExcluded(confirmTarget.schemaName)
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

  const computeJoinPath = (join: SemanticJoin) => {
    const fromKey = tableKey(join.from_schema || model?.base_schema || '', join.from_table)
    const toKey = tableKey(join.to_schema || model?.base_schema || '', join.to_table)
    const fromPos = positions[fromKey]
    const toPos = positions[toKey]
    const fromLayout = cardLayouts.get(fromKey)
    const toLayout = cardLayouts.get(toKey)
    if (!fromPos || !toPos || !fromLayout || !toLayout) return null

    const fromIdx = fromLayout.columnIndex.get(join.from_column)
    const toIdx = toLayout.columnIndex.get(join.to_column)
    if (fromIdx === undefined || toIdx === undefined) return null
    const fromY = fromPos.y + (fromIdx !== undefined ? rowCenterY(fromIdx) : HEADER_HEIGHT / 2)
    const toY = toPos.y + (toIdx !== undefined ? rowCenterY(toIdx) : HEADER_HEIGHT / 2)

    const fromCenterX = fromPos.x + CARD_WIDTH / 2
    const toCenterX = toPos.x + CARD_WIDTH / 2
    const sameColumn = Math.abs(fromCenterX - toCenterX) < CARD_WIDTH * 0.65
    if (sameColumn) {
      const lane = Math.max(fromPos.x, toPos.x) + CARD_WIDTH + 28
      const x1 = fromPos.x + CARD_WIDTH
      const x2 = toPos.x + CARD_WIDTH
      const d = `M ${x1} ${fromY} L ${lane} ${fromY} L ${lane} ${toY} L ${x2} ${toY}`
      return { x1, y1: fromY, x2, y2: toY, d }
    }

    const fromLeft = fromCenterX > toCenterX
    const x1 = fromLeft ? fromPos.x : fromPos.x + CARD_WIDTH
    const x2 = fromLeft ? toPos.x + CARD_WIDTH : toPos.x

    const stub = 18
    const sx = fromLeft ? x1 - stub : x1 + stub
    const tx = fromLeft ? x2 + stub : x2 - stub
    const midX = (sx + tx) / 2
    const d = `M ${x1} ${fromY} L ${sx} ${fromY} L ${midX} ${fromY} L ${midX} ${toY} L ${tx} ${toY} L ${x2} ${toY}`
    return { x1, y1: fromY, x2, y2: toY, d }
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
              Yeniden adlandır
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
              <strong>{includedTables.length}</strong>
              <span>{t('modeling.table_stat_label')}</span>
            </div>
            <div>
              <strong>{dims.length}</strong>
              <span>{t('modeling.dimension_label')}</span>
            </div>
            <div>
              <strong>{metrics.length}</strong>
              <span>{t('modeling.metric_label')}</span>
            </div>
            <div>
              <strong>{joins.length}</strong>
              <span>{t('modeling.join_label')}</span>
            </div>
          </div>

          <div className="modeling-tabs">
            <button className={`modeling-tab ${activeTab === 'tables' ? 'modeling-tab--active' : ''}`} onClick={() => setActiveTab('tables')}>
              {t('modeling.tables_tab')}
            </button>
            <button className={`modeling-tab ${activeTab === 'joins' ? 'modeling-tab--active' : ''}`} onClick={() => setActiveTab('joins')}>
              {t('modeling.joins_tab')}
            </button>
            <button className={`modeling-tab ${activeTab === 'dimensions' ? 'modeling-tab--active' : ''}`} onClick={() => setActiveTab('dimensions')}>
              {t('modeling.dimensions_tab')}
            </button>
            <button className={`modeling-tab ${activeTab === 'metrics' ? 'modeling-tab--active' : ''}`} onClick={() => setActiveTab('metrics')}>
              {t('modeling.metrics_tab')}
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
                      return (
                        <div className={`modeling-join-pill ${isOnCanvas ? 'modeling-join-pill--active' : ''}`} key={tbl.id}>
                          <div className="modeling-join-pill-header">
                            <strong>{tbl.label || tbl.table_name}</strong>
                            <button className="modeling-rename-btn" onClick={() => renameTable(tbl)} title={t('modeling.edit_display_name_title')}>✎</button>
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
                <h3>{t('modeling.metrics_tab')}</h3>
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

        <div className="modeling-canvas-wrap" ref={wrapRef} onMouseDown={onCanvasMouseDown}>
          <div className="modeling-zoom-controls" onMouseDown={(e) => e.stopPropagation()}>
            <button type="button" onClick={() => zoomBy(1.2)} title={t('modeling.zoom_in')}>+</button>
            <button type="button" onClick={() => zoomBy(1 / 1.2)} title={t('modeling.zoom_out')}>−</button>
            <button type="button" onClick={fitView} title={t('modeling.fit_view')}>⤢</button>
            <button type="button" onClick={resetView} title={t('modeling.reset_view')}>1:1</button>
            <span className="modeling-zoom-readout">{Math.round(viewport.scale * 100)}%</span>
          </div>
          <div
            className="modeling-canvas"
            style={{
              width: canvasBounds.width,
              height: canvasBounds.height,
              transform: `translate(${viewport.tx}px, ${viewport.ty}px) scale(${viewport.scale})`,
              transformOrigin: '0 0',
            }}
          >
            <svg className="modeling-lines" width={canvasBounds.width} height={canvasBounds.height} aria-hidden="true">
              <defs>
                <marker id="modeling-arrow" viewBox="0 0 10 10" refX="8" refY="5" markerWidth="6" markerHeight="6" orient="auto-start-reverse">
                  <path d="M 0 0 L 10 5 L 0 10 z" />
                </marker>
              </defs>
              {joins.map((join) => {
                const isHi = highlightJoinId === join.id
                if (highlightJoinId && !isHi) return null
                const path = computeJoinPath(join)
                if (!path) return null
                return (
                  <g key={join.id} className={`modeling-join-line ${isHi ? 'modeling-join-line--hi' : ''}`}>
                    <path d={path.d} markerEnd="url(#modeling-arrow)" />
                    <circle cx={path.x1} cy={path.y1} r={4} />
                    <circle cx={path.x2} cy={path.y2} r={4} />
                  </g>
                )
              })}
            </svg>
            {tableCards.map((table) => {
              const key = tableKey(table.schema_name, table.table_name)
              const pos = positions[key] ?? { x: 0, y: 0 }
              const layout = cardLayouts.get(key)
              if (!layout) return null
              const isBase = baseKey === key
              const isHi = highlightedTables?.has(key) ?? false
              if (highlightedTables && !isHi) return null
              const hiCols = highlightedColumns.get(key)
              const hiddenCount = layout.hiddenCount
              return (
                <article
                  className={`modeling-table-card ${isBase ? 'modeling-table-card--base' : ''} ${isHi ? 'modeling-table-card--hi' : ''}`}
                  key={key}
                  style={{ left: pos.x, top: pos.y, width: CARD_WIDTH, height: layout.height }}
                >
                  <header onMouseDown={onCardDragStart(key)}>
                    <span>{table.schema_name}</span>
                    <strong>{table.table_name}</strong>
                  </header>
                  <ul>
                    {layout.columnsShown.map((column) => {
                      const isJoinCol = hiCols?.has(column.column_name)
                      const colKey = `${key}::${column.column_name}`
                      const isActiveJoinCol = !!highlightedJoinColumns && (highlightedJoinColumns.from === colKey || highlightedJoinColumns.to === colKey)
                      return (
                        <li key={column.id} className={`${isJoinCol ? 'modeling-row--joined' : ''} ${isActiveJoinCol ? 'modeling-row--active' : ''}`}>
                          <span className="modeling-column-name">
                            {column.is_primary_key && <b>{t('modeling.pk_badge')}</b>}
                            {column.is_foreign_key && <b>{t('modeling.fk_badge')}</b>}
                            {column.column_name}
                          </span>
                          <small>{formatDataType(t, column.data_type)}</small>
                        </li>
                      )
                    })}
                    {hiddenCount > 0 && (
                      <li className="modeling-row--more">+{hiddenCount} {t('modeling.more_columns')}</li>
                    )}
                  </ul>
                </article>
              )
            })}
          </div>
        </div>

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
        <div className="modal-backdrop" onClick={closeRename}>
          <form
            className="modal-card modal-card--modeling"
            role="dialog"
            aria-modal="true"
            aria-labelledby="modeling-rename-title"
            onClick={(event) => event.stopPropagation()}
            onSubmit={(event) => {
              event.preventDefault()
              void submitRename()
            }}
          >
            <header className="modal-header">
              <div>
                <h2 id="modeling-rename-title">{renameTarget.title}</h2>
                <p className="modeling-dialog-subtitle">{renameTarget.subtitle}</p>
              </div>
              <button className="modal-close" type="button" onClick={closeRename} aria-label={t('common.close')}>
                ×
              </button>
            </header>
            <div className="modal-body">
              <label className="form-group">
                <span>{t('modeling.display_name_label')}</span>
                <input
                  autoFocus
                  value={renameValue}
                  onChange={(event) => setRenameValue(event.target.value)}
                  placeholder={renameTarget.current}
                  disabled={savingRename}
                />
              </label>
              <div className="modal-actions">
                <button className="btn btn-secondary" type="button" onClick={closeRename} disabled={savingRename}>
                  {t('common.cancel')}
                </button>
                <button className="btn btn-primary" type="submit" disabled={savingRename || !renameValue.trim()}>
                  {savingRename ? t('common.saving') : t('common.update')}
                </button>
              </div>
            </div>
          </form>
        </div>
      )}
      {confirmTarget && (
        <div className="modal-backdrop" onClick={closeConfirm}>
          <div
            className="modal-card modal-card--modeling"
            role="dialog"
            aria-modal="true"
            aria-labelledby="modeling-confirm-title"
            onClick={(event) => event.stopPropagation()}
          >
            <header className="modal-header">
              <h2 id="modeling-confirm-title">{confirmTarget.title}</h2>
              <button className="modal-close" type="button" onClick={closeConfirm} aria-label={t('common.close')}>
                ×
              </button>
            </header>
            <div className="modal-body">
              <p className="modeling-dialog-copy">{confirmTarget.body}</p>
              <div className="modal-actions">
                <button className="btn btn-secondary" type="button" onClick={closeConfirm} disabled={savingConfirm}>
                  {t('common.cancel')}
                </button>
                <button className="btn btn-danger" type="button" onClick={() => void submitConfirm()} disabled={savingConfirm}>
                  {savingConfirm ? t('common.saving') : confirmTarget.action}
                </button>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
