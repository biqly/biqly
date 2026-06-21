import { useMemo } from 'react'

import { localeLanguageTag, useLocale, useT } from '../../../i18n'
import { cn } from '../../../lib/cn'
import type { AuthUser } from '../../../types/auth'
import { formatDateOnly } from '../../../utils/formatters'
import { DEFAULT_TABLE_PAGE_SIZE_OPTIONS } from '../../../utils/paging'
import { DataState } from '../../ui/DataState'
import type { ColumnDef } from '../../ui/DataTable'
import { DataTable } from '../../ui/DataTable'
import { Pagination } from '../../ui/Pagination'
import { Select } from '../../ui/Select'
import {
  adminActiveBadgeClass,
  adminBtnPrimaryClass,
  adminBtnSecondaryClass,
  adminInputClass,
  adminListAvatarClass,
  adminMessageBoxClass,
  adminMfaStatusBadgeClass,
  adminPasskeyBadgeClass,
  adminRowRevealActionClass,
  adminSubtextClass,
  adminTableContainerClass,
  adminTableRowHoverClass,
  adminUserSecurityClass,
  adminVerifiedBadgeClass,
} from '../adminClasses'

interface ActiveUsersTabProps {
  search: string
  setSearch: (v: string) => void
  statusFilter: 'all' | 'active' | 'inactive'
  setStatusFilter: (v: 'all' | 'active' | 'inactive') => void
  actionMessage: { type: 'success' | 'error'; text: string } | null
  setActionMessage: (v: { type: 'success' | 'error'; text: string } | null) => void
  displayedUsers: AuthUser[]
  verificationLoadingId: string | null
  handleResendVerification: (id: string) => Promise<void>
  onSelectUser: (id: string, label: string) => void
  currentPage: number
  totalPages: number
  setCurrentPage: (page: number) => void
  totalItems: number
  pageSize: number
  onPageSizeChange: (size: number) => void
  loading?: boolean
}

export function ActiveUsersTab({
  search,
  setSearch,
  statusFilter,
  setStatusFilter,
  actionMessage,
  setActionMessage,
  displayedUsers,
  verificationLoadingId,
  handleResendVerification,
  onSelectUser,
  currentPage,
  totalPages,
  setCurrentPage,
  totalItems,
  pageSize,
  onPageSizeChange,
  loading = false,
}: ActiveUsersTabProps) {
  const t = useT()
  const [locale] = useLocale()
  const statusOptions = useMemo(
    () => [
      { value: 'all', label: t('admin.users.status_all') },
      { value: 'active', label: t('admin.users.status_active') },
      { value: 'inactive', label: t('admin.users.status_inactive') },
    ],
    [t],
  )

  const columns: ColumnDef<AuthUser>[] = [
    {
      key: 'user',
      header: t('admin.users.col_user'),
      cell: (u) => (
        <div className="flex items-center gap-3">
          <div
            className={cn(
              adminListAvatarClass,
              'text-accent flex h-8 w-8 shrink-0 items-center justify-center overflow-hidden rounded-full bg-[color-mix(in_srgb,var(--accent)_15%,transparent)] text-xs font-bold',
            )}
          >
            {u.avatarUrl ? (
              <img src={u.avatarUrl} alt="" className="h-full w-full object-cover" />
            ) : u.displayName ? (
              u.displayName.slice(0, 2).toUpperCase()
            ) : (
              u.email.slice(0, 2).toUpperCase()
            )}
          </div>
          <div className="flex flex-col">
            <span className="font-semibold">{u.email}</span>
            {u.username && <span className={adminSubtextClass}>{u.username}</span>}
          </div>
        </div>
      ),
    },
    {
      key: 'name',
      header: t('admin.users.col_name'),
      cell: (u) => u.displayName ?? t('common.em_dash'),
    },
    {
      key: 'status',
      header: t('admin.users.col_status'),
      cell: (u) => (
        <span className={adminActiveBadgeClass(u.isActive)}>
          {u.isActive ? t('admin.users.status_active') : t('admin.users.status_inactive')}
        </span>
      ),
    },
    {
      key: 'email_verification',
      header: t('admin.users.col_email_verification'),
      cell: (u) => (
        <div className="flex flex-col items-start gap-1.5">
          <span className={adminVerifiedBadgeClass(u.emailVerified)}>
            {u.emailVerified ? t('admin.users.email_verified') : t('admin.users.email_unverified')}
          </span>
          {!u.emailVerified && (
            <button
              type="button"
              onClick={() => {
                void handleResendVerification(u.id)
              }}
              disabled={verificationLoadingId === u.id}
              className={adminBtnSecondaryClass}
            >
              {verificationLoadingId === u.id ? '...' : t('admin.users.resend_verification')}
            </button>
          )}
        </div>
      ),
    },
    {
      key: 'security',
      header: t('admin.users.col_security'),
      cell: (u) => (
        <div className={adminUserSecurityClass}>
          <span className={adminMfaStatusBadgeClass(u.mfaEnabled, u.mfaPending)}>
            {u.mfaEnabled
              ? t('admin.users.mfa_active')
              : u.mfaPending
                ? t('admin.users.mfa_pending')
                : t('admin.users.mfa_off')}
          </span>
          <span className={adminPasskeyBadgeClass(u.passkeyCount ?? 0)}>
            {(u.passkeyCount ?? 0) > 0
              ? t('admin.users.passkeys_count', { count: u.passkeyCount ?? 0 })
              : t('admin.users.passkeys_none')}
          </span>
        </div>
      ),
    },
    {
      key: 'created_at',
      header: t('admin.users.col_created_at'),
      cell: (u) => formatDateOnly(u.createdAt, localeLanguageTag(locale)),
    },
    {
      key: 'actions',
      header: '',
      align: 'right',
      cell: (u) => (
        <button
          type="button"
          onClick={() => onSelectUser(u.id, u.displayName?.trim() ?? u.email)}
          className={`${adminBtnPrimaryClass} ${adminRowRevealActionClass}`}
        >
          {t('admin.users.manage')}
        </button>
      ),
    },
  ]

  return (
    <>
      <div className="flex flex-wrap items-center gap-3">
        <input
          type="text"
          placeholder={t('admin.users.search_placeholder')}
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className={cn(adminInputClass, 'max-w-xs')}
        />
        <div className="min-w-45">
          <Select
            value={statusFilter}
            options={statusOptions}
            onChange={(v) => setStatusFilter(v as ActiveUsersTabProps['statusFilter'])}
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
        <DataState loading={loading} empty={displayedUsers.length === 0}>
          <DataTable
            columns={columns}
            rows={displayedUsers}
            rowKey={(u) => u.id}
            loading={loading}
            emptyCell={t('admin.users.empty')}
            rowClassName={adminTableRowHoverClass}
          />
        </DataState>
        <Pagination
          currentPage={currentPage}
          totalPages={totalPages}
          onPageChange={setCurrentPage}
          totalItems={totalItems}
          itemsPerPage={pageSize}
          pageSizeOptions={DEFAULT_TABLE_PAGE_SIZE_OPTIONS}
          onPageSizeChange={onPageSizeChange}
          alwaysShow
        />
      </div>
    </>
  )
}
