import type { SubmitEvent } from 'react'

import { type Locale, localeLanguageTag, type useT } from '../../i18n'
import type { AuthUser, UserRoleInfo } from '../../types/auth'
import { Select } from '../ui/Select'

export function UserDetailProfileCard({
  t,
  locale,
  user,
  isSelf,
  canManageUsers,
  verificationSending,
  verificationMessage,
  onToggleActive,
  onResendVerification,
}: {
  t: ReturnType<typeof useT>
  locale: Locale
  user: AuthUser
  isSelf: boolean
  canManageUsers: boolean
  verificationSending: boolean
  verificationMessage: { type: 'success' | 'error'; text: string } | null
  onToggleActive: () => void
  onResendVerification: () => void
}) {
  return (
    <div className="admin-card">
      <div style={{ display: 'flex', gap: 24, alignItems: 'center', flexWrap: 'wrap' }}>
        <div className="admin-avatar-circle" style={{ overflow: 'hidden' }}>
          {user.avatarUrl ? (
            <img
              src={user.avatarUrl}
              alt=""
              style={{ width: '100%', height: '100%', objectFit: 'cover' }}
            />
          ) : user.displayName ? (
            user.displayName.slice(0, 2).toUpperCase()
          ) : (
            user.email.slice(0, 2).toUpperCase()
          )}
        </div>
        <div style={{ flex: 1, display: 'flex', flexDirection: 'column', gap: 4 }}>
          <h2 style={{ margin: 0, fontSize: 22 }}>
            {user.displayName ?? t('admin.user_detail.unnamed_user')}
          </h2>
          <span style={{ fontSize: 14, color: 'var(--text-secondary)', fontWeight: 500 }}>
            {user.email}
          </span>
          <span
            style={{
              fontSize: 12,
              color: 'var(--text-muted)',
              fontFamily: 'var(--font-mono, monospace)',
            }}
          >
            UUID: {user.id}
          </span>
        </div>
        {!(isSelf && user.isActive) && (
          <div style={{ display: 'flex', gap: 12, alignItems: 'center' }}>
            <button
              onClick={onToggleActive}
              disabled={!canManageUsers}
              className={user.isActive ? 'admin-btn-deactivate' : 'admin-btn-activate'}
            >
              {user.isActive
                ? t('admin.user_detail.suspend_account')
                : t('admin.user_detail.activate_account')}
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
          <div
            style={{ display: 'flex', flexDirection: 'column', gap: 8, alignItems: 'flex-start' }}
          >
            <span
              className={user.emailVerified ? 'admin-badge-verified' : 'admin-badge-unverified'}
            >
              {user.emailVerified
                ? t('admin.user_detail.email_approved')
                : t('admin.user_detail.email_pending')}
            </span>
            {!user.emailVerified && (
              <button
                type="button"
                onClick={onResendVerification}
                disabled={verificationSending}
                className="admin-btn-resend"
              >
                {verificationSending ? '...' : t('admin.user_detail.resend_verification')}
              </button>
            )}
            {verificationMessage && (
              <span
                className={
                  verificationMessage.type === 'success'
                    ? 'admin-badge-active'
                    : 'admin-badge-inactive'
                }
                style={{ background: 'transparent', padding: 0 }}
              >
                {verificationMessage.text}
              </span>
            )}
          </div>
        </div>
        <div className="admin-grid-item">
          <span className="admin-label">{t('admin.user_detail.created_at')}</span>
          <span className="admin-val">
            {new Date(user.createdAt).toLocaleString(localeLanguageTag(locale))}
          </span>
        </div>
        <div className="admin-grid-item">
          <span className="admin-label">{t('admin.user_detail.updated_at')}</span>
          <span className="admin-val">
            {new Date(user.updatedAt).toLocaleString(localeLanguageTag(locale))}
          </span>
        </div>
      </div>
    </div>
  )
}

export function UserDetailMfaSupportCard({
  t,
  bypassGenerating,
  bypassCode,
  bypassError,
  onGenerate,
}: {
  t: ReturnType<typeof useT>
  bypassGenerating: boolean
  bypassCode: string | null
  bypassError: string | null
  onGenerate: () => void
}) {
  return (
    <div className="admin-card">
      <h3 style={{ marginTop: 0, marginBottom: 8 }}>{t('admin.user_detail.mfa_support_title')}</h3>
      <p className="admin-text-muted" style={{ padding: 0, marginTop: 0, marginBottom: 16 }}>
        {t('admin.user_detail.mfa_support_desc')}
      </p>
      <button
        type="button"
        onClick={onGenerate}
        disabled={bypassGenerating}
        className="admin-btn-resend"
      >
        {bypassGenerating ? '...' : t('admin.user_detail.mfa_generate_bypass')}
      </button>
      {bypassError && (
        <div className="admin-err-text" style={{ padding: '12px 0 0' }}>
          {bypassError}
        </div>
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
                void navigator.clipboard.writeText(bypassCode)
                alert(t('admin.user_detail.mfa_bypass_copied'))
              }}
            >
              {t('admin.user_detail.copy')}
            </button>
          </div>
        </div>
      )}
    </div>
  )
}

