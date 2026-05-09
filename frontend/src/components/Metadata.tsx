import { Fragment, useEffect, useRef, useState } from 'react'
import { useApi } from '../hooks/useApi'

type BulkStatus = 'pending' | 'running' | 'ok' | 'error' | 'skipped'

interface BulkEntry {
  schema: string
  table: string
  status: BulkStatus
  message?: string
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
  }, [datasourceId]) // eslint-disable-line react-hooks/exhaustive-deps

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
    if (!datasourceId || tables.length === 0) return
    bulkCancelRef.current = false
    setBulkRunning(true)
    setBulkSummary(null)

    const queue: BulkEntry[] = tables.map((t) => {
      if (bulkConfig.skip_existing && t.description) {
        return { schema: t.schema_name, table: t.table_name, status: 'skipped', message: 'already described' }
      }
      return { schema: t.schema_name, table: t.table_name, status: 'pending' }
    })
    setBulkEntries(queue)

    let ok = 0
    let errCount = 0
    let skipped = queue.filter((q) => q.status === 'skipped').length

    for (let i = 0; i < tables.length; i++) {
      if (bulkCancelRef.current) break
      const t = tables[i]
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
            <h2 style={{ margin: 0 }}>Tables ({tables.length})</h2>
            {tables.length > 0 && (
              <button type="button" className="btn btn-sm" onClick={openBulk} disabled={bulkRunning}>
                🤖 Describe All Tables
              </button>
            )}
          </div>
          {tables.length === 0 && !loading && (
            <p style={{ color: 'var(--text-secondary)' }}>
              No tables. Run <strong>Sync</strong> from the Datasources tab.
            </p>
          )}
          <table className="results-table">
            <thead>
              <tr>
                <th>Schema.Table</th>
                <th>Type</th>
                <th>Description</th>
                <th className="actions">Actions</th>
              </tr>
            </thead>
            <tbody>
              {tables.map((t) => (
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
                    <td>{t.table_type}</td>
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
                        <table className="results-table" style={{ margin: 0 }}>
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
            aria-labelledby="bulk-describe-title"
          >
            <header className="modal-header" style={{ flexShrink: 0, padding: '0.75rem 1rem' }}>
              <h2 id="bulk-describe-title" style={{ fontSize: '1rem' }}>🤖 Describe All Tables</h2>
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
                  <p style={{ color: 'var(--text-secondary)', margin: 0 }}>
                    Iterates every table in this datasource, asking the LLM for table + column descriptions
                    and applying them automatically. May take a while and consume LLM tokens.
                  </p>
                  <div className="modal-form-row">
                    <div className="form-group">
                      <label htmlFor="bulk-sample-size">Sample size</label>
                      <input
                        id="bulk-sample-size"
                        type="number"
                        min={1}
                        max={100}
                        value={bulkConfig.sample_size}
                        onChange={(e) => setBulkConfig({ ...bulkConfig, sample_size: Number(e.target.value) })}
                      />
                    </div>
                    <div className="form-group">
                      <label>Options</label>
                      <div className="checkbox-row">
                        <input
                          id="bulk-skip-existing"
                          type="checkbox"
                          checked={bulkConfig.skip_existing}
                          onChange={(e) => setBulkConfig({ ...bulkConfig, skip_existing: e.target.checked })}
                        />
                        <label htmlFor="bulk-skip-existing">Skip tables that already have a description</label>
                      </div>
                    </div>
                  </div>
                  <div className="modal-actions">
                    <button type="button" className="btn btn-ghost btn-sm" onClick={closeBulk}>Cancel</button>
                    <button type="button" className="btn btn-sm" onClick={runBulkDescribe}>
                      Start ({tables.length} tables)
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
                        {bulkEntries.map((e, idx) => (
                          <tr key={`${e.schema}.${e.table}`}>
                            <td className="bulk-col-idx">{idx + 1}</td>
                            <td className="bulk-col-name">
                              <code>{e.schema}.{e.table}</code>
                            </td>
                            <td className="bulk-col-status">
                              <BulkStatusBadge status={e.status} />
                            </td>
                            <td className="bulk-col-detail" style={{ color: 'var(--text-secondary)' }} title={e.message}>
                              {e.message || (e.status === 'pending' ? '—' : '')}
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
                Samples N rows from the source DB and asks the LLM to describe the table and each column.
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
                      {loading ? 'Analyzing…' : 'Generate Descriptions'}
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
                  </p>

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
