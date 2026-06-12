/* eslint-disable react-refresh/only-export-components */
import { useCallback, useMemo, useState } from 'react'

import { listAuditLog } from '../../api/admin'
import { useAdminLookups } from '../../hooks/useAdminLookups'
import { usePaginatedList } from '../../hooks/usePaginatedList'
import { localeLanguageTag, useLocale, useT } from '../../i18n'
import type { AuditLogEntry } from '../../types/auth'
import type { PageQuery } from '../../types/pagination'
import { formatDateTime } from '../../utils/formatters'
import { DataState } from '../ui/DataState'
import type { ColumnDef } from '../ui/DataTable'
import { DataTable } from '../ui/DataTable'
import { EmptyState } from '../ui/EmptyState'
import { Pagination } from '../ui/Pagination'
import { Select } from '../ui/Select'
import { numberSelectOptions, stringSelectOptions, userSelectOptions } from './adminSelectOptions'

const COMMON_ACTIONS = [
  'login.success',
  'login.failed',
  'login.locked',
  'login.mfa_required',
  'logout',
  'user.register',
  'password.reset',
  'password.change',
  'email.verified',
  'session.refresh',
  'session.revoked',
  'oauth.login',
  'mfa.enrolled',
  'mfa.verified',
  'mfa.disabled',
  'role.assigned',
  'role.removed',
  'datasource.grant',
  'datasource.revoke',
  'datasource.update_level',
  'datasource.request_access',
  'share.create',
  'share.revoke',
  'audit.export',
  'user.data_export',
  'admin.blocked_self_change',
  'account.frozen',
  'account.unfrozen',
  'account.soft_deleted',
  'account.restored',
  'account.purged',
  'account.unlocked',
  'login.new_device',
  'session.evicted',
  'admin.force_logout',
  'password.expired',
].sort()

export const DEFAULT_AUDIT_PAGE_SIZE = 10
export const AUDIT_PAGE_SIZE_OPTIONS = [10, 25, 50, 100, 250]

