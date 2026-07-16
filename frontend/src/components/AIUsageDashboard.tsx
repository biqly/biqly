import { useEffect, useMemo, useState } from 'react'
import {
  Bar,
  BarChart,
  CartesianGrid,
  LabelList,
  Legend,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'

import { useApi } from '../hooks/useApi'
import { localeLanguageTag, useLocale, useT } from '../i18n'
import { cardClass } from '../lib/cardClasses'
import { cn } from '../lib/cn'
import type { ModelStats } from '../types/ai'
import { chartAxisStroke, chartGridStroke } from '../utils/chartConfig'
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

// Series colors are the design-system theme tokens (auto-tuned for light/dark).
// The pair validates for CVD separation and contrast; direct value labels plus
// the full table provide the secondary encoding identity never rests on hue.
const SERIES_SUCCESS = 'var(--success)'
const SERIES_CONFIDENCE = 'var(--accent)'

/** Round a percent to one decimal with a locale-aware separator ("54.1%"). */
function formatPercent(value: number, localeTag: string, digits = 1): string {
  if (!Number.isFinite(value)) {
    return '—'
  }
  return `${new Intl.NumberFormat(localeTag, {
    minimumFractionDigits: 0,
    maximumFractionDigits: digits,
  }).format(value)}%`
}

function formatCount(value: number, localeTag: string): string {
  return new Intl.NumberFormat(localeTag).format(value)
}

interface ModelChartRow {
  name: string
  success_rate: number
  confidence: number
}

interface ChartTooltipEntry {
  name?: string
  value?: number
  color?: string
  dataKey?: string | number
}

function ModelChartTooltip({
  active,
  payload,
  label,
  localeTag,
}: {
  active?: boolean
  payload?: ChartTooltipEntry[]
  label?: string
  localeTag: string
}) {
  if (!active || !payload || payload.length === 0) {
    return null
  }
  return (
    <div className="border-border-strong bg-card shadow-card min-w-36 rounded-lg border p-2.5">
      <div className="text-foreground mb-1.5 text-[0.8rem] font-semibold break-all">{label}</div>
      <div className="flex flex-col gap-1">
        {payload.map((entry) => (
          <div key={String(entry.dataKey)} className="flex items-center justify-between gap-3">
            <span className="text-foreground-muted inline-flex items-center gap-1.5 text-[0.75rem]">
              <span
                aria-hidden
                className="inline-block h-2 w-2 shrink-0 rounded-xs"
                style={{ backgroundColor: entry.color }}
              />
              {entry.name}
            </span>
            <span className="text-foreground text-[0.78rem] font-semibold tabular-nums">
              {formatPercent(entry.value ?? 0, localeTag)}
            </span>
          </div>
        ))}
      </div>
    </div>
  )
}

const numCellClass = 'px-3 py-2.5 text-right align-middle tabular-nums whitespace-nowrap'
const headCellClass =
  'px-3 py-2.5 text-[0.68rem] font-bold uppercase tracking-[0.08em] text-foreground-muted border-b border-border-strong align-middle whitespace-nowrap'

function ModelSuccessRates({ models }: { models: ModelStats[] }) {
  const t = useT()
  const [locale] = useLocale()
  const localeTag = localeLanguageTag(locale)

  // Chart: sort by success rate (best first) so the comparison reads top-down.
  const chartData: ModelChartRow[] = useMemo(
    () =>
      [...models]
        .map((m) => ({
          name: m.model_name ?? m.model_id,
          success_rate: Math.round(m.success_rate * 10) / 10,
          confidence: Math.round(m.avg_confidence * 100 * 10) / 10,
        }))
        .sort((a, b) => b.success_rate - a.success_rate),
    [models],
  )

  if (models.length === 0) {
    return null
  }

  const chartHeight = Math.max(180, chartData.length * 56 + 56)

  return (
    <div>
      <h2 className="mb-4">{t('dashboard.model_rates_heading')}</h2>

      <div className={cardClass()}>
        <div
          className="max-w-full overflow-x-auto [-webkit-overflow-scrolling:touch]"
          role="region"
          aria-label={t('dashboard.model_rates_heading')}
        >
          <table className="w-full min-w-2xl border-collapse text-[0.9rem]">
            <thead>
              <tr>
                <th className={cn(headCellClass, 'text-left')}>{t('dashboard.col_model')}</th>
                <th className={headCellClass}>{t('dashboard.tbl_total')}</th>
                <th className={headCellClass}>{t('dashboard.tbl_success')}</th>
                <th className={headCellClass}>{t('dashboard.tbl_fail')}</th>
                <th className={headCellClass}>{t('dashboard.tbl_success_pct')}</th>
                <th className={headCellClass}>{t('dashboard.tbl_confidence')}</th>
                <th className={headCellClass}>{t('dashboard.tbl_latency')}</th>
                <th className={cn(headCellClass, 'text-right')} aria-label={t('dashboard.tbl_up')}>
                  👍
                </th>
                <th
                  className={cn(headCellClass, 'text-right')}
                  aria-label={t('dashboard.tbl_down')}
                >
                  👎
                </th>
              </tr>
            </thead>
            <tbody>
              {models.map((m) => {
                const rateColor = getRateColor(m.success_rate)
                return (
                  <tr
                    key={m.model_id}
                    className="border-border border-b transition-colors last:border-b-0 hover:bg-(--table-stripe-hover)"
                  >
                    <td className="text-foreground px-3 py-2.5 align-middle font-medium break-all">
                      {m.model_name ?? m.model_id}
                    </td>
                    <td className={cn(numCellClass, 'text-foreground-muted')}>
                      {formatCount(m.total_queries, localeTag)}
                    </td>
                    <td className={cn(numCellClass, 'text-success font-medium')}>
                      {formatCount(m.success_count, localeTag)}
                    </td>
                    <td className={cn(numCellClass, 'text-error font-medium')}>
                      {formatCount(m.failure_count, localeTag)}
                    </td>
                    <td className={numCellClass}>
                      <span
                        className="inline-flex items-center rounded-full px-2 py-0.5 text-[0.8rem] font-bold tabular-nums"
                        style={{
                          color: rateColor,
                          backgroundColor: `color-mix(in srgb, ${rateColor} 14%, transparent)`,
                        }}
                      >
                        {formatPercent(m.success_rate, localeTag)}
                      </span>
                    </td>
                    <td className={cn(numCellClass, 'text-foreground-muted')}>
                      {formatPercent(m.avg_confidence * 100, localeTag, 0)}
                    </td>
                    <td className={cn(numCellClass, 'text-foreground-muted')}>
                      {formatDurationMs(m.avg_latency_ms, localeTag)}
                    </td>
                    <td className={cn(numCellClass, 'text-success')}>
                      {formatCount(m.positive_count, localeTag)}
                    </td>
                    <td className={cn(numCellClass, 'text-error')}>
                      {formatCount(m.negative_count, localeTag)}
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      </div>

      <div className={cardClass()}>
        <h3>{t('dashboard.chart_success_compare')}</h3>
        <div className="mt-3 max-w-full overflow-x-auto [-webkit-overflow-scrolling:touch]">
          <div style={{ height: chartHeight, minWidth: 360 }}>
            <ResponsiveContainer width="100%" height="100%" minWidth={0}>
              <BarChart
                layout="vertical"
                data={chartData}
                margin={{ top: 4, right: 44, bottom: 4, left: 8 }}
                barCategoryGap="28%"
                barGap={2}
              >
                <CartesianGrid horizontal={false} strokeDasharray="3 3" stroke={chartGridStroke} />
                <XAxis
                  type="number"
                  domain={[0, 100]}
                  unit="%"
                  stroke={chartAxisStroke}
                  tick={{ fontSize: 11 }}
                  tickLine={false}
                />
                <YAxis
                  type="category"
                  dataKey="name"
                  width={140}
                  stroke={chartAxisStroke}
                  tick={{ fontSize: 11 }}
                  tickLine={false}
                />
                <Tooltip
                  cursor={{ fill: 'color-mix(in srgb, var(--accent) 8%, transparent)' }}
                  content={<ModelChartTooltip localeTag={localeTag} />}
                />
                <Legend wrapperStyle={{ fontSize: 12, paddingTop: 8 }} />
                <Bar
                  dataKey="success_rate"
                  fill={SERIES_SUCCESS}
                  radius={[0, 4, 4, 0]}
                  name={t('dashboard.legend_success_pct')}
                >
                  <LabelList
                    dataKey="success_rate"
                    position="right"
                    fill="var(--text-muted)"
                    fontSize={11}
                    formatter={(v) => formatPercent(Number(v), localeTag)}
                  />
                </Bar>
                <Bar
                  dataKey="confidence"
                  fill={SERIES_CONFIDENCE}
                  radius={[0, 4, 4, 0]}
                  name={t('dashboard.legend_confidence_pct')}
                >
                  <LabelList
                    dataKey="confidence"
                    position="right"
                    fill="var(--text-muted)"
                    fontSize={11}
                    formatter={(v) => formatPercent(Number(v), localeTag)}
                  />
                </Bar>
              </BarChart>
            </ResponsiveContainer>
          </div>
        </div>
      </div>
    </div>
  )
}
