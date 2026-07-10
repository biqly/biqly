import { useCallback, useMemo, useState } from 'react'

import { type AIUsageBreakdownRow, type AIUsageTotals, getAIUsageBreakdown } from '../../api/admin'
import { useAdminLookups } from '../../hooks/useAdminLookups'
import { usePaginatedList } from '../../hooks/usePaginatedList'
import { useT } from '../../i18n'
import type { PageQuery } from '../../types/pagination'
import { formatDurationMs } from '../../utils/formatters'
import { useAuth } from '../auth/AuthProvider'
import { DataState } from '../ui/DataState'
import type { ColumnDef } from '../ui/DataTable'
import { DataTable } from '../ui/DataTable'
import { EmptyState } from '../ui/EmptyState'
import { KPICard } from '../ui/KPICard'
import { LoadingOverlay } from '../ui/LoadingOverlay'
import { Pagination } from '../ui/Pagination'
import { Select } from '../ui/Select'
import {
  adminFormLabelClass,
  adminLabelTextClass,
  adminTableContainerClass,
  adminTdClass,
  adminTdMonoClass,
} from './adminClasses'
import { AdminPanelShell } from './AdminPanelShell'
const DEFAULT_PAGE_SIZE = 25
const PAGE_SIZE_OPTIONS = [10, 25, 50, 100]

function formatTokenCount(n: number): string {
  return n > 0 ? n.toLocaleString() : '—'
}

function formatUSD(v: number): string {
  if (v <= 0) {
    return '—'
  }
  return `$${v.toFixed(4)}`
}

