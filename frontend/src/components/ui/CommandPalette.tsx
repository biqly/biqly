import '../../styles/command-palette.css'

import { type ReactNode, useEffect, useMemo, useRef, useState } from 'react'

import { useShortcut } from '../../hooks/useKeyboardShortcuts'
import { useT } from '../../i18n'

export interface CommandItem {
  id: string
  label: string
  group?: string
  keywords?: string
  icon?: ReactNode
  perform: () => void
}

interface CommandPaletteProps {
  items: CommandItem[]
}

function matches(item: CommandItem, query: string): boolean {
  if (!query) {
    return true
  }
  const haystack = `${item.label} ${item.group ?? ''} ${item.keywords ?? ''}`.toLowerCase()
  return query
    .toLowerCase()
    .split(/\s+/)
    .filter(Boolean)
    .every((token) => haystack.includes(token))
}

export function CommandPalette({ items }: CommandPaletteProps) {
  const t = useT()
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState('')
  const [activeIndex, setActiveIndex] = useState(0)
  const inputRef = useRef<HTMLInputElement>(null)
  const listRef = useRef<HTMLUListElement>(null)

  const filtered = useMemo(() => items.filter((item) => matches(item, query)), [items, query])

  useShortcut({
    id: 'command-palette',
    keys: { key: 'k', mod: true },
    description: t('shortcuts.open_command_palette'),
    group: t('shortcuts.group_general'),
    allowInInput: true,
    handler: () => setOpen((prev) => !prev),
  })

  useEffect(() => {
    if (open) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setQuery('')
      setActiveIndex(0)
      const id = window.setTimeout(() => inputRef.current?.focus(), 0)
      return () => window.clearTimeout(id)
    }
    return undefined
  }, [open])

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setActiveIndex(0)
  }, [query])

  useEffect(() => {
    if (!open) {
      return
    }
    const node = listRef.current?.querySelector<HTMLElement>('[data-active="true"]')
    node?.scrollIntoView({ block: 'nearest' })
  }, [activeIndex, open])

  if (!open) {
    return null
  }

  const close = () => setOpen(false)

  const runItem = (item: CommandItem | undefined) => {
    if (!item) {
      return
    }
    close()
    item.perform()
  }

  const onInputKeyDown = (event: React.KeyboardEvent<HTMLInputElement>) => {
    if (event.key === 'ArrowDown') {
      event.preventDefault()
      setActiveIndex((prev) => (filtered.length ? (prev + 1) % filtered.length : 0))
    } else if (event.key === 'ArrowUp') {
      event.preventDefault()
      setActiveIndex((prev) =>
        filtered.length ? (prev - 1 + filtered.length) % filtered.length : 0,
      )
    } else if (event.key === 'Enter') {
      event.preventDefault()
      runItem(filtered[activeIndex])
    } else if (event.key === 'Escape') {
      event.preventDefault()
      close()
    }
  }

  return (
    <div
      className="cmdk-backdrop"
      role="presentation"
      onMouseDown={(event) => {
        if (event.target === event.currentTarget) {
          close()
        }
      }}
    >
      <div
        className="cmdk-panel"
        role="dialog"
        aria-modal="true"
        aria-label={t('command_palette.title')}
      >
        <div className="cmdk-search">
          <span className="cmdk-search__icon" aria-hidden="true">
            ⌕
          </span>
          <input
            ref={inputRef}
            type="text"
            className="cmdk-search__input"
            placeholder={t('command_palette.placeholder')}
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            onKeyDown={onInputKeyDown}
            role="combobox"
            aria-expanded="true"
            aria-controls="cmdk-list"
            aria-autocomplete="list"
          />
          <kbd className="cmdk-search__hint">Esc</kbd>
        </div>

        {filtered.length === 0 ? (
          <div className="cmdk-empty">{t('command_palette.empty')}</div>
        ) : (
          <ul className="cmdk-list" id="cmdk-list" ref={listRef} role="listbox">
            {filtered.map((item, index) => (
              <li
                key={item.id}
                role="option"
                aria-selected={index === activeIndex}
                data-active={index === activeIndex}
                className="cmdk-option"
                onMouseMove={() => setActiveIndex(index)}
                onClick={() => runItem(item)}
              >
                {item.icon && (
                  <span className="cmdk-option__icon" aria-hidden="true">
                    {item.icon}
                  </span>
                )}
                <span className="cmdk-option__label">{item.label}</span>
                {item.group && <span className="cmdk-option__group">{item.group}</span>}
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  )
}
