import React from 'react'
import type { Invitation } from '../../../types/auth'
import { localeLanguageTag } from '../../../i18n'
import { Pagination } from '../../ui/Pagination'
import { LoadingOverlay } from '../../ui/LoadingOverlay'

interface InvitationsTabProps {
  inviteSearch: string
  setInviteSearch: (v: string) => void
  inviteStatusFilter: 'all' | 'pending' | 'claimed' | 'expired'
  setInviteStatusFilter: (v: 'all' | 'pending' | 'claimed' | 'expired') => void
  actionMessage: { type: 'success' | 'error'; text: string } | null
  setActionMessage: (v: { type: 'success' | 'error'; text: string } | null) => void
  invitations: Invitation[]
  invitesLoading: boolean
  invitesError: string | null
  actionLoadingId: string | null
  handleResend: (id: string) => Promise<void>
  handleRevoke: (id: string) => Promise<void>
  inviteCurrentPage: number
  setInviteCurrentPage: (page: number) => void
  inviteTotalItems: number
  pageSize: number
  locale: 'en' | 'tr'
  t: any
}

export function InvitationsTab({
  inviteSearch,
  setInviteSearch,
  inviteStatusFilter,
  setInviteStatusFilter,
  actionMessage,
  setActionMessage,
  invitations,
  invitesLoading,
  invitesError,
  actionLoadingId,
  handleResend,
  handleRevoke,
  inviteCurrentPage,
  setInviteCurrentPage,
  inviteTotalItems,
  pageSize,
  locale,
  t,
}: InvitationsTabProps) {
  const getInviteStatus = (inv: Invitation): 'claimed' | 'expired' | 'pending' => {
    if (inv.claimed_at) return 'claimed'
    const isExpired = new Date(inv.expires_at).getTime() < Date.now()
    return isExpired ? 'expired' : 'pending'
  }

  return (
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
            onKeyDown={(e) => {
              if (e.key === 'Enter' || e.key === ' ') setActionMessage(null)
            }}
          >
            {actionMessage.text}
          </div>
        )}
        {invitesError ? (
          <div className="admin-err-text">
            {t('common.error')}: {invitesError}
          </div>
        ) : (
          <LoadingOverlay loading={invitesLoading}>
            <div style={{ minHeight: invitations.length === 0 && invitesLoading ? 120 : 'auto', display: 'flex', flexDirection: 'column' }}>
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
                        {invitesLoading ? '' : t('auth.invite_list_empty')}
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
                          <td className="admin-td">
                            {new Date(inv.created_at).toLocaleDateString(localeLanguageTag(locale))}
                          </td>
                          <td className="admin-td">
                            {new Date(inv.expires_at).toLocaleDateString(localeLanguageTag(locale))}
                          </td>
                          <td className="admin-td">
                            <span
                              className={
                                status === 'claimed'
                                  ? 'admin-badge-claimed'
                                  : status === 'expired'
                                  ? 'admin-badge-expired'
                                  : 'admin-badge-pending'
                              }
                            >
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
                                  type="button"
                                  onClick={() => handleResend(inv.id)}
                                  disabled={actionLoadingId === inv.id}
                                  className="admin-btn-secondary"
                                >
                                  {actionLoadingId === inv.id ? '...' : t('auth.btn_resend')}
                                </button>
                                <button
                                  type="button"
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
            </div>
          </LoadingOverlay>
        )}
        {!invitesError && (
          <Pagination
            currentPage={inviteCurrentPage}
            totalPages={Math.ceil(inviteTotalItems / pageSize)}
            onPageChange={setInviteCurrentPage}
            totalItems={inviteTotalItems}
            itemsPerPage={pageSize}
          />
        )}
      </div>
    </>
  )
}
