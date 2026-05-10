import { Fragment, useEffect, useMemo, useRef, useState } from 'react'
import { useApi } from '../hooks/useApi'

type BulkStatus = 'pending' | 'running' | 'ok' | 'error' | 'skipped'

interface BulkEntry {
  schema: string
  table: string
  status: BulkStatus
  message?: string
}

/** Table view: active rows first; skipped (e.g. already described) at the bottom. */
const BULK_DISPLAY_ORDER: Record<BulkStatus, number> = {
  running: 0,
  pending: 1,
  error: 2,
  ok: 3,
  skipped: 4,
}

function sortBulkEntriesForDisplay(entries: BulkEntry[]): BulkEntry[] {
  return [...entries]
    .map((entry, queueIndex) => ({ entry, queueIndex }))
    .sort((a, b) => {
      const da = BULK_DISPLAY_ORDER[a.entry.status]
      const db = BULK_DISPLAY_ORDER[b.entry.status]
      if (da !== db) return da - db
      return a.queueIndex - b.queueIndex
    })
    .map(({ entry }) => entry)
}

function BulkStatusBadge({ status }: { status: BulkStatus }) {
  const map: Record<BulkStatus, { label: string; color: string }> = {
    pending: { label: '⏳ pending', color: 'var(--text-secondary)' },
    running: { label: '⚙️ running', color: '#60a5fa' },
    ok: { label: '✓ ok', color: '#4ade80' },
    error: { label: '✗ error', color: '#f87171' },
    skipped: { label: '↷ skipped', color: 'var(--text-secondary)' },
  }
  const s = map[status]
  return <span style={{ color: s.color, fontSize: '0.85rem', whiteSpace: 'nowrap' }}>{s.label}</span>
}

function objectTypeLabel(tableType: string): string {
  const u = tableType.toUpperCase()
  if (u === 'VIEW') return 'Views'
  if (u === 'BASE TABLE') return 'Base tables'
  return tableType
}

function BulkProgressHeader({
  entries,
  running,
  summary,
}: {
  entries: BulkEntry[]
  running: boolean
  summary: { ok: number; error: number; skipped: number } | null
}) {
  const total = entries.length
  const done = entries.filter((e) => e.status === 'ok' || e.status === 'error' || e.status === 'skipped').length
  const ok = entries.filter((e) => e.status === 'ok').length
  const err = entries.filter((e) => e.status === 'error').length
  const skipped = entries.filter((e) => e.status === 'skipped').length
  const current = entries.find((e) => e.status === 'running')
  const pct = total === 0 ? 0 : Math.round((done / total) * 100)
  return (
    <div style={{ marginBottom: '0.5rem', flexShrink: 0 }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '0.8rem', color: 'var(--text-secondary)', marginBottom: '0.25rem', gap: '0.5rem' }}>
        <span>
          {running
            ? <>Processing {done} / {total} — current: <code>{current ? `${current.schema}.${current.table}` : '…'}</code></>
            : summary
              ? <>Done — {summary.ok} ok, {summary.error} error, {summary.skipped} skipped</>
              : <>{done} / {total}</>}
        </span>
        <span>{pct}%</span>
      </div>
      <div style={{ height: '6px', background: 'var(--bg-card)', borderRadius: '4px', overflow: 'hidden', border: '1px solid var(--border)' }}>
        <div
          style={{
            width: `${pct}%`,
            height: '100%',
            background: err > 0 ? 'linear-gradient(90deg, #4ade80, #f87171)' : '#4ade80',
            transition: 'width 0.2s ease',
          }}
        />
      </div>
      <div style={{ display: 'flex', gap: '0.75rem', marginTop: '0.3rem', fontSize: '0.75rem', color: 'var(--text-secondary)' }}>
        <span style={{ color: '#4ade80' }}>✓ {ok}</span>
        <span style={{ color: '#f87171' }}>✗ {err}</span>
        <span>↷ {skipped}</span>
      </div>
    </div>
  )
}

interface Datasource {
  id: string
  name: string
  type: string
}

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
  referenced_table: string | null
  referenced_column: string | null
}

