import type { ReactNode } from 'react'
import { useEffect, useId, useRef, useState } from 'react'

import { legacyButtonClass } from '../../lib/buttonClasses'
import { legacyCardClass } from '../../lib/cardClasses'
import { cn } from '../../lib/cn'

export interface ActionMenuItem {
  key: string
  label: string
  icon?: ReactNode
  onSelect: () => void
  disabled?: boolean
  danger?: boolean
}

/** Compact dropdown for grouping secondary actions behind a single trigger. */
export function ActionMenu({
  label,
  ariaLabel,
  header,
  items,
}: {
  label: ReactNode
  ariaLabel?: string
  header?: ReactNode
  items: ActionMenuItem[]
}) {
  const [open, setOpen] = useState(false)
  const rootRef = useRef<HTMLDivElement>(null)
  const menuId = useId()

  useEffect(() => {
    if (!open) {
      return
    }
    const onPointerDown = (e: MouseEvent) => {
      if (rootRef.current && !rootRef.current.contains(e.target as Node)) {
        setOpen(false)
      }
    }
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        setOpen(false)
      }
    }
    document.addEventListener('mousedown', onPointerDown)
    document.addEventListener('keydown', onKeyDown)
    return () => {
      document.removeEventListener('mousedown', onPointerDown)
      document.removeEventListener('keydown', onKeyDown)
    }
  }, [open])

  return (
    <div className="relative inline-flex" ref={rootRef}>
      <button
        type="button"
        className={cn(
          legacyButtonClass('btn btn-secondary'),
          'inline-flex items-center gap-[0.35rem]',
        )}
        aria-haspopup="menu"
        aria-expanded={open}
        aria-controls={menuId}
        aria-label={ariaLabel}
        onClick={() => setOpen((v) => !v)}
      >
        {label}
      </button>
      {open && (
        <div
          className={legacyCardClass(
            'border-border-strong bg-card absolute top-[calc(100%+0.35rem)] right-0 z-30 grid min-w-52 gap-[0.1rem] rounded-[0.6rem] border p-[0.35rem] shadow-[0_12px_36px_rgba(0,0,0,0.35)] motion-safe:animate-[action-menu-in_120ms_ease]',
          )}
          id={menuId}
          role="menu"
        >
          {header && (
            <div
              className={`border-border text-foreground-muted mb-[0.2rem] border-b px-[0.6rem] pt-[0.4rem] pb-2 text-[0.78rem]`}
            >
              {header}
            </div>
          )}
          {items.map((item) => (
            <button
              key={item.key}
              type="button"
              role="menuitem"
              className={cn(
                'focus-visible:outline-accent disabled:text-foreground-faint flex w-full cursor-pointer items-center gap-2 rounded-[0.4rem] border-0 bg-transparent px-[0.6rem] py-[0.45rem] text-left text-[0.82rem] transition-colors focus-visible:outline-2 focus-visible:-outline-offset-2 enabled:hover:bg-(--control-hover-bg,rgba(127,127,127,0.12)) disabled:cursor-not-allowed',
                item.danger ? 'text-error' : 'text-foreground',
              )}
              disabled={item.disabled}
              onClick={() => {
                setOpen(false)
                item.onSelect()
              }}
            >
              {item.icon && (
                <span className="w-[1.1rem] shrink-0 text-center" aria-hidden="true">
                  {item.icon}
                </span>
              )}
              {item.label}
            </button>
          ))}
        </div>
      )}
    </div>
  )
}
