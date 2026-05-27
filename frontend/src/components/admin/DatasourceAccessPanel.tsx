import { useEffect, useState } from 'react'
import {
  grantDatasourceAccess,
  listDatasourceAccess,
  revokeDatasourceAccess,
  updateDatasourceAccess,
  listUsers,
} from '../../api/admin'
import { localeLanguageTag, useLocale, useT } from '../../i18n'
import type { DatasourceAccess, AuthUser } from '../../types/auth'
import type { Datasource } from '../../types/metadata'
import { Pagination } from '../ui/Pagination'

export function DatasourceAccessPanel({ token }: { token: string }) {
  const t = useT()
  const [locale] = useLocale()
  const [rows, setRows] = useState<DatasourceAccess[]>([])
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  const [userID, setUserID] = useState('')
  const [datasourceID, setDatasourceID] = useState('')
  const [level, setLevel] = useState<'read' | 'write' | 'admin'>('read')

  // Lookups for friendly name mapping
  const [users, setUsers] = useState<AuthUser[]>([])
  const [datasources, setDatasources] = useState<Datasource[]>([])

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

  // Load lookup metadata
  useEffect(() => {
    let cancelled = false
    async function loadLookups() {
      try {
        const uRes = await listUsers(token)
        if (!cancelled) setUsers(uRes.users)

        const dsRes = await fetch('/api/datasources', { headers: { Authorization: `Bearer ${token}` } })
        if (dsRes.ok && !cancelled) {
          const dsData = await dsRes.json()
          setDatasources(dsData)
        }
      } catch (e) {
        console.error('Failed to load lookups in DatasourceAccessPanel', e)
      }
    }
    loadLookups()
    return () => {
      cancelled = true
    }
  }, [token])

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
    if (!confirm(t('admin.datasource_access.confirm_revoke'))) return
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
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      <h2 style={{ marginTop: 0, fontSize: 18 }}>{t('admin.datasource_access.title')}</h2>

      <form onSubmit={onGrant} style={{ display: 'flex', gap: 12, alignItems: 'flex-end', flexWrap: 'wrap' }}>
        <label style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
          <span style={{ fontSize: 12, color: 'var(--text-secondary, #a1a1aa)' }}>{t('admin.fields.user')}</span>
          <select value={userID} onChange={(e) => setUserID(e.target.value)} style={selectStyle} required>
            <option value="">{t('evaluation.placeholder_select')}</option>
            {users.map((u) => (
              <option key={u.id} value={u.id}>{u.email}</option>
            ))}
          </select>
        </label>
        <label style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
          <span style={{ fontSize: 12, color: 'var(--text-secondary, #a1a1aa)' }}>Datasource</span>
          <select value={datasourceID} onChange={(e) => setDatasourceID(e.target.value)} style={selectStyle} required>
            <option value="">{t('evaluation.placeholder_select')}</option>
            {datasources.map((d) => (
              <option key={d.id} value={d.id}>{d.name}</option>
            ))}
          </select>
        </label>
        <label style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
          <span style={{ fontSize: 12, color: 'var(--text-secondary, #a1a1aa)' }}>{t('admin.datasource_access.level')}</span>
          <select value={level} onChange={(e) => setLevel(e.target.value as 'read' | 'write' | 'admin')} style={selectStyle}>
            <option value="read">read</option>
            <option value="write">write</option>
            <option value="admin">admin</option>
          </select>
        </label>
        <button type="submit" style={btnPrimary}>
          {t('admin.datasource_access.grant')}
        </button>
      </form>

      {loading && <div style={textMuted}>{t('common.loading')}</div>}
      {error && <div style={errStyle}>{t('common.error')}: {error}</div>}

      <div style={tableContainer}>
        <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 14 }}>
          <thead>
            <tr style={theadRow}>
              <th style={thStyle}>{t('admin.fields.user')}</th>
              <th style={thStyle}>Datasource</th>
              <th style={thStyle}>{t('admin.datasource_access.level')}</th>
              <th style={thStyle}>{t('admin.datasource_access.granted_at')}</th>
              <th style={thStyle}></th>
            </tr>
          </thead>
          <tbody>
            {rows.length === 0 ? (
              <tr>
                <td colSpan={5} style={{ padding: 24, textAlign: 'center', color: '#9ca3af' }}>
                  —
                </td>
              </tr>
            ) : (
              rows.map((r) => {
                const userObj = users.find((u) => u.id === r.user_id)
                const dsObj = datasources.find((d) => d.id === r.datasource_id)
                return (
                  <tr key={r.id} style={trStyle}>
                    <td style={tdStyle}>{userObj ? userObj.email : r.user_id}</td>
                    <td style={tdStyle}>{dsObj ? dsObj.name : r.datasource_id}</td>
                    <td style={tdStyle}>
                      <select
                        value={r.access_level}
                        onChange={(e) => onChangeLevel(r.id, e.target.value as 'read' | 'write' | 'admin')}
                        style={{ ...selectStyleSmall, ...getLevelStyle(r.access_level) }}
                      >
                        <option value="read" style={{ background: 'var(--bg-card)', color: 'var(--success)' }}>read</option>
                        <option value="write" style={{ background: 'var(--bg-card)', color: 'var(--warning)' }}>write</option>
                        <option value="admin" style={{ background: 'var(--bg-card)', color: 'var(--error)' }}>admin</option>
                      </select>
                    </td>
                    <td style={tdStyle}>{new Date(r.granted_at).toLocaleString(localeLanguageTag(locale))}</td>
                    <td style={{ ...tdStyle, textAlign: 'right' }}>
                      <button onClick={() => onRevoke(r.user_id, r.datasource_id)} style={btnSecondary}>{t('common.delete')}</button>
                    </td>
                  </tr>
                )
              })
            )}
          </tbody>
        </table>
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

