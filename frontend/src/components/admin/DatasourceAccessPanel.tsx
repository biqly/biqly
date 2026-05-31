import { useEffect, useState } from 'react'
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
        <label className="admin-form-label" style={{ gap: 4 }}>
          <span className="admin-label-text">{t('admin.fields.user')}</span>
          <select value={userID} onChange={(e) => setUserID(e.target.value)} className="admin-select" style={{ minWidth: 240 }} required>
            <option value="">{t('evaluation.placeholder_select')}</option>
            {users.map((u) => (
              <option key={u.id} value={u.id}>{u.email}</option>
            ))}
          </select>
        </label>
        <label className="admin-form-label" style={{ gap: 4 }}>
          <span className="admin-label-text">Datasource</span>
          <select value={datasourceID} onChange={(e) => setDatasourceID(e.target.value)} className="admin-select" style={{ minWidth: 240 }} required>
            <option value="">{t('evaluation.placeholder_select')}</option>
            {datasources.map((d) => (
              <option key={d.id} value={d.id}>{d.name}</option>
            ))}
          </select>
        </label>
        <label className="admin-form-label" style={{ gap: 4 }}>
          <span className="admin-label-text">{t('admin.datasource_access.level')}</span>
          <select value={level} onChange={(e) => setLevel(e.target.value as 'read' | 'write' | 'admin')} className="admin-select" style={{ minWidth: 240 }}>
            <option value="read">read</option>
            <option value="write">write</option>
            <option value="admin">admin</option>
          </select>
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
                          <select
                            value={r.access_level}
                            onChange={(e) => onChangeLevel(r.id, e.target.value as 'read' | 'write' | 'admin')}
                            className={`admin-select-small admin-level-${r.access_level}`}
                          >
                            <option value="read" style={{ background: 'var(--bg-card)', color: 'var(--success)' }}>read</option>
                            <option value="write" style={{ background: 'var(--bg-card)', color: 'var(--warning)' }}>write</option>
                            <option value="admin" style={{ background: 'var(--bg-card)', color: 'var(--error)' }}>admin</option>
                          </select>
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

