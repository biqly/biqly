import { useEffect, useState } from 'react'
import {
  grantDatasourceAccess,
  listDatasourceAccess,
  revokeDatasourceAccess,
  updateDatasourceAccess,
} from '../../api/admin'
import { localeLanguageTag, useLocale, useT } from '../../i18n'
import type { DatasourceAccess } from '../../types/auth'

export function DatasourceAccessPanel({ token }: { token: string }) {
  const t = useT()
  const [locale] = useLocale()
  const [rows, setRows] = useState<DatasourceAccess[]>([])
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  const [userID, setUserID] = useState('')
  const [datasourceID, setDatasourceID] = useState('')
  const [level, setLevel] = useState<'read' | 'write' | 'admin'>('read')

  async function reload() {
    setLoading(true)
    try {
      const r = await listDatasourceAccess(token)
      setRows(r)
      setError(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    reload()
  }, [token])

  async function onGrant(e: React.FormEvent) {
    e.preventDefault()
    if (!userID || !datasourceID) return
    try {
      await grantDatasourceAccess(token, userID, datasourceID, level)
      setUserID('')
      setDatasourceID('')
      reload()
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }

  async function onRevoke(uid: string, dsid: string) {
    if (!confirm(t('admin.datasource_access.confirm_revoke'))) return
    try {
      await revokeDatasourceAccess(token, uid, dsid)
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
      <h2 style={{ marginTop: 0 }}>{t('admin.datasource_access.title')}</h2>

      <form onSubmit={onGrant} style={{ display: 'flex', gap: 8, alignItems: 'flex-end' }}>
        <label style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
          <span style={{ fontSize: 12, color: '#6b7280' }}>{t('admin.fields.user_uuid')}</span>
          <input value={userID} onChange={(e) => setUserID(e.target.value)} style={inputStyle} />
        </label>
        <label style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
          <span style={{ fontSize: 12, color: '#6b7280' }}>Datasource UUID</span>
          <input value={datasourceID} onChange={(e) => setDatasourceID(e.target.value)} style={inputStyle} />
        </label>
        <label style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
          <span style={{ fontSize: 12, color: '#6b7280' }}>{t('admin.datasource_access.level')}</span>
          <select value={level} onChange={(e) => setLevel(e.target.value as 'read' | 'write' | 'admin')} style={inputStyle}>
            <option value="read">read</option>
            <option value="write">write</option>
            <option value="admin">admin</option>
          </select>
        </label>
        <button type="submit" style={{ padding: '8px 14px', background: '#4f46e5', color: 'white', border: 0, borderRadius: 4, cursor: 'pointer' }}>
          {t('admin.datasource_access.grant')}
        </button>
      </form>

      {loading && <div>{t('common.loading')}</div>}
      {error && <div style={{ color: 'crimson' }}>{t('common.error')}: {error}</div>}

      <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 14 }}>
        <thead>
          <tr style={{ textAlign: 'left' }}>
            <th style={th}>{t('admin.fields.user')}</th>
            <th style={th}>Datasource</th>
            <th style={th}>{t('admin.datasource_access.level')}</th>
            <th style={th}>{t('admin.datasource_access.granted_at')}</th>
            <th style={th}></th>
          </tr>
        </thead>
        <tbody>
          {rows.map((r) => (
            <tr key={r.id}>
              <td style={td}>{r.user_id}</td>
              <td style={td}>{r.datasource_id}</td>
              <td style={td}>
                <select value={r.access_level} onChange={(e) => onChangeLevel(r.id, e.target.value as 'read' | 'write' | 'admin')} style={inputStyle}>
                  <option value="read">read</option>
                  <option value="write">write</option>
                  <option value="admin">admin</option>
                </select>
              </td>
              <td style={td}>{new Date(r.granted_at).toLocaleString(localeLanguageTag(locale))}</td>
              <td style={td}>
                <button onClick={() => onRevoke(r.user_id, r.datasource_id)} style={btnSecondary}>{t('common.delete')}</button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

const inputStyle: React.CSSProperties = {
  padding: 8,
  border: '1px solid #d1d5db',
  borderRadius: 4,
  minWidth: 240,
}
const th: React.CSSProperties = { borderBottom: '1px solid #e5e7eb', padding: 6 }
const td: React.CSSProperties = { borderBottom: '1px solid #f3f4f6', padding: 6, fontFamily: 'monospace', fontSize: 12 }
const btnSecondary: React.CSSProperties = { padding: '4px 8px', background: 'transparent', border: '1px solid #d1d5db', borderRadius: 4, cursor: 'pointer' }
