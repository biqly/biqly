import React, { useCallback, useEffect, useMemo, useState } from 'react'

import {
  assignRole,
  generateMFABypassCode,
  getUserDetail,
  getUserRoles,
  listRoles,
  removeRole,
  resendUserVerification,
  updateUserActiveStatus,
} from '../../api/admin'
import { useAsyncState } from '../../hooks/useAsyncState'
import { useConfirm } from '../../hooks/useConfirm'
import { useConfirmedMutation } from '../../hooks/useConfirmedMutation'
import { useLocale, useT } from '../../i18n'
import type { AuthUser, Role, UserRoleInfo } from '../../types/auth'
import { errorMessage } from '../../utils/error'
import { useAuth } from '../auth/AuthProvider'
import { AdminPanelShell } from './AdminPanelShell'
import { roleSelectOptions } from './adminSelectOptions'
import {
  UserDetailMfaSupportCard,
  UserDetailProfileCard,
  UserDetailRolesPanel,
} from './UserDetailSections'

interface UserDetailPageProps {
  token: string
  userID: string
}

export function UserDetailPage({ token, userID }: UserDetailPageProps) {
  const t = useT()
  const [locale] = useLocale()
  const confirm = useConfirm()
  const confirmMutation = useConfirmedMutation()
  const { user: currentUser, roles: currentUserRoles, hasPermission } = useAuth()
  // User activation/update needs admin:users; role assignment/revocation needs admin:roles.
  const canManageUsers = hasPermission('admin:users')
  const canManageRoles = hasPermission('admin:roles')
  const [user, setUser] = useState<AuthUser | null>(null)
  const [userRoles, setUserRoles] = useState<UserRoleInfo[]>([])
  const [availableRoles, setAvailableRoles] = useState<Role[]>([])
  const [loading, setLoading] = useState(true)
  const { error, setError, run } = useAsyncState()

  // Form states
  const [selectedRoleID, setSelectedRoleID] = useState('')
  const [scopeType, setScopeType] = useState('global')
  const [scopeID, setScopeID] = useState('')

  // MFA bypass code (super_admin support flow)
  const [bypassCode, setBypassCode] = useState<string | null>(null)

  const loadData = useCallback(async () => {
    setLoading(true)
    await run(async () => {
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
    })
    setLoading(false)
  }, [token, userID, run])

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void loadData()
  }, [loadData])

  const isSelf = currentUser?.id === userID
  const isSuperAdmin = currentUserRoles.includes('super_admin')

  const assignableRoleOptions = useMemo(() => roleSelectOptions(availableRoles), [availableRoles])
  const scopeTypeOptions = useMemo(
    () => [
      { value: 'global', label: 'global' },
      { value: 'workspace', label: 'workspace' },
    ],
    [],
  )

  async function handleGenerateBypassCode() {
    await confirmMutation(
      async () => {
        const resp = await generateMFABypassCode(token, userID)
        setBypassCode(resp.bypass_code)
      },
      {
        title: t('admin.user_detail.mfa_generate_bypass_confirm'),
        variant: 'default',
      },
    )
  }

  async function handleResendVerification() {
    await confirmMutation(
      async () => {
        await resendUserVerification(token, userID)
      },
      {
        title: t('admin.user_detail.resend_verification_confirm'),
        variant: 'default',
        successMessage: t('admin.user_detail.resend_verification_success'),
      },
    )
  }

  async function handleToggleActive() {
    if (!user) {
      return
    }
    if (isSelf && user.isActive) {
      setError(t('admin.user_detail.cannot_suspend_self'))
      return
    }
    const nextState = !user.isActive
    const ok = await confirm({
      title: t(
        nextState ? 'admin.user_detail.confirm_activate' : 'admin.user_detail.confirm_deactivate',
      ),
      variant: nextState ? 'default' : 'danger',
    })
    if (!ok) {
      return
    }
    try {
      await updateUserActiveStatus(token, userID, nextState)
      void loadData()
    } catch (e) {
      setError(errorMessage(e))
    }
  }

  async function handleAssignRole(e: React.SubmitEvent<HTMLFormElement>) {
    e.preventDefault()
    if (!selectedRoleID) {
      return
    }
    try {
      await assignRole(
        token,
        userID,
        selectedRoleID,
        scopeType,
        scopeType === 'workspace' ? scopeID.trim() || undefined : undefined,
      )
      // reset scope id
      setScopeID('')
      void loadData()
    } catch (e) {
      setError(errorMessage(e))
    }
  }

  async function handleRevokeRole(roleID: string) {
    const ok = await confirm({
      title: t('admin.user_detail.confirm_revoke_role'),
      variant: 'danger',
    })
    if (!ok) {
      return
    }
    try {
      await removeRole(token, userID, roleID)
      void loadData()
    } catch (e) {
      setError(errorMessage(e))
    }
  }

  if (loading) {
    return <div className="text-foreground-muted p-4 text-sm">{t('admin.user_detail.loading')}</div>
  }
  if (error) {
    return (
      <div className="text-error p-4 font-semibold">
        {t('common.error')}: {error}
      </div>
    )
  }
  if (!user) {
    return (
      <div className="text-foreground-muted p-4 text-sm">{t('admin.user_detail.not_found')}</div>
    )
  }

  return (
    <AdminPanelShell title={t('admin.user_detail.title')} description={user.email}>
      <div className="flex flex-col gap-6">
        <UserDetailProfileCard
          t={t}
          locale={locale}
          user={user}
          isSelf={isSelf}
          canManageUsers={canManageUsers}
          verificationSending={false}
          verificationMessage={null}
          onToggleActive={() => void handleToggleActive()}
          onResendVerification={() => void handleResendVerification()}
        />
        {isSuperAdmin && (
          <UserDetailMfaSupportCard
            t={t}
            bypassGenerating={false}
            bypassCode={bypassCode}
            bypassError={null}
            onGenerate={() => void handleGenerateBypassCode()}
          />
        )}
        <UserDetailRolesPanel
          t={t}
          userRoles={userRoles}
          canManageRoles={canManageRoles}
          assignableRoleOptions={assignableRoleOptions}
          selectedRoleID={selectedRoleID}
          scopeType={scopeType}
          scopeID={scopeID}
          scopeTypeOptions={scopeTypeOptions}
          onRoleChange={setSelectedRoleID}
          onScopeTypeChange={setScopeType}
          onScopeIdChange={setScopeID}
          onAssignRole={(event) => {
            void handleAssignRole(event)
          }}
          onRevokeRole={(roleId) => void handleRevokeRole(roleId)}
        />
      </div>
    </AdminPanelShell>
  )
}
