import { useEffect, useState } from 'react'
import { useApi } from '../hooks/useApi'
import { useConfirm } from '../hooks/useConfirm'
import {
  DRIVER_IDS,
  driverDefaultPort,
  driverDsnPlaceholder,
  driverLabelKey,
  driverLogoUrl,
  driverStructuredDefaults,
  isInsecureSslMode,
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

type ConnectionMode = 'structured' | 'raw'

interface StructuredForm {
  host: string
  port: string
  username: string
  password: string
  database_name: string
  ssl_mode: string
}

interface DatasourceForm {
  name: string
  type: string
  dsn: string
}

function emptyStructured(): StructuredForm {
  return {
    host: '',
    port: '',
    username: '',
    password: '',
    database_name: '',
    ssl_mode: '',
  }
}

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
  const { get, postData, putData, deleteData, loading, error } = useApi()
  const confirm = useConfirm()
  const [items, setItems] = useState<Datasource[]>([])
  const [showForm, setShowForm] = useState(false)
  const [editingId, setEditingId] = useState<string | null>(null)
  const [connMode, setConnMode] = useState<ConnectionMode>('structured')
  const [form, setForm] = useState<DatasourceForm>({ name: '', type: 'postgres', dsn: '' })
  const [structured, setStructured] = useState<StructuredForm>(emptyStructured)
  const [syncResult, setSyncResult] = useState<Record<string, string>>({})
  const [testResult, setTestResult] = useState<Record<string, string>>({})
  const [draftTestResult, setDraftTestResult] = useState<string | null>(null)

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

  const driverConnHints = driverStructuredDefaults(form.type)
  const defaultPortHint = driverDefaultPort(form.type)
  const resetForm = () => {
    setEditingId(null)
    setForm({ name: '', type: 'postgres', dsn: '' })
    setStructured(emptyStructured())
    setDraftTestResult(null)
  }

  const openNewForm = () => {
    if (showForm && !editingId) {
      resetForm()
      setShowForm(false)
      return
    }
    resetForm()
    const defaults = driverStructuredDefaults('postgres')
    setStructured({ ...emptyStructured(), port: defaults.port, ssl_mode: defaults.ssl_mode })
    setConnMode('structured')
    setShowForm(true)
  }

  const draftPayload = () => {
    const name = form.name.trim()
    if (!name) return null

    if (connMode === 'raw') {
      if (!editingId && !form.dsn.trim()) return null
      return {
        id: editingId ?? undefined,
        name,
        type: form.type,
        mode: 'raw',
        dsn: form.dsn.trim(),
      }
    }

    if (!structured.host.trim()) return null
    const portStr = structured.port.trim()
    let port: number | undefined
    if (portStr !== '') {
      const n = parseInt(portStr, 10)
      if (Number.isNaN(n) || n <= 0) return null
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

    return {
      id: editingId ?? undefined,
      name,
      type: form.type,
      mode: 'structured',
      connection,
    }
  }

  const save = async () => {
    const payload = draftPayload()
    if (!payload) return

    const saved = editingId
      ? await putData(`/api/datasources/${editingId}`, payload)
      : await postData('/api/datasources', payload)
    if (saved) {
      resetForm()
      setShowForm(false)
      load()
    }
  }

  const testDraft = async () => {
    const payload = draftPayload()
    if (!payload) return
    setDraftTestResult(t('datasources.testing'))
    const res = await postData<{ success: boolean; latency_ms?: number; error?: string }>(
      '/api/datasources/test-connection',
      payload,
    )
    if (res?.success) {
      setDraftTestResult(t('datasources.test_success', { ms: res.latency_ms ?? 0 }))
    } else {
      setDraftTestResult(t('datasources.test_failed', { error: res?.error ?? 'unknown' }))
    }
  }

  const edit = (ds: Datasource) => {
    const mode: ConnectionMode = ds.dsn_mode === 'raw' ? 'raw' : 'structured'
    setEditingId(ds.id)
    setConnMode(mode)
    setForm({ name: ds.name, type: ds.type, dsn: '' })
    setStructured({
      host: ds.host ?? '',
      port: ds.port ? String(ds.port) : driverStructuredDefaults(ds.type).port,
      username: ds.username ?? '',
      password: '',
      database_name: ds.database_name ?? '',
      ssl_mode: ds.ssl_mode ?? driverStructuredDefaults(ds.type).ssl_mode,
    })
    setDraftTestResult(null)
    setShowForm(true)
  }

  const setDriver = (type: string) => {
    setForm({ ...form, type })
    const defaults = driverStructuredDefaults(type)
    setStructured({ ...structured, port: defaults.port, ssl_mode: defaults.ssl_mode })
  }

  const del = async (id: string) => {
    const ok = await confirm({
      title: t('datasources.delete_confirm'),
      message: t('datasources.delete_confirm'),
      variant: 'danger',
    })
    if (!ok) return
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
      ? editingId !== null || form.dsn.trim() !== ''
      : structured.host.trim() !== '' &&
        (structured.port.trim() === '' || (!Number.isNaN(parseInt(structured.port, 10)) && parseInt(structured.port, 10) > 0)))

  return (
    <div className="page-stack">
        <div className="card">
          <div className="card-header-row">
            <h2>{t('datasources.panel_title')}</h2>
            <button className="btn" type="button" onClick={openNewForm}>
              {showForm && !editingId ? t('datasources.cancel') : t('datasources.new')}
            </button>
          </div>

        {showForm && (
          <div style={{ marginTop: '1rem', paddingTop: '1rem', borderTop: '1px solid var(--border)' }}>
            <div className="card-header-row datasource-form-header">
              <h3 style={{ margin: 0 }}>{editingId ? t('datasources.edit_title') : t('datasources.new')}</h3>
              {editingId && (
                <button className="btn btn-sm" type="button" onClick={() => { resetForm(); setShowForm(false) }}>
                  {t('datasources.cancel')}
                </button>
              )}
            </div>
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
                onChange={setDriver}
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
                  {editingId ? t('datasources.dsn_keep_hint') : t('datasources.dsn_hint')}
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
                    {editingId ? t('datasources.password_keep_hint') : t('datasources.dsn_hint')}
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
                  {isInsecureSslMode(structured.ssl_mode) && (
                    <small style={{ color: 'var(--warning)', fontSize: '0.75rem', display: 'block', marginTop: '0.25rem' }}>
                      ⚠ {t('datasources.ssl_insecure_warning')}
                    </small>
                  )}
                </div>
              </>
            )}
            <div className="datasource-form-actions">
              <button className="btn" type="button" onClick={testDraft} disabled={loading || !canSubmit}>
                {t('datasources.test_before_save')}
              </button>
              <button className="btn btn-primary" type="button" onClick={save} disabled={loading || !canSubmit}>
                {editingId ? t('datasources.save') : t('datasources.create')}
              </button>
              {draftTestResult && (
                <small style={{ color: 'var(--text-secondary)' }}>{draftTestResult}</small>
              )}
            </div>
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
                  <button
                    type="button"
                    title={ds.id}
                    aria-label={t('datasources.copy_id_aria', { id: ds.id })}
                    className="datasource-id-pill"
                    onClick={() => {
                      navigator.clipboard?.writeText(ds.id).catch(() => {})
                    }}
                  >
                    <span aria-hidden="true">id · {ds.id.slice(0, 8)}…</span>
                  </button>
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
                    <button type="button" className="btn btn-sm" onClick={() => edit(ds)}>
                      {t('datasources.edit')}
                    </button>
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
