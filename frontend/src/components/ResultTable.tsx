import { useState, type MouseEvent, type KeyboardEvent } from 'react'
import { formatResultCell } from '../utils/resultCellFormat'

interface ResultTableProps {
  columns: { name: string; type?: string }[]
  rows: unknown[][]
  rowCount: number
  durationMs?: number
  question?: string
  onFilterByValue?: (column: string, value: string) => void
  onCellClick?: (column: string, value: string) => void
}

type SortDirection = 'asc' | 'desc' | null

export default function ResultTable({
  columns,
  rows,
  rowCount,
  durationMs,
  question,
  onFilterByValue,
  onCellClick,
}: ResultTableProps) {
  const [sortColIdx, setSortColIdx] = useState<number | null>(null)
  const [sortDir, setSortDir] = useState<SortDirection>(null)
  const [contextMenu, setContextMenu] = useState<{
    x: number
    y: number
    colName: string
    value: string
  } | null>(null)

  const handleSort = (colIdx: number) => {
    if (sortColIdx === colIdx) {
      setSortDir((prev) => (prev === 'asc' ? 'desc' : prev === 'desc' ? null : 'asc'))
      if (sortDir === 'desc') setSortColIdx(null)
    } else {
      setSortColIdx(colIdx)
      setSortDir('asc')
    }
  }

  const sortedRows = (() => {
    if (sortColIdx === null || sortDir === null) return rows
    const dir = sortDir === 'asc' ? 1 : -1
    return [...rows].sort((a, b) => {
      const av = a[sortColIdx]
      const bv = b[sortColIdx]
      if (av == null && bv == null) return 0
      if (av == null) return dir
      if (bv == null) return -dir
      const an = Number(av)
      const bn = Number(bv)
      if (!isNaN(an) && !isNaN(bn)) return (an - bn) * dir
      return String(av).localeCompare(String(bv)) * dir
    })
  })()

  const closeContextMenu = () => setContextMenu(null)

  const handleContextMenu = (e: MouseEvent, colName: string, value: string) => {
    if (!onFilterByValue) return
    e.preventDefault()
    setContextMenu({ x: e.clientX, y: e.clientY, colName, value: String(value ?? '') })
  }

  // Close context menu on outside click or Escape
  const handleGlobalClick = () => closeContextMenu()
  const handleKeyDown = (e: KeyboardEvent) => {
    if (e.key === 'Escape') closeContextMenu()
  }

  return (
    <div onKeyDown={handleKeyDown} style={{ position: 'relative' }}>
      {contextMenu && (
        <>
          <div
            style={{ position: 'fixed', inset: 0, zIndex: 999 }}
            onClick={handleGlobalClick}
          />
          <div
            className="context-menu"
            style={{
              position: 'fixed',
              top: contextMenu.y,
              left: contextMenu.x,
              zIndex: 1000,
            }}
          >
            <button
              className="context-menu-item"
              onClick={() => {
                onFilterByValue?.(contextMenu.colName, contextMenu.value)
                closeContextMenu()
              }}
            >
              Filter: {contextMenu.colName} = "{contextMenu.value}"
            </button>
            <button
              className="context-menu-item"
              onClick={() => {
                navigator.clipboard.writeText(String(contextMenu.value))
                closeContextMenu()
              }}
            >
              Copy value
            </button>
          </div>
        </>
      )}

      <table className="results-table">
        <thead>
          <tr>
            {columns.map((col, colIdx) => {
              const isActive = sortColIdx === colIdx
              const arrow = isActive
                ? sortDir === 'asc'
                  ? ' ↑'
                  : sortDir === 'desc'
                    ? ' ↓'
                    : ''
                : ''
              return (
                <th
                  key={col.name}
                  className={sortDir ? 'sortable' : ''}
                  onClick={() => handleSort(colIdx)}
                  style={{ cursor: 'pointer', userSelect: 'none' }}
                  title={`Click to sort${isActive ? ` (${sortDir})` : ''}`}
                >
                  {col.name}
                  {arrow}
                </th>
              )
            })}
          </tr>
        </thead>
        <tbody>
          {sortedRows.map((row, rowIdx) => (
            <tr key={rowIdx}>
              {row.map((cell, colIdx) => (
                <td
                  key={colIdx}
                  onContextMenu={(e) =>
                    handleContextMenu(e, columns[colIdx]?.name ?? '', String(cell))
                  }
                >
                  <span
                    className={onCellClick ? 'cell-drillable' : ''}
                    onClick={() => onCellClick?.(columns[colIdx]?.name ?? '', String(cell))}
                    style={{ cursor: onCellClick ? 'pointer' : 'default' }}
                  >
                    {formatResultCell(cell, columns[colIdx]?.name ?? '', {
                      question,
                    })}
                  </span>
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>

      <div className="result-footer">
        <span>
          {rowCount} row{rowCount !== 1 ? 's' : ''}
          {durationMs !== undefined ? ` · ${durationMs}ms` : ''}
        </span>
        {sortDir && (
          <span>
            Sorted by {columns[sortColIdx!]?.name} ({sortDir})
          </span>
        )}
      </div>
    </div>
  )
}
