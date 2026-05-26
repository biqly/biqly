import React, { useEffect, useState } from 'react'
import {
  getUserDetail,
  getUserRoles,
  listRoles,
  assignRole,
  removeRole,
  updateUserActiveStatus,
} from '../../api/admin'
import { localeLanguageTag, useLocale, useT } from '../../i18n'
import type { AuthUser, UserRoleInfo, Role } from '../../types/auth'

interface UserDetailPageProps {
  token: string
  userID: string
  onBack: () => void
}

export function UserDetailPage({ token, userID, onBack }: UserDetailPageProps) {
  const t = useT()
  const [locale] = useLocale()
  const [user, setUser] = useState<AuthUser | null>(null)
  const [userRoles, setUserRoles] = useState<UserRoleInfo[]>([])
  const [availableRoles, setAvailableRoles] = useState<Role[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  // Form states
  const [selectedRoleID, setSelectedRoleID] = useState('')
  const [scopeType, setScopeType] = useState('global')
  const [scopeID, setScopeID] = useState('')

  async function loadData() {
    try {
      setLoading(true)
      const [u, ur, arRes] = await Promise.all([
        getUserDetail(token, userID),
        getUserRoles(token, userID),
        listRoles(token),
      ])
      setUser(u)
      setUserRoles(ur)
      setAvailableRoles(arRes.roles)
      if (arRes.roles.length > 0 && arRes.roles[0]) {
        setSelectedRoleID(arRes.roles[0].id)
      }
      setError(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadData()
  }, [token, userID])

  async function handleToggleActive() {
    if (!user) return
    const nextState = !user.isActive
    if (!confirm(t(nextState ? 'admin.user_detail.confirm_activate' : 'admin.user_detail.confirm_deactivate'))) return
    try {
      await updateUserActiveStatus(token, userID, nextState)
      loadData()
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }

  async function handleAssignRole(e: React.FormEvent) {
    e.preventDefault()
    if (!selectedRoleID) return
    try {
      await assignRole(
        token,
        userID,
        selectedRoleID,
        scopeType,
        scopeType === 'workspace' ? scopeID.trim() || undefined : undefined
      )
      // reset scope id
      setScopeID('')
      loadData()
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }

  async function handleRevokeRole(roleID: string) {
    if (!confirm(t('admin.user_detail.confirm_revoke_role'))) return
    try {
      await removeRole(token, userID, roleID)
      loadData()
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }

  if (loading) return <div style={textMuted}>{t('admin.user_detail.loading')}</div>
  if (error) return <div style={errStyle}>{t('common.error')}: {error}</div>
  if (!user) return <div style={textMuted}>{t('admin.user_detail.not_found')}</div>

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 24 }}>
      <div>
        <button onClick={onBack} style={btnSecondary}>
          &larr; {t('admin.user_detail.back_to_list')}
        </button>
      </div>

      {/* User profile details card */}
      <div style={card}>
        <div style={{ display: 'flex', gap: 24, alignItems: 'center', flexWrap: 'wrap' }}>
          <div style={avatarCircle}>
            {user.displayName ? user.displayName.slice(0, 2).toUpperCase() : user.email.slice(0, 2).toUpperCase()}
          </div>
          <div style={{ flex: 1, display: 'flex', flexDirection: 'column', gap: 4 }}>
            <h2 style={{ margin: 0, fontSize: 22 }}>{user.displayName || t('admin.user_detail.unnamed_user')}</h2>
            <span style={{ fontSize: 14, color: '#4b5563', fontWeight: 500 }}>{user.email}</span>
            <span style={{ fontSize: 12, color: '#9ca3af', fontFamily: 'monospace' }}>UUID: {user.id}</span>
          </div>
          <div style={{ display: 'flex', gap: 12, alignItems: 'center' }}>
            <button
              onClick={handleToggleActive}
              style={user.isActive ? btnDeactivate : btnActivate}
            >
              {user.isActive ? t('admin.user_detail.suspend_account') : t('admin.user_detail.activate_account')}
            </button>
          </div>
        </div>

        <div style={grid}>
          <div style={gridItem}>
            <span style={label}>{t('admin.user_detail.account_status')}</span>
            <span style={user.isActive ? badgeActive : badgeInactive}>
              {user.isActive ? t('admin.users.status_active') : t('admin.user_detail.status_locked')}
            </span>
          </div>
          <div style={gridItem}>
            <span style={label}>{t('admin.user_detail.email_verification')}</span>
            <span style={user.emailVerified ? badgeVerified : badgeUnverified}>
              {user.emailVerified ? t('admin.user_detail.email_approved') : t('admin.user_detail.email_pending')}
            </span>
          </div>
          <div style={gridItem}>
            <span style={label}>{t('admin.user_detail.created_at')}</span>
            <span style={val}>{new Date(user.createdAt).toLocaleString(localeLanguageTag(locale))}</span>
          </div>
          <div style={gridItem}>
            <span style={label}>{t('admin.user_detail.updated_at')}</span>
            <span style={val}>{new Date(user.updatedAt).toLocaleString(localeLanguageTag(locale))}</span>
          </div>
        </div>
      </div>

      {/* User Roles lists and Assign role form */}
      <div style={{ display: 'grid', gridTemplateColumns: '3fr 2fr', gap: 24, alignItems: 'start' }}>
        {/* Roles list */}
        <div style={card}>
          <h3 style={{ marginTop: 0, marginBottom: 16 }}>{t('admin.user_detail.assigned_roles', { count: userRoles.length })}</h3>
          {userRoles.length === 0 ? (
            <div style={textMuted}>{t('admin.user_detail.no_roles')}</div>
          ) : (
            <table style={tableStyle}>
              <thead>
                <tr style={theadRow}>
                  <th style={thStyle}>{t('admin.user_detail.role_name')}</th>
                  <th style={thStyle}>{t('admin.user_detail.scope')}</th>
                  <th style={thStyle}>{t('admin.user_detail.scope_id')}</th>
                  <th style={thStyle}></th>
                </tr>
              </thead>
              <tbody>
                {userRoles.map((ur) => (
                  <tr key={`${ur.role_id}-${ur.scope_type}-${ur.scope_id}`} style={trStyle}>
                    <td style={{ ...tdStyle, fontWeight: 600 }}>{ur.role_name}</td>
                    <td style={tdStyle}>
                      <span style={ur.scope_type === 'global' ? badgeGlobal : badgeWorkspace}>
                        {ur.scope_type}
                      </span>
                    </td>
                    <td style={{ ...tdStyle, fontFamily: 'monospace', fontSize: 12 }}>
                      {ur.scope_id === '00000000-0000-0000-0000-000000000000' ? t('admin.user_detail.all_or_none') : ur.scope_id}
                    </td>
                    <td style={{ ...tdStyle, textAlign: 'right' }}>
                      <button onClick={() => handleRevokeRole(ur.role_id)} style={btnRevoke}>
                        {t('admin.user_detail.remove_role')}
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>

        {/* Assign role form */}
        <div style={card}>
          <h3 style={{ marginTop: 0, marginBottom: 16 }}>{t('admin.user_detail.assign_new_role')}</h3>
          <form onSubmit={handleAssignRole} style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
            <label style={formLabel}>
              <span>{t('admin.user_detail.select_role')}</span>
              <select
                value={selectedRoleID}
                onChange={(e) => setSelectedRoleID(e.target.value)}
                style={inputStyle}
              >
                {availableRoles.map((role) => (
                  <option key={role.id} value={role.id}>
                    {role.name} {role.description ? `(${role.description})` : ''}
                  </option>
                ))}
              </select>
            </label>

            <label style={formLabel}>
              <span>{t('admin.user_detail.scope_type')}</span>
              <select
                value={scopeType}
                onChange={(e) => setScopeType(e.target.value)}
                style={inputStyle}
              >
                <option value="global">global</option>
                <option value="workspace">workspace</option>
              </select>
            </label>

            {scopeType === 'workspace' && (
              <label style={formLabel}>
                <span>{t('admin.user_detail.workspace_uuid')}</span>
                <input
                  type="text"
                  placeholder={t('admin.user_detail.workspace_uuid_placeholder')}
                  value={scopeID}
                  onChange={(e) => setScopeID(e.target.value)}
                  style={inputStyle}
                  required
                />
              </label>
            )}

            <button type="submit" style={btnSubmit}>
              {t('admin.user_detail.assign_role')}
            </button>
          </form>
        </div>
      </div>
    </div>
  )
}

const card: React.CSSProperties = {
  background: 'var(--bg-card, #ffffff)',
  border: '1px solid var(--border-color, #e5e7eb)',
  borderRadius: 8,
  padding: 24,
  boxShadow: '0 1px 3px rgba(0,0,0,0.05)',
}

const avatarCircle: React.CSSProperties = {
  width: 56,
  height: 56,
  borderRadius: '50%',
  background: 'var(--bg-avatar, #e0e7ff)',
  color: 'var(--text-avatar, #4f46e5)',
  display: 'flex',
  justifyContent: 'center',
  alignItems: 'center',
  fontWeight: 700,
  fontSize: 18,
}

const grid: React.CSSProperties = {
  display: 'grid',
  gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))',
  gap: 16,
  marginTop: 24,
  paddingTop: 24,
  borderTop: '1px solid var(--border-color, #f3f4f6)',
}

const gridItem: React.CSSProperties = {
  display: 'flex',
  flexDirection: 'column',
  gap: 4,
}

const label: React.CSSProperties = {
  fontSize: 12,
  color: '#9ca3af',
  fontWeight: 600,
  textTransform: 'uppercase',
  letterSpacing: '0.05em',
}

const val: React.CSSProperties = {
  fontSize: 14,
  fontWeight: 500,
  color: 'var(--text-primary, #111827)',
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
  padding: '10px 12px',
  fontWeight: 600,
  color: '#4b5563',
}

const trStyle: React.CSSProperties = {
  borderBottom: '1px solid var(--border-color, #f3f4f6)',
}

const tdStyle: React.CSSProperties = {
  padding: '10px 12px',
}

const formLabel: React.CSSProperties = {
  display: 'flex',
  flexDirection: 'column',
  gap: 6,
  fontSize: 13,
  fontWeight: 600,
  color: '#4b5563',
}

const inputStyle: React.CSSProperties = {
  padding: 8,
  border: '1px solid var(--border-color, #d1d5db)',
  borderRadius: 6,
  fontSize: 14,
  background: 'var(--bg-card, #ffffff)',
}

const btnSecondary: React.CSSProperties = {
  padding: '8px 14px',
  background: 'transparent',
  border: '1px solid var(--border-color, #d1d5db)',
  borderRadius: 6,
  cursor: 'pointer',
  fontSize: 14,
  fontWeight: 500,
  color: '#4b5563',
}

const btnSubmit: React.CSSProperties = {
  padding: '10px',
  background: 'var(--accent, #4f46e5)',
  color: 'white',
  border: 0,
  borderRadius: 6,
  cursor: 'pointer',
  fontSize: 14,
  fontWeight: 600,
  marginTop: 8,
}

const btnActivate: React.CSSProperties = {
  padding: '8px 16px',
  background: '#10b981',
  color: 'white',
  border: 0,
  borderRadius: 6,
  cursor: 'pointer',
  fontSize: 13,
  fontWeight: 600,
}

const btnDeactivate: React.CSSProperties = {
  padding: '8px 16px',
  background: '#ef4444',
  color: 'white',
  border: 0,
  borderRadius: 6,
  cursor: 'pointer',
  fontSize: 13,
  fontWeight: 600,
}

const btnRevoke: React.CSSProperties = {
  padding: '4px 8px',
  background: 'transparent',
  border: '1px solid #ef4444',
  color: '#ef4444',
  borderRadius: 4,
  cursor: 'pointer',
  fontSize: 12,
  fontWeight: 500,
}

const badgeActive: React.CSSProperties = {
  padding: '2px 8px',
  borderRadius: 9999,
  background: '#ecfdf5',
  color: '#065f46',
  fontSize: 13,
  fontWeight: 600,
  width: 'fit-content',
}

const badgeInactive: React.CSSProperties = {
  padding: '2px 8px',
  borderRadius: 9999,
  background: '#fef2f2',
  color: '#991b1b',
  fontSize: 13,
  fontWeight: 600,
  width: 'fit-content',
}

const badgeVerified: React.CSSProperties = {
  padding: '2px 8px',
  borderRadius: 9999,
  background: '#eff6ff',
  color: '#1e40af',
  fontSize: 13,
  fontWeight: 600,
  width: 'fit-content',
}

const badgeUnverified: React.CSSProperties = {
  padding: '2px 8px',
  borderRadius: 9999,
  background: '#fef9c3',
  color: '#854d0e',
  fontSize: 13,
  fontWeight: 600,
  width: 'fit-content',
}

const badgeGlobal: React.CSSProperties = {
  padding: '1px 6px',
  borderRadius: 4,
  background: '#f3f4f6',
  color: '#111827',
  fontSize: 11,
  fontWeight: 600,
  textTransform: 'uppercase',
}

const badgeWorkspace: React.CSSProperties = {
  padding: '1px 6px',
  borderRadius: 4,
  background: '#f0fdf4',
  color: '#166534',
  fontSize: 11,
  fontWeight: 600,
  textTransform: 'uppercase',
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
