import { useEffect, useState } from 'react'
import { useApi } from '../hooks/useApi'
import {
  DRIVER_IDS,
  driverDefaultPort,
  driverDsnPlaceholder,
  driverLabelKey,
  driverLogoUrl,
  driverStructuredDefaults,
} from '../dbDrivers'
import { useT } from '../i18n'
import type { Datasource } from '../types/metadata'
import { DriverTileGrid } from './DriverTileGrid'
import { EmptyState } from './ui/EmptyState'
import { ErrorAlert } from './ui/ErrorAlert'

const dateFormatter = new Intl.DateTimeFormat(undefined, {
  dateStyle: 'medium',
  timeStyle: 'short',
})

function connectionSummary(ds: Datasource): { line1: string; line2?: string } {
  if (ds.dsn_mode === 'structured') {
    const host = ds.host?.trim()
    const db = ds.database_name?.trim()
    if (host && db) return { line1: `${host} · ${db}` }
    if (host) return { line1: host }
    return { line1: '' }
  }
  return { line1: '' }
}

export default function Datasources() {
  const t = useT()
  const { get, postData, deleteData, loading, error } = useApi()
  const [items, setItems] = useState<Datasource[]>([])
  const [showForm, setShowForm] = useState(false)
  const [connMode, setConnMode] = useState<'structured' | 'raw'>('structured')
  const [form, setForm] = useState({ name: '', type: 'postgres', dsn: '' })
  const [structured, setStructured] = useState({
    host: '',
    port: '',
    username: '',
    password: '',
    database_name: '',
    ssl_mode: '',
  })
  const [syncResult, setSyncResult] = useState<Record<string, string>>({})
  const [testResult, setTestResult] = useState<Record<string, string>>({})

  const formatDateTime = (value: string | null | undefined) => {
    if (!value) return t('datasources.never')
    const date = new Date(value)
    if (Number.isNaN(date.getTime())) return t('datasources.never')
    return dateFormatter.format(date)
  }

  const load = async () => {
    const data = await get<Datasource[]>('/api/datasources')
    if (data) setItems(data)
  }

  useEffect(() => {
    load()
  }, [])

  useEffect(() => {
    if (!showForm || connMode !== 'structured') return
    const next = driverStructuredDefaults(form.type)
    setStructured((prev) => ({ ...prev, port: next.port, ssl_mode: next.ssl_mode }))
  }, [showForm, connMode, form.type])

  const driverConnHints = driverStructuredDefaults(form.type)
  const defaultPortHint = driverDefaultPort(form.type)
  const resetForm = () => {
    setForm({ name: '', type: 'postgres', dsn: '' })
    setStructured({
      host: '',
      port: '',
      username: '',
      password: '',
      database_name: '',
      ssl_mode: '',
    })
  }

  const create = async () => {
    const name = form.name.trim()
    if (!name) return

    if (connMode === 'raw') {
      if (!form.dsn.trim()) return
      const created = await postData('/api/datasources', {
        name,
        type: form.type,
        dsn: form.dsn.trim(),
      })
      if (created) {
        resetForm()
        setShowForm(false)
        load()
      }
      return
    }

    if (!structured.host.trim()) return
    const portStr = structured.port.trim()
    let port: number | undefined
    if (portStr !== '') {
      const n = parseInt(portStr, 10)
      if (Number.isNaN(n) || n <= 0) return
      port = n
    }

    const connection: Record<string, unknown> = {
      host: structured.host.trim(),
      username: structured.username,
      password: structured.password,
      database_name: structured.database_name,
    }
    if (port !== undefined) {
      connection.port = port
    }
    const ssl = structured.ssl_mode.trim()
    if (ssl) {
      connection.ssl_mode = ssl
    }

    const created = await postData('/api/datasources', {
      name,
      type: form.type,
      mode: 'structured',
      connection,
    })
    if (created) {
      resetForm()
      setShowForm(false)
      load()
    }
  }

  const del = async (id: string) => {
    if (!confirm(t('datasources.delete_confirm'))) return
    await deleteData(`/api/datasources/${id}`)
    load()
  }

  const test = async (id: string) => {
    setTestResult({ ...testResult, [id]: t('datasources.testing') })
    const res = await postData<{ success: boolean; latency_ms?: number; error?: string }>(`/api/datasources/${id}/test`, {})
    if (res?.success) {
      setTestResult({
        ...testResult,
        [id]: t('datasources.test_success', { ms: res.latency_ms ?? 0 }),
      })
    } else {
      setTestResult({
        ...testResult,
        [id]: t('datasources.test_failed', { error: res?.error ?? 'unknown' }),
      })
    }
  }

  const sync = async (id: string) => {
    setSyncResult({ ...syncResult, [id]: t('datasources.syncing') })
    const res = await postData<{ schemas: number; tables: number; columns: number; relations: number }>(
      `/api/datasources/${id}/sync-metadata`,
      {},
    )
    if (res) {
      setSyncResult({
        ...syncResult,
        [id]: t('datasources.sync_result', { tables: res.tables, columns: res.columns }),
      })
      load()
    } else {
      setSyncResult({ ...syncResult, [id]: t('datasources.sync_failed') })
    }
  }

  const canSubmit =
    form.name.trim() !== '' &&
    (connMode === 'raw'
      ? form.dsn.trim() !== ''
      : structured.host.trim() !== '' &&
        (structured.port.trim() === '' || (!Number.isNaN(parseInt(structured.port, 10)) && parseInt(structured.port, 10) > 0)))

  return (
    <div className="page-stack">
      <div className="card">
        <div className="card-header-row">
          <h2>{t('datasources.panel_title')}</h2>
          <button className="btn" type="button" onClick={() => setShowForm(!showForm)}>
            {showForm ? t('datasources.cancel') : t('datasources.new')}
          </button>
        </div>

        {showForm && (
          <div style={{ marginTop: '1rem', paddingTop: '1rem', borderTop: '1px solid var(--border)' }}>
            <div className="form-group">
              <div style={{ fontWeight: 600, marginBottom: '0.35rem' }}>{t('datasources.connection_mode')}</div>
              <div className="bulk-segmented" role="group" aria-label={t('datasources.connection_mode')}>
                <button
                  type="button"
                  className={`bulk-segmented__btn${connMode === 'structured' ? ' bulk-segmented__btn--active' : ''}`}
                  onClick={() => setConnMode('structured')}
                >
                  {t('datasources.mode_structured')}
                </button>
                <button
                  type="button"
                  className={`bulk-segmented__btn${connMode === 'raw' ? ' bulk-segmented__btn--active' : ''}`}
                  onClick={() => setConnMode('raw')}
                >
                  {t('datasources.mode_raw')}
                </button>
              </div>
            </div>
            <div className="form-group">
              <label htmlFor="datasource-name">{t('datasources.name')}</label>
              <input
                id="datasource-name"
                name="name"
                value={form.name}
                onChange={(e) => setForm({ ...form, name: e.target.value })}
                placeholder="prod-orders-db"
                autoComplete="off"
              />
            </div>
            <div className="form-group">
              <div style={{ fontWeight: 600, marginBottom: '0.35rem' }}>{t('datasources.type')}</div>
              <DriverTileGrid
                value={form.type}
                onChange={(id) => setForm({ ...form, type: id })}
                ids={DRIVER_IDS}
                ariaLabel={t('datasources.pick_driver')}
                t={t}
              />
            </div>
            {connMode === 'raw' ? (
              <div className="form-group">
                <label htmlFor="datasource-dsn">{t('datasources.dsn')}</label>
                <input
                  id="datasource-dsn"
                  name="dsn"
                  type="password"
                  value={form.dsn}
                  onChange={(e) => setForm({ ...form, dsn: e.target.value })}
                  placeholder={driverDsnPlaceholder(form.type)}
                  autoComplete="off"
                  spellCheck={false}
                />
                <small style={{ color: 'var(--text-secondary)', fontSize: '0.75rem' }}>
                  {t('datasources.dsn_hint')}
                </small>
              </div>
            ) : (
              <>
                <div className="form-group">
                  <label htmlFor="ds-host">{t('datasources.fields.host')}</label>
                  <input
                    id="ds-host"
                    value={structured.host}
                    onChange={(e) => setStructured({ ...structured, host: e.target.value })}
                    placeholder="localhost"
                    autoComplete="off"
                  />
                </div>
                <div className="form-group">
                  <label htmlFor="ds-port">{t('datasources.fields.port')}</label>
                  <input
                    id="ds-port"
                    value={structured.port}
                    onChange={(e) => setStructured({ ...structured, port: e.target.value })}
                    placeholder={defaultPortHint > 0 ? String(defaultPortHint) : ''}
                    inputMode="numeric"
                    autoComplete="off"
                  />
                  <small style={{ color: 'var(--text-secondary)', fontSize: '0.75rem' }}>
                    {t('common.optional')}
                  </small>
                </div>
                <div className="form-group">
                  <label htmlFor="ds-db">{t('datasources.fields.database')}</label>
                  <input
                    id="ds-db"
                    value={structured.database_name}
                    onChange={(e) => setStructured({ ...structured, database_name: e.target.value })}
                    autoComplete="off"
                  />
                </div>
                <div className="form-group">
                  <label htmlFor="ds-user">{t('datasources.fields.username')}</label>
                  <input
                    id="ds-user"
                    value={structured.username}
                    onChange={(e) => setStructured({ ...structured, username: e.target.value })}
                    autoComplete="off"
                  />
                </div>
                <div className="form-group">
                  <label htmlFor="ds-pass">{t('datasources.fields.password')}</label>
                  <input
                    id="ds-pass"
                    type="password"
                    value={structured.password}
                    onChange={(e) => setStructured({ ...structured, password: e.target.value })}
                    autoComplete="off"
                  />
                  <small style={{ color: 'var(--text-secondary)', fontSize: '0.75rem' }}>
                    {t('datasources.dsn_hint')}
                  </small>
                </div>
                <div className="form-group">
                  <label htmlFor="ds-ssl">{t('datasources.fields.ssl_mode')}</label>
                  <input
                    id="ds-ssl"
                    value={structured.ssl_mode}
                    onChange={(e) => setStructured({ ...structured, ssl_mode: e.target.value })}
                    placeholder={driverConnHints.ssl_mode || 'disable'}
                    autoComplete="off"
                  />
                  <small style={{ color: 'var(--text-secondary)', fontSize: '0.75rem' }}>
                    {t('common.optional')}
                  </small>
                </div>
              </>
            )}
            <button className="btn" type="button" onClick={create} disabled={loading || !canSubmit}>
              {t('datasources.create')}
            </button>
          </div>
        )}

        <ErrorAlert error={error} className="error--top-gap" />
      </div>

      <div className="card">
        <h2>{t('datasources.registered_count', { count: items.length })}</h2>
        {items.length === 0 && !loading && (
          <EmptyState description={t('datasources.empty')} className="ui-empty-state--inline" />
        )}
        <table className="results-table">
          <thead>
            <tr>
              <th>{t('datasources.record_name')}</th>
              <th>{t('datasources.driver_type')}</th>
              <th>{t('datasources.last_sync')}</th>
              <th className="actions">{t('datasources.actions')}</th>
            </tr>
          </thead>
          <tbody>
            {items.map((ds) => {
              const hint = connectionSummary(ds)
              const logoSrc = driverLogoUrl(ds.type)
              const modeHint =
                hint.line1 ||
                (ds.dsn_mode === 'structured' ? t('datasources.mode_structured') : t('datasources.mode_raw'))
              return (
              <tr key={ds.id}>
                <td>
                  <div>{ds.name}</div>
                  <small style={{ color: 'var(--text-secondary)' }}>{modeHint}</small>
                  <small style={{ display: 'block', color: 'var(--text-secondary)', opacity: 0.85 }}>{ds.id}</small>
                </td>
                <td>
                  <div className={`driver-cell driver-cell--${ds.type}`}>
                    {logoSrc ? (
                      <span className="driver-cell__logo" aria-hidden>
                        <img src={logoSrc} alt="" width={26} height={26} />
                      </span>
                    ) : null}
                    <span className="driver-cell__label">{t(driverLabelKey(ds.type))}</span>
                  </div>
                </td>
                <td>
                  {formatDateTime(ds.last_sync_at)}
                  {syncResult[ds.id] && (
                    <div style={{ fontSize: '0.75rem', color: 'var(--text-secondary)' }}>
                      {syncResult[ds.id]}
                    </div>
                  )}
                  {testResult[ds.id] && (
                    <div style={{ fontSize: '0.75rem', color: 'var(--text-secondary)' }}>
                      {testResult[ds.id]}
                    </div>
                  )}
                </td>
                <td className="actions">
                  <div className="row-actions">
                    <button type="button" className="btn btn-sm" onClick={() => test(ds.id)}>
                      {t('datasources.test')}
                    </button>
                    <button type="button" className="btn btn-sm" onClick={() => sync(ds.id)}>
                      {t('datasources.sync')}
                    </button>
                    <button type="button" className="btn btn-sm btn-danger" onClick={() => del(ds.id)}>
                      {t('datasources.delete')}
                    </button>
                  </div>
                </td>
              </tr>
              )
            })}
          </tbody>
        </table>
      </div>
    </div>
  )
}
