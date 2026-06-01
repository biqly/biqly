import { useCallback, useEffect, useId, useLayoutEffect, useMemo, useRef, useState } from 'react'
import { useT } from '../../i18n'
import { resolveSelectPopoverLayout } from './selectLayout'

export interface SelectOption<T extends string = string> {
  value: T
  label: string
  hint?: string
  count?: number
  disabled?: boolean
}

interface SelectProps<T extends string = string> {
  value: T
  onChange: (value: T) => void
  options: SelectOption<T>[]
  placeholder?: string
  header?: string
  disabled?: boolean
  id?: string
  name?: string
  ariaLabel?: string
  className?: string
  size?: 'sm' | 'md'
  /** When true, shows option hint under the label in the closed trigger (e.g. sidebar workspace). */
  showHintInTrigger?: boolean
}

interface PopoverPos {
  left: number
  top: number
  width: number
  maxHeight: number
  placement: 'down' | 'up'
}

export function Select<T extends string = string>({
  value,
  onChange,
  options,
  placeholder = '— seçin —',
  header,
  disabled = false,
  id,
  name,
  ariaLabel,
  className,
  size = 'md',
  showHintInTrigger = false,
}: SelectProps<T>) {
  const t = useT()
  const reactId = useId()
  const baseId = id ?? `sel-${reactId}`

  const [open, setOpen] = useState(false)
  const [activeIndex, setActiveIndex] = useState(-1)
  const [popover, setPopover] = useState<PopoverPos | null>(null)

  const rootRef = useRef<HTMLDivElement>(null)
  const triggerRef = useRef<HTMLButtonElement>(null)
  const listRef = useRef<HTMLUListElement>(null)

  const selectedIndex = useMemo(() => options.findIndex((o) => o.value === value), [options, value])
  const selected = selectedIndex >= 0 ? options[selectedIndex] : null

  const closeAndFocus = useCallback(() => {
    setOpen(false)
    triggerRef.current?.focus()
  }, [])

  const pickByIndex = useCallback(
    (idx: number) => {
      const opt = options[idx]
      if (!opt || opt.disabled) return
      onChange(opt.value)
      closeAndFocus()
    },
    [options, onChange, closeAndFocus],
  )

  const findNextEnabled = useCallback(
    (start: number, direction: 1 | -1): number => {
      if (options.length === 0) return -1
      let i = start
      for (let step = 0; step < options.length; step++) {
        i = (i + direction + options.length) % options.length
        const opt = options[i]
        if (opt && !opt.disabled) return i
      }
      return -1
    },
    [options],
  )

  const updatePosition = useCallback(() => {
    if (!triggerRef.current) return
    const rect = triggerRef.current.getBoundingClientRect()
    const viewportH = window.innerHeight
    const spaceBelow = viewportH - rect.bottom - 12
    const spaceAbove = rect.top - 12
    const desired = 288
    const placement: 'down' | 'up' = spaceBelow < 220 && spaceAbove > spaceBelow ? 'up' : 'down'
    const maxHeight = Math.max(160, Math.min(desired, placement === 'down' ? spaceBelow : spaceAbove))
    const top = placement === 'down' ? rect.bottom + 6 : Math.max(8, rect.top - 6 - maxHeight)
    const { left, width } = resolveSelectPopoverLayout(rect, options, size === 'sm' ? 11.5 : 12.5)
    setPopover({ left, top, width, maxHeight, placement })
  }, [options, size])

  useLayoutEffect(() => {
    if (!open) return
    updatePosition()
  }, [open, updatePosition])

  useEffect(() => {
    if (!open) return
    const handle = () => updatePosition()
    window.addEventListener('resize', handle)
    window.addEventListener('scroll', handle, true)
    return () => {
      window.removeEventListener('resize', handle)
      window.removeEventListener('scroll', handle, true)
    }
  }, [open, updatePosition])

  useEffect(() => {
    if (!open) return
    const onDown = (e: MouseEvent) => {
      const target = e.target as Node
      const inRoot = rootRef.current?.contains(target)
      const inList = listRef.current?.contains(target)
      if (!inRoot && !inList) setOpen(false)
    }
    document.addEventListener('mousedown', onDown)
    return () => document.removeEventListener('mousedown', onDown)
  }, [open])

  useEffect(() => {
    if (open) {
      const first = findNextEnabled(-1, 1)
      const cur = selectedIndex >= 0 ? options[selectedIndex] : undefined
      setActiveIndex(cur && !cur.disabled ? selectedIndex : first)
    } else {
      setActiveIndex(-1)
    }
  }, [open, options, selectedIndex, findNextEnabled])

  useEffect(() => {
    if (!open || activeIndex < 0) return
    const node = listRef.current?.querySelector<HTMLElement>(`[data-index="${activeIndex}"]`)
    node?.scrollIntoView({ block: 'nearest' })
  }, [open, activeIndex])

  const onTriggerKeyDown = (e: React.KeyboardEvent) => {
    if (disabled) return
    if (!open) {
      if (e.key === 'Enter' || e.key === ' ' || e.key === 'ArrowDown' || e.key === 'ArrowUp') {
        e.preventDefault()
        setOpen(true)
      }
      return
    }
    if (e.key === 'Escape' || e.key === 'Tab') {
      e.preventDefault()
      closeAndFocus()
      return
    }
    if (e.key === 'ArrowDown') {
      e.preventDefault()
      setActiveIndex((i) => findNextEnabled(i < 0 ? -1 : i, 1))
      return
    }
    if (e.key === 'ArrowUp') {
      e.preventDefault()
      setActiveIndex((i) => findNextEnabled(i < 0 ? options.length : i, -1))
      return
    }
    if (e.key === 'Home') {
      e.preventDefault()
      setActiveIndex(findNextEnabled(-1, 1))
      return
    }
    if (e.key === 'End') {
      e.preventDefault()
      setActiveIndex(findNextEnabled(options.length, -1))
      return
    }
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault()
      if (activeIndex >= 0) pickByIndex(activeIndex)
    }
  }

  const triggerLabel = selected ? selected.label : placeholder
  const triggerTitle =
    selected && (showHintInTrigger && selected.hint)
      ? `${selected.label} · ${selected.hint}`
      : selected
        ? selected.label
        : undefined
  const triggerClasses = ['ui-select-trigger']
  if (showHintInTrigger && selected?.hint) triggerClasses.push('ui-select-trigger--stacked')
  if (size === 'sm') triggerClasses.push('ui-select-trigger--sm')
  if (open) triggerClasses.push('is-open')
  if (!selected) triggerClasses.push('is-empty')

  return (
    <div ref={rootRef} className={['ui-select', className].filter(Boolean).join(' ')}>
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
        onClick={() => {
          if (disabled) return
          setOpen((o) => !o)
        }}
        onKeyDown={onTriggerKeyDown}
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
            triggerLabel
          )}
        </span>
        <svg className="ui-select-chevron" viewBox="0 0 12 8" width="9" height="5.5" aria-hidden="true">
          <path d="M1 1.5l5 5 5-5" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" />
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
          <ul
            ref={listRef}
            id={`${baseId}-list`}
            role="listbox"
            aria-activedescendant={activeIndex >= 0 ? `${baseId}-opt-${activeIndex}` : undefined}
            className="ui-select-list"
            style={{ maxHeight: popover.maxHeight }}
            tabIndex={-1}
          >
            {options.length === 0 && (
              <li className="ui-select-empty" role="option" aria-disabled="true">
                {t('common.no_options')}
              </li>
            )}
            {options.map((opt, idx) => {
              const isSelected = opt.value === value
              const isActive = idx === activeIndex
              const classes = ['ui-select-option']
              if (isSelected) classes.push('is-selected')
              if (isActive) classes.push('is-active')
              if (opt.disabled) classes.push('is-disabled')
              return (
                <li
                  key={`${opt.value}-${idx}`}
                  id={`${baseId}-opt-${idx}`}
                  role="option"
                  aria-selected={isSelected}
                  aria-disabled={opt.disabled || undefined}
                  data-index={idx}
                  className={classes.join(' ')}
                  onMouseEnter={() => !opt.disabled && setActiveIndex(idx)}
                  onMouseDown={(e) => e.preventDefault()}
                  onClick={() => pickByIndex(idx)}
                >
                  <span className="ui-select-check" aria-hidden="true">
                    {isSelected ? (
                      <svg viewBox="0 0 12 12" width="10" height="10">
                        <path d="M2.5 6.3l2.5 2.5 4.5-5.1" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" />
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
        </div>
      )}
    </div>
  )
}
