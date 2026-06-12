import { useCallback, useState } from 'react'

import type { SortState } from '../utils/sorting'
import { toggleSort } from '../utils/sorting'

/** Column sort state for DataTable consumers (asc → desc → none cycle). */
export function useSortState(initial: SortState | null = null): {
  sort: SortState | null
  toggle: (key: string) => void
} {
  const [sort, setSort] = useState<SortState | null>(initial)
  const toggle = useCallback((key: string) => setSort((s) => toggleSort(s, key)), [])
  return { sort, toggle }
}
