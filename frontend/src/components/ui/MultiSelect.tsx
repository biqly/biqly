import { useCallback, useEffect, useId, useLayoutEffect, useMemo, useRef, useState } from 'react'

import { useT } from '../../i18n'
import type { SelectOption } from './Select'
import { resolveSelectPopoverLayout } from './selectLayout'

interface MultiSelectProps {
  value: string[]
  onChange: (value: string[]) => void
  options: SelectOption[]
  placeholder?: string
  header?: string
  disabled?: boolean
  id?: string
  ariaLabel?: string
  className?: string
  size?: 'sm' | 'md'
  /** popover = trigger + dropdown; inline = always-visible checklist */
  display?: 'popover' | 'inline'
  maxHeight?: number
}

interface PopoverPos {
  left: number
  top: number
  width: number
  maxHeight: number
  placement: 'down' | 'up'
}

export function MultiSelect({
  value,
  onChange,
  options,
  placeholder,
  header,
  disabled = false,
  id,
  ariaLabel,
  className,
  size = 'md',
  display = 'popover',
  maxHeight = 288,
}: MultiSelectProps) {
  const t = useT()
  const reactId = useId()
  const baseId = id ?? `msel-${reactId}`
  const resolvedPlaceholder = placeholder ?? t('evaluation.placeholder_select')

  const [open, setOpen] = useState(display === 'inline')
  const [activeIndex, setActiveIndex] = useState(-1)
  const [popover, setPopover] = useState<PopoverPos | null>(null)

  const rootRef = useRef<HTMLDivElement>(null)
  const triggerRef = useRef<HTMLButtonElement>(null)
  const listRef = useRef<HTMLUListElement>(null)

  const selectedSet = useMemo(() => new Set(value), [value])

  const toggleValue = useCallback(
    (v: string) => {
      if (selectedSet.has(v)) {
        onChange(value.filter((x) => x !== v))
      } else {
        onChange([...value, v])
      }
    },
    [onChange, selectedSet, value],
  )

  const triggerLabel = useMemo(() => {
    if (value.length === 0) {
      return resolvedPlaceholder
    }
    if (value.length === 1) {
      const opt = options.find((o) => o.value === value[0])
      return opt?.label ?? value[0]
    }
    return `${value.length} ${t('common.selected')}`
  }, [value, options, resolvedPlaceholder, t])

  const updatePosition = useCallback(() => {
    if (display !== 'popover' || !triggerRef.current) {
      return
    }
    const rect = triggerRef.current.getBoundingClientRect()
    const viewportH = window.innerHeight
    const spaceBelow = viewportH - rect.bottom - 12
    const spaceAbove = rect.top - 12
    const placement: 'down' | 'up' = spaceBelow < 220 && spaceAbove > spaceBelow ? 'up' : 'down'
    const listMax = Math.max(
      160,
      Math.min(maxHeight, placement === 'down' ? spaceBelow : spaceAbove),
    )
    const top = placement === 'down' ? rect.bottom + 6 : Math.max(8, rect.top - 6 - listMax)
    const { left, width } = resolveSelectPopoverLayout(rect, options, size === 'sm' ? 11.5 : 12.5)
    setPopover({ left, top, width, maxHeight: listMax, placement })
  }, [display, maxHeight, options, size])

  useLayoutEffect(() => {
    if (display === 'popover' && open) {
      updatePosition()
    }
  }, [display, open, updatePosition])

  useEffect(() => {
    if (display !== 'popover' || !open) {
      return
    }
    const handle = () => updatePosition()
    window.addEventListener('resize', handle)
    window.addEventListener('scroll', handle, true)
    return () => {
      window.removeEventListener('resize', handle)
      window.removeEventListener('scroll', handle, true)
    }
  }, [display, open, updatePosition])

  useEffect(() => {
    if (display !== 'popover' || !open) {
      return
    }
    const onDown = (e: MouseEvent) => {
      const target = e.target as Node
      const inRoot = rootRef.current?.contains(target)
      const inList = listRef.current?.contains(target)
      if (!inRoot && !inList) {
        setOpen(false)
      }
    }
    document.addEventListener('mousedown', onDown)
    return () => document.removeEventListener('mousedown', onDown)
  }, [display, open])

  const listMaxStyle =
    display === 'inline' ? { maxHeight } : popover ? { maxHeight: popover.maxHeight } : undefined

  const renderOptions = () => (
    <ul
      ref={listRef}
      id={`${baseId}-list`}
      role="listbox"
      aria-multiselectable="true"
      aria-label={ariaLabel}
      className="ui-select-list"
      style={listMaxStyle}
      tabIndex={-1}
    >
      {options.length === 0 && (
        <li className="ui-select-empty" role="option" aria-disabled="true">
          {t('common.no_options')}
        </li>
      )}
      {options.map((opt, idx) => {
        const isSelected = selectedSet.has(opt.value)
        return (
          <li
            key={`${opt.value}-${idx}`}
            id={`${baseId}-opt-${idx}`}
            role="option"
            aria-selected={isSelected}
            aria-disabled={opt.disabled || undefined}
            data-index={idx}
            className={[
              'ui-select-option',
              isSelected ? 'is-selected' : '',
              idx === activeIndex ? 'is-active' : '',
              opt.disabled ? 'is-disabled' : '',
            ]
              .filter(Boolean)
              .join(' ')}
            onMouseEnter={() => !opt.disabled && setActiveIndex(idx)}
            onMouseDown={(e) => e.preventDefault()}
            onClick={() => {
              if (opt.disabled) {
                return
              }
              toggleValue(opt.value)
            }}
          >
            <span className="ui-select-check" aria-hidden="true">
              {isSelected ? (
                <svg viewBox="0 0 12 12" width="10" height="10">
                  <path
                    d="M2.5 6.3l2.5 2.5 4.5-5.1"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="1.8"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                  />
                </svg>
              ) : null}
            </span>
            <span className="ui-select-label">
              {opt.label}
              {opt.hint && <span className="ui-select-hint">{opt.hint}</span>}
            </span>
            {typeof opt.count === 'number' && <span className="ui-select-count">{opt.count}</span>}
          </li>
        )
      })}
    </ul>
  )

  if (display === 'inline') {
    return (
      <div
        ref={rootRef}
        className={['ui-select', 'ui-multiselect', 'ui-multiselect--inline', className]
          .filter(Boolean)
          .join(' ')}
      >
        {header && <div className="ui-select-header">{header}</div>}
        {renderOptions()}
      </div>
    )
  }

  const triggerClasses = ['ui-select-trigger']
  if (size === 'sm') {
    triggerClasses.push('ui-select-trigger--sm')
  }
  if (open) {
    triggerClasses.push('is-open')
  }
  if (value.length === 0) {
    triggerClasses.push('is-empty')
  }

  return (
    <div
      ref={rootRef}
      className={['ui-select', 'ui-multiselect', className].filter(Boolean).join(' ')}
    >
      <button
        ref={triggerRef}
        type="button"
        id={baseId}
        className={triggerClasses.join(' ')}
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-label={ariaLabel}
        aria-controls={open ? `${baseId}-list` : undefined}
        disabled={disabled}
        onClick={() => {
          if (disabled) {
            return
          }
          setOpen((o) => !o)
        }}
      >
        <span className={`ui-select-value${value.length === 0 ? ' is-placeholder' : ''}`}>
          {triggerLabel}
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
      {open && popover && (
        <div
          className={`ui-select-popover ui-select-popover--${popover.placement}`}
          style={{
            position: 'fixed',
            left: popover.left,
            top: popover.top,
            width: popover.width,
          }}
          role="presentation"
        >
          {header && <div className="ui-select-header">{header}</div>}
          {renderOptions()}
        </div>
      )}
    </div>
  )
}
