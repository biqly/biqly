import { useEffect, useState } from 'react'

import { listUsers, resendUserVerification } from '../../api/admin'
import { apiListInvitations, apiResendInvitation, apiRevokeInvitation } from '../../api/auth'
import { useConfirm } from '../../hooks/useConfirm'
import { useQueryParam } from '../../hooks/useQueryParam'
import { useLocale, useT } from '../../i18n'
import type { AuthUser, Invitation } from '../../types/auth'
import { useAuth } from '../auth/AuthProvider'
import { ActiveUsersTab } from './userList/ActiveUsersTab'
import { InvitationsTab } from './userList/InvitationsTab'
import { InviteUserModal } from './userList/InviteUserModal'

interface UserListPageProps {
  token: string
  onSelectUser: (id: string, label: string) => void
}

export function UserListPage({ token, onSelectUser }: UserListPageProps) {
  const t = useT()
  const [locale] = useLocale()
  const confirm = useConfirm()
  const [users, setUsers] = useState<AuthUser[]>([])
  const [search, setSearch] = useState('')
  const [debouncedSearch, setDebouncedSearch] = useState('')
  const [statusFilter, setStatusFilter] = useState<'all' | 'active' | 'inactive'>('all')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [currentPage, setCurrentPage] = useState(1)
  const pageSize = 10
  const [totalItems, setTotalItems] = useState(0)

  const { roles } = useAuth()
  const isSuperAdmin = roles.includes('super_admin')
  const [showInviteModal, setShowInviteModal] = useState(false)

  // Invitation tab states
  const [subTabParam, setSubTabParam] = useQueryParam('subTab')
  const subTab = subTabParam === 'invitations' ? 'invitations' : 'active'
  const setSubTab = (val: 'active' | 'invitations') => {
    setSubTabParam(val === 'active' ? '' : val)
  }
  const [invitations, setInvitations] = useState<Invitation[]>([])
  const [inviteSearch, setInviteSearch] = useState('')
  const [debouncedInviteSearch, setDebouncedInviteSearch] = useState('')
  const [inviteStatusFilter, setInviteStatusFilter] = useState<
    'all' | 'pending' | 'claimed' | 'expired'
  >('all')
  const [invitesLoading, setInvitesLoading] = useState(false)
  const [invitesError, setInvitesError] = useState<string | null>(null)
  const [inviteCurrentPage, setInviteCurrentPage] = useState(1)
  const [inviteTotalItems, setInviteTotalItems] = useState(0)

  const [actionLoadingId, setActionLoadingId] = useState<string | null>(null)
  const [verificationLoadingId, setVerificationLoadingId] = useState<string | null>(null)
  const [actionMessage, setActionMessage] = useState<{
    type: 'success' | 'error'
    text: string
  } | null>(null)

  const loadInvitations = async () => {
    if (!isSuperAdmin) {
      return
    }
    try {
      setInvitesLoading(true)
      const res = await apiListInvitations(token, {
        page: inviteCurrentPage,
        pageSize,
        search: debouncedInviteSearch,
        status: inviteStatusFilter,
      })
      setInvitations(res.invitations)
      setInviteTotalItems(res.total)
      setInvitesError(null)
    } catch (e) {
      setInvitesError(e instanceof Error ? e.message : String(e))
    } finally {
      setInvitesLoading(false)
    }
  }

  useEffect(() => {
    if (subTab === 'invitations') {
      loadInvitations()
    }
  }, [token, inviteCurrentPage, debouncedInviteSearch, inviteStatusFilter, subTab])

  useEffect(() => {
    const id = window.setTimeout(() => setDebouncedSearch(search.trim()), 300)
    return () => window.clearTimeout(id)
  }, [search])

  useEffect(() => {
    const id = window.setTimeout(() => setDebouncedInviteSearch(inviteSearch.trim()), 300)
    return () => window.clearTimeout(id)
  }, [inviteSearch])

  useEffect(() => {
    if (subTab !== 'active') {
      return
    }
    let cancelled = false
    async function load() {
      try {
        setLoading(true)
        const res = await listUsers(token, {
          page: currentPage,
          pageSize,
          search: debouncedSearch,
          status: statusFilter,
        })
        if (!cancelled) {
          setUsers(res.users)
          setTotalItems(res.total)
          setError(null)
        }
      } catch (e) {
        if (!cancelled) {
          setError(e instanceof Error ? e.message : String(e))
        }
      } finally {
        if (!cancelled) {
          setLoading(false)
        }
      }
    }
    load()
    return () => {
      cancelled = true
    }
  }, [token, currentPage, debouncedSearch, statusFilter, subTab])

  useEffect(() => {
    setCurrentPage(1)
  }, [debouncedSearch, statusFilter])

  useEffect(() => {
    setInviteCurrentPage(1)
  }, [debouncedInviteSearch, inviteStatusFilter])

  const handleResend = async (id: string) => {
    const ok = await confirm({
      title: t('auth.invite_resend_confirm'),
      variant: 'default',
    })
    if (!ok) {
      return
    }
    setActionLoadingId(id)
    setActionMessage(null)
    try {
      await apiResendInvitation(token, id)
      setActionMessage({ type: 'success', text: t('auth.invite_resend_success') })
      loadInvitations()
    } catch (e: unknown) {
      const message = e instanceof Error ? e.message : 'Resend failed'
      setActionMessage({ type: 'error', text: message })
    } finally {
      setActionLoadingId(null)
    }
  }

  const handleResendVerification = async (id: string) => {
    const ok = await confirm({
      title: t('admin.users.resend_verification_confirm'),
      variant: 'default',
    })
    if (!ok) {
      return
    }
    setVerificationLoadingId(id)
    setActionMessage(null)
    try {
      await resendUserVerification(token, id)
      setActionMessage({ type: 'success', text: t('admin.users.resend_verification_success') })
    } catch (e: unknown) {
      const message = e instanceof Error ? e.message : 'Resend failed'
      setActionMessage({ type: 'error', text: message })
    } finally {
      setVerificationLoadingId(null)
    }
  }

  const handleRevoke = async (id: string) => {
    const ok = await confirm({
      title: t('auth.invite_revoke_confirm'),
      variant: 'danger',
    })
    if (!ok) {
      return
    }
    setActionLoadingId(id)
    setActionMessage(null)
    try {
      await apiRevokeInvitation(token, id)
      setActionMessage({ type: 'success', text: t('auth.invite_revoke_success') })
      loadInvitations()
    } catch (e: unknown) {
      const message = e instanceof Error ? e.message : 'Revoke failed'
      setActionMessage({ type: 'error', text: message })
    } finally {
      setActionLoadingId(null)
    }
  }

  const totalPages = Math.ceil(totalItems / pageSize)
  const displayedUsers = users

  return (
    <div className="page-stack">
      <div className="card-header-row">
        <h2 style={{ margin: 0, fontSize: 20 }}>{t('admin.users.title')}</h2>
        <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
          <span className="admin-count-badge">
            {subTab === 'active'
              ? t('admin.users.count', { count: totalItems })
              : t('admin.users.count', { count: inviteTotalItems })}
          </span>
          {isSuperAdmin && (
            <button
              type="button"
              className="admin-btn-success"
              onClick={() => setShowInviteModal(true)}
            >
              {t('auth.btn_invite_user')}
            </button>
          )}
        </div>
      </div>

      {isSuperAdmin && (
        <div className="admin-tab-container">
          <button
            type="button"
            onClick={() => setSubTab('active')}
            className={`admin-tab-button ${subTab === 'active' ? 'active' : ''}`}
          >
            {t('auth.active_users_tab')}
          </button>
          <button
            type="button"
            onClick={() => setSubTab('invitations')}
            className={`admin-tab-button ${subTab === 'invitations' ? 'active' : ''}`}
          >
            {t('auth.invitations_tab')}
          </button>
        </div>
      )}

      {error && (
        <div className="admin-err-text">
          {t('common.error')}: {error}
        </div>
      )}

      {subTab === 'active' ? (
        <ActiveUsersTab
          search={search}
          setSearch={setSearch}
          statusFilter={statusFilter}
          setStatusFilter={setStatusFilter}
          actionMessage={actionMessage}
          setActionMessage={setActionMessage}
          displayedUsers={displayedUsers}
          verificationLoadingId={verificationLoadingId}
          handleResendVerification={handleResendVerification}
          onSelectUser={onSelectUser}
          currentPage={currentPage}
          totalPages={totalPages}
          setCurrentPage={setCurrentPage}
          totalItems={totalItems}
          pageSize={pageSize}
          locale={locale}
          t={t}
          loading={loading}
        />
      ) : (
        <InvitationsTab
          inviteSearch={inviteSearch}
          setInviteSearch={setInviteSearch}
          inviteStatusFilter={inviteStatusFilter}
          setInviteStatusFilter={setInviteStatusFilter}
          actionMessage={actionMessage}
          setActionMessage={setActionMessage}
          invitations={invitations}
          invitesLoading={invitesLoading}
          invitesError={invitesError}
          actionLoadingId={actionLoadingId}
          handleResend={handleResend}
          handleRevoke={handleRevoke}
          inviteCurrentPage={inviteCurrentPage}
          setInviteCurrentPage={setInviteCurrentPage}
          inviteTotalItems={inviteTotalItems}
          pageSize={pageSize}
          locale={locale}
          t={t}
        />
      )}

      <InviteUserModal
        open={showInviteModal}
        onClose={() => setShowInviteModal(false)}
        token={token}
        onSuccess={() => {
          if (subTab === 'invitations') {
            loadInvitations()
          }
        }}
        t={t}
      />
    </div>
  )
}
