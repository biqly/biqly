import { useEffect, useMemo, useState } from 'react'
import {
  grantDatasourceAccess,
  listDatasourceAccess,
  revokeDatasourceAccess,
  updateDatasourceAccess,
} from '../../api/admin'
import { localeLanguageTag, useLocale, useT } from '../../i18n'
import type { DatasourceAccess } from '../../types/auth'
import { Pagination } from '../ui/Pagination'
import { LoadingOverlay } from '../ui/LoadingOverlay'
import { useConfirm } from '../../hooks/useConfirm'
import { useAdminLookups } from '../../hooks/useAdminLookups'
import { Select } from '../ui/Select'
import {
  datasourceAccessLevelOptions,
  datasourcePickerOptions,
  userSelectOptions,
} from './adminSelectOptions'
import type { DatasourceAccessLevel } from './adminSelectOptions'

export function DatasourceAccessPanel({ token }: { token: string }) {
  const t = useT()
  const [locale] = useLocale()
  const confirm = useConfirm()
  const [rows, setRows] = useState<DatasourceAccess[]>([])
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  const [userID, setUserID] = useState('')
  const [datasourceID, setDatasourceID] = useState('')
  const [level, setLevel] = useState<'read' | 'write' | 'admin'>('read')

  // Lookups for friendly name mapping using custom hook
  const { users, datasources } = useAdminLookups(token)

  // Pagination
  const [currentPage, setCurrentPage] = useState(1)
  const pageSize = 10
  const [totalItems, setTotalItems] = useState(0)
  const totalPages = Math.ceil(totalItems / pageSize)

  const userOptions = useMemo(
    () => userSelectOptions(users, t('evaluation.placeholder_select')),
    [users, t],
  )
  const dsOptions = useMemo(
    () => datasourcePickerOptions(datasources, t('evaluation.placeholder_select')),
    [datasources, t],
  )
  const levelOptions = useMemo(() => datasourceAccessLevelOptions(), [])

  async function reload() {
    setLoading(true)
    try {
      const res = await listDatasourceAccess(token, currentPage, pageSize)
      setRows(res.access)
      setTotalItems(res.total)
      setError(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    reload()
  }, [token, currentPage])

  async function onGrant(e: React.SubmitEvent<HTMLFormElement>) {
    e.preventDefault()
    if (!userID || !datasourceID) return
    try {
      await grantDatasourceAccess(token, userID, datasourceID, level)
      setUserID('')
      setDatasourceID('')
      setCurrentPage(1)
      reload()
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }

  async function onRevoke(uid: string, dsid: string) {
    const ok = await confirm({
      title: t('admin.datasource_access.confirm_revoke'),
      variant: 'danger',
    })
    if (!ok) return
    try {
      await revokeDatasourceAccess(token, uid, dsid)
      setCurrentPage(1)
      reload()
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }


  async function onChangeLevel(id: string, newLevel: 'read' | 'write' | 'admin') {
    try {
      await updateDatasourceAccess(token, id, newLevel)
      reload()
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }

  return (
    <div className="page-stack">
      <h2 style={{ marginTop: 0, fontSize: 18 }}>{t('admin.datasource_access.title')}</h2>

      <form onSubmit={onGrant} style={{ display: 'flex', gap: 12, alignItems: 'flex-end', flexWrap: 'wrap' }}>
        <label className="admin-form-label" style={{ gap: 4, minWidth: 240 }}>
          <span className="admin-label-text">{t('admin.fields.user')}</span>
          <Select value={userID} options={userOptions} onChange={setUserID} placeholder={t('evaluation.placeholder_select')} />
        </label>
        <label className="admin-form-label" style={{ gap: 4, minWidth: 240 }}>
          <span className="admin-label-text">Datasource</span>
          <Select
            value={datasourceID}
            options={dsOptions}
            onChange={setDatasourceID}
            placeholder={t('evaluation.placeholder_select')}
          />
        </label>
        <label className="admin-form-label" style={{ gap: 4, minWidth: 240 }}>
          <span className="admin-label-text">{t('admin.datasource_access.level')}</span>
          <Select
            value={level}
            options={levelOptions}
            onChange={(v) => setLevel(v as DatasourceAccessLevel)}
          />
        </label>
        <button type="submit" className="admin-btn-primary">
          {t('admin.datasource_access.grant')}
        </button>
      </form>

      {error && <div className="admin-err-text">{t('common.error')}: {error}</div>}

      <div className="admin-table-container">
        <LoadingOverlay loading={loading}>
          <div style={{ minHeight: rows.length === 0 && loading ? 120 : 'auto', display: 'flex', flexDirection: 'column' }}>
            <table className="admin-table">
              <thead>
                <tr className="admin-thead-row">
                  <th className="admin-th">{t('admin.fields.user')}</th>
                  <th className="admin-th">Datasource</th>
                  <th className="admin-th">{t('admin.datasource_access.level')}</th>
                  <th className="admin-th">{t('admin.datasource_access.granted_at')}</th>
                  <th className="admin-th"></th>
                </tr>
              </thead>
              <tbody>
                {rows.length === 0 ? (
                  <tr className="admin-tr">
                    <td colSpan={5} style={{ padding: 24, textAlign: 'center', color: '#9ca3af' }}>
                      {loading ? '' : '—'}
                    </td>
                  </tr>
                ) : (
                  rows.map((r) => {
                    const userObj = users.find((u) => u.id === r.user_id)
                    const dsObj = datasources.find((d) => d.id === r.datasource_id)
                    return (
                      <tr key={r.id} className="admin-tr">
                        <td className="admin-td-mono">{userObj ? userObj.email : r.user_id}</td>
                        <td className="admin-td-mono">{dsObj ? dsObj.name : r.datasource_id}</td>
                        <td className="admin-td">
                          <Select
                            size="sm"
                            value={r.access_level}
                            options={levelOptions}
                            onChange={(v) => onChangeLevel(r.id, v as DatasourceAccessLevel)}
                            className={`admin-level-${r.access_level}`}
                          />
                        </td>
                        <td className="admin-td">{new Date(r.granted_at).toLocaleString(localeLanguageTag(locale))}</td>
                        <td className="admin-td" style={{ textAlign: 'right' }}>
                          <button onClick={() => onRevoke(r.user_id, r.datasource_id)} className="admin-btn-secondary">{t('common.delete')}</button>
                        </td>
                      </tr>
                    )
                  })
                )}
              </tbody>
            </table>
          </div>
        </LoadingOverlay>
        <Pagination
          currentPage={currentPage}
          totalPages={totalPages}
          onPageChange={setCurrentPage}
          totalItems={totalItems}
          itemsPerPage={pageSize}
        />
      </div>
    </div>
  )
}