export function UserDetailRolesPanel({
  t,
  userRoles,
  canManageRoles,
  assignableRoleOptions,
  selectedRoleID,
  scopeType,
  scopeID,
  scopeTypeOptions,
  onRoleChange,
  onScopeTypeChange,
  onScopeIdChange,
  onAssignRole,
  onRevokeRole,
}: {
  t: ReturnType<typeof useT>
  userRoles: UserRoleInfo[]
  canManageRoles: boolean
  assignableRoleOptions: { value: string; label: string }[]
  selectedRoleID: string
  scopeType: string
  scopeID: string
  scopeTypeOptions: { value: string; label: string }[]
  onRoleChange: (id: string) => void
  onScopeTypeChange: (type: string) => void
  onScopeIdChange: (id: string) => void
  onAssignRole: (e: SubmitEvent<HTMLFormElement>) => void
  onRevokeRole: (roleId: string) => void
}) {
  return (
    <div className="admin-roles-grid">
      <div className="admin-card" style={{ minWidth: 0, overflow: 'hidden' }}>
        <h3 style={{ marginTop: 0, marginBottom: 16 }}>
          {t('admin.user_detail.assigned_roles', { count: userRoles.length })}
        </h3>
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
                  <td className="admin-td" style={{ fontWeight: 600 }}>
                    {ur.role_name}
                  </td>
                  <td className="admin-td">
                    <span
                      className={
                        ur.scope_type === 'global' ? 'admin-badge-global' : 'admin-badge-workspace'
                      }
                    >
                      {ur.scope_type}
                    </span>
                  </td>
                  <td
                    className="admin-td"
                    style={{ fontFamily: 'var(--font-mono, monospace)', fontSize: 12 }}
                  >
                    {ur.scope_id === '00000000-0000-0000-0000-000000000000'
                      ? t('admin.user_detail.all_or_none')
                      : ur.scope_id}
                  </td>
                  <td className="admin-td" style={{ textAlign: 'right' }}>
                    <button
                      onClick={() => onRevokeRole(ur.role_id)}
                      className="admin-btn-revoke"
                      disabled={!canManageRoles}
                    >
                      {t('admin.user_detail.remove_role')}
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      <div className="admin-card" style={{ minWidth: 0, overflow: 'hidden' }}>
        <h3 style={{ marginTop: 0, marginBottom: 16 }}>{t('admin.user_detail.assign_new_role')}</h3>
        <form onSubmit={(e) => void onAssignRole(e)} className="page-stack" style={{ gap: 12 }}>
          <label className="admin-form-label">
            <span>{t('admin.user_detail.select_role')}</span>
            <Select
              value={selectedRoleID}
              options={assignableRoleOptions}
              onChange={onRoleChange}
              disabled={!canManageRoles || assignableRoleOptions.length === 0}
            />
          </label>
          <label className="admin-form-label">
            <span>{t('admin.user_detail.scope_type')}</span>
            <Select value={scopeType} options={scopeTypeOptions} onChange={onScopeTypeChange} />
          </label>
          {scopeType === 'workspace' && (
            <label className="admin-form-label">
              <span>{t('admin.user_detail.workspace_uuid')}</span>
              <input
                type="text"
                placeholder={t('admin.user_detail.workspace_uuid_placeholder')}
                value={scopeID}
                onChange={(e) => onScopeIdChange(e.target.value)}
                className="admin-input"
                required
              />
            </label>
          )}
          <button type="submit" className="admin-btn-submit" disabled={!canManageRoles}>
            {t('admin.user_detail.assign_role')}
          </button>
        </form>
      </div>
    </div>
  )
}
