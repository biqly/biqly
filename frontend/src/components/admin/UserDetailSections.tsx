import type { SubmitEvent } from 'react'

import { type Locale, localeLanguageTag, type useT } from '../../i18n'
import { legacyLayoutClass } from '../../lib/layoutClasses'
import type { AuthUser, UserRoleInfo } from '../../types/auth'
import { formatDateTime } from '../../utils/formatters'
import { Select } from '../ui/Select'
import {
  adminActivateBtnClass,
  adminActiveBadgeClass,
  adminAvatarCircleClass,
  adminBtnResendClass,
  adminBtnRevokeClass,
  adminBtnSecondaryClass,
  adminBtnSubmitClass,
  adminBypassCodeBoxClass,
  adminCardClass,
  adminErrTextClass,
  adminFormLabelClass,
  adminGridClass,
  adminGridItemClass,
  adminInputClass,
  adminLabelClass,
  adminRolesGridClass,
  adminScopeBadgeClass,
  adminTableClass,
  adminTdClass,
  adminTextMutedClass,
  adminThClass,
  adminTheadRowClass,
  adminTrClass,
  adminValClass,
  adminVerifiedBadgeClass,
} from './adminClasses'

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
    <div className={adminCardClass}>
      <div style={{ display: 'flex', gap: 24, alignItems: 'center', flexWrap: 'wrap' }}>
        <div className={adminAvatarCircleClass} style={{ overflow: 'hidden' }}>
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
              className={adminActivateBtnClass(user.isActive)}
            >
              {user.isActive
                ? t('admin.user_detail.suspend_account')
                : t('admin.user_detail.activate_account')}
            </button>
          </div>
        )}
      </div>

      <div className={adminGridClass}>
        <div className={adminGridItemClass}>
          <span className={adminLabelClass}>{t('admin.user_detail.account_status')}</span>
          <span className={adminActiveBadgeClass(user.isActive)}>
            {user.isActive ? t('admin.users.status_active') : t('admin.user_detail.status_locked')}
          </span>
        </div>
        <div className={adminGridItemClass}>
          <span className={adminLabelClass}>{t('admin.user_detail.email_verification')}</span>
          <div
            style={{ display: 'flex', flexDirection: 'column', gap: 8, alignItems: 'flex-start' }}
          >
            <span className={adminVerifiedBadgeClass(user.emailVerified)}>
              {user.emailVerified
                ? t('admin.user_detail.email_approved')
                : t('admin.user_detail.email_pending')}
            </span>
            {!user.emailVerified && (
              <button
                type="button"
                onClick={onResendVerification}
                disabled={verificationSending}
                className={adminBtnResendClass}
              >
                {verificationSending ? '...' : t('admin.user_detail.resend_verification')}
              </button>
            )}
            {verificationMessage && (
              <span
                className={
                  verificationMessage.type === 'success'
                    ? adminActiveBadgeClass(true)
                    : adminActiveBadgeClass(false)
                }
                style={{ background: 'transparent', padding: 0 }}
              >
                {verificationMessage.text}
              </span>
            )}
          </div>
        </div>
        <div className={adminGridItemClass}>
          <span className={adminLabelClass}>{t('admin.user_detail.created_at')}</span>
          <span className={adminValClass}>
            {formatDateTime(user.createdAt, localeLanguageTag(locale))}
          </span>
        </div>
        <div className={adminGridItemClass}>
          <span className={adminLabelClass}>{t('admin.user_detail.updated_at')}</span>
          <span className={adminValClass}>
            {formatDateTime(user.updatedAt, localeLanguageTag(locale))}
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
    <div className={adminCardClass}>
      <h3 style={{ marginTop: 0, marginBottom: 8 }}>{t('admin.user_detail.mfa_support_title')}</h3>
      <p className={adminTextMutedClass} style={{ padding: 0, marginTop: 0, marginBottom: 16 }}>
        {t('admin.user_detail.mfa_support_desc')}
      </p>
      <button
        type="button"
        onClick={onGenerate}
        disabled={bypassGenerating}
        className={adminBtnResendClass}
      >
        {bypassGenerating ? '...' : t('admin.user_detail.mfa_generate_bypass')}
      </button>
      {bypassError && (
        <div className={adminErrTextClass} style={{ padding: '12px 0 0' }}>
          {bypassError}
        </div>
      )}
      {bypassCode && (
        <div style={{ marginTop: 16, display: 'flex', flexDirection: 'column', gap: 8 }}>
          <span className={adminLabelClass}>{t('admin.user_detail.mfa_bypass_generated')}</span>
          <div style={{ display: 'flex', gap: 8, alignItems: 'center', flexWrap: 'wrap' }}>
            <code className={adminBypassCodeBoxClass}>{bypassCode}</code>
            <button
              type="button"
              className={adminBtnSecondaryClass}
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
    <div className={adminRolesGridClass}>
      <div className={adminCardClass} style={{ minWidth: 0, overflow: 'hidden' }}>
        <h3 style={{ marginTop: 0, marginBottom: 16 }}>
          {t('admin.user_detail.assigned_roles', { count: userRoles.length })}
        </h3>
        {userRoles.length === 0 ? (
          <div className={adminTextMutedClass}>{t('admin.user_detail.no_roles')}</div>
        ) : (
          <table className={adminTableClass}>
            <thead>
              <tr className={adminTheadRowClass}>
                <th className={adminThClass}>{t('admin.user_detail.role_name')}</th>
                <th className={adminThClass}>{t('admin.user_detail.scope')}</th>
                <th className={adminThClass}>{t('admin.user_detail.scope_id')}</th>
                <th className={adminThClass}></th>
              </tr>
            </thead>
            <tbody>
              {userRoles.map((ur) => (
                <tr key={`${ur.role_id}-${ur.scope_type}-${ur.scope_id}`} className={adminTrClass}>
                  <td className={adminTdClass} style={{ fontWeight: 600 }}>
                    {ur.role_name}
                  </td>
                  <td className={adminTdClass}>
                    <span className={adminScopeBadgeClass(ur.scope_type)}>{ur.scope_type}</span>
                  </td>
                  <td
                    className={adminTdClass}
                    style={{ fontFamily: 'var(--font-mono, monospace)', fontSize: 12 }}
                  >
                    {ur.scope_id === '00000000-0000-0000-0000-000000000000'
                      ? t('admin.user_detail.all_or_none')
                      : ur.scope_id}
                  </td>
                  <td className={adminTdClass} style={{ textAlign: 'right' }}>
                    <button
                      onClick={() => onRevokeRole(ur.role_id)}
                      className={adminBtnRevokeClass}
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

      <div className={adminCardClass} style={{ minWidth: 0, overflow: 'hidden' }}>
        <h3 style={{ marginTop: 0, marginBottom: 16 }}>{t('admin.user_detail.assign_new_role')}</h3>
        <form
          onSubmit={(e) => void onAssignRole(e)}
          className={legacyLayoutClass('page-stack')}
          style={{ gap: 12 }}
        >
          <label className={adminFormLabelClass}>
            <span>{t('admin.user_detail.select_role')}</span>
            <Select
              value={selectedRoleID}
              options={assignableRoleOptions}
              onChange={onRoleChange}
              disabled={!canManageRoles || assignableRoleOptions.length === 0}
            />
          </label>
          <label className={adminFormLabelClass}>
            <span>{t('admin.user_detail.scope_type')}</span>
            <Select value={scopeType} options={scopeTypeOptions} onChange={onScopeTypeChange} />
          </label>
          {scopeType === 'workspace' && (
            <label className={adminFormLabelClass}>
              <span>{t('admin.user_detail.workspace_uuid')}</span>
              <input
                type="text"
                placeholder={t('admin.user_detail.workspace_uuid_placeholder')}
                value={scopeID}
                onChange={(e) => onScopeIdChange(e.target.value)}
                className={adminInputClass}
                required
              />
            </label>
          )}
          <button type="submit" className={adminBtnSubmitClass} disabled={!canManageRoles}>
            {t('admin.user_detail.assign_role')}
          </button>
        </form>
      </div>
    </div>
  )
}