export function AIUsageAdminPanel() {
  const t = useT()
  const { accessToken } = useAuth()
  const { users, loading: lookupsLoading } = useAdminLookups(accessToken ?? '')
  const [days, setDays] = useState(30)
  const [totals, setTotals] = useState<AIUsageTotals | null>(null)

  const fetcher = useCallback(
    async (q: PageQuery) => {
      const data = await getAIUsageBreakdown(accessToken ?? '', {
        days,
        page: q.page,
        pageSize: q.pageSize,
      })
      setTotals(data.totals)
      return { items: data.rows, total: data.total }
    },
    [accessToken, days],
  )
  const {
    items: rows,
    loading,
    error,
    page: currentPage,
    setPage: setCurrentPage,
    pageSize,
    setPageSize,
    totalPages,
    total: totalItems,
  } = usePaginatedList<AIUsageBreakdownRow>({
    fetcher,
    initialPageSize: DEFAULT_PAGE_SIZE,
    enabled: Boolean(accessToken),
    fetchKey: accessToken,
    resetPageKey: days,
  })

  const periodOptions = useMemo(
    () => [
      { value: '7', label: t('admin.ai_usage.days_7') },
      { value: '30', label: t('admin.ai_usage.days_30') },
      { value: '90', label: t('admin.ai_usage.days_90') },
    ],
    [t],
  )
  const userLabelByID = useMemo(() => {
    const map = new Map<string, string>()
    for (const u of users) {
      const displayName = u.displayName?.trim() ?? ''
      const email = u.email.trim()
      const label = displayName.length > 0 ? displayName : email.length > 0 ? email : u.id
      map.set(u.id, label)
    }
    return map
  }, [users])

  const columns: ColumnDef<AIUsageBreakdownRow>[] = [
    {
      key: 'user',
      header: t('admin.ai_usage.col_user'),
      className: adminTdClass,
      cell: (row) => (row.user_id ? (userLabelByID.get(row.user_id) ?? row.user_id) : '—'),
    },
    {
      key: 'model',
      header: t('admin.ai_usage.col_model'),
      className: adminTdMonoClass,
      cell: (row) => row.model_used,
    },
    {
      key: 'queries',
      header: t('admin.ai_usage.col_queries'),
      align: 'right',
      className: adminTdMonoClass,
      cell: (row) => row.query_count.toLocaleString(),
    },
    {
      key: 'prompt_tokens',
      header: t('admin.ai_usage.col_prompt_tokens'),
      align: 'right',
      className: adminTdMonoClass,
      cell: (row) => formatTokenCount(row.prompt_tokens),
    },
    {
      key: 'completion_tokens',
      header: t('admin.ai_usage.col_completion_tokens'),
      align: 'right',
      className: adminTdMonoClass,
      cell: (row) => formatTokenCount(row.completion_tokens),
    },
    {
      key: 'total_tokens',
      header: t('admin.ai_usage.col_total_tokens'),
      align: 'right',
      className: adminTdMonoClass,
      cell: (row) => formatTokenCount(row.total_tokens),
    },
    {
      key: 'cost',
      header: t('admin.ai_usage.col_cost'),
      align: 'right',
      className: adminTdMonoClass,
      cell: (row) => formatUSD(row.total_cost_usd),
    },
    {
      key: 'latency',
      header: t('admin.ai_usage.col_latency'),
      align: 'right',
      className: adminTdMonoClass,
      cell: (row) => formatDurationMs(row.avg_latency_ms),
    },
  ]

  return (
    <AdminPanelShell
      title={t('admin.ai_usage.title')}
      description={t('admin.ai_usage.description')}
      error={error}
      action={
        <div style={{ display: 'flex', gap: 12, alignItems: 'flex-end', flexWrap: 'wrap' }}>
          {totalItems > 0 && (
            <span style={{ fontSize: 13, color: 'var(--text-secondary, #a1a1aa)' }}>
              {t('admin.ai_usage.row_count', { count: totalItems })}
            </span>
          )}
          <label className={adminFormLabelClass} style={{ gap: 4, minWidth: 160 }}>
            <span className={adminLabelTextClass}>{t('admin.ai_usage.period')}</span>
            <Select
              value={String(days)}
              options={periodOptions}
              onChange={(v) => {
                setCurrentPage(1)
                setDays(Number(v))
              }}
              size="sm"
            />
          </label>
        </div>
      }
    >
      <div style={{ position: 'relative', marginTop: '1.25rem', minHeight: 120 }}>
        <LoadingOverlay loading={loading || lookupsLoading} />
        {totals && (
          <div
            style={{
              display: 'grid',
              gridTemplateColumns: 'repeat(auto-fit, minmax(160px, 1fr))',
              gap: '0.75rem',
              marginBottom: '1.25rem',
            }}
          >
            <KPICard
              label={t('admin.ai_usage.kpi_queries')}
              value={totals.query_count.toLocaleString()}
            />
            <KPICard
              label={t('admin.ai_usage.kpi_tokens')}
              value={totals.total_tokens.toLocaleString()}
            />
            <KPICard
              label={t('admin.ai_usage.kpi_prompt')}
              value={totals.prompt_tokens.toLocaleString()}
            />
            <KPICard
              label={t('admin.ai_usage.kpi_completion')}
              value={totals.completion_tokens.toLocaleString()}
            />
            <KPICard
              label={t('admin.ai_usage.kpi_cost')}
              value={formatUSD(totals.total_cost_usd)}
            />
            <KPICard label={t('admin.ai_usage.kpi_users')} value={String(totals.unique_users)} />
            <KPICard label={t('admin.ai_usage.kpi_models')} value={String(totals.unique_models)} />
          </div>
        )}

        <div className={adminTableContainerClass}>
          <DataState
            loading={loading}
            error={null}
            empty={!loading && totalItems === 0}
            emptyState={<EmptyState description={t('admin.ai_usage.empty')} />}
            className="overflow-x-auto"
          >
            <DataTable
              columns={columns}
              rows={rows}
              rowKey={(row) => `${row.user_id}:${row.model_used}`}
              loading={loading}
              tableStyle={{ fontSize: 13, minWidth: 980 }}
            />
          </DataState>
          {totalItems > 0 && (
            <Pagination
              currentPage={currentPage}
              totalPages={totalPages}
              onPageChange={setCurrentPage}
              totalItems={totalItems}
              itemsPerPage={pageSize}
              alwaysShow
              pageSizeOptions={PAGE_SIZE_OPTIONS}
              onPageSizeChange={(size) => {
                setPageSize(size)
                setCurrentPage(1)
              }}
              pageSizeLabel={t('admin.audit.page_size')}
            />
          )}
        </div>
      </div>
    </AdminPanelShell>
  )
}
