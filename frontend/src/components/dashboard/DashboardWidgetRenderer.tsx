import { useEffect, useState } from 'react'
import {
  Bar,
  BarChart,
  CartesianGrid,
  Line,
  LineChart,
  Pie,
  PieChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'

import { useApi } from '../../hooks/useApi'
import { legacyTableClass } from '../../lib/tableClasses'
import type { LogicalQuery, QueryResultPayload } from '../../types/ai'
import { chartAxisStroke, chartGridStroke, chartTooltipStyle } from '../../utils/chartConfig'
import { chartColor } from '../../utils/constants'
import { unknownToDisplayString } from '../../utils/formatters'
import { DashboardLegend } from './DashboardLegend'
export interface DashboardWidget {
  id: string
  type: 'chart' | 'table' | 'kpi' | 'text'
  title: string
  w: number
  h: 'small' | 'medium' | 'large'
  saved_query_id?: string
  logical_query?: LogicalQuery
  chart_type?: 'line' | 'bar' | 'pie'
  config?: {
    xAxisColumn?: string
    yAxisColumns?: string[]
    valueColumn?: string
    visibleColumns?: string[]
  }
  content?: string
}

type ChartRow = Record<string, unknown>

const centerMessageStyle = {
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  height: '100%',
  minHeight: '120px',
  color: 'var(--text-muted)',
  fontSize: '0.85rem',
} as const

function WidgetCenterMessage({ children }: { children: React.ReactNode }) {
  return <div style={centerMessageStyle}>{children}</div>
}

function formatKpiValue(val: unknown): string | number {
  if (typeof val === 'number') {
    return val % 1 !== 0
      ? val.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })
      : val.toLocaleString()
  }
  return unknownToDisplayString(val)
}

function WidgetKpiView({
  widget,
  columns,
  data,
}: {
  widget: DashboardWidget
  columns: QueryResultPayload['columns']
  data: ChartRow[]
}) {
  const valCol = widget.config?.valueColumn ?? columns[0]?.name ?? ''
  const val = valCol && data[0]?.[valCol] !== undefined ? data[0][valCol] : 'N/A'
  const formattedVal = formatKpiValue(val)
  return (
    <div
      style={{
        display: 'flex',
        flexDirection: 'column',
        justifyContent: 'center',
        height: '100%',
        minHeight: '100px',
        padding: '0.5rem',
      }}
    >
      <span
        style={{
          fontSize: '2.2rem',
          fontWeight: 800,
          color: 'var(--accent)',
          wordBreak: 'break-all',
        }}
      >
        {formattedVal}
      </span>
      <span style={{ fontSize: '0.8rem', color: 'var(--text-muted)', marginTop: '0.2rem' }}>
        Latest value of {valCol}
      </span>
    </div>
  )
}

function WidgetPieChart({ data, xKey, yKey }: { data: ChartRow[]; xKey: string; yKey: string }) {
  const pieData = data.map((row, idx) => ({
    name: unknownToDisplayString(row[xKey]) || 'Other',
    value: Number(row[yKey]) || 0,
    fill: chartColor(idx),
  }))
  return (
    <div style={{ width: '100%', height: '100%', minHeight: '200px' }}>
      <ResponsiveContainer width="100%" height="100%">
        <PieChart>
          <Pie
            data={pieData}
            dataKey="value"
            nameKey="name"
            cx="50%"
            cy="50%"
            outerRadius="80%"
            label
          />
          <Tooltip contentStyle={chartTooltipStyle} />
          <DashboardLegend />
        </PieChart>
      </ResponsiveContainer>
    </div>
  )
}

function WidgetCartesianChart({
  widget,
  data,
  xKey,
  yKeys,
}: {
  widget: DashboardWidget
  data: ChartRow[]
  xKey: string
  yKeys: string[]
}) {
  return (
    <div style={{ width: '100%', height: '100%', minHeight: '200px' }}>
      <ResponsiveContainer width="100%" height="100%">
        {widget.chart_type === 'bar' ? (
          <BarChart data={data} margin={{ top: 5, right: 5, left: 0, bottom: 5 }}>
            <CartesianGrid strokeDasharray="3 3" stroke={chartGridStroke} />
            <XAxis dataKey={xKey} stroke={chartAxisStroke} tick={{ fontSize: 10 }} />
            <YAxis stroke={chartAxisStroke} tick={{ fontSize: 10 }} />
            <Tooltip contentStyle={chartTooltipStyle} />
            <DashboardLegend />
            {yKeys.map((key, idx) => (
              <Bar key={key} dataKey={key} fill={chartColor(idx)} radius={[4, 4, 0, 0]} />
            ))}
          </BarChart>
        ) : (
          <LineChart data={data} margin={{ top: 5, right: 5, left: 0, bottom: 5 }}>
            <CartesianGrid strokeDasharray="3 3" stroke={chartGridStroke} />
            <XAxis dataKey={xKey} stroke={chartAxisStroke} tick={{ fontSize: 10 }} />
            <YAxis stroke={chartAxisStroke} tick={{ fontSize: 10 }} />
            <Tooltip contentStyle={chartTooltipStyle} />
            <DashboardLegend />
            {yKeys.map((key, idx) => (
              <Line
                key={key}
                type="monotone"
                dataKey={key}
                stroke={chartColor(idx)}
                strokeWidth={2}
                dot={{ r: 3 }}
              />
            ))}
          </LineChart>
        )}
      </ResponsiveContainer>
    </div>
  )
}

