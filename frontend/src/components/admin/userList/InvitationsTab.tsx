import { useEffect, useMemo, useState } from 'react'

import { localeLanguageTag, type TFunction } from '../../../i18n'
import type { Invitation } from '../../../types/auth'
import { formatDateOnly } from '../../../utils/formatters'
import { DataState } from '../../ui/DataState'
import type { ColumnDef } from '../../ui/DataTable'
import { DataTable } from '../../ui/DataTable'
import { Pagination } from '../../ui/Pagination'
import { Select } from '../../ui/Select'

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
  inviteTotalPages: number
  inviteTotalItems: number
  pageSize: number
  locale: 'en' | 'tr'
  t: TFunction
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
  inviteTotalPages,
  inviteTotalItems,
  pageSize,
  locale,
  t,
}: InvitationsTabProps) {
  const [now, setNow] = useState<number | null>(null)

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setNow(Date.now())
  }, [])

  const getInviteStatus = (inv: Invitation): 'claimed' | 'expired' | 'pending' => {
    if (inv.claimed_at) {
      return 'claimed'
    }
    if (now == null) {
      return 'pending'
    }
    const isExpired = new Date(inv.expires_at).getTime() < now
    return isExpired ? 'expired' : 'pending'
  }

  const statusOptions = useMemo(
    () => [
      { value: 'all', label: t('auth.invite_status_all') },
      { value: 'pending', label: t('auth.invite_status_pending') },
      { value: 'claimed', label: t('auth.invite_status_claimed') },
      { value: 'expired', label: t('auth.invite_status_expired') },
    ],
    [t],
  )

  const columns: ColumnDef<Invitation>[] = [
    {
      key: 'email',
      header: t('auth.invite_col_email'),
      cell: (inv) => <span style={{ fontWeight: 600 }}>{inv.email}</span>,
    },
    { key: 'role', header: t('auth.invite_col_role'), cell: (inv) => inv.role_name },
    { key: 'invited_by', header: t('auth.invite_col_invited_by'), cell: (inv) => inv.invited_by },
    {
      key: 'sent_at',
      header: t('auth.invite_col_sent_at'),
      cell: (inv) => formatDateOnly(inv.created_at, localeLanguageTag(locale)),
    },
    {
      key: 'expires_at',
      header: t('auth.invite_col_expires_at'),
      cell: (inv) => formatDateOnly(inv.expires_at, localeLanguageTag(locale)),
    },
    {
      key: 'status',
      header: t('auth.invite_col_status'),
      cell: (inv) => {
        const status = getInviteStatus(inv)
        return (
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
        )
      },
    },
    {
      key: 'actions',
      header: '',
      align: 'right',
      cell: (inv) => {
        const status = getInviteStatus(inv)
        if (status !== 'pending' && status !== 'expired') {
          return null
        }
        return (
          <div style={{ display: 'flex', gap: 6, justifyContent: 'flex-end' }}>
            <button
              type="button"
              onClick={() => {
                void handleResend(inv.id)
              }}
              disabled={actionLoadingId === inv.id}
              className="admin-btn-secondary"
            >
              {actionLoadingId === inv.id ? '...' : t('auth.btn_resend')}
            </button>
            <button
              type="button"
              onClick={() => {
                void handleRevoke(inv.id)
              }}
              disabled={actionLoadingId === inv.id}
              className="admin-btn-danger"
            >
              {actionLoadingId === inv.id ? '...' : t('auth.btn_revoke')}
            </button>
          </div>
        )
      },
    },
  ]

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
        <div style={{ minWidth: 180 }}>
          <Select
            value={inviteStatusFilter}
            options={statusOptions}
            onChange={(v) => setInviteStatusFilter(v as InvitationsTabProps['inviteStatusFilter'])}
          />
        </div>
      </div>

      <div className="admin-table-container">
        {actionMessage && (
          <div
            className={actionMessage.type === 'success' ? 'admin-success-box' : 'admin-err-box'}
            onClick={() => setActionMessage(null)}
            role="button"
            tabIndex={0}
            onKeyDown={(e) => {
              if (e.key === 'Enter' || e.key === ' ') {
                setActionMessage(null)
              }
            }}
          >
            {actionMessage.text}
          </div>
        )}
        <DataState
          loading={invitesLoading}
          error={invitesError}
          errorPrefix={t('common.error')}
          empty={invitations.length === 0}
        >
          <DataTable
            columns={columns}
            rows={invitations}
            rowKey={(inv) => inv.id}
            loading={invitesLoading}
            emptyCell={t('auth.invite_list_empty')}
          />
        </DataState>
        <Pagination
          currentPage={inviteCurrentPage}
          totalPages={inviteTotalPages}
          onPageChange={setInviteCurrentPage}
          totalItems={inviteTotalItems}
          itemsPerPage={pageSize}
          alwaysShow
        />
      </div>
    </>
  )
}
