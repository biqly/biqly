/* eslint-disable react-refresh/only-export-components */

import { useCallback, useEffect, useMemo, useState } from 'react'

import { listAuditLog } from '../../api/admin'
import { useAdminLookups } from '../../hooks/useAdminLookups'
import { localeLanguageTag, useLocale, useT } from '../../i18n'
import type { AuditLogEntry } from '../../types/auth'
import { LoadingOverlay } from '../ui/LoadingOverlay'
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
  const [entries, setEntries] = useState<AuditLogEntry[]>([])
  const [userID, setUserID] = useState('')
  const [action, setAction] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  // Lookups for friendly name mapping using custom hook
  const { users, datasources, workspaces } = useAdminLookups(token)

  // Pagination
  const [currentPage, setCurrentPage] = useState(1)
  const [pageSize, setPageSize] = useState(DEFAULT_AUDIT_PAGE_SIZE)
  const [totalItems, setTotalItems] = useState(0)
  const totalPages = Math.ceil(totalItems / pageSize)
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

  const reload = useCallback(
    async (nextFilters = { userID, action, page: currentPage, pageSize }) => {
      setLoading(true)
      try {
        const res = await listAuditLog(token, {
          userID: nextFilters.userID,
          action: nextFilters.action,
          page: nextFilters.page,
          pageSize: nextFilters.pageSize,
        })
        setEntries(res.entries)
        setTotalItems(res.total)
        setError(null)
      } catch (e) {
        setError(e instanceof Error ? e.message : String(e))
      } finally {
        setLoading(false)
      }
    },
    [action, currentPage, pageSize, token, userID],
  )

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void reload({ userID, action, page: currentPage, pageSize })
    // Filter fields apply on submit; reload when auth or page changes only.
    // eslint-disable-next-line react-hooks/exhaustive-deps -- intentional filter-on-submit
  }, [token, currentPage])

  function onSubmit(e: React.SubmitEvent<HTMLFormElement>) {
    e.preventDefault()
    if (currentPage !== 1) {
      setCurrentPage(1)
    } else {
      void reload({ userID, action, page: 1, pageSize })
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
              const nextSize = Number(v)
              setPageSize(nextSize)
              setCurrentPage(1)
              void reload({ userID, action, page: 1, pageSize: nextSize })
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
            void reload({ userID: '', action: '', page: 1, pageSize: DEFAULT_AUDIT_PAGE_SIZE })
          }}
          className="admin-btn-secondary"
        >
          {t('admin.filters.reset')}
        </button>
      </form>

      {error && (
        <div className="admin-err-text">
          {t('common.error')}: {error}
        </div>
      )}

      <div className="admin-table-container">
        <LoadingOverlay loading={loading} label={t('common.loading')}>
          <div
            style={{
              overflowX: 'auto',
              minHeight: entries.length === 0 && loading ? 120 : 'auto',
              display: 'flex',
              flexDirection: 'column',
            }}
          >
            {entries.length === 0 ? (
              <div
                className="admin-text-muted"
                style={{ padding: 48, textAlign: 'center', width: '100%', border: 0 }}
              >
                {loading ? '' : t('admin.audit.empty')}
              </div>
            ) : (
              <table className="admin-table" style={{ fontSize: 13, minWidth: 980 }}>
                <thead>
                  <tr className="admin-thead-row">
                    <th className="admin-th">{t('admin.audit.time')}</th>
                    <th className="admin-th">{t('admin.audit.action')}</th>
                    <th className="admin-th">{t('admin.fields.user')}</th>
                    <th className="admin-th">{t('admin.audit.resource')}</th>
                    <th className="admin-th">IP</th>
                    <th className="admin-th">Metadata</th>
                  </tr>
                </thead>
                <tbody>
                  {displayedEntries.map((entry) => (
                    <tr key={entry.id} className="admin-tr">
                      <td className="admin-td-mono">
                        {formatDate(entry.created_at, localeLanguageTag(locale))}
                      </td>
                      <td className="admin-td">
                        <span className="admin-badge-action">{entry.action}</span>
                      </td>
                      <td className="admin-td-mono">
                        {entry.user_id
                          ? (userMap.get(entry.user_id) ?? entry.user_id)
                          : t('admin.audit.system_user')}
                      </td>
                      <td className="admin-td-mono">{formatResource(entry, dsMap, wsMap)}</td>
                      <td className="admin-td-mono">{entry.ip_address ?? '-'}</td>
                      <td className="admin-td-metadata">{formatMetadata(entry.metadata)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        </LoadingOverlay>

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

function formatDate(value: string, languageTag: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value
  }
  return date.toLocaleString(languageTag)
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
