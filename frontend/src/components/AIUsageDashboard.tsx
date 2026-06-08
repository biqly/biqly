import { useEffect, useState } from 'react'
import { Bar } from 'recharts/es6/cartesian/Bar'
import { CartesianGrid } from 'recharts/es6/cartesian/CartesianGrid'
import { XAxis } from 'recharts/es6/cartesian/XAxis'
import { YAxis } from 'recharts/es6/cartesian/YAxis'
import { BarChart } from 'recharts/es6/chart/BarChart'
import { ResponsiveContainer } from 'recharts/es6/component/ResponsiveContainer'
import { Tooltip } from 'recharts/es6/component/Tooltip'

import { useApi } from '../hooks/useApi'
import { useT } from '../i18n'
import type { ModelStats } from '../types/ai'
import { chartAxisStroke, chartGridStroke, chartTooltipStyle } from '../utils/chartConfig'
import { getRateColor } from '../utils/formatters'
import { ChartContainer } from './ui/ChartContainer'
import { KPICard } from './ui/KPICard'
import { Skeleton } from './ui/Skeleton'

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

export default function AIUsageDashboard() {
  const t = useT()
  const { get } = useApi()
  const [summary, setSummary] = useState<AIUsageSummary | null>(null)
  const [daily, setDaily] = useState<DayUsage[]>([])
  const [models, setModels] = useState<ModelStats[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    void Promise.all([
      get<{ summary: AIUsageSummary; daily: DayUsage[] }>('/api/ai/usage').then((data) => {
        if (data) {
          setSummary(data.summary)
          setDaily(data.daily.slice(0, 10).reverse())
        }
      }),
      get<ModelStats[]>('/api/ai/stats/models').then((data) => {
        if (data) {
          setModels(data)
        }
      }),
    ]).finally(() => {
      setLoading(false)
    })
  }, [get])

  if (loading) {
    return <AIUsageSkeleton heading={t('dashboard.ai_usage_last_30')} />
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '2rem' }}>
      {summary && <AIUsageSection summary={summary} daily={daily} />}
      {models.length > 0 && <ModelSuccessRates models={models} />}
    </div>
  )
}

function AIUsageSkeleton({ heading }: { heading: string }) {
  return (
    <div>
      <h2 style={{ marginBottom: '1rem' }}>{heading}</h2>
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))',
          gap: '1rem',
          marginBottom: '1.5rem',
        }}
      >
        {Array.from({ length: 6 }, (_, i) => (
          <div key={i} className="card" style={{ display: 'grid', gap: '0.6rem' }}>
            <Skeleton height="0.7rem" width="55%" />
            <Skeleton height="1.6rem" width="70%" />
          </div>
        ))}
      </div>
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fit, minmax(400px, 1fr))',
          gap: '1.5rem',
        }}
      >
        {Array.from({ length: 2 }, (_, i) => (
          <div key={i} className="card" style={{ display: 'grid', gap: '0.8rem' }}>
            <Skeleton height="1rem" width="40%" />
            <Skeleton height={250} radius="0.5rem" />
          </div>
        ))}
      </div>
    </div>
  )
}

