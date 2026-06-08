import { unknownToDisplayString } from '../../utils/formatters'
import type { IndexedRow, SortDirection } from './types'

export type { SortDirection }

export function compareCellValues(av: unknown, bv: unknown, dir: 1 | -1): number {
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

export function indexRows(rows: unknown[][]): IndexedRow[] {
  return rows.map((row, originalIndex) => ({ row, originalIndex }))
}

export function sortIndexedRows(
  indexedRows: IndexedRow[],
  sortColIdx: number | null,
  sortDir: SortDirection,
): IndexedRow[] {
  if (sortColIdx === null || sortDir === null) {
    return indexedRows
  }
  const dir = sortDir === 'asc' ? 1 : -1
  return [...indexedRows].sort((a, b) =>
    compareCellValues(a.row[sortColIdx], b.row[sortColIdx], dir),
  )
}

export function cycleSortState(
  currentColIdx: number | null,
  currentDir: SortDirection,
  clickedColIdx: number,
): { sortColIdx: number | null; sortDir: SortDirection } {
  if (currentColIdx !== clickedColIdx) {
    return { sortColIdx: clickedColIdx, sortDir: 'asc' }
  }
  if (currentDir === 'asc') {
    return { sortColIdx: clickedColIdx, sortDir: 'desc' }
  }
  if (currentDir === 'desc') {
    return { sortColIdx: null, sortDir: null }
  }
  return { sortColIdx: clickedColIdx, sortDir: 'asc' }
}

export function ariaSortValue(
  sortColIdx: number | null,
  sortDir: SortDirection,
  colIdx: number,
): 'none' | 'ascending' | 'descending' {
  if (sortColIdx !== colIdx || !sortDir) {
    return 'none'
  }
  return sortDir === 'asc' ? 'ascending' : 'descending'
}

export function sortArrow(
  sortColIdx: number | null,
  sortDir: SortDirection,
  colIdx: number,
): string {
  if (sortColIdx !== colIdx || !sortDir) {
    return ''
  }
  return sortDir === 'asc' ? '↑' : '↓'
}
