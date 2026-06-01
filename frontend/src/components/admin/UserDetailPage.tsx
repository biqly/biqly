import React, { useEffect, useMemo, useState } from 'react'
import {
  getUserDetail,
  getUserRoles,
  listRoles,
  assignRole,
  removeRole,
  updateUserActiveStatus,
  resendUserVerification,
  generateMFABypassCode,
} from '../../api/admin'
import { localeLanguageTag, useLocale, useT } from '../../i18n'
import type { AuthUser, UserRoleInfo, Role } from '../../types/auth'
import { useAuth } from '../auth/AuthProvider'
import { useConfirm } from '../../hooks/useConfirm'
import { Select } from '../ui/Select'
import { roleSelectOptions } from './adminSelectOptions'

interface UserDetailPageProps {
  token: string
  userID: string
}

export function UserDetailPage({ token, userID }: UserDetailPageProps) {
  const t = useT()
  const [locale] = useLocale()
  const confirm = useConfirm()
  const { user: currentUser, roles: currentUserRoles } = useAuth()
  const [user, setUser] = useState<AuthUser | null>(null)
  const [verificationSending, setVerificationSending] = useState(false)
  const [verificationMessage, setVerificationMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null)
  const [userRoles, setUserRoles] = useState<UserRoleInfo[]>([])
  const [availableRoles, setAvailableRoles] = useState<Role[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  // Form states
  const [selectedRoleID, setSelectedRoleID] = useState('')
  const [scopeType, setScopeType] = useState('global')
  const [scopeID, setScopeID] = useState('')

  // MFA bypass code (super_admin support flow)
  const [bypassCode, setBypassCode] = useState<string | null>(null)
  const [bypassGenerating, setBypassGenerating] = useState(false)
  const [bypassError, setBypassError] = useState<string | null>(null)

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

  const isSelf = currentUser?.id === userID
  const isSuperAdmin = currentUserRoles?.includes('super_admin') ?? false

  const assignableRoleOptions = useMemo(() => roleSelectOptions(availableRoles), [availableRoles])
  const scopeTypeOptions = useMemo(
    () => [
      { value: 'global', label: 'global' },
      { value: 'workspace', label: 'workspace' },
    ],
    [],
  )

  async function handleGenerateBypassCode() {
    const ok = await confirm({
      title: t('admin.user_detail.mfa_generate_bypass_confirm'),
      variant: 'default',
    })
    if (!ok) return
    setBypassGenerating(true)
    setBypassError(null)
    setBypassCode(null)
    try {
      const resp = await generateMFABypassCode(token, userID)
      setBypassCode(resp.bypass_code)
    } catch (e) {
      setBypassError(e instanceof Error ? e.message : String(e))
    } finally {
      setBypassGenerating(false)
    }
  }

  async function handleResendVerification() {
    const ok = await confirm({
      title: t('admin.user_detail.resend_verification_confirm'),
      variant: 'default',
    })
    if (!ok) return
    setVerificationSending(true)
    setVerificationMessage(null)
    try {
      await resendUserVerification(token, userID)
      setVerificationMessage({ type: 'success', text: t('admin.user_detail.resend_verification_success') })
    } catch (e) {
      setVerificationMessage({
        type: 'error',
        text: e instanceof Error ? e.message : t('common.error'),
      })
    } finally {
      setVerificationSending(false)
    }
  }

  async function handleToggleActive() {
    if (!user) return
    if (isSelf && user.isActive) {
      setError(t('admin.user_detail.cannot_suspend_self'))
      return
    }
    const nextState = !user.isActive
    const ok = await confirm({
      title: t(nextState ? 'admin.user_detail.confirm_activate' : 'admin.user_detail.confirm_deactivate'),
      variant: nextState ? 'default' : 'danger',
    })
    if (!ok) return
    try {
      await updateUserActiveStatus(token, userID, nextState)
      loadData()
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }

  async function handleAssignRole(e: React.SubmitEvent<HTMLFormElement>) {
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
    const ok = await confirm({
      title: t('admin.user_detail.confirm_revoke_role'),
      variant: 'danger',
    })
    if (!ok) return
    try {
      await removeRole(token, userID, roleID)
      loadData()
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }


  if (loading) return <div className="admin-text-muted">{t('admin.user_detail.loading')}</div>
  if (error) return <div className="admin-err-text">{t('common.error')}: {error}</div>
  if (!user) return <div className="admin-text-muted">{t('admin.user_detail.not_found')}</div>

  return (
    <div className="page-stack" style={{ gap: 24 }}>
      {/* User profile details card */}
      <div className="admin-card">
        <div style={{ display: 'flex', gap: 24, alignItems: 'center', flexWrap: 'wrap' }}>
          <div className="admin-avatar-circle" style={{ overflow: 'hidden' }}>
            {user.avatarUrl ? (
              <img src={user.avatarUrl} alt="" style={{ width: '100%', height: '100%', objectFit: 'cover' }} />
            ) : (
              user.displayName ? user.displayName.slice(0, 2).toUpperCase() : user.email.slice(0, 2).toUpperCase()
            )}
          </div>
          <div style={{ flex: 1, display: 'flex', flexDirection: 'column', gap: 4 }}>
            <h2 style={{ margin: 0, fontSize: 22 }}>{user.displayName || t('admin.user_detail.unnamed_user')}</h2>
            <span style={{ fontSize: 14, color: 'var(--text-secondary)', fontWeight: 500 }}>{user.email}</span>
            <span style={{ fontSize: 12, color: 'var(--text-muted)', fontFamily: 'var(--font-mono, monospace)' }}>UUID: {user.id}</span>
          </div>
          {!(isSelf && user.isActive) && (
            <div style={{ display: 'flex', gap: 12, alignItems: 'center' }}>
              <button
                onClick={handleToggleActive}
                className={user.isActive ? 'admin-btn-deactivate' : 'admin-btn-activate'}
              >
                {user.isActive ? t('admin.user_detail.suspend_account') : t('admin.user_detail.activate_account')}
              </button>
            </div>
          )}
        </div>

        <div className="admin-grid">
          <div className="admin-grid-item">
            <span className="admin-label">{t('admin.user_detail.account_status')}</span>
            <span className={user.isActive ? 'admin-badge-active' : 'admin-badge-inactive'}>
              {user.isActive ? t('admin.users.status_active') : t('admin.user_detail.status_locked')}
            </span>
          </div>
          <div className="admin-grid-item">
            <span className="admin-label">{t('admin.user_detail.email_verification')}</span>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 8, alignItems: 'flex-start' }}>
              <span className={user.emailVerified ? 'admin-badge-verified' : 'admin-badge-unverified'}>
                {user.emailVerified ? t('admin.user_detail.email_approved') : t('admin.user_detail.email_pending')}
              </span>
              {!user.emailVerified && (
                <button
                  type="button"
                  onClick={handleResendVerification}
                  disabled={verificationSending}
                  className="admin-btn-resend"
                >
                  {verificationSending ? '...' : t('admin.user_detail.resend_verification')}
                </button>
              )}
              {verificationMessage && (
                <span className={verificationMessage.type === 'success' ? 'admin-badge-active' : 'admin-badge-inactive'} style={{ background: 'transparent', padding: 0 }}>
                  {verificationMessage.text}
                </span>
              )}
            </div>
          </div>
          <div className="admin-grid-item">
            <span className="admin-label">{t('admin.user_detail.created_at')}</span>
            <span className="admin-val">{new Date(user.createdAt).toLocaleString(localeLanguageTag(locale))}</span>
          </div>
          <div className="admin-grid-item">
            <span className="admin-label">{t('admin.user_detail.updated_at')}</span>
            <span className="admin-val">{new Date(user.updatedAt).toLocaleString(localeLanguageTag(locale))}</span>
          </div>
        </div>
      </div>

      {/* 2FA Support — super_admin only, single-use bypass code */}
      {isSuperAdmin && (
        <div className="admin-card">
          <h3 style={{ marginTop: 0, marginBottom: 8 }}>{t('admin.user_detail.mfa_support_title')}</h3>
          <p className="admin-text-muted" style={{ padding: 0, marginTop: 0, marginBottom: 16 }}>
            {t('admin.user_detail.mfa_support_desc')}
          </p>
          <button
            type="button"
            onClick={handleGenerateBypassCode}
            disabled={bypassGenerating}
            className="admin-btn-resend"
          >
            {bypassGenerating ? '...' : t('admin.user_detail.mfa_generate_bypass')}
          </button>
          {bypassError && (
            <div className="admin-err-text" style={{ padding: '12px 0 0' }}>{bypassError}</div>
          )}
          {bypassCode && (
            <div style={{ marginTop: 16, display: 'flex', flexDirection: 'column', gap: 8 }}>
              <span className="admin-label">{t('admin.user_detail.mfa_bypass_generated')}</span>
              <div style={{ display: 'flex', gap: 8, alignItems: 'center', flexWrap: 'wrap' }}>
                <code className="admin-bypass-code-box">{bypassCode}</code>
                <button
                  type="button"
                  className="admin-btn-secondary"
                  onClick={() => {
                    navigator.clipboard.writeText(bypassCode)
                    alert(t('admin.user_detail.mfa_bypass_copied'))
                  }}
                >
                  {t('admin.user_detail.copy')}
                </button>
              </div>
            </div>
          )}
        </div>
      )}

      {/* User Roles lists and Assign role form */}
      <div style={{ display: 'grid', gridTemplateColumns: '3fr 2fr', gap: 24, alignItems: 'start' }}>
        {/* Roles list */}
        <div className="admin-card">
          <h3 style={{ marginTop: 0, marginBottom: 16 }}>{t('admin.user_detail.assigned_roles', { count: userRoles.length })}</h3>
          {userRoles.length === 0 ? (
            <div className="admin-text-muted">{t('admin.user_detail.no_roles')}</div>
          ) : (
            <table className="admin-table">
              <thead>
                <tr className="admin-thead-row">
                  <th className="admin-th">{t('admin.user_detail.role_name')}</th>
                  <th className="admin-th">{t('admin.user_detail.scope')}</th>
                  <th className="admin-th">{t('admin.user_detail.scope_id')}</th>
                  <th className="admin-th"></th>
                </tr>
              </thead>
              <tbody>
                {userRoles.map((ur) => (
                  <tr key={`${ur.role_id}-${ur.scope_type}-${ur.scope_id}`} className="admin-tr">
                    <td className="admin-td" style={{ fontWeight: 600 }}>{ur.role_name}</td>
                    <td className="admin-td">
                      <span className={ur.scope_type === 'global' ? 'admin-badge-global' : 'admin-badge-workspace'}>
                        {ur.scope_type}
                      </span>
                    </td>
                    <td className="admin-td" style={{ fontFamily: 'var(--font-mono, monospace)', fontSize: 12 }}>
                      {ur.scope_id === '00000000-0000-0000-0000-000000000000' ? t('admin.user_detail.all_or_none') : ur.scope_id}
                    </td>
                    <td className="admin-td" style={{ textAlign: 'right' }}>
                      <button onClick={() => handleRevokeRole(ur.role_id)} className="admin-btn-revoke">
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
        <div className="admin-card">
          <h3 style={{ marginTop: 0, marginBottom: 16 }}>{t('admin.user_detail.assign_new_role')}</h3>
          <form onSubmit={handleAssignRole} className="page-stack" style={{ gap: 12 }}>
            <label className="admin-form-label">
              <span>{t('admin.user_detail.select_role')}</span>
              <Select
                value={selectedRoleID}
                options={assignableRoleOptions}
                onChange={setSelectedRoleID}
                disabled={assignableRoleOptions.length === 0}
              />
            </label>

            <label className="admin-form-label">
              <span>{t('admin.user_detail.scope_type')}</span>
              <Select value={scopeType} options={scopeTypeOptions} onChange={setScopeType} />
            </label>

            {scopeType === 'workspace' && (
              <label className="admin-form-label">
                <span>{t('admin.user_detail.workspace_uuid')}</span>
                <input
                  type="text"
                  placeholder={t('admin.user_detail.workspace_uuid_placeholder')}
                  value={scopeID}
                  onChange={(e) => setScopeID(e.target.value)}
                  className="admin-input"
                  required
                />
              </label>
            )}

            <button type="submit" className="admin-btn-submit">
              {t('admin.user_detail.assign_role')}
            </button>
          </form>
        </div>
      </div>
    </div>
  )
}
