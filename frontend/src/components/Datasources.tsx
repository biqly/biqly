import { useCallback, useEffect, useState } from 'react'

import { getMyDatasources } from '../api/admin'
import { driverLabelKey, driverLogoUrl, driverStructuredDefaults } from '../dbDrivers'
import { useApi } from '../hooks/useApi'
import { useConfirm } from '../hooks/useConfirm'
import { useT } from '../i18n'
import {
  datasourceAccessBadgeClass,
  datasourceAccessBadgeIconClass,
  datasourceAccessNoteClass,
  datasourceIdCopyButtonClass,
} from '../lib/badgeClasses'
import { legacyButtonClass, rowActionsClass } from '../lib/buttonClasses'
import { legacyCardClass } from '../lib/cardClasses'
import { driverCellClass, driverCellLabelClass, driverCellLogoClass } from '../lib/driverClasses'
import { errorAlertTopGapClass, uiEmptyStateInlineClass } from '../lib/feedbackClasses'
import { legacyLayoutClass } from '../lib/layoutClasses'
import {
  datasourceConnectionHintClass,
  datasourceRowStatusClass,
  datasourcesColActionsWidthClass,
  datasourcesColDriverWidthClass,
  datasourcesColRecordWidthClass,
  datasourcesColSyncWidthClass,
  datasourceTableSectionLabelClass,
  resultsTableDatasourcesListClass,
  resultsTableScrollClass,
} from '../lib/tableClasses'
import type { Datasource } from '../types/metadata'
import { noop } from '../utils/constants'
import { useAuth } from './auth/AuthProvider'
import { buildDatasourceAccessView } from './datasources/accessView'
import { DatasourceFormModal } from './datasources/DatasourceFormModal'
import { EmptyState } from './ui/EmptyState'
import { ErrorAlert } from './ui/ErrorAlert'
import { LoadingScreen } from './ui/LoadingScreen'
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
    if (host && db) {
      return { line1: `${host} · ${db}` }
    }
    if (host) {
      return { line1: host }
    }
    return { line1: '' }
  }
  return { line1: '' }
}

