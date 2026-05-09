import { useEffect, useState } from 'react'
import { useApi } from '../hooks/useApi'

interface Datasource {
  id: string
  name: string
  type: string
  is_active: boolean
  last_sync_at: string | null
  created_at: string
}

const TYPES = ['postgres', 'mysql', 'sqlserver', 'clickhouse']

const dateFormatter = new Intl.DateTimeFormat(undefined, {
  dateStyle: 'medium',
  timeStyle: 'short',
})

const formatDateTime = (value: string | null) => {
  if (!value) return 'Never'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return 'Never'
  return dateFormatter.format(date)
}

export default function Datasources() {
  const { get, postData, deleteData, loading, error } = useApi()
  const [items, setItems] = useState<Datasource[]>([])
  const [showForm, setShowForm] = useState(false)
  const [form, setForm] = useState({ name: '', type: 'postgres', dsn: '' })
  const [syncResult, setSyncResult] = useState<Record<string, string>>({})
  const [testResult, setTestResult] = useState<Record<string, string>>({})

  const load = async () => {
    const data = await get<Datasource[]>('/api/datasources')
    if (data) setItems(data)
  }

  useEffect(() => { load() }, []) // eslint-disable-line react-hooks/exhaustive-deps

  const create = async () => {
    if (!form.name || !form.dsn) return
    const created = await postData('/api/datasources', form)
    if (created) {
      setForm({ name: '', type: 'postgres', dsn: '' })
      setShowForm(false)
      load()
    }
  }

  const del = async (id: string) => {
    if (!confirm('Delete datasource and all its metadata?')) return
    await deleteData(`/api/datasources/${id}`)
    load()
  }

  const test = async (id: string) => {
    setTestResult({ ...testResult, [id]: 'Testing…' })
    const res = await postData<{ success: boolean; latency_ms?: number; error?: string }>(`/api/datasources/${id}/test`, {})
    if (res?.success) {
      setTestResult({ ...testResult, [id]: `OK (${res.latency_ms}ms)` })
    } else {
      setTestResult({ ...testResult, [id]: `Failed: ${res?.error ?? 'unknown'}` })
    }
  }

  const sync = async (id: string) => {
    setSyncResult({ ...syncResult, [id]: 'Syncing…' })
    const res = await postData<{ schemas: number; tables: number; columns: number; relations: number }>(
      `/api/datasources/${id}/sync-metadata`,
      {}
    )
    if (res) {
      setSyncResult({ ...syncResult, [id]: `${res.tables} tables, ${res.columns} columns` })
      load()
    } else {
      setSyncResult({ ...syncResult, [id]: 'failed' })
    }
  }

  return (
    <div>
      <div className="card">
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <h2>Connection Registry</h2>
          <button className="btn" onClick={() => setShowForm(!showForm)}>
              {showForm ? 'Cancel' : '+ New Datasource'}
          </button>
        </div>

        {showForm && (
          <div style={{ marginTop: '1rem', paddingTop: '1rem', borderTop: '1px solid var(--border)' }}>
            <div className="form-group">
              <label htmlFor="datasource-name">Name</label>
              <input id="datasource-name" name="name" value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} placeholder="prod-orders-db…" autoComplete="off" />
            </div>
            <div className="form-group">
              <label htmlFor="datasource-type">Type</label>
              <select id="datasource-type" name="type" value={form.type} onChange={(e) => setForm({ ...form, type: e.target.value })}>
                {TYPES.map((t) => <option key={t} value={t}>{t}</option>)}
              </select>
            </div>
            <div className="form-group">
              <label htmlFor="datasource-dsn">DSN / Connection String</label>
              <input
                id="datasource-dsn"
                name="dsn"
                type="password"
                value={form.dsn}
                onChange={(e) => setForm({ ...form, dsn: e.target.value })}
                placeholder="postgres://user:pass@host:5432/db?sslmode=require…"
                autoComplete="off"
                spellCheck={false}
              />
              <small style={{ color: 'var(--text-secondary)', fontSize: '0.75rem' }}>
                Stored encrypted. Use a read-only DB user.
              </small>
            </div>
            <button className="btn" onClick={create} disabled={loading}>Create</button>
          </div>
        )}

        {error && <div className="error" style={{ marginTop: '1rem' }}>{error}</div>}
      </div>

      <div className="card">
        <h2>Registered ({items.length})</h2>
        {items.length === 0 && !loading && (
          <p style={{ color: 'var(--text-secondary)' }}>No datasources yet. Add one above.</p>
        )}
        <table className="results-table">
          <thead>
            <tr>
              <th>Name</th>
              <th>Type</th>
              <th>Last Sync</th>
              <th className="actions">Actions</th>
            </tr>
          </thead>
          <tbody>
            {items.map((ds) => (
              <tr key={ds.id}>
                <td>
                  <div>{ds.name}</div>
                  <small style={{ color: 'var(--text-secondary)' }}>{ds.id}</small>
                </td>
                <td>{ds.type}</td>
                <td>
                  {formatDateTime(ds.last_sync_at)}
                  {syncResult[ds.id] && <div style={{ fontSize: '0.75rem', color: 'var(--text-secondary)' }}>{syncResult[ds.id]}</div>}
                  {testResult[ds.id] && <div style={{ fontSize: '0.75rem', color: 'var(--text-secondary)' }}>{testResult[ds.id]}</div>}
                </td>
                <td className="actions">
                  <div className="row-actions">
                    <button className="btn btn-sm" onClick={() => test(ds.id)}>Test</button>
                    <button className="btn btn-sm" onClick={() => sync(ds.id)}>Sync</button>
                    <button className="btn btn-sm btn-danger" onClick={() => del(ds.id)}>Delete</button>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}
