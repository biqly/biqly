import { useEffect, useMemo, useState } from 'react'
import { listAuditLog, listUsers, listWorkspaces } from '../../api/admin'
import { localeLanguageTag, useLocale, useT } from '../../i18n'
import type { AuditLogEntry, AuthUser, Workspace } from '../../types/auth'
import type { Datasource } from '../../types/metadata'
import { Pagination } from '../ui/Pagination'

const COMMON_ACTIONS = [
  'login.success',
  'login.failed',
  'login.locked',
  'login.mfa_required',
  'logout',
  'user.register',
  'password.reset',
  'password.change',
  'email.verified',
  'session.refresh',
  'session.revoked',
  'oauth.login',
  'mfa.enrolled',
  'mfa.verified',
  'mfa.disabled',
  'role.assigned',
  'role.removed',
  'datasource.grant',
  'datasource.revoke',
  'datasource.update_level',
  'datasource.request_access',
  'share.create',
  'share.revoke',
  'audit.export',
  'user.data_export',
  'admin.blocked_self_change',
  'account.frozen',
  'account.unfrozen',
  'account.soft_deleted',
  'account.restored',
  'account.purged',
  'account.unlocked',
  'login.new_device',
  'session.evicted',
  'admin.force_logout',
  'password.expired'
].sort()

export const DEFAULT_AUDIT_PAGE_SIZE = 10
export const AUDIT_PAGE_SIZE_OPTIONS = [10, 25, 50, 100, 250]

export function AuditLogPanel({ token }: { token: string }) {
  const t = useT()
  const [locale] = useLocale()
  const [entries, setEntries] = useState<AuditLogEntry[]>([])
  const [userID, setUserID] = useState('')
  const [action, setAction] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  // Lookups for friendly name mapping
  const [users, setUsers] = useState<AuthUser[]>([])
  const [datasources, setDatasources] = useState<Datasource[]>([])
  const [workspaces, setWorkspaces] = useState<Workspace[]>([])

  // Pagination
  const [currentPage, setCurrentPage] = useState(1)
  const [pageSize, setPageSize] = useState(DEFAULT_AUDIT_PAGE_SIZE)
  const [totalItems, setTotalItems] = useState(0)
  const totalPages = Math.ceil(totalItems / pageSize)
  const displayedEntries = entries

  const userMap = useMemo(() => new Map(users.map((u) => [u.id, u.email])), [users])
  const dsMap = useMemo(() => new Map(datasources.map((d) => [d.id, d.name])), [datasources])
  const wsMap = useMemo(() => new Map(workspaces.map((w) => [w.id, w.name])), [workspaces])

  async function reload(nextFilters = { userID, action, page: currentPage, pageSize }) {
    setLoading(true)
    try {
      const res = await listAuditLog(token, {
        userID: nextFilters.userID,
        action: nextFilters.action,
        page: nextFilters.page,
        pageSize: nextFilters.pageSize,
      })
      setEntries(res.entries || [])
      setTotalItems(res.total || 0)
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
        if (!cancelled) setUsers(uRes.users || [])

        const dsRes = await fetch('/api/datasources', { headers: { Authorization: `Bearer ${token}` } })
        if (dsRes.ok && !cancelled) {
          const dsData = await dsRes.json()
          setDatasources(dsData || [])
        }

        const wsRes = await listWorkspaces(token)
        if (!cancelled) setWorkspaces(wsRes.workspaces || [])
      } catch (e) {
        console.error('Failed to load lookups in AuditLogPanel', e)
      }
    }
    loadLookups()
    return () => {
      cancelled = true
    }
  }, [token])

  useEffect(() => {
    reload({ userID, action, page: currentPage, pageSize })
  }, [token, currentPage])

  function onSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (currentPage !== 1) {
      setCurrentPage(1)
    } else {
      reload({ userID, action, page: 1, pageSize })
    }
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
        <div style={{ color: 'var(--text-secondary, #a1a1aa)', fontSize: 13 }}>{t('admin.audit.count', { count: totalItems })}</div>
      </div>

      <form onSubmit={onSubmit} style={{ display: 'flex', gap: 12, alignItems: 'flex-end', flexWrap: 'wrap' }}>
        <label style={labelStyle}>
          <span style={labelTextStyle}>{t('admin.fields.user')}</span>
          <select value={userID} onChange={(e) => setUserID(e.target.value)} style={selectStyle}>
            <option value="">{t('admin.filters.all')}</option>
            {users.map((u) => (
              <option key={u.id} value={u.id}>
                {u.email} {u.displayName ? `(${u.displayName})` : ''}
              </option>
            ))}
          </select>
        </label>
        <label style={labelStyle}>
          <span style={labelTextStyle}>{t('admin.audit.action')}</span>
          <select value={action} onChange={(e) => setAction(e.target.value)} style={selectStyle}>
            <option value="">{t('admin.filters.all')}</option>
            {COMMON_ACTIONS.map((act) => (
              <option key={act} value={act}>{act}</option>
            ))}
          </select>
        </label>
        <label style={labelStyle}>
          <span style={labelTextStyle}>{t('admin.audit.page_size')}</span>
          <select
            value={pageSize}
            onChange={(e) => {
              const nextSize = Number(e.target.value)
              setPageSize(nextSize)
              setCurrentPage(1)
              reload({ userID, action, page: 1, pageSize: nextSize })
            }}
            style={selectStyle}
          >
            {AUDIT_PAGE_SIZE_OPTIONS.map((size) => (
              <option key={size} value={size}>{size}</option>
            ))}
          </select>
        </label>
        <button type="submit" style={primaryButtonStyle}>{t('admin.filters.apply')}</button>
        <button
          type="button"
          onClick={() => {
            setUserID('')
            setAction('')
            setPageSize(DEFAULT_AUDIT_PAGE_SIZE)
            setCurrentPage(1)
            reload({ userID: '', action: '', page: 1, pageSize: DEFAULT_AUDIT_PAGE_SIZE })
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
                    <td style={monoTd}>{entry.user_id ? (userMap.get(entry.user_id) || entry.user_id) : t('admin.audit.system_user')}</td>
                    <td style={monoTd}>{formatResource(entry, dsMap, wsMap)}</td>
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
            totalItems={totalItems}
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

function formatResource(entry: AuditLogEntry, dsMap: Map<string, string>, wsMap: Map<string, string>) {
  if (!entry.resource && !entry.resource_id) return '-'
  if (entry.resource === 'datasource' && entry.resource_id && dsMap.has(entry.resource_id)) {
    return `datasource:${dsMap.get(entry.resource_id)}`
  }
  if (entry.resource === 'workspace' && entry.resource_id && wsMap.has(entry.resource_id)) {
    return `workspace:${wsMap.get(entry.resource_id)}`
  }
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
