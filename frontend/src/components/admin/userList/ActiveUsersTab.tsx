import { useMemo } from 'react'
import type { AuthUser } from '../../../types/auth'
import { localeLanguageTag } from '../../../i18n'
import { Pagination } from '../../ui/Pagination'
import { LoadingOverlay } from '../../ui/LoadingOverlay'
import { Select } from '../../ui/Select'

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
  locale: 'en' | 'tr'
  t: any
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
  locale,
  t,
  loading = false,
}: ActiveUsersTabProps) {
  const statusOptions = useMemo(
    () => [
      { value: 'all', label: t('admin.users.status_all') },
      { value: 'active', label: t('admin.users.status_active') },
      { value: 'inactive', label: t('admin.users.status_inactive') },
    ],
    [t],
  )

  return (
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
        <div style={{ minWidth: 180 }}>
          <Select
            value={statusFilter}
            options={statusOptions}
            onChange={(v) => setStatusFilter(v as ActiveUsersTabProps['statusFilter'])}
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
              if (e.key === 'Enter' || e.key === ' ') setActionMessage(null)
            }}
          >
            {actionMessage.text}
          </div>
        )}
        <LoadingOverlay loading={loading}>
          <div style={{ minHeight: displayedUsers.length === 0 && loading ? 120 : 'auto', display: 'flex', flexDirection: 'column' }}>
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
                      {loading ? '' : t('admin.users.empty')}
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
                      <td className="admin-td">
                        {new Date(u.createdAt).toLocaleDateString(localeLanguageTag(locale))}
                      </td>
                      <td className="admin-td" style={{ textAlign: 'right' }}>
                        <button
                          onClick={() => onSelectUser(u.id, u.displayName?.trim() || u.email)}
                          className="admin-btn-primary"
                        >
                          {t('admin.users.manage')}
                        </button>
                      </td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </div>
        </LoadingOverlay>
        <Pagination
          currentPage={currentPage}
          totalPages={totalPages}
          onPageChange={setCurrentPage}
          totalItems={totalItems}
          itemsPerPage={pageSize}
          alwaysShow
        />
      </div>
    </>
  )
}