const inputStyle: React.CSSProperties = {
  padding: '8px 12px',
  border: '1px solid var(--border, rgba(255, 255, 255, 0.06))',
  borderRadius: 6,
  fontSize: 14,
  minWidth: 240,
  background: 'var(--input-bg, #fff)',
  color: 'var(--text-primary, #111)',
}

const selectStyle: React.CSSProperties = {
  padding: '8px 12px',
  border: '1px solid var(--border, rgba(255, 255, 255, 0.06))',
  borderRadius: 6,
  fontSize: 14,
  minWidth: 240,
  background: 'var(--input-bg, #fff)',
  color: 'var(--text-primary, #111)',
}

const selectStyleSmall: React.CSSProperties = {
  padding: '4px 8px',
  border: '1px solid var(--border, rgba(255, 255, 255, 0.06))',
  borderRadius: 4,
  fontSize: 12,
  background: 'var(--input-bg, #fff)',
  color: 'var(--text-primary, #111)',
}

const btnPrimary: React.CSSProperties = {
  padding: '8px 16px',
  background: 'var(--accent, #4f46e5)',
  color: 'white',
  border: 0,
  borderRadius: 6,
  cursor: 'pointer',
  fontSize: 13,
  fontWeight: 500,
}

const tableContainer: React.CSSProperties = {
  background: 'var(--bg-card, #ffffff)',
  border: '1px solid var(--border, rgba(255, 255, 255, 0.06))',
  borderRadius: 8,
  overflow: 'hidden',
  boxShadow: 'var(--shadow-sm, 0 1px 3px rgba(0,0,0,0.05))',
}

const theadRow: React.CSSProperties = {
  background: 'var(--table-header-bg, #f9fafb)',
  borderBottom: '1px solid var(--border, rgba(255, 255, 255, 0.06))',
  textAlign: 'left',
}

const thStyle: React.CSSProperties = {
  padding: '12px 16px',
  fontWeight: 600,
  color: 'var(--table-header-fg, #4b5563)',
}

const trStyle: React.CSSProperties = {
  borderBottom: '1px solid var(--border, rgba(255, 255, 255, 0.06))',
}

const tdStyle: React.CSSProperties = {
  padding: '12px 16px',
  fontFamily: 'var(--font-mono, monospace)',
  fontSize: 12,
  color: 'var(--text-primary, #f4f4f5)',
}

const btnSecondary: React.CSSProperties = {
  padding: '4px 8px',
  background: 'transparent',
  border: '1px solid var(--border, rgba(255, 255, 255, 0.06))',
  color: 'var(--error, crimson)',
  borderRadius: 4,
  cursor: 'pointer',
  fontSize: 12,
}

const textMuted: React.CSSProperties = {
  color: 'var(--text-secondary, #8a8a92)',
  fontSize: 14,
  padding: 16,
}

const errStyle: React.CSSProperties = {
  color: 'var(--error, crimson)',
  padding: 16,
  fontWeight: 600,
}

function getLevelStyle(lvl: string): React.CSSProperties {
  switch (lvl) {
    case 'admin':
      return { background: 'rgba(239, 68, 68, 0.12)', color: 'var(--error, #ef4444)', fontWeight: 600 }
    case 'write':
      return { background: 'rgba(245, 158, 11, 0.14)', color: 'var(--warning, #f59e0b)', fontWeight: 600 }
    case 'read':
    default:
      return { background: 'rgba(16, 185, 129, 0.12)', color: 'var(--success, #10b981)', fontWeight: 600 }
  }
}
