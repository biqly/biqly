import { useMemo } from 'react'
import { useT } from '../../i18n'
import { Modal } from './Modal'
import type { ShortcutDef, ShortcutKeys } from '../../hooks/useKeyboardShortcuts'
import '../../styles/shortcuts-help.css'

const isMac =
  typeof navigator !== 'undefined' && /mac|iphone|ipad/i.test(navigator.userAgent)

function comboParts(keys: ShortcutKeys): string[] {
  const parts: string[] = []
  if (keys.mod) parts.push(isMac ? '⌘' : 'Ctrl')
  if (keys.shift) parts.push(isMac ? '⇧' : 'Shift')
  if (keys.alt) parts.push(isMac ? '⌥' : 'Alt')
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
      handler: () => {},
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
      <div className="shortcuts-help">
        {groups.map(([group, defs]) => (
          <section key={group} className="shortcuts-help__group">
            <h3 className="shortcuts-help__heading">{group}</h3>
            <ul className="shortcuts-help__list">
              {defs.map((def) => (
                <li key={def.id} className="shortcuts-help__row">
                  <span className="shortcuts-help__desc">{def.description}</span>
                  <span className="shortcuts-help__keys">
                    {comboParts(def.keys).map((part, i) => (
                      <kbd key={i} className="shortcuts-help__kbd">
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
