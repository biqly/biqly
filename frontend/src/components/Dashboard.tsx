import { useEffect, useMemo, useState } from 'react'
import { Bar, BarChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'
import { useApi } from '../hooks/useApi'
import { useT } from '../i18n'
import { chartAxisStroke, chartGridStroke, chartTooltipStyle } from '../utils/chartConfig'
import { chartColor } from '../utils/constants'
import { getRateColor } from '../utils/formatters'
import { KPICard } from './ui/KPICard'
import { ChartContainer } from './ui/ChartContainer'
import type { ModelStats } from '../types/ai'

export default function Dashboard() {
  const t = useT()
  const [selectedRange, setSelectedRange] = useState('7d')

  const sampleData = useMemo(
    () => ({
      revenue: [
        { name: t('dashboard.weekday_mon'), value: 4200 },
        { name: t('dashboard.weekday_tue'), value: 3800 },
        { name: t('dashboard.weekday_wed'), value: 5100 },
        { name: t('dashboard.weekday_thu'), value: 4600 },
        { name: t('dashboard.weekday_fri'), value: 6200 },
        { name: t('dashboard.weekday_sat'), value: 3100 },
        { name: t('dashboard.weekday_sun'), value: 2800 },
      ],
      countries: [
        { name: 'USA', value: 45 },
        { name: 'UK', value: 25 },
        { name: 'Germany', value: 15 },
        { name: 'France', value: 10 },
        { name: 'Japan', value: 5 },
      ],
      orders: [
        { name: t('dashboard.order_done'), value: 340 },
        { name: t('dashboard.order_pending'), value: 120 },
        { name: t('dashboard.order_cancelled'), value: 45 },
      ],
    }),
    [t],
  )

  const kpiCards = useMemo(
    () =>
      [
        { label: `${t('dashboard.kpi_total_revenue')} ${t('dashboard.demo_kpi_revenue_trend')}`, value: '$29,800' },
        { label: `${t('dashboard.kpi_orders')} ${t('dashboard.demo_kpi_orders_trend')}`, value: '505' },
        { label: `${t('dashboard.kpi_aov')} ${t('dashboard.demo_kpi_aov_trend')}`, value: '$59,01' },
        { label: `${t('dashboard.kpi_active_users')} ${t('dashboard.demo_kpi_users_trend')}`, value: '1.247' },
      ] as const,
    [t],
  )

  const recentQueryRows = useMemo(
    () =>
      [
        { time: t('dashboard.demo_time_ago_2m'), model: 'orders', rows: 12, ms: 45 },
        { time: t('dashboard.demo_time_ago_5m'), model: 'users', rows: 8, ms: 23 },
        { time: t('dashboard.demo_time_ago_12m'), model: 'products', rows: 25, ms: 67 },
        { time: t('dashboard.demo_time_ago_18m'), model: 'orders', rows: 3, ms: 12 },
      ] as const,
    [t],
  )

  const rangeButtons = useMemo(
    () =>
      [
        { key: '24h', label: t('dashboard.range_24h') },
        { key: '7d', label: t('dashboard.range_7d') },
        { key: '30d', label: t('dashboard.range_30d') },
        { key: '90d', label: t('dashboard.range_90d') },
      ] as const,
    [t],
  )

  return (
    <div>
      <div className="card">
        <h2>{t('dashboard.range_title')}</h2>
        <div className="range-toggle" role="group" aria-label={t('dashboard.range_aria')}>
          {rangeButtons.map(({ key, label }) => (
            <button
              key={key}
              type="button"
              className={selectedRange === key ? 'btn' : 'btn btn--neutral'}
              onClick={() => setSelectedRange(key)}
              aria-pressed={selectedRange === key}
            >
              {label}
            </button>
          ))}
        </div>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(250px, 1fr))', gap: '1.5rem', marginBottom: '1.5rem' }}>
        {kpiCards.map((card, i) => (
          <KPICard key={i} label={card.label} value={card.value} color="var(--success)" />
        ))}
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(400px, 1fr))', gap: '1.5rem' }}>
        <div className="card">
          <h3>{t('dashboard.chart_revenue')}</h3>
          <ChartContainer data={sampleData.revenue} type="bar" barRadius={[4, 4, 0, 0]} />
        </div>

        <div className="card">
          <h3>{t('dashboard.chart_countries')}</h3>
          <ChartContainer data={sampleData.countries} type="pie" />
        </div>

        <div className="card">
          <h3>{t('dashboard.chart_order_status')}</h3>
          <ChartContainer data={sampleData.orders} type="pie" colorFn={(i) => chartColor(i + 3)} />
        </div>

        <div className="card">
          <h3>{t('dashboard.recent_queries')}</h3>
          <table className="results-table">
            <thead>
              <tr>
                <th>{t('dashboard.col_time')}</th>
                <th>{t('dashboard.col_model')}</th>
                <th>{t('dashboard.col_rows')}</th>
                <th>{t('dashboard.col_duration')}</th>
              </tr>
            </thead>
            <tbody>
              {recentQueryRows.map((q, i) => (
                <tr key={i}>
                  <td>{q.time}</td>
                  <td>{q.model}</td>
                  <td>{q.rows}</td>
                  <td>{t('evaluation.latency_ms', { ms: q.ms })}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      <AIUsageSection />
      <ModelSuccessRates />
    </div>
  )
}

interface AIUsageSummary {
  total_queries: number
  success_rate: number
  failure_rate: number
  avg_retry_count: number
  avg_latency_ms: number
  total_cost: number
  positive_feedback?: number
  negative_feedback?: number
}

interface DayUsage {
  date: string
  total_queries: number
  failure_rate: number
  avg_retry_count: number
  avg_latency_ms: number
  total_cost: number
  total_tokens: number
}

function AIUsageSection() {
  const t = useT()
  const { get } = useApi()
  const [summary, setSummary] = useState<AIUsageSummary | null>(null)
  const [daily, setDaily] = useState<DayUsage[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    get<{ summary: AIUsageSummary; daily: DayUsage[] }>('/api/ai/usage').then((data) => {
      if (data) {
        setSummary(data.summary)
        setDaily(data.daily.slice(0, 10).reverse())
      }
      setLoading(false)
    })
  }, [])

  if (loading) return null
  if (!summary) return null

  const trendData = daily.map((d) => ({
    name: d.date.slice(5),
    queries: d.total_queries,
    cost: parseFloat(d.total_cost.toFixed(3)),
  }))

  return (
    <div style={{ marginTop: '2rem' }}>
      <h2 style={{ marginBottom: '1rem' }}>{t('dashboard.ai_usage_last_30')}</h2>

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))', gap: '1rem', marginBottom: '1.5rem' }}>
        <KPICard label={t('dashboard.kpi_total_ai_queries')} value={summary.total_queries} color="var(--accent)" />
        <KPICard label={t('dashboard.kpi_success_rate')} value={`${(summary.success_rate * 100).toFixed(0)}%`} color={getRateColor(summary.success_rate * 100)} />
        <KPICard label={t('dashboard.kpi_failure_rate')} value={`${(summary.failure_rate * 100).toFixed(0)}%`} color={getRateColor(100 - summary.failure_rate * 100)} />
        <KPICard label={t('dashboard.kpi_avg_retry')} value={summary.avg_retry_count.toFixed(2)} color="var(--text-muted)" />
        <KPICard label={t('dashboard.kpi_avg_latency')} value={t('evaluation.latency_ms', { ms: Math.round(summary.avg_latency_ms) })} color="var(--warning)" />
        <KPICard label={t('dashboard.kpi_total_cost')} value={`$${summary.total_cost.toFixed(4)}`} color="var(--success)" />
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(400px, 1fr))', gap: '1.5rem' }}>
        <div className="card">
          <h3>{t('dashboard.daily_queries')}</h3>
          {trendData.length > 0 ? (
            <ChartContainer data={trendData} type="line" height={250} dataKey="queries" />
          ) : (
            <p style={{ color: 'var(--text-muted)', textAlign: 'center', paddingTop: '4rem' }}>{t('dashboard.no_ai_queries')}</p>
          )}
        </div>

        <div className="card">
          <h3>{t('dashboard.daily_cost')}</h3>
          {trendData.length > 0 ? (
            <ChartContainer data={trendData} type="bar" height={250} dataKey="cost" fill="#f59e0b" barRadius={[4, 4, 0, 0]} />
          ) : (
            <p style={{ color: 'var(--text-muted)', textAlign: 'center', paddingTop: '4rem' }}>{t('dashboard.no_cost_data')}</p>
          )}
        </div>
      </div>
    </div>
  )
}

