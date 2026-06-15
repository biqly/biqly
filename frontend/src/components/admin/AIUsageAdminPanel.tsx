import { useCallback, useMemo, useState } from 'react'

import { type AIUsageBreakdownRow, type AIUsageTotals, getAIUsageBreakdown } from '../../api/admin'
import { useAdminLookups } from '../../hooks/useAdminLookups'
import { usePaginatedList } from '../../hooks/usePaginatedList'
import { useT } from '../../i18n'
import { legacyFeedbackClass } from '../../lib/feedbackClasses'
import type { PageQuery } from '../../types/pagination'
import { formatDurationMs } from '../../utils/formatters'
import {
  aiHistoryMonoClass,
  aiHistoryTableClass,
  aiHistoryTableWrapClass,
} from '../ai/aiJobsClasses'
import { useAuth } from '../auth/AuthProvider'
import { ErrorAlert } from '../ui/ErrorAlert'
import { KPICard } from '../ui/KPICard'
import { LoadingOverlay } from '../ui/LoadingOverlay'
import { Pagination } from '../ui/Pagination'
import { Select } from '../ui/Select'
import {
  adminFormLabelClass,
  adminLabelTextClass,
  adminPanelClass,
  adminPanelHeaderClass,
  adminTableContainerClass,
} from './adminClasses'
const DEFAULT_PAGE_SIZE = 25
const PAGE_SIZE_OPTIONS = [10, 25, 50, 100]

function formatTokens(prompt: number, completion: number, total: number): string {
  if (prompt > 0 || completion > 0) {
    return `${prompt.toLocaleString()} + ${completion.toLocaleString()} = ${total.toLocaleString()}`
  }
  if (total > 0) {
    return total.toLocaleString()
  }
  return '—'
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

  const thStyle: React.CSSProperties = {
    textAlign: 'left',
    padding: '0.6rem 0.75rem',
    fontSize: '0.75rem',
    fontWeight: 600,
    color: 'var(--text-muted)',
    borderBottom: '1px solid var(--border)',
  }
  const tdStyle: React.CSSProperties = {
    padding: '0.65rem 0.75rem',
    fontSize: '0.875rem',
    borderBottom: '1px solid var(--border-subtle, var(--border))',
    verticalAlign: 'top',
  }

  return (
    <div className={adminPanelClass}>
      <div
        className={adminPanelHeaderClass}
        style={{
          display: 'flex',
          flexWrap: 'wrap',
          gap: '1rem',
          alignItems: 'center',
          justifyContent: 'space-between',
        }}
      >
        <div>
          <h2 style={{ margin: 0 }}>{t('admin.ai_usage.title')}</h2>
          <p style={{ margin: '0.35rem 0 0', color: 'var(--text-muted)', fontSize: '0.875rem' }}>
            {t('admin.ai_usage.description')}
          </p>
        </div>
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
      </div>

      <ErrorAlert error={error} className={legacyFeedbackClass('error--top-gap')} />

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

        <div className={`${aiHistoryTableWrapClass} ${adminTableContainerClass}`}>
          <table className={aiHistoryTableClass}>
            <thead>
              <tr>
                <th style={thStyle}>{t('admin.ai_usage.col_user')}</th>
                <th style={thStyle}>{t('admin.ai_usage.col_model')}</th>
                <th style={thStyle}>{t('admin.ai_usage.col_queries')}</th>
                <th style={thStyle}>{t('admin.ai_usage.col_tokens')}</th>
                <th style={thStyle}>{t('admin.ai_usage.col_cost')}</th>
                <th style={thStyle}>{t('admin.ai_usage.col_latency')}</th>
              </tr>
            </thead>
            <tbody>
              {!loading && totalItems === 0 ? (
                <tr>
                  <td colSpan={6} style={{ ...tdStyle, color: 'var(--text-muted)' }}>
                    {t('admin.ai_usage.empty')}
                  </td>
                </tr>
              ) : (
                rows.map((row) => {
                  const userLabel = row.user_id
                    ? (userLabelByID.get(row.user_id) ?? row.user_id)
                    : '—'
                  const key = `${row.user_id}:${row.model_used}`
                  return (
                    <tr key={key}>
                      <td style={tdStyle}>{userLabel}</td>
                      <td className={aiHistoryMonoClass} style={tdStyle}>
                        {row.model_used}
                      </td>
                      <td style={tdStyle}>{row.query_count.toLocaleString()}</td>
                      <td style={tdStyle}>
                        {formatTokens(row.prompt_tokens, row.completion_tokens, row.total_tokens)}
                      </td>
                      <td style={tdStyle}>{formatUSD(row.total_cost_usd)}</td>
                      <td style={tdStyle}>{formatDurationMs(row.avg_latency_ms)}</td>
                    </tr>
                  )
                })
              )}
            </tbody>
          </table>
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
    </div>
  )
}