export default function Datasources() {
  const t = useT()
  const { get, postData, putData, deleteData, loading, error } = useApi()
  const { accessToken } = useAuth()
  const confirm = useConfirm()
  const [initLoading, setInitLoading] = useState(true)
  const [items, setItems] = useState<Datasource[]>([])
  const [accessibleDatasourceIDs, setAccessibleDatasourceIDs] = useState<string[] | null>(null)
  const [showForm, setShowForm] = useState(false)
  const [editingId, setEditingId] = useState<string | null>(null)
  const [connMode, setConnMode] = useState<ConnectionMode>('structured')
  const [form, setForm] = useState<DatasourceForm>({ name: '', type: 'postgres', dsn: '' })
  const [structured, setStructured] = useState<StructuredForm>(emptyStructured)
  const [syncResult, setSyncResult] = useState<Record<string, string>>({})
  const [testResult, setTestResult] = useState<Record<string, string>>({})
  const [draftTestResult, setDraftTestResult] = useState<string | null>(null)

  const formatDateTime = (value: string | null | undefined) => {
    if (!value) {
      return t('datasources.never')
    }
    const date = new Date(value)
    if (Number.isNaN(date.getTime())) {
      return t('datasources.never')
    }
    return dateFormatter.format(date)
  }

  const load = useCallback(async () => {
    setInitLoading(true)
    try {
      const options = authRequestOptions(accessToken)
      const [data, accessibleIDs] = await Promise.all([
        get<Datasource[]>('/api/datasources', options),
        accessToken ? getMyDatasources(accessToken).catch(() => null) : Promise.resolve(null),
      ])
      if (data) {
        setItems(data)
      }
      setAccessibleDatasourceIDs(accessibleIDs)
    } finally {
      setInitLoading(false)
    }
  }, [accessToken, get])

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load()
  }, [load])

  const resetForm = () => {
    setEditingId(null)
    setForm({ name: '', type: 'postgres', dsn: '' })
    setStructured(emptyStructured())
    setDraftTestResult(null)
  }

  const openNewForm = () => {
    resetForm()
    const defaults = driverStructuredDefaults('postgres')
    setStructured({ ...emptyStructured(), port: defaults.port, ssl_mode: defaults.ssl_mode })
    setConnMode('structured')
    setShowForm(true)
  }

  const closeForm = () => {
    resetForm()
    setShowForm(false)
  }

  const draftPayload = () => {
    const name = form.name.trim()
    if (!name) {
      return null
    }

    if (connMode === 'raw') {
      if (!editingId && !form.dsn.trim()) {
        return null
      }
      return {
        id: editingId ?? undefined,
        name,
        type: form.type,
        mode: 'raw',
        dsn: form.dsn.trim(),
      }
    }

    if (!structured.host.trim()) {
      return null
    }
    const portStr = structured.port.trim()
    let port: number | undefined
    if (portStr !== '') {
      const n = parseInt(portStr, 10)
      if (Number.isNaN(n) || n <= 0) {
        return null
      }
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
    if (!payload) {
      return
    }

    const saved = editingId
      ? await putData(`/api/datasources/${editingId}`, payload, authRequestOptions(accessToken))
      : await postData('/api/datasources', payload, authRequestOptions(accessToken))
    if (saved) {
      closeForm()
      void load()
    }
  }

  const testDraft = async () => {
    const payload = draftPayload()
    if (!payload) {
      return
    }
    setDraftTestResult(t('datasources.testing'))
    const res = await postData<{ success: boolean; latency_ms?: number; error?: string }>(
      '/api/datasources/test-connection',
      payload,
      authRequestOptions(accessToken),
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
    if (!ok) {
      return
    }
    await deleteData(`/api/datasources/${id}`, authRequestOptions(accessToken))
    void load()
  }

  const test = async (id: string) => {
    setTestResult({ ...testResult, [id]: t('datasources.testing') })
    const res = await postData<{ success: boolean; latency_ms?: number; error?: string }>(
      `/api/datasources/${id}/test`,
      {},
      authRequestOptions(accessToken),
    )
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
    const res = await postData<{
      schemas: number
      tables: number
      columns: number
      relations: number
    }>(`/api/datasources/${id}/sync-metadata`, {}, authRequestOptions(accessToken))
    if (res) {
      setSyncResult({
        ...syncResult,
        [id]: t('datasources.sync_result', { tables: res.tables, columns: res.columns }),
      })
      void load()
    } else {
      setSyncResult({ ...syncResult, [id]: t('datasources.sync_failed') })
    }
  }

  const canSubmit =
    form.name.trim() !== '' &&
    (connMode === 'raw'
      ? editingId !== null || form.dsn.trim() !== ''
      : structured.host.trim() !== '' &&
        (structured.port.trim() === '' ||
          (!Number.isNaN(parseInt(structured.port, 10)) && parseInt(structured.port, 10) > 0)))
  const datasourceRows = buildDatasourceAccessView(items, accessibleDatasourceIDs)

  if (initLoading && items.length === 0) {
    return <LoadingScreen minHeight="300px" />
  }

  const showAccessBadge = accessibleDatasourceIDs !== null

  return (
    <div className={legacyLayoutClass('page-stack min-w-0')}>
      <ErrorAlert error={error} className={errorAlertTopGapClass} />

      <div className={legacyCardClass('card')}>
        <div className={legacyCardClass('card-intro card-intro--compact')}>
          <div className={legacyCardClass('card-header-row')}>
            <h2>{t('datasources.panel_title')}</h2>
            <button
              className={legacyButtonClass('btn btn-primary')}
              type="button"
              onClick={openNewForm}
            >
              + {t('datasources.new')}
            </button>
          </div>
          <p className={legacyCardClass('card-lead')}>{t('datasources.form_subtitle')}</p>
        </div>

        {accessibleDatasourceIDs !== null && datasourceRows.length < items.length && (
          <p className={datasourceAccessNoteClass}>
            {t(
              items.length - datasourceRows.length === 1
                ? 'datasources.hidden_by_access_policy'
                : 'datasources.hidden_by_access_policy_plural',
              { count: items.length - datasourceRows.length },
            )}
          </p>
        )}

        {datasourceRows.length === 0 && !loading ? (
          <EmptyState description={t('datasources.empty')} className={uiEmptyStateInlineClass} />
        ) : (
          <>
            <p className={datasourceTableSectionLabelClass}>
              {t('datasources.registered_count', { count: datasourceRows.length })}
            </p>
            <div className={resultsTableScrollClass}>
              <table className={resultsTableDatasourcesListClass}>
                <colgroup>
                  <col className={datasourcesColRecordWidthClass} />
                  <col className={datasourcesColDriverWidthClass} />
                  <col className={datasourcesColSyncWidthClass} />
                  <col className={datasourcesColActionsWidthClass} />
                </colgroup>
                <thead>
                  <tr>
                    <th className="datasources-col-record">{t('datasources.record_name')}</th>
                    <th className="datasources-col-driver">{t('datasources.driver_type')}</th>
                    <th className="datasources-col-sync">{t('datasources.last_sync')}</th>
                    <th className="datasources-col-actions actions">{t('datasources.actions')}</th>
                  </tr>
                </thead>
                <tbody>
                  {datasourceRows.map(({ datasource: ds, access }) => {
                    const hint = connectionSummary(ds)
                    const logoSrc = driverLogoUrl(ds.type)
                    const modeHint =
                      hint.line1 ||
                      (ds.dsn_mode === 'structured'
                        ? t('datasources.mode_structured')
                        : t('datasources.mode_raw'))
                    return (
                      <tr key={ds.id}>
                        <td className="datasources-col-record">
                          <div className="flex min-w-0 flex-col gap-1.5">
                            <div className="flex flex-wrap items-center gap-x-2.5 gap-y-1.5">
                              <span className="text-foreground text-[0.9375rem] font-semibold">
                                {ds.name}
                              </span>
                              {showAccessBadge && access === 'allowed' && (
                                <span className={datasourceAccessBadgeClass}>
                                  <span className={datasourceAccessBadgeIconClass} aria-hidden>
                                    ✓
                                  </span>
                                  {t('datasources.access_allowed')}
                                </span>
                              )}
                            </div>
                            {modeHint ? (
                              <div className={datasourceConnectionHintClass}>{modeHint}</div>
                            ) : null}
                            <div className="flex flex-wrap items-center gap-1.5">
                              <button
                                type="button"
                                title={ds.id}
                                aria-label={t('datasources.copy_id_aria', { id: ds.id })}
                                className={datasourceIdCopyButtonClass}
                                onClick={() => {
                                  navigator.clipboard.writeText(ds.id).catch(noop)
                                }}
                              >
                                <span aria-hidden="true">id · {ds.id.slice(0, 8)}…</span>
                              </button>
                            </div>
                          </div>
                        </td>
                        <td className="datasources-col-driver">
                          <div className={driverCellClass()}>
                            {logoSrc ? (
                              <span className={driverCellLogoClass(ds.type)} aria-hidden>
                                <img src={logoSrc} alt="" width={26} height={26} />
                              </span>
                            ) : null}
                            <span className={driverCellLabelClass()}>
                              {t(driverLabelKey(ds.type))}
                            </span>
                          </div>
                        </td>
                        <td className="datasources-col-sync">
                          <span>{formatDateTime(ds.last_sync_at)}</span>
                          {syncResult[ds.id] && (
                            <div className={datasourceRowStatusClass}>{syncResult[ds.id]}</div>
                          )}
                          {testResult[ds.id] && (
                            <div className={datasourceRowStatusClass}>{testResult[ds.id]}</div>
                          )}
                        </td>
                        <td className="actions">
                          <div className={rowActionsClass}>
                            <button
                              type="button"
                              className={legacyButtonClass('btn btn-sm btn-ghost')}
                              onClick={() => edit(ds)}
                            >
                              {t('datasources.edit')}
                            </button>
                            <button
                              type="button"
                              className={legacyButtonClass('btn btn-sm btn-secondary')}
                              onClick={() => {
                                void test(ds.id)
                              }}
                            >
                              {t('datasources.test')}
                            </button>
                            <button
                              type="button"
                              className={legacyButtonClass('btn btn-sm btn-secondary')}
                              onClick={() => {
                                void sync(ds.id)
                              }}
                            >
                              {t('datasources.sync')}
                            </button>
                            <button
                              type="button"
                              className={legacyButtonClass('btn btn-sm btn-danger-outline')}
                              onClick={() => {
                                void del(ds.id)
                              }}
                            >
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
          </>
        )}
      </div>

      <DatasourceFormModal
        open={showForm}
        editingId={editingId}
        connMode={connMode}
        form={form}
        structured={structured}
        draftTestResult={draftTestResult}
        loading={loading}
        canSubmit={canSubmit}
        onClose={closeForm}
        onConnModeChange={setConnMode}
        onFormChange={setForm}
        onStructuredChange={setStructured}
        onDriverChange={setDriver}
        onTest={() => {
          void testDraft()
        }}
        onSave={() => {
          void save()
        }}
      />
    </div>
  )
}

function authRequestOptions(accessToken: string | null) {
  return accessToken ? { headers: { Authorization: `Bearer ${accessToken}` } } : undefined
}
