import React, { useEffect, useState } from 'react'
import { listUsers } from '../../api/admin'
import { localeLanguageTag, useLocale, useT } from '../../i18n'
import type { AuthUser } from '../../types/auth'

interface UserListPageProps {
  token: string
  onSelectUser: (id: string) => void
}

export function UserListPage({ token, onSelectUser }: UserListPageProps) {
  const t = useT()
  const [locale] = useLocale()
  const [users, setUsers] = useState<AuthUser[]>([])
  const [search, setSearch] = useState('')
  const [statusFilter, setStatusFilter] = useState<'all' | 'active' | 'inactive'>('all')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    async function load() {
      try {
        setLoading(true)
        const u = await listUsers(token)
        if (!cancelled) {
          setUsers(u)
          setError(null)
        }
      } catch (e) {
        if (!cancelled) setError(e instanceof Error ? e.message : String(e))
      } finally {
        if (!cancelled) setLoading(false)
      }
    }
    load()
    return () => {
      cancelled = true
    }
  }, [token])

  const filteredUsers = users.filter((u) => {
    const matchesSearch =
      u.email.toLowerCase().includes(search.toLowerCase()) ||
      (u.displayName && u.displayName.toLowerCase().includes(search.toLowerCase())) ||
      (u.username && u.username.toLowerCase().includes(search.toLowerCase()))

    const matchesStatus =
      statusFilter === 'all' ||
      (statusFilter === 'active' && u.isActive) ||
      (statusFilter === 'inactive' && !u.isActive)

    return matchesSearch && matchesStatus
  })

  if (loading) return <div style={textMuted}>{t('admin.users.loading')}</div>
  if (error) return <div style={errStyle}>{t('common.error')}: {error}</div>

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <h2 style={{ margin: 0, fontSize: 20 }}>{t('admin.users.title')}</h2>
        <span style={countBadge}>{t('admin.users.count', { count: filteredUsers.length })}</span>
      </div>

      <div style={{ display: 'flex', gap: 12, alignItems: 'center', flexWrap: 'wrap' }}>
        <input
          type="text"
          placeholder={t('admin.users.search_placeholder')}
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          style={inputStyle}
        />
        <select
          value={statusFilter}
          onChange={(e) => setStatusFilter(e.target.value as any)}
          style={selectStyle}
        >
          <option value="all">{t('admin.users.status_all')}</option>
          <option value="active">{t('admin.users.status_active')}</option>
          <option value="inactive">{t('admin.users.status_inactive')}</option>
        </select>
      </div>

      <div style={tableContainer}>
        <table style={tableStyle}>
          <thead>
            <tr style={theadRow}>
              <th style={thStyle}>{t('admin.users.col_user')}</th>
              <th style={thStyle}>{t('admin.users.col_name')}</th>
              <th style={thStyle}>{t('admin.users.col_status')}</th>
              <th style={thStyle}>{t('admin.users.col_email_verification')}</th>
              <th style={thStyle}>{t('admin.users.col_created_at')}</th>
              <th style={thStyle}></th>
            </tr>
          </thead>
          <tbody>
            {filteredUsers.length === 0 ? (
              <tr>
                <td colSpan={6} style={{ padding: 24, textAlign: 'center', color: '#9ca3af' }}>
                  {t('admin.users.empty')}
                </td>
              </tr>
            ) : (
              filteredUsers.map((u) => (
                <tr key={u.id} style={trStyle}>
                  <td style={tdStyle}>
                    <div style={{ display: 'flex', flexDirection: 'column' }}>
                      <span style={{ fontWeight: 600 }}>{u.email}</span>
                      {u.username && <span style={subtext}>{u.username}</span>}
                    </div>
                  </td>
                  <td style={tdStyle}>{u.displayName || t('common.em_dash')}</td>
                  <td style={tdStyle}>
                    <span style={u.isActive ? badgeActive : badgeInactive}>
                      {u.isActive ? t('admin.users.status_active') : t('admin.users.status_inactive')}
                    </span>
                  </td>
                  <td style={tdStyle}>
                    <span style={u.emailVerified ? badgeVerified : badgeUnverified}>
                      {u.emailVerified ? t('admin.users.email_verified') : t('admin.users.email_unverified')}
                    </span>
                  </td>
                  <td style={tdStyle}>{new Date(u.createdAt).toLocaleDateString(localeLanguageTag(locale))}</td>
                  <td style={{ ...tdStyle, textAlign: 'right' }}>
                    <button onClick={() => onSelectUser(u.id)} style={btnPrimary}>
                      {t('admin.users.manage')}
                    </button>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}

const tableContainer: React.CSSProperties = {
  background: 'var(--bg-card, #ffffff)',
  border: '1px solid var(--border-color, #e5e7eb)',
  borderRadius: 8,
  overflow: 'hidden',
  boxShadow: '0 1px 3px rgba(0,0,0,0.05)',
}

const tableStyle: React.CSSProperties = {
  width: '100%',
  borderCollapse: 'collapse',
  fontSize: 14,
  textAlign: 'left',
}

const theadRow: React.CSSProperties = {
  background: 'var(--bg-thead, #f9fafb)',
  borderBottom: '1px solid var(--border-color, #e5e7eb)',
}

const thStyle: React.CSSProperties = {
  padding: '12px 16px',
  fontWeight: 600,
  color: 'var(--text-secondary, #4b5563)',
}

const trStyle: React.CSSProperties = {
  borderBottom: '1px solid var(--border-color, #f3f4f6)',
}

const tdStyle: React.CSSProperties = {
  padding: '12px 16px',
}

const subtext: React.CSSProperties = {
  fontSize: 11,
  color: '#9ca3af',
  fontFamily: 'monospace',
}

const inputStyle: React.CSSProperties = {
  padding: '8px 12px',
  border: '1px solid var(--border-color, #d1d5db)',
  borderRadius: 6,
  fontSize: 14,
  width: '100%',
  maxWidth: 320,
}

const selectStyle: React.CSSProperties = {
  padding: '8px 12px',
  border: '1px solid var(--border-color, #d1d5db)',
  borderRadius: 6,
  fontSize: 14,
  background: 'var(--bg-card, #ffffff)',
}

const btnPrimary: React.CSSProperties = {
  padding: '6px 12px',
  background: 'var(--accent, #4f46e5)',
  color: 'white',
  border: 0,
  borderRadius: 4,
  cursor: 'pointer',
  fontSize: 13,
  fontWeight: 500,
}

const badgeActive: React.CSSProperties = {
  padding: '2px 8px',
  borderRadius: 9999,
  background: '#ecfdf5',
  color: '#065f46',
  fontSize: 12,
  fontWeight: 500,
}

const badgeInactive: React.CSSProperties = {
  padding: '2px 8px',
  borderRadius: 9999,
  background: '#fef2f2',
  color: '#991b1b',
  fontSize: 12,
  fontWeight: 500,
}

const badgeVerified: React.CSSProperties = {
  padding: '2px 8px',
  borderRadius: 9999,
  background: '#eff6ff',
  color: '#1e40af',
  fontSize: 12,
  fontWeight: 500,
}

const badgeUnverified: React.CSSProperties = {
  padding: '2px 8px',
  borderRadius: 9999,
  background: '#fffbp8',
  color: '#854d0e',
  backgroundColor: '#fef9c3',
  fontSize: 12,
  fontWeight: 500,
}

const countBadge: React.CSSProperties = {
  fontSize: 12,
  background: '#f3f4f6',
  padding: '4px 8px',
  borderRadius: 12,
  color: '#4b5563',
  fontWeight: 600,
}

const textMuted: React.CSSProperties = {
  color: '#6b7280',
  fontSize: 14,
  padding: 16,
}

const errStyle: React.CSSProperties = {
  color: 'crimson',
  padding: 16,
  fontWeight: 600,
}
