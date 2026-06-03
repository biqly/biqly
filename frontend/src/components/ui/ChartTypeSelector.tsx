import { ToggleButtonGroup, type ToggleButtonOption } from './ToggleButtonGroup'

export type ChartTypeOption = 'bar' | 'line' | 'pie' | 'table'

const FALLBACK_LABELS: Record<ChartTypeOption, string> = {
  bar: 'Bar',
  line: 'Line',
  pie: 'Pie',
  table: 'Table',
}

const DEFAULT_OPTIONS: readonly ChartTypeOption[] = ['bar', 'line', 'pie']

interface ChartTypeSelectorProps<T extends ChartTypeOption = ChartTypeOption> {
  value: T
  onChange: (next: T) => void
  options?: readonly T[]
  className?: string
  variant?: 'toggle' | 'group'
  ariaLabel?: string
  labels?: Partial<Record<ChartTypeOption, string>>
}

export function ChartTypeSelector<T extends ChartTypeOption>({
  value,
  onChange,
  options,
  className,
  variant = 'toggle',
  ariaLabel = 'Chart type',
  labels,
}: ChartTypeSelectorProps<T>) {
  const items = options ?? (DEFAULT_OPTIONS as readonly T[])
  const wrapperClass =
    variant === 'group'
      ? `toggle-group${className ? ` ${className}` : ''}`
      : `chart-toggle${className ? ` ${className}` : ''}`
  const btnClass = variant === 'group' ? 'toggle-btn' : undefined
  const toggleOptions: ToggleButtonOption<T>[] = items.map((opt) => ({
    value: opt,
    label: labels?.[opt] ?? FALLBACK_LABELS[opt],
  }))

  return (
    <ToggleButtonGroup
      value={value}
      options={toggleOptions}
      onChange={onChange}
      ariaLabel={ariaLabel}
      className={wrapperClass}
      buttonClassName={btnClass}
    />
  )
}
