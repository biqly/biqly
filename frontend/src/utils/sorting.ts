import { unknownToDisplayString } from './formatters'

/**
 * Shared key-based column sort state (Faz 4,
 * tasks/frontend-table-pagination-standardization.md). Semantics mirror
 * components/resultTable/sort.ts (asc → desc → none cycle, null-last,
 * numeric-then-locale compare) but operate on column keys instead of indexes.
 */
export interface SortState {
  key: string
  dir: 'asc' | 'desc'
}

export function toggleSort(current: SortState | null, key: string): SortState | null {
  if (current?.key !== key) {
    return { key, dir: 'asc' }
  }
  if (current.dir === 'asc') {
    return { key, dir: 'desc' }
  }
  return null
}

export function compareValues(av: unknown, bv: unknown, dir: 1 | -1): number {
  if (av == null && bv == null) {
    return 0
  }
  if (av == null) {
    return dir
  }
  if (bv == null) {
    return -dir
  }
  const an = Number(av)
  const bn = Number(bv)
  if (!isNaN(an) && !isNaN(bn)) {
    return (an - bn) * dir
  }
  return unknownToDisplayString(av).localeCompare(unknownToDisplayString(bv)) * dir
}

/** Returns a sorted copy; the input array is never mutated. No sort → same reference. */
export function sortRows<T>(
  rows: T[],
  sort: SortState | null,
  valueOf: (row: T, key: string) => unknown,
): T[] {
  if (!sort) {
    return rows
  }
  const dir = sort.dir === 'asc' ? 1 : -1
  return [...rows].sort((a, b) => compareValues(valueOf(a, sort.key), valueOf(b, sort.key), dir))
}

export function ariaSortFor(
  sort: SortState | null,
  key: string,
): 'none' | 'ascending' | 'descending' {
  if (sort?.key !== key) {
    return 'none'
  }
  return sort.dir === 'asc' ? 'ascending' : 'descending'
}

export function sortArrowFor(sort: SortState | null, key: string): string {
  if (sort?.key !== key) {
    return ''
  }
  return sort.dir === 'asc' ? '↑' : '↓'
}
