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
  const items = (options ?? (DEFAULT_OPTIONS as readonly T[]))
  const wrapperClass = variant === 'group'
    ? `toggle-group${className ? ` ${className}` : ''}`
    : `chart-toggle${className ? ` ${className}` : ''}`
  const btnClass = variant === 'group' ? 'toggle-btn' : ''

  return (
    <div className={wrapperClass} role="group" aria-label={ariaLabel}>
      {items.map((opt) => (
        <button
          key={opt}
          type="button"
          className={`${btnClass} ${value === opt ? 'active' : ''}`.trim()}
          onClick={() => onChange(opt)}
          aria-pressed={value === opt}
        >
          {labels?.[opt] ?? FALLBACK_LABELS[opt]}
        </button>
      ))}
    </div>
  )
}
