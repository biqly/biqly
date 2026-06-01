import { useEffect, useMemo, useState } from 'react'
import { useApi } from '../../hooks/useApi'
import { useAuth } from '../auth/AuthProvider'
import { useAdminLookups } from '../../hooks/useAdminLookups'
import { useT } from '../../i18n'
import { LoadingOverlay } from '../ui/LoadingOverlay'
import { KPICard } from '../ui/KPICard'

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
  if (total > 0) return total.toLocaleString()
  return '—'
}

function formatUSD(v: number): string {
  if (v <= 0) return '—'
  return `$${v.toFixed(4)}`
}

export function AIUsageAdminPanel() {
  const t = useT()
  const { get } = useApi()
  const { accessToken } = useAuth()
  const { users, loading: lookupsLoading } = useAdminLookups(accessToken || '')
  const [days, setDays] = useState(30)
  const [totals, setTotals] = useState<AIUsageTotals | null>(null)
  const [rows, setRows] = useState<AIUsageBreakdownRow[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const userLabelByID = useMemo(() => {
    const map = new Map<string, string>()
    for (const u of users) {
      const label = u.displayName?.trim() || u.email?.trim() || u.id
      map.set(u.id, label)
    }
    return map
  }, [users])

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    get<{ totals: AIUsageTotals; rows: AIUsageBreakdownRow[] }>(`/api/ai/usage/breakdown?days=${days}`)
      .then((data) => {
        if (cancelled || !data) return
        setTotals(data.totals)
        setRows(data.rows || [])
        setError(null)
      })
      .catch((err) => {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : String(err))
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
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
      <div className="admin-panel__header" style={{ display: 'flex', flexWrap: 'wrap', gap: '1rem', alignItems: 'center', justifyContent: 'space-between' }}>
        <div>
          <h2 style={{ margin: 0 }}>{t('admin.ai_usage.title')}</h2>
          <p style={{ margin: '0.35rem 0 0', color: 'var(--text-muted)', fontSize: '0.875rem' }}>
            {t('admin.ai_usage.description')}
          </p>
        </div>
        <label style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', fontSize: '0.875rem' }}>
          <span>{t('admin.ai_usage.period')}</span>
          <select
            className="input"
            value={days}
            onChange={(e) => setDays(Number(e.target.value))}
            style={{ minWidth: 120 }}
          >
            <option value={7}>{t('admin.ai_usage.days_7')}</option>
            <option value={30}>{t('admin.ai_usage.days_30')}</option>
            <option value={90}>{t('admin.ai_usage.days_90')}</option>
          </select>
        </label>
      </div>

      {error && (
        <p className="error-text" style={{ marginTop: '1rem' }}>{error}</p>
      )}

      <div style={{ position: 'relative', marginTop: '1.25rem', minHeight: 120 }}>
        <LoadingOverlay loading={loading || lookupsLoading} />
        {totals && (
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(160px, 1fr))', gap: '0.75rem', marginBottom: '1.25rem' }}>
            <KPICard label={t('admin.ai_usage.kpi_queries')} value={totals.query_count.toLocaleString()} />
            <KPICard label={t('admin.ai_usage.kpi_tokens')} value={totals.total_tokens.toLocaleString()} />
            <KPICard label={t('admin.ai_usage.kpi_prompt')} value={totals.prompt_tokens.toLocaleString()} />
            <KPICard label={t('admin.ai_usage.kpi_completion')} value={totals.completion_tokens.toLocaleString()} />
            <KPICard label={t('admin.ai_usage.kpi_cost')} value={formatUSD(totals.total_cost_usd)} />
            <KPICard label={t('admin.ai_usage.kpi_users')} value={String(totals.unique_users)} />
            <KPICard label={t('admin.ai_usage.kpi_models')} value={String(totals.unique_models)} />
          </div>
        )}

        <div className="ai-history__table-wrap">
          <table className="ai-history__table" style={{ borderCollapse: 'collapse', width: '100%' }}>
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
                rows.map((row) => {
                  const userLabel = row.user_id
                    ? userLabelByID.get(row.user_id) || row.user_id
                    : t('admin.ai_usage.anonymous')
                  const key = `${row.user_id}:${row.model_used}`
                  return (
                    <tr key={key}>
                      <td style={tdStyle}>{userLabel}</td>
                      <td className="ai-history__mono" style={{ ...tdStyle, fontFamily: 'var(--font-mono, monospace)' }}>
                        {row.model_used}
                      </td>
                      <td style={tdStyle}>{row.query_count.toLocaleString()}</td>
                      <td style={tdStyle}>
                        {formatTokens(row.prompt_tokens, row.completion_tokens, row.total_tokens)}
                      </td>
                      <td style={tdStyle}>{formatUSD(row.total_cost_usd)}</td>
                      <td style={tdStyle}>
                        {row.avg_latency_ms > 0 ? `${Math.round(row.avg_latency_ms)}ms` : '—'}
                      </td>
                    </tr>
                  )
                })
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  )
}
