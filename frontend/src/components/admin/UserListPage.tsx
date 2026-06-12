import { useCallback, useState } from 'react'

import { listUsers, resendUserVerification } from '../../api/admin'
import { apiListInvitations, apiResendInvitation, apiRevokeInvitation } from '../../api/auth'
import { useConfirm } from '../../hooks/useConfirm'
import { useDebouncedValue } from '../../hooks/useDebouncedValue'
import { usePaginatedList } from '../../hooks/usePaginatedList'
import { useQueryParam } from '../../hooks/useQueryParam'
import { useT } from '../../i18n'
import type { AuthUser, Invitation } from '../../types/auth'
import type { PageQuery } from '../../types/pagination'
import { useAuth } from '../auth/AuthProvider'
import { ErrorAlert } from '../ui/ErrorAlert'
import { ActiveUsersTab } from './userList/ActiveUsersTab'
import { InvitationsTab } from './userList/InvitationsTab'
import { InviteUserModal } from './userList/InviteUserModal'

interface UserListPageProps {
  token: string
  onSelectUser: (id: string, label: string) => void
}

export function UserListPage({ token, onSelectUser }: UserListPageProps) {
  const t = useT()
  const confirm = useConfirm()
  const [search, setSearch] = useState('')
  const debouncedSearch = useDebouncedValue(search.trim(), 300)
  const [statusFilter, setStatusFilter] = useState<'all' | 'active' | 'inactive'>('all')
  const pageSize = 10

  const { roles } = useAuth()
  const isSuperAdmin = roles.includes('super_admin')
  const [showInviteModal, setShowInviteModal] = useState(false)

  // Invitation tab states
  const [subTabParam, setSubTabParam] = useQueryParam('subTab')
  const subTab = subTabParam === 'invitations' ? 'invitations' : 'active'
  const setSubTab = (val: 'active' | 'invitations') => {
    setSubTabParam(val === 'active' ? '' : val)
  }
  const [inviteSearch, setInviteSearch] = useState('')
  const debouncedInviteSearch = useDebouncedValue(inviteSearch.trim(), 300)
  const [inviteStatusFilter, setInviteStatusFilter] = useState<
    'all' | 'pending' | 'claimed' | 'expired'
  >('all')

  const [actionLoadingId, setActionLoadingId] = useState<string | null>(null)
  const [verificationLoadingId, setVerificationLoadingId] = useState<string | null>(null)
  const [actionMessage, setActionMessage] = useState<{
    type: 'success' | 'error'
    text: string
  } | null>(null)

  const usersFetcher = useCallback(
    async (q: PageQuery) => {
      const res = await listUsers(token, {
        page: q.page,
        pageSize: q.pageSize,
        search: debouncedSearch,
        status: statusFilter,
      })
      return { items: res.users, total: res.total }
    },
    [token, debouncedSearch, statusFilter],
  )
  const {
    items: users,
    loading,
    error,
    page: currentPage,
    setPage: setCurrentPage,
    totalPages,
    total: totalItems,
  } = usePaginatedList<AuthUser>({
    fetcher: usersFetcher,
    initialPageSize: pageSize,
    enabled: subTab === 'active',
    fetchKey: token,
    resetPageKey: `${debouncedSearch}|${statusFilter}`,
  })

  const invitationsFetcher = useCallback(
    async (q: PageQuery) => {
      const res = await apiListInvitations(token, {
        page: q.page,
        pageSize: q.pageSize,
        search: debouncedInviteSearch,
        status: inviteStatusFilter,
      })
      return { items: res.invitations, total: res.total }
    },
    [token, debouncedInviteSearch, inviteStatusFilter],
  )
  const {
    items: invitations,
    loading: invitesLoading,
    error: invitesError,
    page: inviteCurrentPage,
    setPage: setInviteCurrentPage,
    totalPages: inviteTotalPages,
    total: inviteTotalItems,
    reload: reloadInvitations,
  } = usePaginatedList<Invitation>({
    fetcher: invitationsFetcher,
    initialPageSize: pageSize,
    enabled: subTab === 'invitations' && isSuperAdmin,
    fetchKey: token,
    resetPageKey: `${debouncedInviteSearch}|${inviteStatusFilter}`,
  })

  const handleSearchChange = (value: string) => {
    setCurrentPage(1)
    setSearch(value)
  }

  const handleStatusFilterChange = (value: 'all' | 'active' | 'inactive') => {
    setCurrentPage(1)
    setStatusFilter(value)
  }

  const handleInviteSearchChange = (value: string) => {
    setInviteCurrentPage(1)
    setInviteSearch(value)
  }

  const handleInviteStatusFilterChange = (value: 'all' | 'pending' | 'claimed' | 'expired') => {
    setInviteCurrentPage(1)
    setInviteStatusFilter(value)
  }

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
      reloadInvitations()
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
      reloadInvitations()
    } catch (e: unknown) {
      const message = e instanceof Error ? e.message : 'Revoke failed'
      setActionMessage({ type: 'error', text: message })
    } finally {
      setActionLoadingId(null)
    }
  }

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

      {error && <ErrorAlert error={`${t('common.error')}: ${error}`} />}

      {subTab === 'active' ? (
        <ActiveUsersTab
          search={search}
          setSearch={handleSearchChange}
          statusFilter={statusFilter}
          setStatusFilter={handleStatusFilterChange}
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
          loading={loading}
        />
      ) : (
        <InvitationsTab
          inviteSearch={inviteSearch}
          setInviteSearch={handleInviteSearchChange}
          inviteStatusFilter={inviteStatusFilter}
          setInviteStatusFilter={handleInviteStatusFilterChange}
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
          inviteTotalPages={inviteTotalPages}
          inviteTotalItems={inviteTotalItems}
          pageSize={pageSize}
        />
      )}

      <InviteUserModal
        open={showInviteModal}
        onClose={() => setShowInviteModal(false)}
        token={token}
        onSuccess={() => {
          if (subTab === 'invitations') {
            reloadInvitations()
          }
        }}
        t={t}
      />
    </div>
  )
}
