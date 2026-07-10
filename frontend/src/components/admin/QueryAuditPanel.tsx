import { useCallback, useMemo, useState } from 'react'

import {
  getQueryAuditDetail,
  listQueryAudit,
  type QueryAuditDetail,
  type QueryAuditEvent,
} from '../../api/queryAudit'
import { useAdminLookups } from '../../hooks/useAdminLookups'
import { useDebouncedValue } from '../../hooks/useDebouncedValue'
import { useFetch } from '../../hooks/useFetch'
import { usePaginatedList } from '../../hooks/usePaginatedList'
import { localeLanguageTag, useLocale, useT } from '../../i18n'
import { cn } from '../../lib/cn'
import type { PageQuery } from '../../types/pagination'
import { formatDateTime } from '../../utils/formatters'
import { DataState } from '../ui/DataState'
import type { ColumnDef } from '../ui/DataTable'
import { DataTable } from '../ui/DataTable'
import { EmptyState } from '../ui/EmptyState'
import { Modal } from '../ui/Modal'
import { Pagination } from '../ui/Pagination'
import {
  adminBadgeActiveClass,
  adminBadgeInactiveClass,
  adminBadgeNeutralClass,
  adminBtnGhostClass,
  adminBtnSecondaryClass,
  adminGridClass,
  adminGridItemClass,
  adminInputClass,
  adminLabelTextClass,
  adminTableContainerClass,
  adminTdClass,
  adminTdMonoClass,
  adminValClass,
  jobDetailModalBodyClass,
  jobDetailModalClass,
} from './adminClasses'
import { AdminPanelShell } from './AdminPanelShell'

const DEFAULT_QUERY_AUDIT_PAGE_SIZE = 25
const QUERY_AUDIT_PAGE_SIZE_OPTIONS = [25, 50, 100, 200]

const sqlPreClass =
  'm-0 max-h-64 overflow-auto rounded-md bg-card-raised p-3 font-mono text-xs whitespace-pre-wrap wrap-break-word text-foreground'

const policyChipClass =
  'inline-flex items-center rounded-full bg-[var(--accent-glow)] px-2 py-0.5 font-mono text-2xs text-accent'

const nowrapTdClass = cn(adminTdClass, 'whitespace-nowrap')
const nowrapTdMonoClass = cn(adminTdMonoClass, 'whitespace-nowrap')

