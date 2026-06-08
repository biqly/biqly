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
import { useConfirm } from '../../hooks/useConfirm'
import { useLocale, useT } from '../../i18n'
import type { AuthUser, Role, UserRoleInfo } from '../../types/auth'
import { useAuth } from '../auth/AuthProvider'
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
  const { user: currentUser, roles: currentUserRoles, hasPermission } = useAuth()
  // User activation/update needs admin:users; role assignment/revocation needs admin:roles.
  const canManageUsers = hasPermission('admin:users')
  const canManageRoles = hasPermission('admin:roles')
  const [user, setUser] = useState<AuthUser | null>(null)
  const [verificationSending, setVerificationSending] = useState(false)
  const [verificationMessage, setVerificationMessage] = useState<{
    type: 'success' | 'error'
    text: string
  } | null>(null)
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

  const loadData = useCallback(async () => {
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
  }, [token, userID])

  useEffect(() => {
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
    const ok = await confirm({
      title: t('admin.user_detail.mfa_generate_bypass_confirm'),
      variant: 'default',
    })
    if (!ok) {
      return
    }
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
    if (!ok) {
      return
    }
    setVerificationSending(true)
    setVerificationMessage(null)
    try {
      await resendUserVerification(token, userID)
      setVerificationMessage({
        type: 'success',
        text: t('admin.user_detail.resend_verification_success'),
      })
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
      setError(e instanceof Error ? e.message : String(e))
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
      setError(e instanceof Error ? e.message : String(e))
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
      setError(e instanceof Error ? e.message : String(e))
    }
  }

  if (loading) {
    return <div className="admin-text-muted">{t('admin.user_detail.loading')}</div>
  }
  if (error) {
    return (
      <div className="admin-err-text">
        {t('common.error')}: {error}
      </div>
    )
  }
  if (!user) {
    return <div className="admin-text-muted">{t('admin.user_detail.not_found')}</div>
  }

  return (
    <div className="page-stack" style={{ gap: 24 }}>
      <UserDetailProfileCard
        t={t}
        locale={locale}
        user={user}
        isSelf={isSelf}
        canManageUsers={canManageUsers}
        verificationSending={verificationSending}
        verificationMessage={verificationMessage}
        onToggleActive={() => void handleToggleActive()}
        onResendVerification={() => void handleResendVerification()}
      />
      {isSuperAdmin && (
        <UserDetailMfaSupportCard
          t={t}
          bypassGenerating={bypassGenerating}
          bypassCode={bypassCode}
          bypassError={bypassError}
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
  )
}
