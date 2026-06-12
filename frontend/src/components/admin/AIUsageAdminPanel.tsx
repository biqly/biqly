import { useEffect, useMemo, useState } from 'react'

import { useAdminLookups } from '../../hooks/useAdminLookups'
import { useApi } from '../../hooks/useApi'
import { useClientPagination } from '../../hooks/useClientPagination'
import { useT } from '../../i18n'
import { formatDurationMs } from '../../utils/formatters'
import { useAuth } from '../auth/AuthProvider'
import { ErrorAlert } from '../ui/ErrorAlert'
import { KPICard } from '../ui/KPICard'
import { LoadingOverlay } from '../ui/LoadingOverlay'
import { Pagination } from '../ui/Pagination'
import { Select } from '../ui/Select'
import { numberSelectOptions } from './adminSelectOptions'

const DEFAULT_PAGE_SIZE = 25
const PAGE_SIZE_OPTIONS = [10, 25, 50, 100]

interface AIUsageTotals {
  query_count: number
  prompt_tokens: number
  completion_tokens: number
  total_tokens: number
  total_cost_usd: number
  unique_users: number
  unique_models: number
}

interface AIUsageBreakdownRow {
  user_id: string
  model_used: string
  query_count: number
  prompt_tokens: number
  completion_tokens: number
  total_tokens: number
  total_cost_usd: number
  avg_latency_ms: number
}

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
  const { get } = useApi()
  const { accessToken } = useAuth()
  const { users, loading: lookupsLoading } = useAdminLookups(accessToken ?? '')
  const [days, setDays] = useState(30)
  const [totals, setTotals] = useState<AIUsageTotals | null>(null)
  const [rows, setRows] = useState<AIUsageBreakdownRow[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const {
    page: clampedCurrentPage,
    setPage: setCurrentPage,
    pageSize,
    setPageSize,
    totalPages,
    total: totalItems,
    pageRows,
  } = useClientPagination(rows, DEFAULT_PAGE_SIZE)

  const periodOptions = useMemo(
    () => [
      { value: '7', label: t('admin.ai_usage.days_7') },
      { value: '30', label: t('admin.ai_usage.days_30') },
      { value: '90', label: t('admin.ai_usage.days_90') },
    ],
    [t],
  )
  const pageSizeOptions = useMemo(() => numberSelectOptions(PAGE_SIZE_OPTIONS), [])

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

  useEffect(() => {
    let cancelled = false
    get<{ totals: AIUsageTotals; rows: AIUsageBreakdownRow[] }>(
      `/api/ai/usage/breakdown?days=${days}`,
    )
      .then((data) => {
        if (cancelled || !data) {
          return
        }
        setTotals(data.totals)
        setRows(data.rows)
        setError(null)
      })
      .catch((err) => {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : String(err))
        }
      })
      .finally(() => {
        if (!cancelled) {
          setLoading(false)
        }
      })
    return () => {
      cancelled = true
    }
  }, [days, get])

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
    <div className="admin-panel">
      <div
        className="admin-panel__header"
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
          <label className="admin-form-label" style={{ gap: 4, minWidth: 160 }}>
            <span className="admin-label-text">{t('admin.ai_usage.period')}</span>
            <Select
              value={String(days)}
              options={periodOptions}
              onChange={(v) => {
                setLoading(true)
                setCurrentPage(1)
                setDays(Number(v))
              }}
              size="sm"
            />
          </label>
          <label className="admin-form-label" style={{ gap: 4, minWidth: 120 }}>
            <span className="admin-label-text">{t('admin.audit.page_size')}</span>
            <Select
              value={String(pageSize)}
              options={pageSizeOptions}
              onChange={(v) => {
                setPageSize(Number(v))
                setCurrentPage(1)
              }}
              size="sm"
            />
          </label>
        </div>
      </div>

      <ErrorAlert error={error} className="error--top-gap" />

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

        <div className="ai-history__table-wrap admin-table-container">
          <table
            className="ai-history__table"
            style={{ borderCollapse: 'collapse', width: '100%' }}
          >
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
              {!loading && rows.length === 0 ? (
                <tr>
                  <td colSpan={6} style={{ ...tdStyle, color: 'var(--text-muted)' }}>
                    {t('admin.ai_usage.empty')}
                  </td>
                </tr>
              ) : (
                pageRows.map((row) => {
                  const userLabel = row.user_id
                    ? (userLabelByID.get(row.user_id) ?? row.user_id)
                    : '—'
                  const key = `${row.user_id}:${row.model_used}`
                  return (
                    <tr key={key}>
                      <td style={tdStyle}>{userLabel}</td>
                      <td
                        className="ai-history__mono"
                        style={{ ...tdStyle, fontFamily: 'var(--font-mono, monospace)' }}
                      >
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
          {rows.length > 0 && (
            <Pagination
              currentPage={clampedCurrentPage}
              totalPages={totalPages}
              onPageChange={setCurrentPage}
              totalItems={totalItems}
              itemsPerPage={pageSize}
              alwaysShow
            />
          )}
        </div>
      </div>
    </div>
  )
}