export function AuditLogPanel({ token }: { token: string }) {
  const t = useT()
  const [locale] = useLocale()
  const [userID, setUserID] = useState('')
  const [action, setAction] = useState('')

  // Lookups for friendly name mapping using custom hook
  const { users, datasources, workspaces } = useAdminLookups(token)

  // Filters apply on submit: the fetcher reads the currently-typed values, but
  // usePaginatedList only refetches on page/pageSize/reload — typing never fetches.
  const fetcher = useCallback(
    async (q: PageQuery) => {
      const res = await listAuditLog(token, {
        userID,
        action,
        page: q.page,
        pageSize: q.pageSize,
      })
      return { items: res.entries, total: res.total }
    },
    [token, userID, action],
  )
  const {
    items: entries,
    loading,
    error,
    page: currentPage,
    setPage: setCurrentPage,
    pageSize,
    setPageSize,
    totalPages,
    total: totalItems,
    reload,
  } = usePaginatedList<AuditLogEntry>({
    fetcher,
    initialPageSize: DEFAULT_AUDIT_PAGE_SIZE,
    fetchKey: token,
    syncToUrl: 'auditPage',
  })
  const displayedEntries = entries

  const userMap = useMemo(() => new Map(users.map((u) => [u.id, u.email])), [users])
  const dsMap = useMemo(() => new Map(datasources.map((d) => [d.id, d.name])), [datasources])
  const wsMap = useMemo(() => new Map(workspaces.map((w) => [w.id, w.name])), [workspaces])

  const userFilterOptions = useMemo(
    () => userSelectOptions(users, t('admin.filters.all')),
    [users, t],
  )
  const actionFilterOptions = useMemo(
    () => stringSelectOptions(COMMON_ACTIONS, t('admin.filters.all')),
    [t],
  )
  const pageSizeOptions = useMemo(() => numberSelectOptions(AUDIT_PAGE_SIZE_OPTIONS), [])

  const auditColumns: ColumnDef<AuditLogEntry>[] = [
    {
      key: 'time',
      header: t('admin.audit.time'),
      className: 'admin-td-mono',
      cell: (entry) => formatDateTime(entry.created_at, localeLanguageTag(locale)),
    },
    {
      key: 'action',
      header: t('admin.audit.action'),
      cell: (entry) => <span className="admin-badge-action">{entry.action}</span>,
    },
    {
      key: 'user',
      header: t('admin.fields.user'),
      className: 'admin-td-mono',
      cell: (entry) =>
        entry.user_id
          ? (userMap.get(entry.user_id) ?? entry.user_id)
          : t('admin.audit.system_user'),
    },
    {
      key: 'resource',
      header: t('admin.audit.resource'),
      className: 'admin-td-mono',
      cell: (entry) => formatResource(entry, dsMap, wsMap),
    },
    {
      key: 'ip',
      header: 'IP',
      className: 'admin-td-mono',
      cell: (entry) => entry.ip_address ?? '-',
    },
    {
      key: 'metadata',
      header: 'Metadata',
      className: 'admin-td-metadata',
      cell: (entry) => formatMetadata(entry.metadata),
    },
  ]

  function onSubmit(e: React.SubmitEvent<HTMLFormElement>) {
    e.preventDefault()
    if (currentPage !== 1) {
      setCurrentPage(1)
    } else {
      reload()
    }
  }

  return (
    <div className="page-stack">
      <div className="card-header-row" style={{ alignItems: 'flex-start' }}>
        <div>
          <h2 style={{ margin: 0, fontSize: 18 }}>{t('admin.audit.title')}</h2>
          <p style={{ margin: '4px 0 0', color: 'var(--text-secondary, #a1a1aa)', fontSize: 13 }}>
            {t('admin.audit.description')}
          </p>
        </div>
        <div style={{ color: 'var(--text-secondary, #a1a1aa)', fontSize: 13 }}>
          {t('admin.audit.count', { count: totalItems })}
        </div>
      </div>

      <form
        onSubmit={onSubmit}
        style={{ display: 'flex', gap: 12, alignItems: 'flex-end', flexWrap: 'wrap' }}
      >
        <label className="admin-form-label" style={{ gap: 4, minWidth: 220 }}>
          <span className="admin-label-text">{t('admin.fields.user')}</span>
          <Select value={userID} options={userFilterOptions} onChange={setUserID} />
        </label>
        <label className="admin-form-label" style={{ gap: 4, minWidth: 200 }}>
          <span className="admin-label-text">{t('admin.audit.action')}</span>
          <Select value={action} options={actionFilterOptions} onChange={setAction} />
        </label>
        <label className="admin-form-label" style={{ gap: 4, minWidth: 120 }}>
          <span className="admin-label-text">{t('admin.audit.page_size')}</span>
          <Select
            value={String(pageSize)}
            options={pageSizeOptions}
            onChange={(v) => {
              setPageSize(Number(v))
              setCurrentPage(1)
              reload()
            }}
          />
        </label>
        <button type="submit" className="admin-btn-primary">
          {t('admin.filters.apply')}
        </button>
        <button
          type="button"
          onClick={() => {
            setUserID('')
            setAction('')
            setPageSize(DEFAULT_AUDIT_PAGE_SIZE)
            setCurrentPage(1)
            reload()
          }}
          className="admin-btn-secondary"
        >
          {t('admin.filters.reset')}
        </button>
      </form>

      <div className="admin-table-container">
        <DataState
          loading={loading}
          error={error}
          errorPrefix={t('common.error')}
          empty={entries.length === 0}
          emptyState={<EmptyState description={t('admin.audit.empty')} />}
          className="data-state__body--scroll-x"
        >
          <DataTable
            columns={auditColumns}
            rows={displayedEntries}
            rowKey={(entry) => entry.id}
            loading={loading}
            tableStyle={{ fontSize: 13, minWidth: 980 }}
          />
        </DataState>

        {entries.length > 0 && (
          <Pagination
            currentPage={currentPage}
            totalPages={totalPages}
            onPageChange={setCurrentPage}
            totalItems={totalItems}
            itemsPerPage={pageSize}
          />
        )}
      </div>
    </div>
  )
}

function formatResource(
  entry: AuditLogEntry,
  dsMap: Map<string, string>,
  wsMap: Map<string, string>,
) {
  if (!entry.resource && !entry.resource_id) {
    return '-'
  }
  if (entry.resource === 'datasource' && entry.resource_id && dsMap.has(entry.resource_id)) {
    return `datasource:${dsMap.get(entry.resource_id)}`
  }
  if (entry.resource === 'workspace' && entry.resource_id && wsMap.has(entry.resource_id)) {
    return `workspace:${wsMap.get(entry.resource_id)}`
  }
  if (!entry.resource_id) {
    return entry.resource
  }
  if (!entry.resource) {
    return entry.resource_id
  }
  return `${entry.resource}:${entry.resource_id}`
}

function formatMetadata(value: unknown) {
  if (value === undefined || value === null) {
    return '-'
  }
  if (typeof value === 'string') {
    return value
  }
  return JSON.stringify(value)
}