export function QueryAuditPanel({ token }: { token: string }) {
  const t = useT()
  const [locale] = useLocale()
  const [selectedID, setSelectedID] = useState<string | null>(null)
  const { users, datasources } = useAdminLookups(token)

  const [search, setSearch] = useState('')
  const debouncedSearch = useDebouncedValue(search.trim(), 300)

  const fetcher = useCallback(
    async (q: PageQuery) => {
      const res = await listQueryAudit({
        page: q.page,
        pageSize: q.pageSize,
        search: debouncedSearch,
      })
      return { items: res.entries, total: res.total }
    },
    [debouncedSearch],
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
  } = usePaginatedList<QueryAuditEvent>({
    fetcher,
    initialPageSize: DEFAULT_QUERY_AUDIT_PAGE_SIZE,
    fetchKey: token,
    resetPageKey: debouncedSearch,
  })

  const userMap = useMemo(() => new Map(users.map((u) => [u.id, u.email])), [users])
  const dsMap = useMemo(() => new Map(datasources.map((d) => [d.id, d.name])), [datasources])

  const columns: ColumnDef<QueryAuditEvent>[] = [
    {
      key: 'time',
      header: t('admin.query_audit.time'),
      className: nowrapTdMonoClass,
      cell: (e) => formatDateTime(e.timestamp, localeLanguageTag(locale)),
    },
    {
      key: 'status',
      header: t('admin.query_audit.status'),
      className: nowrapTdClass,
      cell: (e) => <StatusBadge eventType={e.event_type} />,
    },
    {
      key: 'user',
      header: t('admin.fields.user'),
      className: nowrapTdMonoClass,
      cell: (e) =>
        e.user_id ? (userMap.get(e.user_id) ?? e.user_id) : t('admin.audit.system_user'),
    },
    {
      key: 'channel',
      header: t('admin.query_audit.channel'),
      className: nowrapTdClass,
      cell: (e) => <span className={adminBadgeNeutralClass}>{e.details?.channel ?? 'api'}</span>,
    },
    {
      key: 'datasource',
      header: t('admin.query_audit.datasource'),
      className: nowrapTdMonoClass,
      cell: (e) => dsMap.get(e.datasource_id) ?? e.datasource_id,
    },
    {
      key: 'policy',
      header: t('admin.query_audit.policy'),
      className: nowrapTdClass,
      cell: (e) => <PolicySummary details={e.details} />,
    },
    {
      key: 'rows',
      header: t('admin.query_audit.rows'),
      align: 'right',
      className: nowrapTdMonoClass,
      cell: (e) => e.details?.row_count ?? '-',
    },
    {
      key: 'duration',
      header: t('admin.query_audit.duration'),
      align: 'right',
      className: nowrapTdMonoClass,
      cell: (e) => (e.details?.duration_ms !== undefined ? `${e.details.duration_ms} ms` : '-'),
    },
    {
      key: 'detail',
      header: '',
      className: nowrapTdClass,
      cell: (e) =>
        e.details?.history_id ? (
          <button
            type="button"
            className={adminBtnGhostClass}
            onClick={() => setSelectedID(e.details?.history_id ?? null)}
          >
            {t('admin.query_audit.detail')}
          </button>
        ) : null,
    },
  ]

  return (
    <AdminPanelShell
      title={t('admin.query_audit.title')}
      description={t('admin.query_audit.description')}
      action={
        <div className="text-foreground-muted text-caption">
          {t('admin.audit.count', { count: totalItems })}
        </div>
      }
    >
      <div className="flex flex-wrap items-center gap-3">
        <input
          type="text"
          placeholder={t('admin.query_audit.search_placeholder')}
          value={search}
          onChange={(e) => {
            setSearch(e.target.value)
            setCurrentPage(1)
          }}
          className={cn(adminInputClass, 'max-w-xs')}
        />
      </div>

      <div className={adminTableContainerClass}>
        <DataState
          loading={loading}
          error={error}
          errorPrefix={t('common.error')}
          empty={entries.length === 0}
          emptyState={<EmptyState description={t('admin.query_audit.empty')} />}
          className="overflow-x-auto"
        >
          <DataTable
            columns={columns}
            rows={entries}
            rowKey={(e) => e.id}
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
            pageSizeOptions={QUERY_AUDIT_PAGE_SIZE_OPTIONS}
            onPageSizeChange={(size) => {
              setPageSize(size)
              setCurrentPage(1)
            }}
            pageSizeLabel={t('admin.audit.page_size')}
          />
        )}
      </div>
      {selectedID && (
        <QueryAuditDetailModal historyID={selectedID} onClose={() => setSelectedID(null)} />
      )}
    </AdminPanelShell>
  )
}

function StatusBadge({ eventType }: { eventType: QueryAuditEvent['event_type'] }) {
  const t = useT()
  return eventType === 'query.executed' ? (
    <span className={adminBadgeActiveClass}>{t('admin.query_audit.executed')}</span>
  ) : (
    <span className={adminBadgeInactiveClass}>{t('admin.query_audit.failed')}</span>
  )
}

function PolicySummary({ details }: { details: QueryAuditEvent['details'] }) {
  const t = useT()
  const rls = details?.row_filters?.length ?? 0
  const masked = details?.masked_columns?.length ?? 0
  const hidden = details?.hidden_columns?.length ?? 0
  if (rls + masked + hidden === 0) {
    return <span className={adminLabelTextClass}>{t('admin.query_audit.no_policy')}</span>
  }
  return (
    <span className="flex flex-wrap gap-1">
      {rls > 0 && <span className={policyChipClass}>RLS ×{rls}</span>}
      {masked > 0 && (
        <span className={policyChipClass}>
          {t('admin.query_audit.masked')} ×{masked}
        </span>
      )}
      {hidden > 0 && (
        <span className={policyChipClass}>
          {t('admin.query_audit.hidden')} ×{hidden}
        </span>
      )}
    </span>
  )
}