function AIUsageSection({ summary, daily }: { summary: AIUsageSummary; daily: DayUsage[] }) {
  const t = useT()

  const trendData = daily.map((d) => ({
    name: d.date.slice(5),
    queries: d.total_queries,
    cost: parseFloat(d.total_cost.toFixed(3)),
  }))

  return (
    <div>
      <h2 style={{ marginBottom: '1rem' }}>{t('dashboard.ai_usage_last_30')}</h2>

      <div
        style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))',
          gap: '1rem',
          marginBottom: '1.5rem',
        }}
      >
        <KPICard
          label={t('dashboard.kpi_total_ai_queries')}
          value={summary.total_queries}
          color="var(--accent)"
        />
        <KPICard
          label={t('dashboard.kpi_success_rate')}
          value={`${(summary.success_rate * 100).toFixed(0)}%`}
          color={getRateColor(summary.success_rate * 100)}
        />
        <KPICard
          label={t('dashboard.kpi_failure_rate')}
          value={`${(summary.failure_rate * 100).toFixed(0)}%`}
          color={getRateColor(100 - summary.failure_rate * 100)}
        />
        <KPICard
          label={t('dashboard.kpi_avg_retry')}
          value={summary.avg_retry_count.toFixed(2)}
          color="var(--text-muted)"
        />
        <KPICard
          label={t('dashboard.kpi_avg_latency')}
          value={t('evaluation.latency_ms', { ms: Math.round(summary.avg_latency_ms) })}
          color="var(--warning)"
        />
        <KPICard
          label={t('dashboard.kpi_total_cost')}
          value={`$${summary.total_cost.toFixed(4)}`}
          color="var(--success)"
        />
      </div>

      <div
        style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fit, minmax(400px, 1fr))',
          gap: '1.5rem',
        }}
      >
        <div className="card">
          <h3>{t('dashboard.daily_queries')}</h3>
          {trendData.length > 0 ? (
            <ChartContainer data={trendData} type="line" height={250} dataKey="queries" />
          ) : (
            <p style={{ color: 'var(--text-muted)', textAlign: 'center', paddingTop: '4rem' }}>
              {t('dashboard.no_ai_queries')}
            </p>
          )}
        </div>

        <div className="card">
          <h3>{t('dashboard.daily_cost')}</h3>
          {trendData.length > 0 ? (
            <ChartContainer
              data={trendData}
              type="bar"
              height={250}
              dataKey="cost"
              fill="#f59e0b"
              barRadius={[4, 4, 0, 0]}
            />
          ) : (
            <p style={{ color: 'var(--text-muted)', textAlign: 'center', paddingTop: '4rem' }}>
              {t('dashboard.no_cost_data')}
            </p>
          )}
        </div>
      </div>
    </div>
  )
}

function ModelSuccessRates({ models }: { models: ModelStats[] }) {
  const t = useT()
  if (models.length === 0) {
    return null
  }

  return (
    <div>
      <h2 style={{ marginBottom: '1rem' }}>{t('dashboard.model_rates_heading')}</h2>
      <div className="results-table-scroll">
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
                <td>{m.model_name ?? m.model_id}</td>
                <td style={{ textAlign: 'right' }}>{m.total_queries}</td>
                <td style={{ textAlign: 'right', color: 'var(--success)' }}>{m.success_count}</td>
                <td style={{ textAlign: 'right', color: 'var(--error)' }}>{m.failure_count}</td>
                <td style={{ textAlign: 'right' }}>
                  <span
                    style={{
                      color: getRateColor(m.success_rate),
                      fontWeight: 700,
                    }}
                  >
                    {m.success_rate.toFixed(1)}%
                  </span>
                </td>
                <td style={{ textAlign: 'right' }}>{(m.avg_confidence * 100).toFixed(0)}%</td>
                <td style={{ textAlign: 'right' }}>
                  {t('evaluation.latency_ms', { ms: Math.round(m.avg_latency_ms) })}
                </td>
                <td style={{ textAlign: 'right', color: 'var(--success)' }}>{m.positive_count}</td>
                <td style={{ textAlign: 'right', color: 'var(--error)' }}>{m.negative_count}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="card" style={{ marginTop: '1rem' }}>
        <h3>{t('dashboard.chart_success_compare')}</h3>
        <div style={{ height: 250 }}>
          <ResponsiveContainer width="100%" height="100%">
            <BarChart
              data={models.map((m) => ({
                name: m.model_name ?? m.model_id,
                success_rate: m.success_rate,
                confidence: m.avg_confidence * 100,
              }))}
            >
              <CartesianGrid strokeDasharray="3 3" stroke={chartGridStroke} />
              <XAxis dataKey="name" stroke={chartAxisStroke} tick={{ fontSize: 11 }} />
              <YAxis stroke={chartAxisStroke} domain={[0, 100]} />
              <Tooltip contentStyle={chartTooltipStyle} />
              <Bar
                dataKey="success_rate"
                fill="#22c55e"
                radius={[4, 4, 0, 0]}
                name={t('dashboard.legend_success_pct')}
              />
              <Bar
                dataKey="confidence"
                fill="#3b82f6"
                radius={[4, 4, 0, 0]}
                name={t('dashboard.legend_confidence_pct')}
              />
            </BarChart>
          </ResponsiveContainer>
        </div>
      </div>
    </div>
  )
}
