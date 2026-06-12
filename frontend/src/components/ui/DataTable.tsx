import type { CSSProperties, ReactNode } from 'react'

export interface ColumnDef<T> {
  /** Stable column id (React key for th/td). */
  key: string
  header: ReactNode
  cell: (row: T) => ReactNode
  /** td class for this column; falls back to the table-level cellClassName. */
  className?: string
  /** Horizontal alignment of the td (matches the old inline textAlign styles). */
  align?: 'left' | 'right' | 'center'
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
  tableClassName = 'admin-table',
  tableStyle,
  headRowClassName = 'admin-thead-row',
  headerCellClassName = 'admin-th',
  rowClassName = 'admin-tr',
  cellClassName = 'admin-td',
}: DataTableProps<T>) {
  return (
    <table className={tableClassName} style={tableStyle}>
      <thead>
        <tr className={headRowClassName}>
          {columns.map((col) => (
            <th key={col.key} className={headerCellClassName}>
              {col.header}
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
