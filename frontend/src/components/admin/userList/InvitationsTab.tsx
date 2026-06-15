import { useEffect, useMemo, useState } from 'react'

import { localeLanguageTag, useLocale, useT } from '../../../i18n'
import type { Invitation } from '../../../types/auth'
import { formatDateOnly } from '../../../utils/formatters'
import { DEFAULT_TABLE_PAGE_SIZE_OPTIONS } from '../../../utils/paging'
import { DataState } from '../../ui/DataState'
import type { ColumnDef } from '../../ui/DataTable'
import { DataTable } from '../../ui/DataTable'
import { Pagination } from '../../ui/Pagination'
import { Select } from '../../ui/Select'
import {
  adminBtnDangerClass,
  adminBtnSecondaryClass,
  adminInputClass,
  adminInvitationBadgeClass,
  adminMessageBoxClass,
  adminTableContainerClass,
} from '../adminClasses'

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
  onPageSizeChange: (size: number) => void
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
  onPageSizeChange,
}: InvitationsTabProps) {
  const t = useT()
  const [locale] = useLocale()
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
          <span className={adminInvitationBadgeClass(status)}>
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
              className={adminBtnSecondaryClass}
            >
              {actionLoadingId === inv.id ? '...' : t('auth.btn_resend')}
            </button>
            <button
              type="button"
              onClick={() => {
                void handleRevoke(inv.id)
              }}
              disabled={actionLoadingId === inv.id}
              className={adminBtnDangerClass}
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
          className={adminInputClass}
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

      <div className={adminTableContainerClass}>
        {actionMessage && (
          <div
            className={adminMessageBoxClass(actionMessage.type)}
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
          pageSizeOptions={DEFAULT_TABLE_PAGE_SIZE_OPTIONS}
          onPageSizeChange={onPageSizeChange}
          alwaysShow
        />
      </div>
    </>
  )
}
