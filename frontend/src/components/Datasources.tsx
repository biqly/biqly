import { useEffect, useState } from 'react'
import { useApi } from '../hooks/useApi'
import type { Datasource } from '../types/metadata'
import { Select } from './ui/Select'

const TYPES = ['postgres', 'mysql', 'sqlserver', 'clickhouse']

const dateFormatter = new Intl.DateTimeFormat(undefined, {
  dateStyle: 'medium',
  timeStyle: 'short',
})

const formatDateTime = (value: string | null | undefined) => {
  if (!value) return 'Hiç'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return 'Hiç'
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

  useEffect(() => { load() }, [])

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
    if (!confirm('Veri kaynağını ve tüm metadata kayıtlarını silmek istediğinize emin misiniz?')) return
    await deleteData(`/api/datasources/${id}`)
    load()
  }

  const test = async (id: string) => {
    setTestResult({ ...testResult, [id]: 'Test ediliyor…' })
    const res = await postData<{ success: boolean; latency_ms?: number; error?: string }>(`/api/datasources/${id}/test`, {})
    if (res?.success) {
      setTestResult({ ...testResult, [id]: `Başarılı (${res.latency_ms} ms)` })
    } else {
      setTestResult({ ...testResult, [id]: `Başarısız: ${res?.error ?? 'bilinmeyen'}` })
    }
  }

  const sync = async (id: string) => {
    setSyncResult({ ...syncResult, [id]: 'Eşitleniyor…' })
    const res = await postData<{ schemas: number; tables: number; columns: number; relations: number }>(
      `/api/datasources/${id}/sync-metadata`,
      {}
    )
    if (res) {
      setSyncResult({ ...syncResult, [id]: `${res.tables} tablo, ${res.columns} kolon` })
      load()
    } else {
      setSyncResult({ ...syncResult, [id]: 'başarısız' })
    }
  }

  return (
    <div className="page-stack">
      <div className="card">
        <div className="card-header-row">
          <h2>Bağlantı Kayıtları</h2>
          <button className="btn" type="button" onClick={() => setShowForm(!showForm)}>
              {showForm ? 'İptal' : '+ Yeni Veri Kaynağı'}
          </button>
        </div>

        {showForm && (
          <div style={{ marginTop: '1rem', paddingTop: '1rem', borderTop: '1px solid var(--border)' }}>
            <div className="form-group">
              <label htmlFor="datasource-name">Ad</label>
              <input id="datasource-name" name="name" value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} placeholder="prod-orders-db…" autoComplete="off" />
            </div>
            <div className="form-group">
              <label htmlFor="datasource-type">Tür</label>
              <Select
                id="datasource-type"
                name="type"
                value={form.type}
                onChange={(v) => setForm({ ...form, type: v })}
                options={TYPES.map((t) => ({ value: t, label: t }))}
              />
            </div>
            <div className="form-group">
              <label htmlFor="datasource-dsn">DSN / Bağlantı Dizesi</label>
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
                Şifrelenmiş olarak saklanır. Salt okunur bir veritabanı kullanıcısı tercih edin.
              </small>
            </div>
            <button className="btn" onClick={create} disabled={loading}>Oluştur</button>
          </div>
        )}

        {error && <div className="error" style={{ marginTop: '1rem' }}>{error}</div>}
      </div>

      <div className="card">
        <h2>Kayıtlı ({items.length})</h2>
        {items.length === 0 && !loading && (
          <p style={{ color: 'var(--text-secondary)' }}>Henüz veri kaynağı yok. Yukarıdan ekleyin.</p>
        )}
        <table className="results-table">
          <thead>
            <tr>
              <th>Kayıt adı</th>
              <th>Veri kaynağı türü</th>
              <th>Son eşitleme</th>
              <th className="actions">İşlemler</th>
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
                    <button className="btn btn-sm" onClick={() => test(ds.id)}>Test Et</button>
                    <button className="btn btn-sm" onClick={() => sync(ds.id)}>Eşitle</button>
                    <button className="btn btn-sm btn-danger" onClick={() => del(ds.id)}>Sil</button>
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
