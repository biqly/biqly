import type { RefObject } from 'react'

import {
  selectChevronClass,
  selectTriggerClass,
  selectValueClass,
  selectValueHintClass,
  selectValuePrimaryClass,
} from '../../lib/selectClasses'
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
  const stacked = Boolean(showHintInTrigger && selected?.hint)

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
      className={selectTriggerClass({
        size,
        open,
        empty: !selected,
        stacked,
      })}
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
        className={selectValueClass({
          placeholder: !selected,
          stacked,
        })}
      >
        {stacked ? (
          <>
            <span className={selectValuePrimaryClass}>{selected!.label}</span>
            <span className={selectValueHintClass}>{selected!.hint}</span>
          </>
        ) : (
          placeholder
        )}
      </span>
      <svg
        className={selectChevronClass(open)}
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
