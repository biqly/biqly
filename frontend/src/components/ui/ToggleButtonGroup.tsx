import { cn } from '../../lib/cn'
import { toggleBtnClass } from '../../lib/toggleClasses'

export interface ToggleButtonOption<T extends string> {
  value: T
  label: string
}

interface ToggleButtonGroupProps<T extends string> {
  value: T
  options: readonly ToggleButtonOption<T>[]
  onChange: (value: T) => void
  ariaLabel: string
  className?: string
  buttonClassName?: string
  activeClassName?: string
  toggleButtons?: boolean
}

export function ToggleButtonGroup<T extends string>({
  value,
  options,
  onChange,
  ariaLabel,
  className,
  buttonClassName,
  activeClassName = 'active',
  toggleButtons = false,
}: ToggleButtonGroupProps<T>) {
  return (
    <div className={className} role="group" aria-label={ariaLabel}>
      {options.map((option) => {
        const active = value === option.value
        return (
          <button
            key={option.value}
            type="button"
            className={
              toggleButtons
                ? toggleBtnClass(active, buttonClassName)
                : cn(buttonClassName, active && activeClassName)
            }
            onClick={() => onChange(option.value)}
            aria-pressed={active}
          >
            {option.label}
          </button>
        )
      })}
    </div>
  )
}
