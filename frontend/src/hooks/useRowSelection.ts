import { useCallback, useState } from 'react'

import { setIds, toggleId } from '../utils/selection'

export interface RowSelection {
  selected: Set<string>
  isSelected: (id: string) => boolean
  toggle: (id: string) => void
  /** Select/deselect a whole group (select-all checkbox). */
  setMany: (ids: readonly string[], on: boolean) => void
  /** Replace the whole selection (load-from-server, discard-changes). */
  replace: (ids: Iterable<string>) => void
  clear: () => void
}

/**
 * Id-set selection state for lists/tables (Faz 5.2). The hook never resets
 * itself on page or filter changes — whether selection survives a page flip
 * is a per-screen decision (OQ-4), made where the page state lives.
 */
export function useRowSelection(initial?: Iterable<string>): RowSelection {
  const [selected, setSelected] = useState<Set<string>>(() => new Set(initial))

  const toggle = useCallback((id: string) => setSelected((prev) => toggleId(prev, id)), [])
  const setMany = useCallback(
    (ids: readonly string[], on: boolean) => setSelected((prev) => setIds(prev, ids, on)),
    [],
  )
  const replace = useCallback((ids: Iterable<string>) => setSelected(new Set(ids)), [])
  const clear = useCallback(() => setSelected(new Set()), [])
  const isSelected = useCallback((id: string) => selected.has(id), [selected])

  return { selected, isSelected, toggle, setMany, replace, clear }
}
