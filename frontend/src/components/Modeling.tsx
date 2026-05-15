import { useEffect, useMemo, useState } from 'react'
import { useApi } from '../hooks/useApi'
import type { Datasource } from '../types/metadata'
import { ErrorAlert } from './ui/ErrorAlert'
import { Select } from './ui/Select'

interface TableRow {
  id: string
  schema_name: string
  table_name: string
  table_type: string
  description: string | null
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

const CARD_WIDTH = 220
const CARD_HEIGHT = 270
const GRID_X = 270
const GRID_Y = 330
const ORIGIN_X = 28
const ORIGIN_Y = 28

function tableKey(schema: string, table: string) {
  return `${schema}.${table}`
}

function splitTableKey(key: string) {
  const idx = key.indexOf('.')
  if (idx === -1) return { schema: '', table: key }
  return { schema: key.slice(0, idx), table: key.slice(idx + 1) }
}

function columnOptions(columns: ColumnRow[], tableRef: string) {
  const { schema, table } = splitTableKey(tableRef)
  return columns
    .filter((c) => c.schema_name === schema && c.table_name === table)
    .sort((a, b) => Number(b.is_primary_key) - Number(a.is_primary_key) || a.column_name.localeCompare(b.column_name))
}

function defaultJoinForm(tables: TableRow[], columns: ColumnRow[], model: SemanticModelDetail | null): JoinForm {
  const base = model ? tableKey(model.base_schema, model.base_table) : tables[0] ? tableKey(tables[0].schema_name, tables[0].table_name) : ''
  const target = tables.find((t) => tableKey(t.schema_name, t.table_name) !== base)
  const toTable = target ? tableKey(target.schema_name, target.table_name) : base
  return {
    fromTable: base,
    fromColumn: columnOptions(columns, base)[0]?.column_name ?? '',
    toTable,
    toColumn: columnOptions(columns, toTable)[0]?.column_name ?? '',
    joinType: 'LEFT',
    relationship: 'many_to_one',
  }
}

function joinName(form: JoinForm) {
  const from = splitTableKey(form.fromTable)
  const to = splitTableKey(form.toTable)
  return `${from.table}_${form.fromColumn}_to_${to.table}_${form.toColumn}`.replace(/[^a-zA-Z0-9_]+/g, '_').toLowerCase()
}

type Tab = 'joins' | 'dimensions' | 'metrics'

export default function Modeling() {
  const { get, postData, deleteData, loading, error } = useApi()
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

  useEffect(() => {
    get<Datasource[]>('/api/datasources').then((data) => {
      if (!data) return
      setDatasources(data)
      setDatasourceId((prev) => (prev && data.some((d) => d.id === prev) ? prev : data[0]?.id ?? ''))
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
    get<SemanticModelDetail>(`/api/semantic/models/${modelId}`).then((data) => {
      if (!data) return
      setModel(data)
      setJoinForm(defaultJoinForm(tables, columns, data))
    })
    get<SuggestedJoin[]>(`/api/semantic/models/${modelId}/suggested-joins`).then((data) => {
      setSuggestedJoins(data ?? [])
    })
  }, [modelId, tables.length, columns.length])

  const tableCards = useMemo(() => {
    const keys = new Set<string>()
    if (model) keys.add(tableKey(model.base_schema, model.base_table))
    for (const join of model?.joins ?? []) {
      keys.add(tableKey(join.from_schema || model?.base_schema || '', join.from_table))
      keys.add(tableKey(join.to_schema || model?.base_schema || '', join.to_table))
    }
    const preferred = tables.filter((t) => keys.has(tableKey(t.schema_name, t.table_name)))
    const rest = tables.filter((t) => !keys.has(tableKey(t.schema_name, t.table_name))).slice(0, Math.max(0, 12 - preferred.length))
    return [...preferred, ...rest]
  }, [model, tables])

  const cardPositions = useMemo(() => {
    const out = new Map<string, { x: number; y: number }>()
    tableCards.forEach((table, idx) => {
      const col = idx % 3
      const row = Math.floor(idx / 3)
      out.set(tableKey(table.schema_name, table.table_name), {
        x: ORIGIN_X + col * GRID_X,
        y: ORIGIN_Y + row * GRID_Y,
      })
    })
    return out
  }, [tableCards])

  const canvasSize = useMemo(() => {
    const rows = Math.max(1, Math.ceil(tableCards.length / 3))
    return {
      width: ORIGIN_X * 2 + 3 * CARD_WIDTH + 2 * (GRID_X - CARD_WIDTH),
      height: ORIGIN_Y * 2 + rows * CARD_HEIGHT + Math.max(0, rows - 1) * (GRID_Y - CARD_HEIGHT),
    }
  }, [tableCards.length])

  const fromColumns = useMemo(() => columnOptions(columns, joinForm.fromTable), [columns, joinForm.fromTable])
  const toColumns = useMemo(() => columnOptions(columns, joinForm.toTable), [columns, joinForm.toTable])

  const updateJoinForm = (patch: Partial<JoinForm>) => {
    setJoinForm((prev) => {
      const next = { ...prev, ...patch }
      if (patch.fromTable) next.fromColumn = columnOptions(columns, patch.fromTable)[0]?.column_name ?? ''
      if (patch.toTable) next.toColumn = columnOptions(columns, patch.toTable)[0]?.column_name ?? ''
      return next
    })
  }

  const refreshModels = async (selectedId?: string) => {
    const list = await get<SemanticModelSummary[]>(`/api/semantic/models?datasource_id=${encodeURIComponent(datasourceId)}`)
    if (list) setModels(list)
    const id = selectedId ?? modelId
    if (id) {
      const full = await get<SemanticModelDetail>(`/api/semantic/models/${id}`)
      if (full) setModel(full)
    }
  }

  const createModel = async () => {
    if (!datasourceId || creatingModel) return
    setCreatingModel(true)
    setMessage(null)
    try {
      const res = await postData<GenerateSemanticModelResponse>('/api/semantic/models/generate', {
        datasource_id: datasourceId,
        publish: true,
      })
      if (!res?.model) return
      setModelId(res.model.id)
      setModel(res.model)
      await refreshModels(res.model.id)
      setMessage(res.published ? 'Model oluşturuldu ve yayınlandı.' : 'Model oluşturuldu; yayın için doğrulama bekliyor.')
    } finally {
      setCreatingModel(false)
    }
  }

  const publishModel = async () => {
    if (!model || publishing) return
    setPublishing(true)
    setMessage(null)
    try {
      await postData(`/api/semantic/models/${model.id}/publish`, { published_by: 'modeling-ui' })
      await refreshModels(model.id)
      setMessage('Model yayınlandı.')
    } finally {
      setPublishing(false)
    }
  }

  const saveJoin = async () => {
    if (!model || !joinForm.fromTable || !joinForm.fromColumn || !joinForm.toTable || !joinForm.toColumn) return
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
      setMessage('İlişki eklendi.')
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
      setMessage('FK ilişkisi eklendi.')
    } catch {
      setMessage('İlişki eklenemedi.')
    }
  }

  const deleteJoin = async (joinId: string) => {
    if (!model) return
    await deleteData(`/api/semantic/models/${model.id}/joins/${joinId}`)
    await refreshModels(model.id)
    await loadSuggestedJoins()
    setMessage('İlişki silindi.')
  }

  const deleteDimension = async (dimId: string) => {
    if (!model) return
    await deleteData(`/api/semantic/models/${model.id}/dimensions/${dimId}`)
    await refreshModels(model.id)
    setMessage('Dimension silindi.')
  }

  const deleteMetric = async (metricId: string) => {
    if (!model) return
    await deleteData(`/api/semantic/models/${model.id}/metrics/${metricId}`)
    await refreshModels(model.id)
    setMessage('Metric silindi.')
  }

  const loadSuggestedJoins = async () => {
    if (!modelId) return
    const data = await get<SuggestedJoin[]>(`/api/semantic/models/${modelId}/suggested-joins`)
    setSuggestedJoins(data ?? [])
  }

  const tableOptions = tables.map((t) => ({ value: tableKey(t.schema_name, t.table_name), label: `${t.schema_name}.${t.table_name}`, hint: t.table_type }))

  const dims = model?.dimensions ?? []
  const metrics = model?.metrics ?? []
  const joins = model?.joins ?? []

  return (
    <div className="modeling-page">
      <section className="modeling-toolbar">
        <div className="form-group">
          <label htmlFor="modeling-datasource">Veri kaynağı</label>
          <Select
            id="modeling-datasource"
            name="datasource"
            value={datasourceId}
            onChange={setDatasourceId}
            placeholder="— seçin —"
            header="Veri kaynakları"
            options={datasources.map((d) => ({ value: d.id, label: d.name, hint: d.type }))}
          />
        </div>
        <div className="form-group">
          <label htmlFor="modeling-model">Semantic model</label>
          <Select
            id="modeling-model"
            name="model"
            value={modelId}
            onChange={setModelId}
            placeholder={models.length === 0 ? 'Bu kaynakta model yok' : '— model seçin —'}
            header="Semantic modeller"
            options={models.map((m) => ({ value: m.id, label: m.label || m.name, hint: `${m.status} · ${m.base_schema}.${m.base_table}` }))}
          />
        </div>
        <div className="modeling-toolbar-actions">
          <button className="btn btn-primary" type="button" onClick={createModel} disabled={!datasourceId || creatingModel}>
            {creatingModel ? 'Oluşturuluyor…' : "Metadata'dan oluştur"}
          </button>
          {model && (
            <button className="btn btn-secondary" type="button" onClick={publishModel} disabled={publishing || model.status === 'published'}>
              {publishing ? 'Yayınlanıyor…' : model.status === 'published' ? 'Yayında' : 'Yayınla'}
            </button>
          )}
        </div>
      </section>

      {error && <ErrorAlert error={error} />}
      {message && <div className="semantic-model-setup semantic-model-setup--success">{message}</div>}

      <section className="modeling-shell">
        <aside className="modeling-palette" aria-label="Model özeti">
          <div>
            <span className="modeling-kicker">Semantic layer</span>
            <h2>{model?.label || model?.name || 'Model seçilmedi'}</h2>
            <p>
              Text-to-SQL burada tablo adlarına tahminle gitmez; entity, metric ve relationship bilgisini bu modelden okur.
            </p>
          </div>
          <div className="modeling-stat-grid">
            <div>
              <strong>{dims.length}</strong>
              <span>Dimension</span>
            </div>
            <div>
              <strong>{metrics.length}</strong>
              <span>Metric</span>
            </div>
            <div>
              <strong>{joins.length}</strong>
              <span>Join</span>
            </div>
          </div>

          <div className="modeling-tabs">
            <button className={`modeling-tab ${activeTab === 'joins' ? 'modeling-tab--active' : ''}`} onClick={() => setActiveTab('joins')}>
              İlişkiler
            </button>
            <button className={`modeling-tab ${activeTab === 'dimensions' ? 'modeling-tab--active' : ''}`} onClick={() => setActiveTab('dimensions')}>
              Dimensions
            </button>
            <button className={`modeling-tab ${activeTab === 'metrics' ? 'modeling-tab--active' : ''}`} onClick={() => setActiveTab('metrics')}>
              Metrics
            </button>
          </div>

          <div className="modeling-tab-content">
            {activeTab === 'joins' && (
              <div className="modeling-join-list">
                <h3>Aktif ilişkiler</h3>
                {joins.length === 0 ? (
                  <p className="modeling-empty">Henüz ilişki yok.</p>
                ) : (
                  joins.map((join) => (
                    <div className="modeling-join-pill" key={join.id}>
                      <div className="modeling-join-pill-header">
                        <strong>{join.name}</strong>
                        <button className="modeling-delete-btn" onClick={() => deleteJoin(join.id)} title="İlişkiyi sil">×</button>
                      </div>
                      <span>{join.from_table}.{join.from_column} → {join.to_table}.{join.to_column}</span>
                      <span className="modeling-join-meta">{join.join_type} · {join.relationship}</span>
                    </div>
                  ))
                )}
                {suggestedJoins.length > 0 && (
                  <>
                    <h3>Önerilen FK ilişkileri</h3>
                    {suggestedJoins.map((s, idx) => (
                      <div className="modeling-join-pill modeling-join-pill--suggested" key={idx}>
                        <div className="modeling-join-pill-header">
                          <strong>{s.from_table}.{s.from_column} → {s.to_table}.{s.to_column}</strong>
                          <button className="modeling-add-btn" onClick={() => addSuggestedJoin(s)} title="Ekle">+</button>
                        </div>
                      </div>
                    ))}
                  </>
                )}
              </div>
            )}

            {activeTab === 'dimensions' && (
              <div className="modeling-join-list">
                <h3>Dimensions</h3>
                {dims.length === 0 ? (
                  <p className="modeling-empty">Dimension yok.</p>
                ) : (
                  dims.map((dim) => (
                    <div className="modeling-join-pill" key={dim.id}>
                      <div className="modeling-join-pill-header">
                        <strong>{dim.name}</strong>
                        <button className="modeling-delete-btn" onClick={() => deleteDimension(dim.id)} title="Dimension sil">×</button>
                      </div>
                      <span>{dim.column_ref}</span>
                      <span className="modeling-join-meta">{dim.type}</span>
                    </div>
                  ))
                )}
              </div>
            )}

            {activeTab === 'metrics' && (
              <div className="modeling-join-list">
                <h3>Metrics</h3>
                {metrics.length === 0 ? (
                  <p className="modeling-empty">Metric yok.</p>
                ) : (
                  metrics.map((metric) => (
                    <div className="modeling-join-pill" key={metric.id}>
                      <div className="modeling-join-pill-header">
                        <strong>{metric.name}</strong>
                        <button className="modeling-delete-btn" onClick={() => deleteMetric(metric.id)} title="Metric sil">×</button>
                      </div>
                      <span>{metric.expression}</span>
                      <span className="modeling-join-meta">{metric.aggregation}</span>
                    </div>
                  ))
                )}
              </div>
            )}
          </div>
        </aside>

        <div className="modeling-canvas-wrap">
          <div className="modeling-canvas" style={{ width: canvasSize.width, height: canvasSize.height }}>
            <svg className="modeling-lines" width={canvasSize.width} height={canvasSize.height} aria-hidden="true">
              {joins.map((join) => {
                const from = cardPositions.get(tableKey(join.from_schema || model?.base_schema || '', join.from_table))
                const to = cardPositions.get(tableKey(join.to_schema || model?.base_schema || '', join.to_table))
                if (!from || !to) return null
                const x1 = from.x + CARD_WIDTH
                const y1 = from.y + 54
                const x2 = to.x
                const y2 = to.y + 54
                const mid = (x1 + x2) / 2
                return (
                  <path
                    key={join.id}
                    d={`M ${x1} ${y1} C ${mid} ${y1}, ${mid} ${y2}, ${x2} ${y2}`}
                    vectorEffect="non-scaling-stroke"
                  />
                )
              })}
            </svg>
            {tableCards.map((table) => {
              const key = tableKey(table.schema_name, table.table_name)
              const pos = cardPositions.get(key) ?? { x: 0, y: 0 }
              const tableColumns = columnOptions(columns, key).slice(0, 9)
              const isBase = model && table.schema_name === model.base_schema && table.table_name === model.base_table
              return (
                <article className={`modeling-table-card ${isBase ? 'modeling-table-card--base' : ''}`} key={key} style={{ left: pos.x, top: pos.y }}>
                  <header>
                    <span>{table.schema_name}</span>
                    <strong>{table.table_name}</strong>
                  </header>
                  <ul>
                    {tableColumns.map((column) => (
                      <li key={column.id}>
                        <span className="modeling-column-name">
                          {column.is_primary_key && <b>PK</b>}
                          {column.is_foreign_key && <b>FK</b>}
                          {column.column_name}
                        </span>
                        <small>{column.data_type}</small>
                      </li>
                    ))}
                  </ul>
                </article>
              )
            })}
          </div>
        </div>

        <aside className="modeling-editor" aria-label="İlişki editörü">
          <div>
            <span className="modeling-kicker">Manual relationship</span>
            <h2>FK yoksa bağlantı kur</h2>
            <p>Bu bağlantı fiziksel veritabanını değiştirmez; Biqly semantic compiler için join yolu tanımlar.</p>
          </div>
          <div className="form-group">
            <label>Kaynak tablo</label>
            <Select name="fromTable" value={joinForm.fromTable} onChange={(value) => updateJoinForm({ fromTable: value })} placeholder="— tablo —" header="Kaynak tablo" options={tableOptions} />
          </div>
          <div className="form-group">
            <label>Kaynak kolon</label>
            <select value={joinForm.fromColumn} onChange={(event) => updateJoinForm({ fromColumn: event.target.value })}>
              {fromColumns.map((column) => <option key={column.id} value={column.column_name}>{column.column_name}</option>)}
            </select>
          </div>
          <div className="form-group">
            <label>Hedef tablo</label>
            <Select name="toTable" value={joinForm.toTable} onChange={(value) => updateJoinForm({ toTable: value })} placeholder="— tablo —" header="Hedef tablo" options={tableOptions} />
          </div>
          <div className="form-group">
            <label>Hedef kolon</label>
            <select value={joinForm.toColumn} onChange={(event) => updateJoinForm({ toColumn: event.target.value })}>
              {toColumns.map((column) => <option key={column.id} value={column.column_name}>{column.column_name}</option>)}
            </select>
          </div>
          <div className="modeling-editor-grid">
            <div className="form-group">
              <label>Join</label>
              <select value={joinForm.joinType} onChange={(event) => updateJoinForm({ joinType: event.target.value as JoinForm['joinType'] })}>
                <option value="LEFT">LEFT</option>
                <option value="INNER">INNER</option>
                <option value="RIGHT">RIGHT</option>
              </select>
            </div>
            <div className="form-group">
              <label>Kardinalite</label>
              <select value={joinForm.relationship} onChange={(event) => updateJoinForm({ relationship: event.target.value as JoinForm['relationship'] })}>
                <option value="many_to_one">many_to_one</option>
                <option value="one_to_many">one_to_many</option>
                <option value="one_to_one">one_to_one</option>
                <option value="many_to_many">many_to_many</option>
              </select>
            </div>
          </div>
          <button className="btn btn-primary" type="button" onClick={saveJoin} disabled={!model || savingJoin || loading}>
            {savingJoin ? 'Kaydediliyor…' : 'İlişki ekle'}
          </button>
        </aside>
      </section>
    </div>
  )
}
