import { useEffect, useMemo, useState } from 'react'
import { listAuditLog } from '../../api/admin'
import type { AuditLogEntry } from '../../types/auth'

export function AuditLogPanel({ token }: { token: string }) {
  const [entries, setEntries] = useState<AuditLogEntry[]>([])
  const [userID, setUserID] = useState('')
  const [action, setAction] = useState('')
  const [limit, setLimit] = useState(100)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  const actions = useMemo(() => {
    return Array.from(new Set(entries.map((entry) => entry.action))).sort()
  }, [entries])

  async function reload(nextFilters = { userID, action, limit }) {
    setLoading(true)
    try {
      const rows = await listAuditLog(token, nextFilters)
      setEntries(rows)
      setError(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    reload({ userID: '', action: '', limit })
  }, [token])

  function onSubmit(e: React.FormEvent) {
    e.preventDefault()
    reload()
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', gap: 16, alignItems: 'flex-start' }}>
        <div>
          <h2 style={{ margin: 0 }}>Denetim Günlüğü</h2>
          <p style={{ margin: '4px 0 0', color: '#6b7280', fontSize: 13 }}>
            Auth olayları, kaynak değişiklikleri ve yönetim aksiyonları.
          </p>
        </div>
        <div style={{ color: '#6b7280', fontSize: 13 }}>{entries.length} kayıt</div>
      </div>

      <form onSubmit={onSubmit} style={{ display: 'flex', gap: 8, alignItems: 'flex-end', flexWrap: 'wrap' }}>
        <label style={labelStyle}>
          <span style={labelTextStyle}>Kullanıcı UUID</span>
          <input value={userID} onChange={(e) => setUserID(e.target.value)} placeholder="Tümü" style={inputStyle} />
        </label>
        <label style={labelStyle}>
          <span style={labelTextStyle}>Aksiyon</span>
          <input list="audit-actions" value={action} onChange={(e) => setAction(e.target.value)} placeholder="Tümü" style={inputStyle} />
          <datalist id="audit-actions">
            {actions.map((value) => (
              <option key={value} value={value} />
            ))}
          </datalist>
        </label>
        <label style={labelStyle}>
          <span style={labelTextStyle}>Limit</span>
          <select value={limit} onChange={(e) => setLimit(Number(e.target.value))} style={{ ...inputStyle, minWidth: 120 }}>
            <option value={25}>25</option>
            <option value={50}>50</option>
            <option value={100}>100</option>
            <option value={250}>250</option>
            <option value={500}>500</option>
          </select>
        </label>
        <button type="submit" style={primaryButtonStyle}>Filtrele</button>
        <button
          type="button"
          onClick={() => {
            setUserID('')
            setAction('')
            setLimit(100)
            reload({ userID: '', action: '', limit: 100 })
          }}
          style={secondaryButtonStyle}
        >
          Sıfırla
        </button>
      </form>

      {loading && <div>Yükleniyor…</div>}
      {error && <div style={{ color: 'crimson' }}>Hata: {error}</div>}
      {!loading && !error && entries.length === 0 && <div style={{ color: '#6b7280' }}>Denetim kaydı bulunamadı.</div>}

      {entries.length > 0 && (
        <div style={{ overflowX: 'auto', border: '1px solid #e5e7eb', borderRadius: 6 }}>
          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13, minWidth: 980 }}>
            <thead>
              <tr style={{ textAlign: 'left', background: '#f9fafb' }}>
                <th style={th}>Zaman</th>
                <th style={th}>Aksiyon</th>
                <th style={th}>Kullanıcı</th>
                <th style={th}>Kaynak</th>
                <th style={th}>IP</th>
                <th style={th}>Metadata</th>
              </tr>
            </thead>
            <tbody>
              {entries.map((entry) => (
                <tr key={entry.id}>
                  <td style={monoTd}>{formatDate(entry.created_at)}</td>
                  <td style={td}><span style={actionBadgeStyle}>{entry.action}</span></td>
                  <td style={monoTd}>{entry.user_id || 'system'}</td>
                  <td style={monoTd}>{formatResource(entry)}</td>
                  <td style={monoTd}>{entry.ip_address || '-'}</td>
                  <td style={metadataTd}>{formatMetadata(entry.metadata)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}

function formatDate(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString()
}

function formatResource(entry: AuditLogEntry) {
  if (!entry.resource && !entry.resource_id) return '-'
  if (!entry.resource_id) return entry.resource
  if (!entry.resource) return entry.resource_id
  return `${entry.resource}:${entry.resource_id}`
}

function formatMetadata(value: unknown) {
  if (value === undefined || value === null) return '-'
  if (typeof value === 'string') return value
  return JSON.stringify(value)
}

const labelStyle: React.CSSProperties = { display: 'flex', flexDirection: 'column', gap: 4 }
const labelTextStyle: React.CSSProperties = { fontSize: 12, color: '#6b7280' }
const inputStyle: React.CSSProperties = {
  padding: 8,
  border: '1px solid #d1d5db',
  borderRadius: 4,
  minWidth: 220,
}
const primaryButtonStyle: React.CSSProperties = {
  padding: '8px 14px',
  background: '#4f46e5',
  color: 'white',
  border: 0,
  borderRadius: 4,
  cursor: 'pointer',
}
const secondaryButtonStyle: React.CSSProperties = {
  padding: '8px 14px',
  background: 'transparent',
  border: '1px solid #d1d5db',
  borderRadius: 4,
  cursor: 'pointer',
}
const th: React.CSSProperties = { borderBottom: '1px solid #e5e7eb', padding: 8 }
const td: React.CSSProperties = { borderBottom: '1px solid #f3f4f6', padding: 8, verticalAlign: 'top' }
const monoTd: React.CSSProperties = { ...td, fontFamily: 'monospace', fontSize: 12 }
const metadataTd: React.CSSProperties = {
  ...monoTd,
  maxWidth: 360,
  overflow: 'hidden',
  textOverflow: 'ellipsis',
  whiteSpace: 'nowrap',
}
const actionBadgeStyle: React.CSSProperties = {
  display: 'inline-flex',
  alignItems: 'center',
  border: '1px solid #dbeafe',
  background: '#eff6ff',
  color: '#1d4ed8',
  borderRadius: 4,
  padding: '2px 6px',
  fontFamily: 'monospace',
  fontSize: 12,
}
