import type { KeyboardEvent, Ref, RefObject } from 'react'

import type { SelectOption } from './Select'
import { handleSelectTriggerKeyDown, type SelectKeyboardContext } from './selectKeyboard'

interface PopoverPos {
  left: number
  top: number
  width: number
  maxHeight: number
  placement: 'down' | 'up'
}

export function SelectPopover<T extends string>({
  baseId,
  popover,
  header,
  searchable,
  search,
  setSearch,
  searchRef,
  displayOptions,
  value,
  activeIndex,
  setActiveIndex,
  pickByIndex,
  keyboardCtx,
  listRef,
  searchPlaceholder,
  emptyLabel,
}: {
  baseId: string
  popover: PopoverPos
  header?: string
  searchable: boolean
  search: string
  setSearch: (value: string) => void
  searchRef: RefObject<HTMLInputElement | null>
  displayOptions: SelectOption<T>[]
  value: T
  activeIndex: number
  setActiveIndex: (index: number | ((prev: number) => number)) => void
  pickByIndex: (idx: number) => void
  keyboardCtx: SelectKeyboardContext
  listRef: Ref<HTMLUListElement>
  searchPlaceholder: string
  emptyLabel: string
}) {
  const onKeyDown = (e: KeyboardEvent) => handleSelectTriggerKeyDown(e, keyboardCtx)

  return (
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
      {searchable && (
        <div className="ui-select-search">
          <input
            ref={searchRef}
            type="search"
            className="ui-select-search-input"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder={searchPlaceholder}
            autoComplete="off"
            onKeyDown={(e) => {
              if (
                e.key === 'ArrowDown' ||
                e.key === 'ArrowUp' ||
                e.key === 'Enter' ||
                e.key === 'Escape'
              ) {
                onKeyDown(e)
              }
              if (e.key === ' ') {
                e.stopPropagation()
              }
            }}
          />
        </div>
      )}
      <ul
        ref={listRef}
        id={`${baseId}-list`}
        role="listbox"
        aria-activedescendant={activeIndex >= 0 ? `${baseId}-opt-${activeIndex}` : undefined}
        className="ui-select-list"
        style={{
          maxHeight: searchable ? Math.max(120, popover.maxHeight - 44) : popover.maxHeight,
        }}
        tabIndex={-1}
        onKeyDown={onKeyDown}
      >
        {displayOptions.length === 0 && (
          <li className="ui-select-empty" role="option" aria-disabled="true">
            {emptyLabel}
          </li>
        )}
        {displayOptions.map((opt, idx) => {
          const isSelected = opt.value === value
          const isActive = idx === activeIndex
          const classes = ['ui-select-option']
          if (isSelected) {
            classes.push('is-selected')
          }
          if (isActive) {
            classes.push('is-active')
          }
          if (opt.disabled) {
            classes.push('is-disabled')
          }
          return (
            <li
              key={`${opt.value}-${idx}`}
              id={`${baseId}-opt-${idx}`}
              role="option"
              aria-selected={isSelected}
              aria-disabled={opt.disabled ?? undefined}
              data-index={idx}
              className={classes.join(' ')}
              onMouseEnter={() => !opt.disabled && setActiveIndex(idx)}
              onMouseDown={(e) => e.preventDefault()}
              onClick={() => pickByIndex(idx)}
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
              {typeof opt.count === 'number' && (
                <span className="ui-select-count">{opt.count}</span>
              )}
            </li>
          )
        })}
      </ul>
    </div>
  )
}
