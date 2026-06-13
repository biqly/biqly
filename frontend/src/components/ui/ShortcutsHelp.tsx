import { useMemo } from 'react'

import type { ShortcutDef, ShortcutKeys } from '../../hooks/useKeyboardShortcuts'
import { useT } from '../../i18n'
import { legacyCardClass } from '../../lib/cardClasses'
import { noop } from '../../utils/constants'
import { Modal } from './Modal'

const isMac = typeof navigator !== 'undefined' && /mac|iphone|ipad/i.test(navigator.userAgent)

function comboParts(keys: ShortcutKeys): string[] {
  const parts: string[] = []
  if (keys.mod) {
    parts.push(isMac ? '⌘' : 'Ctrl')
  }
  if (keys.shift) {
    parts.push(isMac ? '⇧' : 'Shift')
  }
  if (keys.alt) {
    parts.push(isMac ? '⌥' : 'Alt')
  }
  parts.push(keys.key.length === 1 ? keys.key.toUpperCase() : keys.key)
  return parts
}

interface ShortcutsHelpProps {
  open: boolean
  shortcuts: ShortcutDef[]
  onClose: () => void
}

export function ShortcutsHelp({ open, shortcuts, onClose }: ShortcutsHelpProps) {
  const t = useT()

  const groups = useMemo(() => {
    const help: ShortcutDef = {
      id: '__help',
      keys: { key: '?' },
      description: t('shortcuts.show_help'),
      group: t('shortcuts.group_general'),
      handler: noop,
    }
    const all = [help, ...shortcuts]
    const buckets = new Map<string, ShortcutDef[]>()
    for (const def of all) {
      const group = def.group ?? t('shortcuts.group_general')
      const prev = buckets.get(group) ?? []
      prev.push(def)
      buckets.set(group, prev)
    }
    return Array.from(buckets.entries())
  }, [shortcuts, t])

  return (
    <Modal open={open} title={t('shortcuts.title')} onClose={onClose}>
      <div className="grid gap-[1.1rem]">
        {groups.map(([group, defs]) => (
          <section key={group} className="grid gap-[0.4rem]">
            <h3 className="m-0 text-[0.72rem] font-semibold uppercase tracking-[0.05em] text-foreground-muted">
              {group}
            </h3>
            <ul className="m-0 grid list-none gap-[0.15rem] p-0">
              {defs.map((def) => (
                <li
                  key={def.id}
                  className={legacyCardClass(
                    'flex items-center justify-between gap-4 rounded-[0.4rem] px-2 py-[0.4rem] hover:bg-card-raised',
                  )}
                >
                  <span className="text-[0.88rem] text-foreground">{def.description}</span>
                  <span className="inline-flex shrink-0 gap-1">
                    {comboParts(def.keys).map((part, i) => (
                      <kbd
                        key={i}
                        className={legacyCardClass(
                          'inline-grid h-6 min-w-6 place-items-center rounded-[0.35rem] border-x border-t border-b-2 border-border-strong bg-card-raised px-[0.4rem] text-[0.78rem] leading-none text-foreground [font-family:inherit]',
                        )}
                      >
                        {part}
                      </kbd>
                    ))}
                  </span>
                </li>
              ))}
            </ul>
          </section>
        ))}
      </div>
    </Modal>
  )
}
