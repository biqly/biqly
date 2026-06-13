import type { CSSProperties, ReactNode } from 'react'

import type { SortState } from '../../utils/sorting'
import { ariaSortFor, sortArrowFor } from '../../utils/sorting'
import {
  adminTableClass,
  adminTdClass,
  adminThClass,
  adminTheadRowClass,
  adminTrClass,
} from '../admin/adminClasses'

export interface ColumnDef<T> {
  /** Stable column id (React key for th/td, and the SortState key). */
  key: string
  header: ReactNode
  cell: (row: T) => ReactNode
  /** td class for this column; falls back to the table-level cellClassName. */
  className?: string
  /** Horizontal alignment of the td (matches the old inline textAlign styles). */
  align?: 'left' | 'right' | 'center'
  /** Makes the header a sort toggle. Requires the table-level sort/onSortToggle props. */
  sortable?: boolean
}

interface DataTableProps<T> {
  columns: ColumnDef<T>[]
  rows: T[]
  rowKey: (row: T) => string
  /** Blanks the empty placeholder while loading (the screens' `loading ? '' : …` pattern). */
  loading?: boolean
  /**
   * Content of the tbody placeholder row shown when rows is empty (headers stay
   * visible). Default '—'. Screens that replace the whole table when empty use
   * DataState.emptyState instead and never render this.
   */
  emptyCell?: ReactNode
  /** Class defaults target the admin table look; override for other table families. */
  tableClassName?: string
  tableStyle?: CSSProperties
  headRowClassName?: string
  headerCellClassName?: string
  rowClassName?: string
  cellClassName?: string
  /**
   * Current sort + toggle callback enable the sortable headers. DataTable does
   * NOT sort the rows itself — client-side screens sort with utils/sorting
   * sortRows before passing rows in, so a server-side sort can reuse this UI.
   */
  sort?: SortState | null
  onSortToggle?: (key: string) => void
}

/**
 * Column-config table for list screens (Faz 3,
 * tasks/frontend-table-pagination-standardization.md). Renders the exact DOM
 * the admin screens already use (table/thead/tbody + admin-* classes), so
 * migrations are markup-neutral. Sorting and row selection land in Faz 4–5.
 */
export function DataTable<T>({
  columns,
  rows,
  rowKey,
  loading = false,
  emptyCell = '—',
  tableClassName = adminTableClass,
  tableStyle,
  headRowClassName = adminTheadRowClass,
  headerCellClassName = adminThClass,
  rowClassName = adminTrClass,
  cellClassName = adminTdClass,
  sort = null,
  onSortToggle,
}: DataTableProps<T>) {
  return (
    <table className={tableClassName} style={tableStyle}>
      <thead>
        <tr className={headRowClassName}>
          {columns.map((col) => (
            <th
              key={col.key}
              className={headerCellClassName}
              aria-sort={col.sortable && onSortToggle ? ariaSortFor(sort, col.key) : undefined}
            >
              {col.sortable && onSortToggle ? (
                <button
                  type="button"
                  className="inline-flex cursor-pointer items-center gap-1 border-0 bg-transparent p-0 text-inherit [font:inherit] hover:text-foreground"
                  onClick={() => onSortToggle(col.key)}
                >
                  {col.header}
                  <span className="text-[0.85em] leading-none" aria-hidden="true">
                    {sortArrowFor(sort, col.key)}
                  </span>
                </button>
              ) : (
                col.header
              )}
            </th>
          ))}
        </tr>
      </thead>
      <tbody>
        {rows.length === 0 ? (
          <tr className={rowClassName}>
            <td
              colSpan={columns.length}
              style={{ padding: 24, textAlign: 'center', color: '#9ca3af' }}
            >
              {loading ? '' : emptyCell}
            </td>
          </tr>
        ) : (
          rows.map((row) => (
            <tr key={rowKey(row)} className={rowClassName}>
              {columns.map((col) => (
                <td
                  key={col.key}
                  className={col.className ?? cellClassName}
                  style={col.align && col.align !== 'left' ? { textAlign: col.align } : undefined}
                >
                  {col.cell(row)}
                </td>
              ))}
            </tr>
          ))
        )}
      </tbody>
    </table>
  )
}
