import type { RefObject } from 'react'

import type { SelectOption } from './Select'

export function SelectTrigger<T extends string>({
  baseId,
  name,
  ariaLabel,
  open,
  disabled,
  selected,
  placeholder,
  showHintInTrigger,
  size,
  triggerRef,
  onToggle,
  onKeyDown,
}: {
  baseId: string
  name?: string
  ariaLabel?: string
  open: boolean
  disabled: boolean
  selected: SelectOption<T> | null
  placeholder: string
  showHintInTrigger: boolean
  size: 'sm' | 'md'
  triggerRef: RefObject<HTMLButtonElement | null>
  onToggle: () => void
  onKeyDown: (e: React.KeyboardEvent) => void
}) {
  const triggerClasses = ['ui-select-trigger']
  if (showHintInTrigger && selected?.hint) {
    triggerClasses.push('ui-select-trigger--stacked')
  }
  if (size === 'sm') {
    triggerClasses.push('ui-select-trigger--sm')
  }
  if (open) {
    triggerClasses.push('is-open')
  }
  if (!selected) {
    triggerClasses.push('is-empty')
  }

  const triggerTitle =
    selected && showHintInTrigger && selected.hint
      ? `${selected.label} · ${selected.hint}`
      : selected
        ? selected.label
        : undefined

  return (
    <button
      ref={triggerRef}
      type="button"
      id={baseId}
      name={name}
      className={triggerClasses.join(' ')}
      aria-haspopup="listbox"
      aria-expanded={open}
      aria-label={ariaLabel}
      aria-controls={open ? `${baseId}-list` : undefined}
      disabled={disabled}
      title={triggerTitle}
      onClick={onToggle}
      onKeyDown={onKeyDown}
    >
      <span
        className={[
          'ui-select-value',
          !selected ? 'is-placeholder' : '',
          showHintInTrigger && selected?.hint ? 'ui-select-value--stacked' : '',
        ]
          .filter(Boolean)
          .join(' ')}
      >
        {showHintInTrigger && selected?.hint ? (
          <>
            <span className="ui-select-value-primary">{selected.label}</span>
            <span className="ui-select-value-hint">{selected.hint}</span>
          </>
        ) : (
          placeholder
        )}
      </span>
      <svg
        className="ui-select-chevron"
        viewBox="0 0 12 8"
        width="9"
        height="5.5"
        aria-hidden="true"
      >
        <path
          d="M1 1.5l5 5 5-5"
          fill="none"
          stroke="currentColor"
          strokeWidth="1.6"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
      </svg>
    </button>
  )
}
