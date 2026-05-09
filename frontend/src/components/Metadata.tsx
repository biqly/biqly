import { Fragment, useEffect, useState } from 'react'
import { useApi } from '../hooks/useApi'

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
          <h2>Tables ({tables.length})</h2>
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
