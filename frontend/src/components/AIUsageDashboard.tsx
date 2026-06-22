import { useEffect, useState } from 'react'
import { Bar, BarChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'

import { useApi } from '../hooks/useApi'
import { useT } from '../i18n'
import { cardClass } from '../lib/cardClasses'
import { legacyTableClass } from '../lib/tableClasses'
import type { ModelStats } from '../types/ai'
import { chartAxisStroke, chartGridStroke, chartTooltipStyle } from '../utils/chartConfig'
import { formatDurationMs, getRateColor } from '../utils/formatters'
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
    <div className="flex flex-col gap-8">
      {summary && <AIUsageSection summary={summary} daily={daily} />}
      {models.length > 0 && <ModelSuccessRates models={models} />}
    </div>
  )
}

function AIUsageSkeleton({ heading }: { heading: string }) {
  return (
    <div>
      <h2 className="mb-4">{heading}</h2>
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))',
          gap: '1rem',
          marginBottom: '1.5rem',
        }}
      >
        {Array.from({ length: 6 }, (_, i) => (
          <div key={i} className={`${cardClass()} grid gap-[0.6rem]`}>
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
          <div key={i} className={`${cardClass()} grid gap-[0.8rem]`}>
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
      <h2 className="mb-4">{t('dashboard.ai_usage_last_30')}</h2>

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
          value={formatDurationMs(summary.avg_latency_ms)}
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
        <div className={cardClass()}>
          <h3>{t('dashboard.daily_queries')}</h3>
          {trendData.length > 0 ? (
            <ChartContainer data={trendData} type="line" height={250} dataKey="queries" />
          ) : (
            <p className="text-foreground-faint pt-16 text-center">
              {t('dashboard.no_ai_queries')}
            </p>
          )}
        </div>

        <div className={cardClass()}>
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
            <p className="text-foreground-faint pt-16 text-center">{t('dashboard.no_cost_data')}</p>
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
      <h2 className="mb-4">{t('dashboard.model_rates_heading')}</h2>
      <div className={legacyTableClass('results-table-scroll')}>
        <table className={legacyTableClass('results-table')}>
          <thead>
            <tr>
              <th>{t('dashboard.col_model')}</th>
              <th className="text-right">{t('dashboard.tbl_total')}</th>
              <th className="text-right">{t('dashboard.tbl_success')}</th>
              <th className="text-right">{t('dashboard.tbl_fail')}</th>
              <th className="text-right">{t('dashboard.tbl_success_pct')}</th>
              <th className="text-right">{t('dashboard.tbl_confidence')}</th>
              <th className="text-right">{t('dashboard.tbl_latency')}</th>
              <th className="text-right">👍</th>
              <th className="text-right">👎</th>
            </tr>
          </thead>
          <tbody>
            {models.map((m) => (
              <tr key={m.model_id}>
                <td>{m.model_name ?? m.model_id}</td>
                <td className="text-right">{m.total_queries}</td>
                <td className="text-success text-right">{m.success_count}</td>
                <td className="text-error text-right">{m.failure_count}</td>
                <td className="text-right">
                  <span style={{ color: getRateColor(m.success_rate) }} className="font-bold">
                    {m.success_rate.toFixed(1)}%
                  </span>
                </td>
                <td className="text-right">{(m.avg_confidence * 100).toFixed(0)}%</td>
                <td className="text-right">{formatDurationMs(m.avg_latency_ms)}</td>
                <td className="text-success text-right">{m.positive_count}</td>
                <td className="text-error text-right">{m.negative_count}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className={`${cardClass()} mt-4`}>
        <h3>{t('dashboard.chart_success_compare')}</h3>
        <div className="h-62.5">
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
