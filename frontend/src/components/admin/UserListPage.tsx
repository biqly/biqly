import React, { useEffect, useState } from 'react'
import { listUsers } from '../../api/admin'
import { localeLanguageTag, useLocale, useT } from '../../i18n'
import type { AuthUser, Invitation } from '../../types/auth'
import { Pagination } from '../ui/Pagination'
import { useAuth } from '../auth/AuthProvider'
import { apiInviteUser, apiListInvitations, apiRevokeInvitation, apiResendInvitation } from '../../api/auth'

interface UserListPageProps {
  token: string
  onSelectUser: (id: string) => void
}

export function UserListPage({ token, onSelectUser }: UserListPageProps) {
  const t = useT()
  const [locale] = useLocale()
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
  const [subTab, setSubTab] = useState<'active' | 'invitations'>('active')
  const [invitations, setInvitations] = useState<Invitation[]>([])
  const [inviteSearch, setInviteSearch] = useState('')
  const [inviteStatusFilter, setInviteStatusFilter] = useState<'all' | 'pending' | 'claimed' | 'expired'>('all')
  const [invitesLoading, setInvitesLoading] = useState(false)
  const [invitesError, setInvitesError] = useState<string | null>(null)
  const [inviteCurrentPage, setInviteCurrentPage] = useState(1)
  const [inviteTotalItems, setInviteTotalItems] = useState(0)

  const [actionLoadingId, setActionLoadingId] = useState<string | null>(null)
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
    if (!window.confirm(t('auth.invite_resend_confirm'))) return
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

  const handleRevoke = async (id: string) => {
    if (!window.confirm(t('auth.invite_revoke_confirm'))) return
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

  if (loading && users.length === 0) return <div style={textMuted}>{t('admin.users.loading')}</div>
  if (error) return <div style={errStyle}>{t('common.error')}: {error}</div>

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <h2 style={{ margin: 0, fontSize: 20 }}>{t('admin.users.title')}</h2>
        <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
          <span style={countBadge}>
            {subTab === 'active'
              ? t('admin.users.count', { count: totalItems })
              : t('admin.users.count', { count: inviteTotalItems })}
          </span>
          {isSuperAdmin && (
            <button
              type="button"
              style={btnSuccess}
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
        <div style={tabContainer}>
          <button
            type="button"
            onClick={() => setSubTab('active')}
            style={tabButton(subTab === 'active')}
          >
            {t('auth.active_users_tab')}
          </button>
          <button
            type="button"
            onClick={() => setSubTab('invitations')}
            style={tabButton(subTab === 'invitations')}
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
              style={inputStyle}
            />
            <select
              value={statusFilter}
              onChange={(e) => setStatusFilter(e.target.value as any)}
              style={selectStyle}
            >
              <option value="all">{t('admin.users.status_all')}</option>
              <option value="active">{t('admin.users.status_active')}</option>
              <option value="inactive">{t('admin.users.status_inactive')}</option>
            </select>
          </div>

          <div style={tableContainer}>
            <table style={tableStyle}>
              <thead>
                <tr style={theadRow}>
                  <th style={thStyle}>{t('admin.users.col_user')}</th>
                  <th style={thStyle}>{t('admin.users.col_name')}</th>
                  <th style={thStyle}>{t('admin.users.col_status')}</th>
                  <th style={thStyle}>{t('admin.users.col_email_verification')}</th>
                  <th style={thStyle}>{t('admin.users.col_created_at')}</th>
                  <th style={thStyle}></th>
                </tr>
              </thead>
              <tbody>
                {displayedUsers.length === 0 ? (
                  <tr>
                    <td colSpan={6} style={{ padding: 24, textAlign: 'center', color: '#9ca3af' }}>
                      {t('admin.users.empty')}
                    </td>
                  </tr>
                ) : (
                  displayedUsers.map((u) => (
                    <tr key={u.id} style={trStyle}>
                      <td style={tdStyle}>
                        <div style={{ display: 'flex', flexDirection: 'column' }}>
                          <span style={{ fontWeight: 600 }}>{u.email}</span>
                          {u.username && <span style={subtext}>{u.username}</span>}
                        </div>
                      </td>
                      <td style={tdStyle}>{u.displayName || t('common.em_dash')}</td>
                      <td style={tdStyle}>
                        <span style={u.isActive ? badgeActive : badgeInactive}>
                          {u.isActive ? t('admin.users.status_active') : t('admin.users.status_inactive')}
                        </span>
                      </td>
                      <td style={tdStyle}>
                        <span style={u.emailVerified ? badgeVerified : badgeUnverified}>
                          {u.emailVerified ? t('admin.users.email_verified') : t('admin.users.email_unverified')}
                        </span>
                      </td>
                      <td style={tdStyle}>{new Date(u.createdAt).toLocaleDateString(localeLanguageTag(locale))}</td>
                      <td style={{ ...tdStyle, textAlign: 'right' }}>
                        <button onClick={() => onSelectUser(u.id)} style={btnPrimary}>
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
              style={inputStyle}
            />
            <select
              value={inviteStatusFilter}
              onChange={(e) => setInviteStatusFilter(e.target.value as any)}
              style={selectStyle}
            >
              <option value="all">{t('auth.invite_status_all')}</option>
              <option value="pending">{t('auth.invite_status_pending')}</option>
              <option value="claimed">{t('auth.invite_status_claimed')}</option>
              <option value="expired">{t('auth.invite_status_expired')}</option>
            </select>
          </div>

          <div style={tableContainer}>
            {actionMessage && (
              <div
                style={actionMessage.type === 'success' ? successBox : errStyleBox}
                onClick={() => setActionMessage(null)}
                role="button"
                tabIndex={0}
                onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') setActionMessage(null) }}
              >
                {actionMessage.text}
              </div>
            )}
            {invitesLoading && invitations.length === 0 ? (
              <div style={textMuted}>{t('auth.invite_list_loading')}</div>
            ) : invitesError ? (
              <div style={errStyle}>{t('common.error')}: {invitesError}</div>
            ) : (
              <>
                <table style={tableStyle}>
                  <thead>
                    <tr style={theadRow}>
                      <th style={thStyle}>{t('auth.invite_col_email')}</th>
                      <th style={thStyle}>{t('auth.invite_col_role')}</th>
                      <th style={thStyle}>{t('auth.invite_col_invited_by')}</th>
                      <th style={thStyle}>{t('auth.invite_col_sent_at')}</th>
                      <th style={thStyle}>{t('auth.invite_col_expires_at')}</th>
                      <th style={thStyle}>{t('auth.invite_col_status')}</th>
                      <th style={thStyle}></th>
                    </tr>
                  </thead>
                  <tbody>
                    {invitations.length === 0 ? (
                      <tr>
                        <td colSpan={7} style={{ padding: 24, textAlign: 'center', color: '#9ca3af' }}>
                          {t('auth.invite_list_empty')}
                        </td>
                      </tr>
                    ) : (
                      invitations.map((inv) => {
                        const status = getInviteStatus(inv)
                        const isPendingOrExpired = status === 'pending' || status === 'expired'
                        return (
                          <tr key={inv.id} style={trStyle}>
                            <td style={tdStyle}>
                              <span style={{ fontWeight: 600 }}>{inv.email}</span>
                            </td>
                            <td style={tdStyle}>{inv.role_name}</td>
                            <td style={tdStyle}>{inv.invited_by}</td>
                            <td style={tdStyle}>{new Date(inv.created_at).toLocaleDateString(localeLanguageTag(locale))}</td>
                            <td style={tdStyle}>{new Date(inv.expires_at).toLocaleDateString(localeLanguageTag(locale))}</td>
                            <td style={tdStyle}>
                              <span style={status === 'claimed' ? badgeClaimed : status === 'expired' ? badgeExpired : badgePending}>
                                {status === 'claimed'
                                  ? t('auth.invite_status_claimed')
                                  : status === 'expired'
                                  ? t('auth.invite_status_expired')
                                  : t('auth.invite_status_pending')}
                              </span>
                            </td>
                            <td style={{ ...tdStyle, textAlign: 'right' }}>
                              {isPendingOrExpired && (
                                <div style={{ display: 'flex', gap: 6, justifyContent: 'flex-end' }}>
                                  <button
                                    onClick={() => handleResend(inv.id)}
                                    disabled={actionLoadingId === inv.id}
                                    style={btnSecondary}
                                  >
                                    {actionLoadingId === inv.id ? '...' : t('auth.btn_resend')}
                                  </button>
                                  <button
                                    onClick={() => handleRevoke(inv.id)}
                                    disabled={actionLoadingId === inv.id}
                                    style={btnDanger}
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
              <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
                <div style={successBox}>
                  {t('auth.invite_user_success', { email: inviteEmail })}
                </div>
                <button
                  type="button"
                  style={btnPrimary}
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
                style={{ display: 'flex', flexDirection: 'column', gap: 16 }}
              >
                {inviteError && <div style={errStyleBox}>{t('auth.invite_user_failed', { error: inviteError })}</div>}

                <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
                  <label style={{ fontSize: 13, fontWeight: 500 }} htmlFor="invite-email-input">
                    {t('auth.invite_user_email')}
                  </label>
                  <input
                    id="invite-email-input"
                    type="email"
                    required
                    value={inviteEmail}
                    onChange={(e) => setInviteEmail(e.target.value)}
                    style={inputStyleWide}
                  />
                </div>

                <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
                  <label style={{ fontSize: 13, fontWeight: 500 }} htmlFor="invite-role-input">
                    {t('auth.invite_user_role')}
                  </label>
                  <select
                    id="invite-role-input"
                    value={inviteRole}
                    onChange={(e) => setInviteRole(e.target.value)}
                    style={selectStyleWide}
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
                    style={btnGhost}
                    onClick={() => setShowInviteModal(false)}
                    disabled={inviteLoading}
                  >
                    {t('common.cancel')}
                  </button>
                  <button
                    type="submit"
                    style={btnPrimary}
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

const tableContainer: React.CSSProperties = {
  background: 'var(--bg-card, #ffffff)',
  border: '1px solid var(--border, rgba(255, 255, 255, 0.06))',
  borderRadius: 8,
  overflow: 'hidden',
  boxShadow: 'var(--shadow-sm, 0 1px 3px rgba(0,0,0,0.05))',
}

const tableStyle: React.CSSProperties = {
  width: '100%',
  borderCollapse: 'collapse',
  fontSize: 14,
  textAlign: 'left',
}

const theadRow: React.CSSProperties = {
  background: 'var(--table-header-bg, #f9fafb)',
  borderBottom: '1px solid var(--border, rgba(255, 255, 255, 0.06))',
}

const thStyle: React.CSSProperties = {
  padding: '12px 16px',
  fontWeight: 600,
  color: 'var(--table-header-fg, #4b5563)',
}

const trStyle: React.CSSProperties = {
  borderBottom: '1px solid var(--border, rgba(255, 255, 255, 0.06))',
}

const tdStyle: React.CSSProperties = {
  padding: '12px 16px',
}

const subtext: React.CSSProperties = {
  fontSize: 11,
  color: '#9ca3af',
  fontFamily: 'monospace',
}

const inputStyle: React.CSSProperties = {
  padding: '8px 12px',
  border: '1px solid var(--border, rgba(255, 255, 255, 0.06))',
  background: 'var(--input-bg, #fff)',
  color: 'var(--text-primary, #111)',
  borderRadius: 6,
  fontSize: 14,
  width: '100%',
  maxWidth: 320,
}

const selectStyle: React.CSSProperties = {
  padding: '8px 12px',
  border: '1px solid var(--border, rgba(255, 255, 255, 0.06))',
  borderRadius: 6,
  fontSize: 14,
  background: 'var(--input-bg, #fff)',
  color: 'var(--text-primary, #111)',
}

const btnPrimary: React.CSSProperties = {
  padding: '6px 12px',
  background: 'var(--accent, #4f46e5)',
  color: 'white',
  border: 0,
  borderRadius: 4,
  cursor: 'pointer',
  fontSize: 13,
  fontWeight: 500,
}

const badgeActive: React.CSSProperties = {
  padding: '2px 8px',
  borderRadius: 9999,
  background: 'rgba(16, 185, 129, 0.12)',
  color: 'var(--success, #10b981)',
  fontSize: 12,
  fontWeight: 500,
}

const badgeInactive: React.CSSProperties = {
  padding: '2px 8px',
  borderRadius: 9999,
  background: 'rgba(239, 68, 68, 0.12)',
  color: 'var(--error, #ef4444)',
  fontSize: 12,
  fontWeight: 500,
}

const badgeVerified: React.CSSProperties = {
  padding: '2px 8px',
  borderRadius: 9999,
  background: 'var(--accent-glow, rgba(99, 102, 241, 0.15))',
  color: 'var(--accent, #6366f1)',
  fontSize: 12,
  fontWeight: 500,
}

const badgeUnverified: React.CSSProperties = {
  padding: '2px 8px',
  borderRadius: 9999,
  background: 'rgba(245, 158, 11, 0.14)',
  color: 'var(--warning, #f59e0b)',
  fontSize: 12,
  fontWeight: 500,
}

const countBadge: React.CSSProperties = {
  fontSize: 12,
  background: 'var(--bg-card-raised, rgba(255, 255, 255, 0.08))',
  border: '1px solid var(--border, rgba(255, 255, 255, 0.06))',
  padding: '4px 8px',
  borderRadius: 12,
  color: 'var(--text-secondary, #a1a1aa)',
  fontWeight: 600,
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

const btnSuccess: React.CSSProperties = {
  padding: '6px 12px',
  background: '#10b981',
  color: 'white',
  border: 0,
  borderRadius: 4,
  cursor: 'pointer',
  fontSize: 13,
  fontWeight: 500,
}

const btnGhost: React.CSSProperties = {
  padding: '6px 12px',
  background: 'transparent',
  color: 'var(--text-primary, #111)',
  border: '1px solid var(--border, rgba(255, 255, 255, 0.06))',
  borderRadius: 4,
  cursor: 'pointer',
  fontSize: 13,
  fontWeight: 500,
}

const inputStyleWide: React.CSSProperties = {
  padding: '8px 12px',
  border: '1px solid var(--border, rgba(255, 255, 255, 0.06))',
  background: 'var(--input-bg, #fff)',
  color: 'var(--text-primary, #111)',
  borderRadius: 6,
  fontSize: 14,
  width: '100%',
  boxSizing: 'border-box',
}

const selectStyleWide: React.CSSProperties = {
  padding: '8px 12px',
  border: '1px solid var(--border, rgba(255, 255, 255, 0.06))',
  background: 'var(--input-bg, #fff)',
  color: 'var(--text-primary, #111)',
  borderRadius: 6,
  fontSize: 14,
  width: '100%',
}

const successBox: React.CSSProperties = {
  padding: '12px',
  background: 'rgba(16, 185, 129, 0.12)',
  color: '#10b981',
  borderRadius: 6,
  fontSize: 14,
}

const errStyleBox: React.CSSProperties = {
  padding: '12px',
  background: 'rgba(239, 68, 68, 0.12)',
  color: '#ef4444',
  borderRadius: 6,
  fontSize: 14,
}

const tabContainer: React.CSSProperties = {
  display: 'flex',
  borderBottom: '1px solid var(--border, rgba(255, 255, 255, 0.06))',
  marginBottom: 8,
  gap: 16,
}

const tabButton = (active: boolean): React.CSSProperties => ({
  padding: '8px 16px 12px 16px',
  background: 'transparent',
  border: 0,
  borderBottom: active ? '2px solid var(--accent, #4f46e5)' : '2px solid transparent',
  color: active ? 'var(--text-primary, #111)' : 'var(--text-secondary, #6b7280)',
  cursor: 'pointer',
  fontSize: 14,
  fontWeight: active ? 600 : 500,
  outline: 'none',
  transition: 'all 0.2s',
})

const badgeClaimed: React.CSSProperties = {
  padding: '2px 8px',
  borderRadius: 9999,
  background: 'rgba(16, 185, 129, 0.12)',
  color: 'var(--success, #10b981)',
  fontSize: 12,
  fontWeight: 500,
  display: 'inline-block',
}

const badgePending: React.CSSProperties = {
  padding: '2px 8px',
  borderRadius: 9999,
  background: 'rgba(245, 158, 11, 0.14)',
  color: 'var(--warning, #f59e0b)',
  fontSize: 12,
  fontWeight: 500,
  display: 'inline-block',
}

const badgeExpired: React.CSSProperties = {
  padding: '2px 8px',
  borderRadius: 9999,
  background: 'rgba(239, 68, 68, 0.12)',
  color: 'var(--error, #ef4444)',
  fontSize: 12,
  fontWeight: 500,
  display: 'inline-block',
}

const btnDanger: React.CSSProperties = {
  padding: '4px 10px',
  background: 'rgba(239, 68, 68, 0.1)',
  color: '#ef4444',
  border: '1px solid rgba(239, 68, 68, 0.2)',
  borderRadius: 4,
  cursor: 'pointer',
  fontSize: 12,
  fontWeight: 500,
  transition: 'all 0.2s',
}

const btnSecondary: React.CSSProperties = {
  padding: '4px 10px',
  background: 'var(--bg-card-raised, rgba(255, 255, 255, 0.05))',
  color: 'var(--text-primary, #111)',
  border: '1px solid var(--border, rgba(255, 255, 255, 0.06))',
  borderRadius: 4,
  cursor: 'pointer',
  fontSize: 12,
  fontWeight: 500,
  transition: 'all 0.2s',
}

