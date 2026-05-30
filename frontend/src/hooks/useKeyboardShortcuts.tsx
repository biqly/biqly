import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from 'react'
import { ShortcutsHelp } from '../components/ui/ShortcutsHelp'

export interface ShortcutKeys {
  key: string
  /** Cmd on macOS / Ctrl elsewhere. */
  mod?: boolean
  shift?: boolean
  alt?: boolean
}

export interface ShortcutDef {
  id: string
  keys: ShortcutKeys
  description: string
  group?: string
  handler: () => void
  /** Allow firing while focus is in an input/textarea/contenteditable. */
  allowInInput?: boolean
}

interface ShortcutsContextValue {
  register: (def: ShortcutDef) => () => void
  list: () => ShortcutDef[]
}

const ShortcutsContext = createContext<ShortcutsContextValue | null>(null)

function isEditableTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false
  const tag = target.tagName
  return (
    tag === 'INPUT' ||
    tag === 'TEXTAREA' ||
    tag === 'SELECT' ||
    target.isContentEditable
  )
}

function matchesEvent(keys: ShortcutKeys, event: KeyboardEvent): boolean {
  if (event.key.toLowerCase() !== keys.key.toLowerCase()) return false
  const mod = event.metaKey || event.ctrlKey
  if (keys.mod !== undefined && keys.mod !== mod) return false
  if (keys.shift !== undefined && keys.shift !== event.shiftKey) return false
  if (keys.alt !== undefined && keys.alt !== event.altKey) return false
  return true
}

export function ShortcutsProvider({ children }: { children: ReactNode }) {
  const registry = useRef(new Map<string, ShortcutDef>())
  const [helpOpen, setHelpOpen] = useState(false)
  const [, setVersion] = useState(0)

  const bump = useCallback(() => setVersion((v) => v + 1), [])

  const register = useCallback(
    (def: ShortcutDef) => {
      registry.current.set(def.id, def)
      bump()
      return () => {
        registry.current.delete(def.id)
        bump()
      }
    },
    [bump],
  )

  const list = useCallback(() => Array.from(registry.current.values()), [])

  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      const editable = isEditableTarget(event.target)
      if (!editable && (event.key === '?' || (event.key === '/' && event.shiftKey))) {
        event.preventDefault()
        setHelpOpen((prev) => !prev)
        return
      }
      for (const def of registry.current.values()) {
        if (!def.allowInInput && editable) continue
        if (matchesEvent(def.keys, event)) {
          event.preventDefault()
          def.handler()
          return
        }
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])

  const value = useMemo<ShortcutsContextValue>(() => ({ register, list }), [register, list])

  return (
    <ShortcutsContext.Provider value={value}>
      {children}
      <ShortcutsHelp open={helpOpen} shortcuts={list()} onClose={() => setHelpOpen(false)} />
    </ShortcutsContext.Provider>
  )
}

/** Register a keyboard shortcut for the lifetime of the calling component. */
export function useShortcut(def: ShortcutDef) {
  const ctx = useContext(ShortcutsContext)
  if (!ctx) {
    throw new Error('useShortcut must be used within ShortcutsProvider')
  }
  const { register } = ctx
  const handlerRef = useRef(def.handler)
  handlerRef.current = def.handler

  const { id, description, group, allowInInput } = def
  const { key, mod, shift, alt } = def.keys

  useEffect(() => {
    return register({
      id,
      description,
      group,
      allowInInput,
      keys: { key, mod, shift, alt },
      handler: () => handlerRef.current(),
    })
  }, [register, id, description, group, allowInInput, key, mod, shift, alt])
}

export function useShortcutsList(): ShortcutDef[] {
  const ctx = useContext(ShortcutsContext)
  if (!ctx) {
    throw new Error('useShortcutsList must be used within ShortcutsProvider')
  }
  return ctx.list()
}