interface DescribeResult {
  schema: string
  table: string
  description: string
  columns: { name: string; description: string }[]
  applied: boolean
  sample_rows: number
  translation_applied?: boolean
  translation_model?: string
  translation_error?: string
}

export default function Metadata() {
  const { get, postData, patchData, loading, error } = useApi()
  const [datasources, setDatasources] = useState<Datasource[]>([])
  const [datasourceId, setDatasourceId] = useState('')
  const [tables, setTables] = useState<TableRow[]>([])
  const [openTableId, setOpenTableId] = useState<string | null>(null)
  const [columns, setColumns] = useState<ColumnRow[]>([])
  const [editing, setEditing] = useState<{ kind: 'table' | 'column'; id: string; value: string } | null>(null)
  const [describeOpen, setDescribeOpen] = useState<TableRow | null>(null)
  const [describeForm, setDescribeForm] = useState({ sample_size: 10, auto_apply: false })
  const [describeResult, setDescribeResult] = useState<DescribeResult | null>(null)
  const [bulkOpen, setBulkOpen] = useState(false)
  const [bulkConfig, setBulkConfig] = useState({ sample_size: 10, skip_existing: true })
  const [bulkRunning, setBulkRunning] = useState(false)
  const [bulkEntries, setBulkEntries] = useState<BulkEntry[]>([])
  const [bulkSummary, setBulkSummary] = useState<{ ok: number; error: number; skipped: number } | null>(null)
  const bulkCancelRef = useRef(false)
  const [tableFilterSchema, setTableFilterSchema] = useState('')
  const [tableFilterType, setTableFilterType] = useState('')
  /** Batch modal: which table_type values to include (all keys set true in openBulk). */
  const [bulkTypeEnabled, setBulkTypeEnabled] = useState<Record<string, boolean>>({})
  const [bulkSchemaRestrict, setBulkSchemaRestrict] = useState(false)
  const [bulkSchemasSelected, setBulkSchemasSelected] = useState<string[]>([])

  useEffect(() => {
    get<Datasource[]>('/api/datasources').then((data) => {
      if (data) {
        setDatasources(data)
        if (data[0]) setDatasourceId(data[0].id)
      }
    })
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    if (!datasourceId) return
    get<TableRow[]>(`/api/datasources/${datasourceId}/tables`).then((data) => setTables(data || []))
    setOpenTableId(null)
    setColumns([])
    setTableFilterSchema('')
    setTableFilterType('')
  }, [datasourceId]) // eslint-disable-line react-hooks/exhaustive-deps

  const schemaOptions = useMemo(
    () => [...new Set(tables.map((t) => t.schema_name))].sort((a, b) => a.localeCompare(b)),
    [tables]
  )
  const typeOptions = useMemo(
    () => [...new Set(tables.map((t) => t.table_type))].sort((a, b) => a.localeCompare(b)),
    [tables]
  )
  const filteredTables = useMemo(
    () =>
      tables.filter((t) => {
        if (tableFilterSchema && t.schema_name !== tableFilterSchema) return false
        if (tableFilterType && t.table_type !== tableFilterType) return false
        return true
      }),
    [tables, tableFilterSchema, tableFilterType]
  )

  useEffect(() => {
    if (!openTableId) return
    if (!filteredTables.some((t) => t.id === openTableId)) {
      setOpenTableId(null)
      setColumns([])
    }
  }, [filteredTables, openTableId])

  const bulkTargetTables = useMemo(() => {
    const restrictTypes = Object.keys(bulkTypeEnabled).length > 0
    return tables.filter((t) => {
      if (restrictTypes && !bulkTypeEnabled[t.table_type]) return false
      if (bulkSchemaRestrict) {
        if (bulkSchemasSelected.length === 0) return false
        if (!bulkSchemasSelected.includes(t.schema_name)) return false
      }
      return true
    })
  }, [tables, bulkTypeEnabled, bulkSchemaRestrict, bulkSchemasSelected])

  const bulkHasObjectType = typeOptions.length === 0 || typeOptions.some((ty) => bulkTypeEnabled[ty])
  const bulkCanStart =
    bulkTargetTables.length > 0 && bulkHasObjectType && (!bulkSchemaRestrict || bulkSchemasSelected.length > 0)

  const bulkEntriesDisplay = useMemo(
    () => (bulkEntries.length > 0 ? sortBulkEntriesForDisplay(bulkEntries) : []),
    [bulkEntries]
  )

  useEffect(() => {
    if (!describeOpen) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setDescribeOpen(null)
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [describeOpen])

  const toggleTable = async (t: TableRow) => {
    if (openTableId === t.id) {
      setOpenTableId(null)
      setColumns([])
      return
    }
    setOpenTableId(t.id)
    const data = await get<ColumnRow[]>(
      `/api/datasources/${datasourceId}/columns?schema=${encodeURIComponent(t.schema_name)}&table=${encodeURIComponent(t.table_name)}`
    )
    setColumns(data || [])
  }

  const saveDescription = async () => {
    if (!editing) return
    const url = editing.kind === 'table' ? `/api/metadata/tables/${editing.id}` : `/api/metadata/columns/${editing.id}`
    const value = editing.value.trim() === '' ? null : editing.value
    const res = await patchData(url, { description: value })
    if (res) {
      if (editing.kind === 'table') {
        setTables(tables.map((t) => (t.id === editing.id ? { ...t, description: value } : t)))
      } else {
        setColumns(columns.map((c) => (c.id === editing.id ? { ...c, description: value } : c)))
      }
    }
    setEditing(null)
  }

  const openDescribe = (t: TableRow) => {
    setDescribeOpen(t)
    setDescribeResult(null)
    setDescribeForm({ sample_size: 10, auto_apply: false })
  }

  const runDescribe = async () => {
    if (!describeOpen) return
    const res = await postData<DescribeResult>('/api/ai/metadata/describe', {
      datasource_id: datasourceId,
      schema: describeOpen.schema_name,
      table: describeOpen.table_name,
      sample_size: describeForm.sample_size,
      auto_apply: describeForm.auto_apply,
    })
    if (res) {
      setDescribeResult(res)
      if (res.applied) {
        // refresh table + columns
        get<TableRow[]>(`/api/datasources/${datasourceId}/tables`).then((d) => setTables(d || []))
        if (openTableId === describeOpen.id) {
          get<ColumnRow[]>(
            `/api/datasources/${datasourceId}/columns?schema=${encodeURIComponent(describeOpen.schema_name)}&table=${encodeURIComponent(describeOpen.table_name)}`
          ).then((d) => setColumns(d || []))
        }
      }
    }
  }

  const openBulk = () => {
    const types = [...new Set(tables.map((t) => t.table_type))].sort((a, b) => a.localeCompare(b))
    setBulkTypeEnabled(Object.fromEntries(types.map((ty) => [ty, true])))
    setBulkSchemaRestrict(false)
    setBulkSchemasSelected([])
    setBulkOpen(true)
    setBulkEntries([])
    setBulkSummary(null)
    setBulkRunning(false)
    bulkCancelRef.current = false
  }

  const closeBulk = () => {
    if (bulkRunning) bulkCancelRef.current = true
    setBulkOpen(false)
  }

  const runBulkDescribe = async () => {
    const targets = bulkTargetTables
    if (!datasourceId || targets.length === 0) return
    bulkCancelRef.current = false
    setBulkRunning(true)
    setBulkSummary(null)

    const queue: BulkEntry[] = targets.map((t) => {
      if (bulkConfig.skip_existing && t.description) {
        return { schema: t.schema_name, table: t.table_name, status: 'skipped', message: 'already described' }
      }
      return { schema: t.schema_name, table: t.table_name, status: 'pending' }
    })
    setBulkEntries(queue)

    let ok = 0
    let errCount = 0
    let skipped = queue.filter((q) => q.status === 'skipped').length

    for (let i = 0; i < targets.length; i++) {
      if (bulkCancelRef.current) break
      const t = targets[i]
      const entry = queue[i]
      if (!t || !entry || entry.status === 'skipped') continue

      const schema = t.schema_name
      const table = t.table_name
      queue[i] = { schema, table, status: 'running' }
      setBulkEntries([...queue])

      try {
        const res = await fetch('/api/ai/metadata/describe', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            datasource_id: datasourceId,
            schema,
            table,
            sample_size: bulkConfig.sample_size,
            auto_apply: true,
          }),
        })
        const text = await res.text()
        const data = text ? JSON.parse(text) : null
        if (!res.ok) {
          queue[i] = { schema, table, status: 'error', message: data?.error || `HTTP ${res.status}` }
          errCount++
        } else {
          const cols = data?.columns?.length ?? 0
          queue[i] = { schema, table, status: 'ok', message: `${cols} columns described` }
          ok++
        }
      } catch (err) {
        queue[i] = { schema, table, status: 'error', message: err instanceof Error ? err.message : 'network error' }
        errCount++
      }
      setBulkEntries([...queue])
    }

    setBulkRunning(false)
    setBulkSummary({ ok, error: errCount, skipped })
    // refresh table list to pick up new descriptions
    const fresh = await get<TableRow[]>(`/api/datasources/${datasourceId}/tables`)
    if (fresh) setTables(fresh)
  }

  const applySuggestion = async (kind: 'table' | 'column', name: string, description: string) => {
    if (!describeOpen) return
    if (kind === 'table') {
      await patchData(`/api/metadata/tables/${describeOpen.id}`, { description })
      setTables(tables.map((t) => (t.id === describeOpen.id ? { ...t, description } : t)))
    } else {
      const col = columns.find((c) => c.column_name === name)
      if (!col) return
      await patchData(`/api/metadata/columns/${col.id}`, { description })
      setColumns(columns.map((c) => (c.id === col.id ? { ...c, description } : c)))
    }
  }

  return (
    <div>
      <div className="card">
        <h2>Metadata Browser</h2>
        <div className="form-group">
          <label>Datasource</label>
          <select value={datasourceId} onChange={(e) => setDatasourceId(e.target.value)}>
            <option value="">— select —</option>
            {datasources.map((d) => (
              <option key={d.id} value={d.id}>{d.name} ({d.type})</option>
            ))}
          </select>
        </div>
        {error && <div className="error">{error}</div>}
      </div>

      {datasourceId && (
        <div className="card">
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '0.75rem', flexWrap: 'wrap' }}>
            <h2 style={{ margin: 0, fontSize: '1.05rem' }}>
              Tables (
              {filteredTables.length}
              {filteredTables.length !== tables.length ? ` / ${tables.length}` : ''})
            </h2>
            {tables.length > 0 && (
              <button type="button" className="btn btn-sm" onClick={openBulk} disabled={bulkRunning}>
                AI metadata generator
              </button>
            )}
          </div>
          {tables.length === 0 && !loading && (
            <p style={{ color: 'var(--text-secondary)' }}>
              No tables. Run <strong>Sync</strong> from the Datasources tab.
            </p>
          )}
          {tables.length > 0 && (
            <div className="metadata-table-filters">
              <div className="form-group metadata-filter-field">
                <label htmlFor="metadata-filter-schema">Schema</label>
                <select
                  id="metadata-filter-schema"
                  value={tableFilterSchema}
                  onChange={(e) => setTableFilterSchema(e.target.value)}
                >
                  <option value="">All schemas</option>
                  {schemaOptions.map((s) => (
                    <option key={s} value={s}>{s}</option>
                  ))}
                </select>
              </div>
              <div className="form-group metadata-filter-field">
                <label htmlFor="metadata-filter-type">Type</label>
                <select
                  id="metadata-filter-type"
                  value={tableFilterType}
                  onChange={(e) => setTableFilterType(e.target.value)}
                >
                  <option value="">All types</option>
                  {typeOptions.map((ty) => (
                    <option key={ty} value={ty}>{ty}</option>
                  ))}
                </select>
              </div>
            </div>
          )}
          <table className="results-table results-table--metadata-list">
            <thead>
              <tr>
                <th>Schema.Table</th>
                <th className="metadata-col-type">Type</th>
                <th>Description</th>
                <th className="actions">Actions</th>
              </tr>
            </thead>
            <tbody>
              {filteredTables.length === 0 && tables.length > 0 && (
                <tr>
                  <td colSpan={4} style={{ color: 'var(--text-secondary)', fontSize: '0.85rem', padding: '0.75rem' }}>
                    No tables match the current filters.
                  </td>
                </tr>
              )}
              {filteredTables.map((t) => (
                <Fragment key={t.id}>
                  <tr>
                    <td>
                      <button
                        type="button"
                        className="icon-btn"
                        aria-expanded={openTableId === t.id}
                        aria-label={`${openTableId === t.id ? 'Collapse' : 'Expand'} ${t.schema_name}.${t.table_name}`}
                        onClick={() => toggleTable(t)}
                      >
                        <span className="chevron">{openTableId === t.id ? '▼' : '▶'}</span>
                        {t.schema_name}.{t.table_name}
                      </button>
                    </td>
                    <td className="metadata-col-type">{t.table_type}</td>
                    <td onDoubleClick={() => setEditing({ kind: 'table', id: t.id, value: t.description ?? '' })}>
                      {editing?.kind === 'table' && editing.id === t.id ? (
                        <input
                          autoFocus
                          value={editing.value}
                          onChange={(e) => setEditing({ ...editing, value: e.target.value })}
                          onBlur={saveDescription}
                          onKeyDown={(e) => { if (e.key === 'Enter') saveDescription(); if (e.key === 'Escape') setEditing(null) }}
                        />
                      ) : (
                        <span style={{ color: t.description ? 'var(--text-primary)' : 'var(--text-secondary)', fontStyle: t.description ? 'normal' : 'italic' }}>
                          {t.description || '(double-click to edit)'}
                        </span>
                      )}
                    </td>
                    <td className="actions">
                      <button type="button" className="btn btn-sm" onClick={() => openDescribe(t)}>
                        🤖 AI Describe
                      </button>
                    </td>
                  </tr>
                  {openTableId === t.id && columns.length > 0 && (
                    <tr>
                      <td colSpan={4} style={{ background: 'var(--bg-card)', padding: 0 }}>
                        <table className="results-table results-table--metadata-list results-table--nested">
                          <thead>
                            <tr>
                              <th>Column</th>
                              <th>Type</th>
                              <th>Flags</th>
                              <th>Description</th>
                            </tr>
                          </thead>
                          <tbody>
                            {columns.map((c) => (
                              <tr key={c.id}>
                                <td>{c.column_name}</td>
                                <td>{c.data_type}{c.nullable ? '' : ' NOT NULL'}</td>
                                <td style={{ fontSize: '0.75rem', color: 'var(--text-secondary)' }}>
                                  {c.is_primary_key && 'PK '}
                                  {c.is_foreign_key && c.referenced_table && `FK→${c.referenced_table}.${c.referenced_column}`}
                                </td>
                                <td onDoubleClick={() => setEditing({ kind: 'column', id: c.id, value: c.description ?? '' })}>
                                  {editing?.kind === 'column' && editing.id === c.id ? (
                                    <input
                                      autoFocus
                                      value={editing.value}
                                      onChange={(e) => setEditing({ ...editing, value: e.target.value })}
                                      onBlur={saveDescription}
                                      onKeyDown={(e) => { if (e.key === 'Enter') saveDescription(); if (e.key === 'Escape') setEditing(null) }}
                                    />
                                  ) : (
                                    <span style={{ color: c.description ? 'var(--text-primary)' : 'var(--text-secondary)', fontStyle: c.description ? 'normal' : 'italic' }}>
                                      {c.description || '(double-click to edit)'}
                                    </span>
                                  )}
                                </td>
                              </tr>
                            ))}
                          </tbody>
                        </table>
                      </td>
                    </tr>
                  )}
                </Fragment>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {bulkOpen && (
        <div
          className="modal-backdrop"
          role="presentation"
          onClick={(e) => { if (e.target === e.currentTarget && !bulkRunning) closeBulk() }}
        >
          <section
            className="modal-card modal-card--bulk-describe"
            role="dialog"
            aria-modal="true"
            aria-labelledby="bulk-metadata-title"
          >
            <header className="modal-header modal-header--compact">
              <div>
                <h2 id="bulk-metadata-title" className="bulk-modal-title">AI metadata generator</h2>
                <p className="bulk-modal-subtitle">Turkish-first LLM descriptions for selected tables &amp; columns</p>
              </div>
              <button
                type="button"
                className="modal-close"
                aria-label="Close"
                onClick={closeBulk}
              >
                ×
              </button>
            </header>
            <div className={`modal-body${bulkEntries.length > 0 ? ' modal-body--scroll' : ''}`}>
              {bulkEntries.length === 0 && !bulkRunning && (
                <>
                  <p className="bulk-lede">
                    Descriptions are inferred from sampled rows and saved to metadata. They are generated in Turkish by default, while keeping useful technical table/column names. Large selections use more tokens and time.
                  </p>
                  <div className="bulk-panel-grid">
                    <fieldset className="bulk-fieldset">
                      <legend className="bulk-legend">Object types</legend>
                      <div className="bulk-pill-row" role="group" aria-label="Object types to include">
                        {typeOptions.map((ty) => (
                          <button
                            key={ty}
                            type="button"
                            className={`bulk-pill${bulkTypeEnabled[ty] === true ? ' bulk-pill--on' : ''}`}
                            aria-pressed={bulkTypeEnabled[ty] === true}
                            onClick={() =>
                              setBulkTypeEnabled((prev) => ({ ...prev, [ty]: !prev[ty] }))
                            }
                          >
                            <span className="bulk-pill-label">{objectTypeLabel(ty)}</span>
                            <span className="bulk-pill-code">{ty}</span>
                          </button>
                        ))}
                      </div>
                      {!bulkHasObjectType && (
                        <p className="bulk-modal-warn">Turn on at least one type.</p>
                      )}
                    </fieldset>
                    <fieldset className="bulk-fieldset">
                      <legend className="bulk-legend">Schemas</legend>
                      <div
                        className="bulk-segmented"
                        role="group"
                        aria-label="Schema scope"
                      >
                        <button
                          type="button"
                          className={!bulkSchemaRestrict ? 'bulk-segmented__btn bulk-segmented__btn--active' : 'bulk-segmented__btn'}
                          onClick={() => {
                            setBulkSchemaRestrict(false)
                            setBulkSchemasSelected([])
                          }}
                        >
                          All schemas
                        </button>
                        <button
                          type="button"
                          className={bulkSchemaRestrict ? 'bulk-segmented__btn bulk-segmented__btn--active' : 'bulk-segmented__btn'}
                          onClick={() => {
                            setBulkSchemaRestrict(true)
                            setBulkSchemasSelected((prev) => (prev.length > 0 ? prev : [...schemaOptions]))
                          }}
                        >
                          Choose…
                        </button>
                      </div>
                      <div
                        className={`bulk-schema-box${bulkSchemaRestrict ? ' bulk-schema-box--active' : ''}`}
                      >
                        {!bulkSchemaRestrict ? (
                          <p className="bulk-schema-placeholder">Every schema in this datasource is included.</p>
                        ) : (
                          <>
                            <select
                              id="bulk-schema-multiselect"
                              className="bulk-schema-multiselect"
                              multiple
                              size={Math.min(8, Math.max(4, schemaOptions.length))}
                              value={bulkSchemasSelected}
                              onChange={(e) =>
                                setBulkSchemasSelected([...e.target.selectedOptions].map((o) => o.value))
                              }
                              aria-label="Schemas to include"
                            >
                              {schemaOptions.map((s) => (
                                <option key={s} value={s}>{s}</option>
                              ))}
                            </select>
                            <div className="bulk-schema-multiselect-tools">
                              <button
                                type="button"
                                className="btn btn-ghost btn-sm"
                                onClick={() => setBulkSchemasSelected([...schemaOptions])}
                              >
                                All
                              </button>
                              <button
                                type="button"
                                className="btn btn-ghost btn-sm"
                                onClick={() => setBulkSchemasSelected([])}
                              >
                                None
                              </button>
                              <span className="bulk-schema-hint">Ctrl/Cmd-click to multi-select</span>
                            </div>
                          </>
                        )}
                      </div>
                    </fieldset>
                  </div>
                  <div className="bulk-options-row">
                    <div className="form-group bulk-opt-field">
                      <label className="bulk-opt-label" htmlFor="bulk-sample-size">Sample rows</label>
                      <input
                        id="bulk-sample-size"
                        type="number"
                        min={1}
                        max={100}
                        className="bulk-opt-input"
                        value={bulkConfig.sample_size}
                        onChange={(e) => setBulkConfig({ ...bulkConfig, sample_size: Number(e.target.value) })}
                      />
                    </div>
                    <label className="bulk-skip-label" htmlFor="bulk-skip-existing">
                      <input
                        id="bulk-skip-existing"
                        type="checkbox"
                        checked={bulkConfig.skip_existing}
                        onChange={(e) => setBulkConfig({ ...bulkConfig, skip_existing: e.target.checked })}
                      />
                      <span>Skip if table already has a description</span>
                    </label>
                  </div>
                  <div className="bulk-scope-footer">
                    <span className="bulk-scope-stat">
                      <strong>{bulkTargetTables.length}</strong> object{bulkTargetTables.length === 1 ? '' : 's'} in scope
                      {bulkTargetTables.length !== tables.length && (
                        <span className="bulk-scope-of"> · {tables.length} total</span>
                      )}
                    </span>
                  </div>
                  <div className="modal-actions">
                    <button type="button" className="btn btn-ghost btn-sm" onClick={closeBulk}>Cancel</button>
                    <button
                      type="button"
                      className="btn btn-sm"
                      onClick={runBulkDescribe}
                      disabled={!bulkCanStart}
                    >
                      Start ({bulkTargetTables.length} in scope)
                    </button>
                  </div>
                </>
              )}

              {bulkEntries.length > 0 && (
                <>
                  <BulkProgressHeader entries={bulkEntries} running={bulkRunning} summary={bulkSummary} />
                  <div className="bulk-describe-scroll">
                    <table className="results-table results-table--dense" style={{ margin: 0 }}>
                      <thead>
                        <tr>
                          <th className="bulk-col-idx">#</th>
                          <th>Schema.Table</th>
                          <th className="bulk-col-status">Status</th>
                          <th>Detail</th>
                        </tr>
                      </thead>
                      <tbody>
                        {bulkEntriesDisplay.map((e, idx) => (
                          <tr key={`${e.schema}.${e.table}`}>
                            <td className="bulk-col-idx">{idx + 1}</td>
                            <td className="bulk-col-name">
                              <code>{e.schema}.{e.table}</code>
                            </td>
                            <td className="bulk-col-status">
                              <BulkStatusBadge status={e.status} />
                            </td>
                            <td className="bulk-col-detail" style={{ color: 'var(--text-secondary)' }}>
                              <span className="bulk-col-detail-inner" title={e.message}>
                                {e.message || (e.status === 'pending' ? '—' : '')}
                              </span>
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                  <div className="modal-actions">
                    {bulkRunning ? (
                      <button
                        type="button"
                        className="btn btn-ghost btn-sm"
                        onClick={() => { bulkCancelRef.current = true }}
                      >
                        Stop after current
                      </button>
                    ) : (
                      <button type="button" className="btn btn-sm" onClick={closeBulk}>Close</button>
                    )}
                  </div>
                </>
              )}
            </div>
          </section>
        </div>
      )}

      {describeOpen && (
        <div
          className="modal-backdrop"
          role="presentation"
          onClick={(e) => { if (e.target === e.currentTarget) setDescribeOpen(null) }}
        >
          <section
            className="modal-card"
            role="dialog"
            aria-modal="true"
            aria-labelledby="describe-title"
          >
            <header className="modal-header">
              <h2 id="describe-title">
                🤖 AI Describe — {describeOpen.schema_name}.{describeOpen.table_name}
              </h2>
              <button
                type="button"
                className="modal-close"
                aria-label="Close AI Describe"
                onClick={() => setDescribeOpen(null)}
              >
                ×
              </button>
            </header>

            <div className="modal-body">
              <p style={{ color: 'var(--text-secondary)', margin: 0 }}>
                Samples N rows from the source DB and asks the LLM to describe the table and each column in Turkish, keeping useful technical names for schema matching.
              </p>

              {!describeResult && (
                <>
                  <div className="modal-form-row">
                    <div className="form-group">
                      <label htmlFor="describe-sample-size">Sample size</label>
                      <input
                        id="describe-sample-size"
                        name="sample_size"
                        type="number"
                        min={1}
                        max={100}
                        value={describeForm.sample_size}
                        onChange={(e) => setDescribeForm({ ...describeForm, sample_size: Number(e.target.value) })}
                      />
                    </div>
                    <div className="form-group">
                      <label>Options</label>
                      <div className="checkbox-row">
                        <input
                          id="auto-apply"
                          name="auto_apply"
                          type="checkbox"
                          checked={describeForm.auto_apply}
                          onChange={(e) => setDescribeForm({ ...describeForm, auto_apply: e.target.checked })}
                        />
                        <label htmlFor="auto-apply">Auto-apply suggestions</label>
                      </div>
                    </div>
                  </div>
                  {error && <div className="error">{error}</div>}
                  <div className="modal-actions">
                    <button
                      type="button"
                      className="btn btn-ghost btn-sm"
                      onClick={() => setDescribeOpen(null)}
                    >
                      Cancel
                    </button>
                    <button
                      type="button"
                      className="btn btn-sm"
                      onClick={runDescribe}
                      disabled={loading}
                    >
                      {loading ? 'Analyzing…' : 'Generate Turkish Descriptions'}
                    </button>
                  </div>
                </>
              )}

              {describeResult && (
                <>
                  <p style={{ color: 'var(--text-secondary)', margin: 0 }}>
                    Sampled {describeResult.sample_rows} rows.{' '}
                    {describeResult.applied
                      ? <span className="success">All suggestions applied.</span>
                      : 'Review and apply selectively.'}
                    {describeResult.translation_applied && describeResult.translation_model ? (
                      <> Translation normalized by <code>{describeResult.translation_model}</code>.</>
                    ) : null}
                  </p>
                  {describeResult.translation_error && (
                    <p style={{ margin: 0, color: 'var(--error)' }}>
                      Translation layer failed; showing the original Turkish-first AI descriptions. {describeResult.translation_error}
                    </p>
                  )}

                  <div>
                    <h3 style={{ marginBottom: '0.4rem' }}>Table description</h3>
                    <div className="suggestion-block">
                      {describeResult.description || <em style={{ color: 'var(--text-secondary)' }}>(none)</em>}
                    </div>
                    {!describeResult.applied && describeResult.description && (
                      <div className="modal-actions">
                        <button
                          type="button"
                          className="btn btn-sm"
                          onClick={() => applySuggestion('table', '', describeResult.description)}
                        >
                          Apply to table
                        </button>
                      </div>
                    )}
                  </div>

                  <div>
                    <h3 style={{ marginBottom: '0.4rem' }}>Columns</h3>
                    <table className="results-table">
                      <thead>
                        <tr>
                          <th>Column</th>
                          <th>Suggestion</th>
                          {!describeResult.applied && <th style={{ textAlign: 'right' }}>Action</th>}
                        </tr>
                      </thead>
                      <tbody>
                        {describeResult.columns.map((c) => (
                          <tr key={c.name}>
                            <td><code>{c.name}</code></td>
                            <td>{c.description || <em style={{ color: 'var(--text-secondary)' }}>(none)</em>}</td>
                            {!describeResult.applied && (
                              <td className="actions">
                                {c.description && (
                                  <button
                                    type="button"
                                    className="btn btn-sm"
                                    onClick={() => applySuggestion('column', c.name, c.description)}
                                  >
                                    Apply
                                  </button>
                                )}
                              </td>
                            )}
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>

                  <div className="modal-actions">
                    <button
                      type="button"
                      className="btn btn-ghost btn-sm"
                      onClick={() => { setDescribeResult(null); setDescribeOpen(null) }}
                    >
                      Close
                    </button>
                  </div>
                </>
              )}
            </div>
          </section>
        </div>
      )}
    </div>
  )
}
