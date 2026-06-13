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
      className="fixed inset-0 z-[var(--z-cmdk,1200)] grid [place-items:start_center] p-[8vh_1rem_2rem] overflow-y-auto bg-black/50 backdrop-blur-[4px] animate-cmdk-fade motion-reduce:animate-none"
      role="presentation"
      onMouseDown={(event) => {
        if (event.target === event.currentTarget) {
          close()
        }
      }}
    >
      <div
        className="w-full max-w-[36rem] border border-border-strong rounded-xl bg-card shadow-[0_24px_64px_rgba(0,0,0,0.55)] text-foreground overflow-hidden animate-cmdk-pop motion-reduce:animate-none"
        role="dialog"
        aria-modal="true"
        aria-label={t('command_palette.title')}
      >
        <div className={`flex items-center gap-[0.6rem] border-b border-border p-[0.85rem_1rem]`}>
          <span className="text-foreground-muted text-[1.1rem] leading-none" aria-hidden="true">
            ⌕
          </span>
          <input
            ref={inputRef}
            type="text"
            className="flex-1 border-none bg-transparent text-foreground text-[1rem] outline-none placeholder:text-foreground-muted"
            placeholder={t('command_palette.placeholder')}
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            onKeyDown={onInputKeyDown}
            role="combobox"
            aria-expanded="true"
            aria-controls="cmdk-list"
            aria-autocomplete="list"
          />
          <kbd
            className={`border border-border rounded-[0.35rem] bg-card-raised text-foreground-muted text-[0.7rem] p-[0.1rem_0.4rem]`}
          >
            Esc
          </kbd>
        </div>

        {filtered.length === 0 ? (
          <div className="p-[1.5rem_1rem] text-center text-foreground-muted text-[0.9rem]">
            {t('command_palette.empty')}
          </div>
        ) : (
          <ul
            className="list-none m-0 p-[0.4rem] max-h-[22rem] overflow-y-auto"
            id="cmdk-list"
            ref={listRef}
            role="listbox"
          >
            {filtered.map((item, index) => (
              <li
                key={item.id}
                role="option"
                aria-selected={index === activeIndex}
                data-active={index === activeIndex}
                className="flex items-center gap-[0.7rem] rounded-lg p-[0.55rem_0.7rem] cursor-pointer text-foreground aria-selected:bg-[var(--accent-glow)]"
                onMouseMove={() => setActiveIndex(index)}
                onClick={() => runItem(item)}
              >
                {item.icon && (
                  <span
                    className="inline-grid place-items-center w-5 h-5 text-foreground-muted [&>svg]:w-[1.1rem] [&>svg]:h-[1.1rem]"
                    aria-hidden="true"
                  >
                    {item.icon}
                  </span>
                )}
                <span className="flex-1 text-[0.9rem]">{item.label}</span>
                {item.group && (
                  <span className="text-[0.72rem] uppercase tracking-[0.04em] text-foreground-muted">
                    {item.group}
                  </span>
                )}
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  )
}
