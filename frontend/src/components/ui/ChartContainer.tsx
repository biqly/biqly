import {
  Bar,
  BarChart,
  CartesianGrid,
  Cell,
  Line,
  LineChart,
  Pie,
  PieChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'

import { cn } from '../../lib/cn'
import { chartContainerClass } from '../../lib/feedbackClasses'
import { chartAxisStroke, chartGridStroke, chartTooltipStyle } from '../../utils/chartConfig'
import { chartColor } from '../../utils/constants'

export type ChartKind = 'bar' | 'line' | 'pie'

export interface ChartDatum {
  name: string
  value?: number
  [key: string]: unknown
}

interface ChartContainerProps {
  data: ChartDatum[]
  type: ChartKind
  height?: number
  fill?: string
  className?: string
  dataKey?: string
  nameKey?: string
  barRadius?: [number, number, number, number]
  colorFn?: (index: number) => string
  outerRadius?: number
  showLabel?: boolean
  ariaLabel?: string
}

export function ChartContainer({
  data,
  type,
  height = 300,
  fill = '#3b82f6',
  className,
  dataKey = 'value',
  nameKey = 'name',
  barRadius,
  colorFn = chartColor,
  outerRadius = 100,
  showLabel = true,
  ariaLabel,
}: ChartContainerProps) {
  return (
    <div
      className={cn(chartContainerClass, className)}
      style={{ height }}
      role="img"
      aria-label={ariaLabel ?? `${type} chart`}
    >
      <ResponsiveContainer width="100%" height="100%">
        {type === 'bar' ? (
          <BarChart data={data}>
            <CartesianGrid strokeDasharray="3 3" stroke={chartGridStroke} />
            <XAxis dataKey={nameKey} stroke={chartAxisStroke} />
            <YAxis stroke={chartAxisStroke} />
            <Tooltip contentStyle={chartTooltipStyle} />
            <Bar dataKey={dataKey} fill={fill} radius={barRadius} />
          </BarChart>
        ) : type === 'line' ? (
          <LineChart data={data}>
            <CartesianGrid strokeDasharray="3 3" stroke={chartGridStroke} />
            <XAxis dataKey={nameKey} stroke={chartAxisStroke} />
            <YAxis stroke={chartAxisStroke} />
            <Tooltip contentStyle={chartTooltipStyle} />
            <Line type="monotone" dataKey={dataKey} stroke={fill} strokeWidth={2} />
          </LineChart>
        ) : (
          <PieChart>
            <Pie
              data={data}
              dataKey={dataKey}
              nameKey={nameKey}
              cx="50%"
              cy="50%"
              outerRadius={outerRadius}
              label={showLabel}
            >
              {data.map((_, i) => (
                <Cell key={i} fill={colorFn(i)} />
              ))}
            </Pie>
            <Tooltip contentStyle={chartTooltipStyle} />
          </PieChart>
        )}
      </ResponsiveContainer>
    </div>
  )
}
