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
import { AdminFormSection } from './AdminFormSection'

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
      <div className="flex flex-wrap items-center gap-6">
        <div className={`${adminAvatarCircleClass} overflow-hidden`}>
          {user.avatarUrl ? (
            <img src={user.avatarUrl} alt="" className="size-full object-cover" />
          ) : user.displayName ? (
            user.displayName.slice(0, 2).toUpperCase()
          ) : (
            user.email.slice(0, 2).toUpperCase()
          )}
        </div>
        <div className="flex flex-1 flex-col gap-1">
          <h2 className="m-0 text-[22px]">
            {user.displayName ?? t('admin.user_detail.unnamed_user')}
          </h2>
          <span className="text-foreground-muted text-[14px] leading-none font-medium">
            {user.email}
          </span>
          <span className="text-foreground-faint font-mono text-xs leading-none">
            UUID: {user.id}
          </span>
        </div>
        {!(isSelf && user.isActive) && (
          <div className="flex items-center gap-3">
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
          <div className="flex flex-col items-start gap-2">
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
    <AdminFormSection title={t('admin.user_detail.mfa_support_title')}>
      <p className={`${adminTextMutedClass} m-0 p-0`}>{t('admin.user_detail.mfa_support_desc')}</p>
      <button
        type="button"
        onClick={onGenerate}
        disabled={bypassGenerating}
        className={adminBtnResendClass}
      >
        {bypassGenerating ? '...' : t('admin.user_detail.mfa_generate_bypass')}
      </button>
      {bypassError && <div className={`${adminErrTextClass} p-0 pt-3`}>{bypassError}</div>}
      {bypassCode && (
        <div className="mt-4 flex flex-col gap-2">
          <span className={adminLabelClass}>{t('admin.user_detail.mfa_bypass_generated')}</span>
          <div className="flex flex-wrap items-center gap-2">
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
    </AdminFormSection>
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
      <div className={`${adminCardClass} min-w-0 overflow-hidden`}>
        <h3 className="m-0 mb-4">
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
                  <td className={`${adminTdClass} font-semibold`}>{ur.role_name}</td>
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
                  <td className={`${adminTdClass} text-right`}>
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

      <AdminFormSection title={t('admin.user_detail.assign_new_role')}>
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
      </AdminFormSection>
    </div>
  )
}
