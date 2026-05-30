import { Bar } from 'recharts/es6/cartesian/Bar'
import { BarChart } from 'recharts/es6/chart/BarChart'
import { CartesianGrid } from 'recharts/es6/cartesian/CartesianGrid'
import { Cell } from 'recharts/es6/component/Cell'
import { Line } from 'recharts/es6/cartesian/Line'
import { LineChart } from 'recharts/es6/chart/LineChart'
import { Pie } from 'recharts/es6/polar/Pie'
import { PieChart } from 'recharts/es6/chart/PieChart'
import { ResponsiveContainer } from 'recharts/es6/component/ResponsiveContainer'
import { Tooltip } from 'recharts/es6/component/Tooltip'
import { XAxis } from 'recharts/es6/cartesian/XAxis'
import { YAxis } from 'recharts/es6/cartesian/YAxis'
import clsx from 'clsx'
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
      className={clsx('chart-container', className)}
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
            <Pie data={data} dataKey={dataKey} nameKey={nameKey} cx="50%" cy="50%" outerRadius={outerRadius} label={showLabel}>
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