function WidgetTableView({
  widget,
  columns,
  data,
}: {
  widget: DashboardWidget
  columns: QueryResultPayload['columns']
  data: ChartRow[]
}) {
  const showCols = widget.config?.visibleColumns ?? columns.map((col) => col.name)
  if (showCols.length === 0) {
    return <WidgetCenterMessage>⚠️ Please select columns to display.</WidgetCenterMessage>
  }

  return (
    <div
      className={legacyTableClass('results-table-scroll')}
      style={{ maxHeight: '100%', overflowY: 'auto' }}
    >
      <table
        className={legacyTableClass('results-table')}
        style={{ fontSize: '0.85rem', width: '100%' }}
      >
        <thead>
          <tr>
            {showCols.map((col) => (
              <th key={col} style={{ padding: '0.5rem' }}>
                {col}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {data.slice(0, 100).map((row, idx) => (
            <tr key={idx}>
              {showCols.map((col) => {
                const val = row[col]
                const displayVal =
                  val === null || val === undefined ? (
                    <em style={{ color: 'var(--text-muted)' }}>null</em>
                  ) : (
                    unknownToDisplayString(val)
                  )
                return (
                  <td key={col} style={{ padding: '0.4rem 0.5rem' }}>
                    {displayVal}
                  </td>
                )
              })}
            </tr>
          ))}
        </tbody>
      </table>
      {data.length > 100 && (
        <p
          style={{
            fontSize: '0.75rem',
            color: 'var(--text-muted)',
            textAlign: 'center',
            margin: '0.5rem 0 0 0',
          }}
        >
          Showing top 100 rows
        </p>
      )}
    </div>
  )
}

function useResetStateOnDepsChange<T>(initialValue: T, deps: unknown[]) {
  const [state, setState] = useState<T>(initialValue)
  const [prevDeps, setPrevDeps] = useState(deps)

  const changed = deps.some((d, i) => d !== prevDeps[i])
  if (changed) {
    setPrevDeps(deps)
    setState(initialValue)
  }

  return [state, setState] as const
}

function getWidgetDataStateElement(
  loading: boolean,
  error: string | null,
  data: unknown[] | null,
  savedQueryId?: string,
) {
  if (!savedQueryId) {
    return (
      <WidgetCenterMessage>⚠️ Not configured yet. Edit settings to link data.</WidgetCenterMessage>
    )
  }

  if (loading) {
    return (
      <div
        style={{
          ...centerMessageStyle,
          flexDirection: 'column',
          gap: '0.5rem',
        }}
      >
        <span
          className="status-dot"
          style={{ background: 'var(--accent)', animation: 'pulse 1.5s infinite' }}
        />
        <span style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>Executing query...</span>
      </div>
    )
  }

  if (error) {
    return (
      <div
        style={{
          color: 'var(--error)',
          padding: '1rem',
          height: '100%',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          fontSize: '0.85rem',
        }}
      >
        ⚠️ Error: {error}
      </div>
    )
  }

  if (!data || data.length === 0) {
    return <WidgetCenterMessage>No data returned.</WidgetCenterMessage>
  }

  return null
}

export function DashboardWidgetRenderer({ widget }: { widget: DashboardWidget }) {
  const { postData, loading, error, abort } = useApi()
  const [data, setData] = useResetStateOnDepsChange<ChartRow[] | null>(null, [
    widget.logical_query,
    widget.type,
  ])
  const [columns, setColumns] = useState<QueryResultPayload['columns']>([])

  useEffect(() => {
    if (widget.type === 'text' || !widget.logical_query) {
      return
    }

    let active = true
    void postData<QueryResultPayload>('/api/query/run', widget.logical_query).then((res) => {
      if (!active) {
        return
      }
      if (!res) {
        setData([])
        return
      }
      setColumns(res.columns)
      const mapped = res.rows.map((row) => {
        const obj: ChartRow = {}
        res.columns.forEach((col, idx) => {
          obj[col.name] = row[idx]
        })
        return obj
      })
      setData(mapped)
    })
    return () => {
      active = false
      abort()
      setData(null)
    }
  }, [widget.logical_query, widget.type, postData, abort, setData])

  if (widget.type === 'text') {
    return (
      <div
        style={{
          padding: '0.5rem',
          whiteSpace: 'pre-wrap',
          overflow: 'auto',
          height: '100%',
          fontSize: '0.95rem',
          lineHeight: '1.5',
        }}
      >
        {widget.content ?? <span style={{ color: 'var(--text-muted)' }}>Empty text widget.</span>}
      </div>
    )
  }

  const stateElement = getWidgetDataStateElement(loading, error, data, widget.saved_query_id)
  if (stateElement) {
    return stateElement
  }
  if (!data) {
    return null
  }

  if (widget.type === 'kpi') {
    return <WidgetKpiView widget={widget} columns={columns} data={data} />
  }

  if (widget.type === 'chart') {
    const xKey = widget.config?.xAxisColumn ?? columns[0]?.name
    const yKeys = widget.config?.yAxisColumns ?? []
    if (!xKey || yKeys.length === 0) {
      return <WidgetCenterMessage>⚠️ Please map chart dimensions and metrics.</WidgetCenterMessage>
    }
    if (widget.chart_type === 'pie' && yKeys[0]) {
      return <WidgetPieChart data={data} xKey={xKey} yKey={yKeys[0]} />
    }
    return <WidgetCartesianChart widget={widget} data={data} xKey={xKey} yKeys={yKeys} />
  }

  return <WidgetTableView widget={widget} columns={columns} data={data} />
}
