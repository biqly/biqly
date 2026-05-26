import { useEffect, useMemo, useState } from 'react'
import { listAuditLog } from '../../api/admin'
import { localeLanguageTag, useLocale, useT } from '../../i18n'
import type { AuditLogEntry } from '../../types/auth'
import { Pagination } from '../ui/Pagination'

export function AuditLogPanel({ token }: { token: string }) {
  const t = useT()
  const [locale] = useLocale()
  const [entries, setEntries] = useState<AuditLogEntry[]>([])
  const [userID, setUserID] = useState('')
  const [action, setAction] = useState('')
  const [limit, setLimit] = useState(100)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  // Pagination
  const [currentPage, setCurrentPage] = useState(1)
  const pageSize = 10
  const totalPages = Math.ceil(entries.length / pageSize)
  const displayedEntries = entries.slice((currentPage - 1) * pageSize, currentPage * pageSize)

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

  useEffect(() => {
    setCurrentPage(1)
  }, [entries.length])

  function onSubmit(e: React.FormEvent) {
    e.preventDefault()
    reload()
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', gap: 16, alignItems: 'flex-start' }}>
        <div>
          <h2 style={{ margin: 0, fontSize: 18 }}>{t('admin.audit.title')}</h2>
          <p style={{ margin: '4px 0 0', color: 'var(--text-secondary, #a1a1aa)', fontSize: 13 }}>
            {t('admin.audit.description')}
          </p>
        </div>
        <div style={{ color: 'var(--text-secondary, #a1a1aa)', fontSize: 13 }}>{t('admin.audit.count', { count: entries.length })}</div>
      </div>

      <form onSubmit={onSubmit} style={{ display: 'flex', gap: 12, alignItems: 'flex-end', flexWrap: 'wrap' }}>
        <label style={labelStyle}>
          <span style={labelTextStyle}>{t('admin.fields.user_uuid')}</span>
          <input value={userID} onChange={(e) => setUserID(e.target.value)} placeholder={t('admin.filters.all')} style={inputStyle} />
        </label>
        <label style={labelStyle}>
          <span style={labelTextStyle}>{t('admin.audit.action')}</span>
          <input list="audit-actions" value={action} onChange={(e) => setAction(e.target.value)} placeholder={t('admin.filters.all')} style={inputStyle} />
          <datalist id="audit-actions">
            {actions.map((value) => (
              <option key={value} value={value} />
            ))}
          </datalist>
        </label>
        <label style={labelStyle}>
          <span style={labelTextStyle}>{t('admin.audit.limit')}</span>
          <select value={limit} onChange={(e) => setLimit(Number(e.target.value))} style={selectStyle}>
            <option value={25}>25</option>
            <option value={50}>50</option>
            <option value={100}>100</option>
            <option value={250}>250</option>
            <option value={500}>500</option>
          </select>
        </label>
        <button type="submit" style={primaryButtonStyle}>{t('admin.filters.apply')}</button>
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
          {t('admin.filters.reset')}
        </button>
      </form>

      {loading && <div style={textMuted}>{t('common.loading')}</div>}
      {error && <div style={errStyle}>{t('common.error')}: {error}</div>}
      {!loading && !error && entries.length === 0 && <div style={textMuted}>{t('admin.audit.empty')}</div>}

      {entries.length > 0 && (
        <div style={containerStyle}>
          <div style={{ overflowX: 'auto' }}>
            <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13, minWidth: 980 }}>
              <thead>
                <tr style={theadRow}>
                  <th style={thStyle}>{t('admin.audit.time')}</th>
                  <th style={thStyle}>{t('admin.audit.action')}</th>
                  <th style={thStyle}>{t('admin.fields.user')}</th>
                  <th style={thStyle}>{t('admin.audit.resource')}</th>
                  <th style={thStyle}>IP</th>
                  <th style={thStyle}>Metadata</th>
                </tr>
              </thead>
              <tbody>
                {displayedEntries.map((entry) => (
                  <tr key={entry.id} style={trStyle}>
                    <td style={monoTd}>{formatDate(entry.created_at, localeLanguageTag(locale))}</td>
                    <td style={tdStyle}><span style={actionBadgeStyle}>{entry.action}</span></td>
                    <td style={monoTd}>{entry.user_id || t('admin.audit.system_user')}</td>
                    <td style={monoTd}>{formatResource(entry)}</td>
                    <td style={monoTd}>{entry.ip_address || '-'}</td>
                    <td style={metadataTd}>{formatMetadata(entry.metadata)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <Pagination
            currentPage={currentPage}
            totalPages={totalPages}
            onPageChange={setCurrentPage}
            totalItems={entries.length}
            itemsPerPage={pageSize}
          />
        </div>
      )}
    </div>
  )
}

function formatDate(value: string, languageTag: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString(languageTag)
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
const labelTextStyle: React.CSSProperties = { fontSize: 12, color: 'var(--text-secondary, #a1a1aa)' }
const inputStyle: React.CSSProperties = {
  padding: '8px 12px',
  border: '1px solid var(--border, rgba(255, 255, 255, 0.06))',
  borderRadius: 6,
  minWidth: 220,
  background: 'var(--input-bg, #fff)',
  color: 'var(--text-primary, #111)',
  fontSize: 14,
}
const selectStyle: React.CSSProperties = {
  ...inputStyle,
  minWidth: 120,
}
const primaryButtonStyle: React.CSSProperties = {
  padding: '8px 16px',
  background: 'var(--accent, #4f46e5)',
  color: 'white',
  border: 0,
  borderRadius: 6,
  cursor: 'pointer',
  fontSize: 13,
  fontWeight: 500,
}
const secondaryButtonStyle: React.CSSProperties = {
  padding: '8px 16px',
  background: 'transparent',
  border: '1px solid var(--border, rgba(255, 255, 255, 0.06))',
  color: 'var(--text-secondary, #a1a1aa)',
  borderRadius: 6,
  cursor: 'pointer',
  fontSize: 13,
}
const containerStyle: React.CSSProperties = {
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
  verticalAlign: 'top',
  color: 'var(--text-primary, #f4f4f5)',
}
const monoTd: React.CSSProperties = {
  ...tdStyle,
  fontFamily: 'var(--font-mono, monospace)',
  fontSize: 12,
}
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
  background: 'var(--accent-glow, rgba(99, 102, 241, 0.15))',
  border: '1px solid var(--border, rgba(255, 255, 255, 0.06))',
  color: 'var(--accent, #6366f1)',
  borderRadius: 4,
  padding: '2px 6px',
  fontFamily: 'var(--font-mono, monospace)',
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