function QueryAuditDetailModal({ historyID, onClose }: { historyID: string; onClose: () => void }) {
  const t = useT()
  const { data, loading, error } = useFetch(() => getQueryAuditDetail(historyID), [historyID])

  return (
    <Modal
      open
      title={t('admin.query_audit.detail_title')}
      onClose={onClose}
      className={jobDetailModalClass}
      bodyClassName={jobDetailModalBodyClass}
    >
      <DataState loading={loading} error={error} errorPrefix={t('common.error')} empty={!data}>
        {data && <QueryAuditDetailBody detail={data} />}
      </DataState>
    </Modal>
  )
}

function QueryAuditDetailBody({ detail }: { detail: QueryAuditDetail }) {
  const t = useT()
  const [locale] = useLocale()
  const [copied, setCopied] = useState(false)
  const { history, audit } = detail
  const details = audit?.details

  const copySQL = () => {
    if (!history.compiled_sql) {
      return
    }
    void navigator.clipboard.writeText(history.compiled_sql).then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), 1500)
    })
  }

  return (
    <div className="flex flex-col gap-4">
      <div className={adminGridClass}>
        <MetaItem label={t('admin.query_audit.time')}>
          {formatDateTime(history.created_at, localeLanguageTag(locale))}
        </MetaItem>
        <MetaItem label={t('admin.query_audit.status')}>{history.status}</MetaItem>
        <MetaItem label={t('admin.query_audit.channel')}>{details?.channel ?? 'api'}</MetaItem>
        <MetaItem label={t('admin.query_audit.rows')}>{history.row_count ?? '-'}</MetaItem>
        <MetaItem label={t('admin.query_audit.duration')}>
          {history.duration_ms !== null ? `${history.duration_ms} ms` : '-'}
        </MetaItem>
        <MetaItem label={t('admin.query_audit.fingerprint')}>
          <span className="font-mono text-xs">{history.fingerprint ?? '-'}</span>
        </MetaItem>
      </div>

      <PolicyDetailSection details={details} />

      <section className="flex flex-col gap-2">
        <div className="flex items-center justify-between">
          <h4 className="m-0 text-sm font-semibold">{t('admin.query_audit.governed_sql')}</h4>
          <button type="button" className={adminBtnSecondaryClass} onClick={copySQL}>
            {copied ? t('admin.query_audit.copied') : t('admin.query_audit.copy')}
          </button>
        </div>
        <pre className={sqlPreClass}>{history.compiled_sql ?? '-'}</pre>
      </section>

      {history.error_message && (
        <section className="flex flex-col gap-2">
          <h4 className="text-error m-0 text-sm font-semibold">{t('admin.query_audit.error')}</h4>
          <pre className={sqlPreClass}>{history.error_message}</pre>
        </section>
      )}
    </div>
  )
}

function MetaItem({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className={adminGridItemClass}>
      <span className={adminLabelTextClass}>{label}</span>
      <span className={adminValClass}>{children}</span>
    </div>
  )
}

function PolicyDetailSection({ details }: { details: QueryAuditEvent['details'] }) {
  const t = useT()
  const rowFilters = details?.row_filters ?? []
  const masked = details?.masked_columns ?? []
  const hidden = details?.hidden_columns ?? []

  return (
    <section className="flex flex-col gap-2">
      <h4 className="m-0 text-sm font-semibold">{t('admin.query_audit.policy')}</h4>
      {rowFilters.length + masked.length + hidden.length === 0 ? (
        <span className={adminLabelTextClass}>{t('admin.query_audit.no_policy')}</span>
      ) : (
        <div className="flex flex-wrap gap-1">
          {rowFilters.map((f) => (
            <span key={`rls-${f.field}-${f.operator}`} className={policyChipClass}>
              RLS: {f.field} {f.operator} {formatFilterValue(f.value)}
            </span>
          ))}
          {masked.map((c) => (
            <span key={`m-${c}`} className={policyChipClass}>
              {t('admin.query_audit.masked')}: {c}
            </span>
          ))}
          {hidden.map((c) => (
            <span key={`h-${c}`} className={policyChipClass}>
              {t('admin.query_audit.hidden')}: {c}
            </span>
          ))}
        </div>
      )}
    </section>
  )
}

function formatFilterValue(value: unknown): string {
  if (value === undefined || value === null) {
    return ''
  }
  if (typeof value === 'string') {
    return value
  }
  return JSON.stringify(value)
}
