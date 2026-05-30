import React, { useEffect, useState } from 'react'
import { listUsers, resendUserVerification } from '../../api/admin'
import { localeLanguageTag, useLocale, useT } from '../../i18n'
import type { AuthUser, Invitation } from '../../types/auth'
import { Pagination } from '../ui/Pagination'
import { useAuth } from '../auth/AuthProvider'
import { apiInviteUser, apiListInvitations, apiRevokeInvitation, apiResendInvitation } from '../../api/auth'
import { useQueryParam } from '../../hooks/useQueryParam'
import { useConfirm } from '../../hooks/useConfirm'

interface UserListPageProps {
  token: string
  onSelectUser: (id: string) => void
}

export function UserListPage({ token, onSelectUser }: UserListPageProps) {
  const t = useT()
  const [locale] = useLocale()
  const confirm = useConfirm()
  const [users, setUsers] = useState<AuthUser[]>([])
  const [search, setSearch] = useState('')
  const [statusFilter, setStatusFilter] = useState<'all' | 'active' | 'inactive'>('all')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [currentPage, setCurrentPage] = useState(1)
  const pageSize = 10
  const [totalItems, setTotalItems] = useState(0)

  const { roles } = useAuth()
  const isSuperAdmin = roles.includes('super_admin')
  const [showInviteModal, setShowInviteModal] = useState(false)
  const [inviteEmail, setInviteEmail] = useState('')
  const [inviteRole, setInviteRole] = useState('viewer')
  const [inviteLoading, setInviteLoading] = useState(false)
  const [inviteSuccess, setInviteSuccess] = useState(false)
  const [inviteError, setInviteError] = useState<string | null>(null)

  // Invitation tab states
  const [subTabParam, setSubTabParam] = useQueryParam('subTab')
  const subTab = subTabParam === 'invitations' ? 'invitations' : 'active'
  const setSubTab = (val: 'active' | 'invitations') => {
    setSubTabParam(val === 'active' ? '' : val)
  }
  const [invitations, setInvitations] = useState<Invitation[]>([])
  const [inviteSearch, setInviteSearch] = useState('')
  const [inviteStatusFilter, setInviteStatusFilter] = useState<'all' | 'pending' | 'claimed' | 'expired'>('all')
  const [invitesLoading, setInvitesLoading] = useState(false)
  const [invitesError, setInvitesError] = useState<string | null>(null)
  const [inviteCurrentPage, setInviteCurrentPage] = useState(1)
  const [inviteTotalItems, setInviteTotalItems] = useState(0)

  const [actionLoadingId, setActionLoadingId] = useState<string | null>(null)
  const [verificationLoadingId, setVerificationLoadingId] = useState<string | null>(null)
  const [actionMessage, setActionMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null)

  const loadInvitations = async () => {
    if (!isSuperAdmin) return
    try {
      setInvitesLoading(true)
      const res = await apiListInvitations(token, {
        page: inviteCurrentPage,
        pageSize,
        search: inviteSearch,
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
  }, [token, inviteCurrentPage, inviteSearch, inviteStatusFilter, subTab])

  useEffect(() => {
    if (subTab !== 'active') return
    let cancelled = false
    async function load() {
      try {
        setLoading(true)
        const res = await listUsers(token, {
          page: currentPage,
          pageSize,
          search,
          status: statusFilter,
        })
        if (!cancelled) {
          setUsers(res.users)
          setTotalItems(res.total)
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
  }, [token, currentPage, search, statusFilter, subTab])

  useEffect(() => {
    setCurrentPage(1)
  }, [search, statusFilter])

  useEffect(() => {
    setInviteCurrentPage(1)
  }, [inviteSearch, inviteStatusFilter])

  const handleResend = async (id: string) => {
    const ok = await confirm({
      title: t('auth.invite_resend_confirm'),
      variant: 'default',
    })
    if (!ok) return
    setActionLoadingId(id)
    setActionMessage(null)
    try {
      await apiResendInvitation(token, id)
      setActionMessage({ type: 'success', text: t('auth.invite_resend_success') })
      loadInvitations()
    } catch (e: any) {
      setActionMessage({ type: 'error', text: e.message || 'Resend failed' })
    } finally {
      setActionLoadingId(null)
    }
  }

  const handleResendVerification = async (id: string) => {
    const ok = await confirm({
      title: t('admin.users.resend_verification_confirm'),
      variant: 'default',
    })
    if (!ok) return
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
    if (!ok) return
    setActionLoadingId(id)
    setActionMessage(null)
    try {
      await apiRevokeInvitation(token, id)
      setActionMessage({ type: 'success', text: t('auth.invite_revoke_success') })
      loadInvitations()
    } catch (e: any) {
      setActionMessage({ type: 'error', text: e.message || 'Revoke failed' })
    } finally {
      setActionLoadingId(null)
    }
  }


  const getInviteStatus = (inv: Invitation): 'claimed' | 'expired' | 'pending' => {
    if (inv.claimed_at) return 'claimed'
    const isExpired = new Date(inv.expires_at).getTime() < Date.now()
    return isExpired ? 'expired' : 'pending'
  }

  const totalPages = Math.ceil(totalItems / pageSize)
  const displayedUsers = users

  if (loading && users.length === 0) return <div className="admin-text-muted">{t('admin.users.loading')}</div>
  if (error) return <div className="admin-err-text">{t('common.error')}: {error}</div>

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
              onClick={() => {
                setShowInviteModal(true)
                setInviteEmail('')
                setInviteRole('viewer')
                setInviteSuccess(false)
                setInviteError(null)
              }}
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

      {subTab === 'active' ? (
        <>
          <div style={{ display: 'flex', gap: 12, alignItems: 'center', flexWrap: 'wrap' }}>
            <input
              type="text"
              placeholder={t('admin.users.search_placeholder')}
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="admin-input"
              style={{ maxWidth: 320 }}
            />
            <select
              value={statusFilter}
              onChange={(e) => setStatusFilter(e.target.value as any)}
              className="admin-select"
            >
              <option value="all">{t('admin.users.status_all')}</option>
              <option value="active">{t('admin.users.status_active')}</option>
              <option value="inactive">{t('admin.users.status_inactive')}</option>
            </select>
          </div>

          <div className="admin-table-container">
            {actionMessage && (
              <div
                className={actionMessage.type === 'success' ? 'admin-success-box' : 'admin-err-box'}
                onClick={() => setActionMessage(null)}
                role="button"
                tabIndex={0}
                onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') setActionMessage(null) }}
              >
                {actionMessage.text}
              </div>
            )}
            <table className="admin-table">
              <thead>
                <tr className="admin-thead-row">
                  <th className="admin-th">{t('admin.users.col_user')}</th>
                  <th className="admin-th">{t('admin.users.col_name')}</th>
                  <th className="admin-th">{t('admin.users.col_status')}</th>
                  <th className="admin-th">{t('admin.users.col_email_verification')}</th>
                  <th className="admin-th">{t('admin.users.col_created_at')}</th>
                  <th className="admin-th"></th>
                </tr>
              </thead>
              <tbody>
                {displayedUsers.length === 0 ? (
                  <tr className="admin-tr">
                    <td colSpan={6} style={{ padding: 24, textAlign: 'center', color: '#9ca3af' }}>
                      {t('admin.users.empty')}
                    </td>
                  </tr>
                ) : (
                  displayedUsers.map((u) => (
                    <tr key={u.id} className="admin-tr">
                      <td className="admin-td">
                        <div style={{ display: 'flex', flexDirection: 'column' }}>
                          <span style={{ fontWeight: 600 }}>{u.email}</span>
                          {u.username && <span className="admin-subtext">{u.username}</span>}
                        </div>
                      </td>
                      <td className="admin-td">{u.displayName || t('common.em_dash')}</td>
                      <td className="admin-td">
                        <span className={u.isActive ? 'admin-badge-active' : 'admin-badge-inactive'}>
                          {u.isActive ? t('admin.users.status_active') : t('admin.users.status_inactive')}
                        </span>
                      </td>
                      <td className="admin-td">
                        <div style={{ display: 'flex', flexDirection: 'column', gap: 6, alignItems: 'flex-start' }}>
                          <span className={u.emailVerified ? 'admin-badge-verified' : 'admin-badge-unverified'}>
                            {u.emailVerified ? t('admin.users.email_verified') : t('admin.users.email_unverified')}
                          </span>
                          {!u.emailVerified && (
                            <button
                              type="button"
                              onClick={() => handleResendVerification(u.id)}
                              disabled={verificationLoadingId === u.id}
                              className="admin-btn-secondary"
                            >
                              {verificationLoadingId === u.id ? '...' : t('admin.users.resend_verification')}
                            </button>
                          )}
                        </div>
                      </td>
                      <td className="admin-td">{new Date(u.createdAt).toLocaleDateString(localeLanguageTag(locale))}</td>
                      <td className="admin-td" style={{ textAlign: 'right' }}>
                        <button onClick={() => onSelectUser(u.id)} className="admin-btn-primary">
                          {t('admin.users.manage')}
                        </button>
                      </td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
            <Pagination
              currentPage={currentPage}
              totalPages={totalPages}
              onPageChange={setCurrentPage}
              totalItems={totalItems}
              itemsPerPage={pageSize}
            />
          </div>
        </>
      ) : (
        <>
          <div style={{ display: 'flex', gap: 12, alignItems: 'center', flexWrap: 'wrap' }}>
            <input
              type="text"
              placeholder={t('admin.users.search_placeholder')}
              value={inviteSearch}
              onChange={(e) => setInviteSearch(e.target.value)}
              className="admin-input"
              style={{ maxWidth: 320 }}
            />
            <select
              value={inviteStatusFilter}
              onChange={(e) => setInviteStatusFilter(e.target.value as any)}
              className="admin-select"
            >
              <option value="all">{t('auth.invite_status_all')}</option>
              <option value="pending">{t('auth.invite_status_pending')}</option>
              <option value="claimed">{t('auth.invite_status_claimed')}</option>
              <option value="expired">{t('auth.invite_status_expired')}</option>
            </select>
          </div>

          <div className="admin-table-container">
            {actionMessage && (
              <div
                className={actionMessage.type === 'success' ? 'admin-success-box' : 'admin-err-box'}
                onClick={() => setActionMessage(null)}
                role="button"
                tabIndex={0}
                onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') setActionMessage(null) }}
              >
                {actionMessage.text}
              </div>
            )}
            {invitesLoading && invitations.length === 0 ? (
              <div className="admin-text-muted">{t('auth.invite_list_loading')}</div>
            ) : invitesError ? (
              <div className="admin-err-text">{t('common.error')}: {invitesError}</div>
            ) : (
              <>
                <table className="admin-table">
                  <thead>
                    <tr className="admin-thead-row">
                      <th className="admin-th">{t('auth.invite_col_email')}</th>
                      <th className="admin-th">{t('auth.invite_col_role')}</th>
                      <th className="admin-th">{t('auth.invite_col_invited_by')}</th>
                      <th className="admin-th">{t('auth.invite_col_sent_at')}</th>
                      <th className="admin-th">{t('auth.invite_col_expires_at')}</th>
                      <th className="admin-th">{t('auth.invite_col_status')}</th>
                      <th className="admin-th"></th>
                    </tr>
                  </thead>
                  <tbody>
                    {invitations.length === 0 ? (
                      <tr className="admin-tr">
                        <td colSpan={7} style={{ padding: 24, textAlign: 'center', color: '#9ca3af' }}>
                          {t('auth.invite_list_empty')}
                        </td>
                      </tr>
                    ) : (
                      invitations.map((inv) => {
                        const status = getInviteStatus(inv)
                        const isPendingOrExpired = status === 'pending' || status === 'expired'
                        return (
                          <tr key={inv.id} className="admin-tr">
                            <td className="admin-td">
                              <span style={{ fontWeight: 600 }}>{inv.email}</span>
                            </td>
                            <td className="admin-td">{inv.role_name}</td>
                            <td className="admin-td">{inv.invited_by}</td>
                            <td className="admin-td">{new Date(inv.created_at).toLocaleDateString(localeLanguageTag(locale))}</td>
                            <td className="admin-td">{new Date(inv.expires_at).toLocaleDateString(localeLanguageTag(locale))}</td>
                            <td className="admin-td">
                              <span className={status === 'claimed' ? 'admin-badge-claimed' : status === 'expired' ? 'admin-badge-expired' : 'admin-badge-pending'}>
                                {status === 'claimed'
                                  ? t('auth.invite_status_claimed')
                                  : status === 'expired'
                                  ? t('auth.invite_status_expired')
                                  : t('auth.invite_status_pending')}
                              </span>
                            </td>
                            <td className="admin-td" style={{ textAlign: 'right' }}>
                              {isPendingOrExpired && (
                                <div style={{ display: 'flex', gap: 6, justifyContent: 'flex-end' }}>
                                  <button
                                    onClick={() => handleResend(inv.id)}
                                    disabled={actionLoadingId === inv.id}
                                    className="admin-btn-secondary"
                                  >
                                    {actionLoadingId === inv.id ? '...' : t('auth.btn_resend')}
                                  </button>
                                  <button
                                    onClick={() => handleRevoke(inv.id)}
                                    disabled={actionLoadingId === inv.id}
                                    className="admin-btn-danger"
                                  >
                                    {actionLoadingId === inv.id ? '...' : t('auth.btn_revoke')}
                                  </button>
                                </div>
                              )}
                            </td>
                          </tr>
                        )
                      })
                    )}
                  </tbody>
                </table>
                <Pagination
                  currentPage={inviteCurrentPage}
                  totalPages={Math.ceil(inviteTotalItems / pageSize)}
                  onPageChange={setInviteCurrentPage}
                  totalItems={inviteTotalItems}
                  itemsPerPage={pageSize}
                />
              </>
            )}
          </div>
        </>
      )}

      {showInviteModal && (
        <div
          className="modal-backdrop"
          role="presentation"
          onClick={(e) => { if (e.target === e.currentTarget) setShowInviteModal(false) }}
          style={{
            position: 'fixed',
            top: 0,
            left: 0,
            right: 0,
            bottom: 0,
            backgroundColor: 'rgba(0,0,0,0.5)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            zIndex: 1000,
          }}
        >
          <section
            className="modal-card"
            role="dialog"
            aria-modal="true"
            style={{
              background: 'var(--bg-card, #ffffff)',
              border: '1px solid var(--border, rgba(255, 255, 255, 0.06))',
              borderRadius: 8,
              width: '100%',
              maxWidth: 400,
              padding: 24,
              boxShadow: '0 20px 25px -5px rgba(0,0,0,0.1), 0 10px 10px -5px rgba(0,0,0,0.04)',
            }}
          >
            <header style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
              <h2 style={{ margin: 0, fontSize: 18 }}>{t('auth.invite_user_modal_title')}</h2>
              <button
                type="button"
                onClick={() => setShowInviteModal(false)}
                style={{ border: 0, background: 'transparent', fontSize: 20, cursor: 'pointer', color: 'var(--text-secondary)' }}
              >
                ×
              </button>
            </header>

            {inviteSuccess ? (
              <div className="page-stack" style={{ gap: 16 }}>
                <div className="admin-success-box">
                  {t('auth.invite_user_success', { email: inviteEmail })}
                </div>
                <button
                  type="button"
                  className="admin-btn-primary"
                  onClick={() => {
                    setShowInviteModal(false)
                    if (subTab === 'invitations') {
                      loadInvitations()
                    }
                  }}
                >
                  {t('common.close')}
                </button>
              </div>
            ) : (
              <form
                onSubmit={async (e) => {
                  e.preventDefault()
                  setInviteLoading(true)
                  setInviteError(null)
                  try {
                    await apiInviteUser(token, inviteEmail, inviteRole)
                    setInviteSuccess(true)
                    loadInvitations()
                  } catch (err: any) {
                    setInviteError(err.message || 'Invitation failed')
                  } finally {
                    setInviteLoading(false)
                  }
                }}
                className="page-stack"
                style={{ gap: 16 }}
              >
                {inviteError && <div className="admin-err-box">{t('auth.invite_user_failed', { error: inviteError })}</div>}

                <div className="page-stack" style={{ gap: 6 }}>
                  <label style={{ fontSize: 13, fontWeight: 500 }} htmlFor="invite-email-input">
                    {t('auth.invite_user_email')}
                  </label>
                  <input
                    id="invite-email-input"
                    type="email"
                    required
                    value={inviteEmail}
                    onChange={(e) => setInviteEmail(e.target.value)}
                    className="admin-input-wide"
                  />
                </div>

                <div className="page-stack" style={{ gap: 6 }}>
                  <label style={{ fontSize: 13, fontWeight: 500 }} htmlFor="invite-role-input">
                    {t('auth.invite_user_role')}
                  </label>
                  <select
                    id="invite-role-input"
                    value={inviteRole}
                    onChange={(e) => setInviteRole(e.target.value)}
                    className="admin-select-wide"
                  >
                    <option value="viewer">Viewer</option>
                    <option value="analyst">Analyst</option>
                    <option value="developer">Developer</option>
                    <option value="admin">Admin</option>
                    <option value="super_admin">Super Admin</option>
                  </select>
                </div>

                <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 12, marginTop: 8 }}>
                  <button
                    type="button"
                    className="admin-btn-ghost"
                    onClick={() => setShowInviteModal(false)}
                    disabled={inviteLoading}
                  >
                    {t('common.cancel')}
                  </button>
                  <button
                    type="submit"
                    className="admin-btn-primary"
                    disabled={inviteLoading || !inviteEmail}
                  >
                    {inviteLoading && <span className="spinner" style={{ marginRight: 6, display: 'inline-block', width: 12, height: 12 }} />}
                    {t('auth.btn_invite_user')}
                  </button>
                </div>
              </form>
            )}
          </section>
        </div>
      )}
    </div>
  )
}