function ModelSuccessRates() {
  const t = useT()
  const { get } = useApi()
  const [models, setModels] = useState<ModelStats[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    get<ModelStats[]>('/api/ai/stats/models').then((data) => {
      if (data) setModels(data)
      setLoading(false)
    })
  }, [])

  if (loading) return null
  if (models.length === 0) return null

  return (
    <div style={{ marginTop: '2rem' }}>
      <h2 style={{ marginBottom: '1rem' }}>{t('dashboard.model_rates_heading')}</h2>
      <table className="results-table">
        <thead>
          <tr>
            <th>{t('dashboard.col_model')}</th>
            <th style={{ textAlign: 'right' }}>{t('dashboard.tbl_total')}</th>
            <th style={{ textAlign: 'right' }}>{t('dashboard.tbl_success')}</th>
            <th style={{ textAlign: 'right' }}>{t('dashboard.tbl_fail')}</th>
            <th style={{ textAlign: 'right' }}>{t('dashboard.tbl_success_pct')}</th>
            <th style={{ textAlign: 'right' }}>{t('dashboard.tbl_confidence')}</th>
            <th style={{ textAlign: 'right' }}>{t('dashboard.tbl_latency')}</th>
            <th style={{ textAlign: 'right' }}>👍</th>
            <th style={{ textAlign: 'right' }}>👎</th>
          </tr>
        </thead>
        <tbody>
          {models.map((m) => (
            <tr key={m.model_id}>
              <td>{m.model_name || m.model_id}</td>
              <td style={{ textAlign: 'right' }}>{m.total_queries}</td>
              <td style={{ textAlign: 'right', color: 'var(--success)' }}>{m.success_count}</td>
              <td style={{ textAlign: 'right', color: 'var(--error)' }}>{m.failure_count}</td>
              <td style={{ textAlign: 'right' }}>
                <span style={{
                  color: getRateColor(m.success_rate),
                  fontWeight: 700,
                }}>
                  {m.success_rate.toFixed(1)}%
                </span>
              </td>
              <td style={{ textAlign: 'right' }}>{(m.avg_confidence * 100).toFixed(0)}%</td>
              <td style={{ textAlign: 'right' }}>{t('evaluation.latency_ms', { ms: Math.round(m.avg_latency_ms) })}</td>
              <td style={{ textAlign: 'right', color: 'var(--success)' }}>{m.positive_count}</td>
              <td style={{ textAlign: 'right', color: 'var(--error)' }}>{m.negative_count}</td>
            </tr>
          ))}
        </tbody>
      </table>

      <div className="card" style={{ marginTop: '1rem' }}>
        <h3>{t('dashboard.chart_success_compare')}</h3>
        <div style={{ height: 250 }}>
          <ResponsiveContainer width="100%" height="100%">
            <BarChart data={models.map((m) => ({
              name: m.model_name || m.model_id,
              success_rate: m.success_rate,
              confidence: m.avg_confidence * 100,
            }))}>
              <CartesianGrid strokeDasharray="3 3" stroke={chartGridStroke} />
              <XAxis dataKey="name" stroke={chartAxisStroke} tick={{ fontSize: 11 }} />
              <YAxis stroke={chartAxisStroke} domain={[0, 100]} />
              <Tooltip contentStyle={chartTooltipStyle} />
              <Bar dataKey="success_rate" fill="#22c55e" radius={[4, 4, 0, 0]} name={t('dashboard.legend_success_pct')} />
              <Bar dataKey="confidence" fill="#3b82f6" radius={[4, 4, 0, 0]} name={t('dashboard.legend_confidence_pct')} />
            </BarChart>
          </ResponsiveContainer>
        </div>
      </div>
    </div>
  )
}
