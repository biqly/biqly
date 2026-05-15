import { useMemo, useState, type MouseEvent, type KeyboardEvent } from 'react'
import { formatResultCell } from '../utils/resultCellFormat'
import type { ResultAnomaly } from '../types/ai'

interface ResultTableProps {
  columns: { name: string; type?: string }[]
  rows: unknown[][]
  rowCount: number
  durationMs?: number
  question?: string
  anomalies?: ResultAnomaly[]
  onFilterByValue?: (column: string, value: string) => void
  onCellClick?: (column: string, value: string) => void
}

type SortDirection = 'asc' | 'desc' | null

export function ResultTable({
  columns,
  rows,
  rowCount,
  durationMs,
  question,
  anomalies,
  onFilterByValue,
  onCellClick,
}: ResultTableProps) {
  const anomalyCells = useMemo(() => {
    const set = new Set<string>()
    for (const a of anomalies ?? []) {
      set.add(`${a.row_index}:${a.column}`)
    }
    return set
  }, [anomalies])
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

  const indexedRows = useMemo(
    () => rows.map((row, originalIndex) => ({ row, originalIndex })),
    [rows],
  )

  const sortedRows = (() => {
    if (sortColIdx === null || sortDir === null) return indexedRows
    const dir = sortDir === 'asc' ? 1 : -1
    return [...indexedRows].sort((a, b) => {
      const av = a.row[sortColIdx]
      const bv = b.row[sortColIdx]
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
              Filtre: {contextMenu.colName} = "{contextMenu.value}"
            </button>
            <button
              className="context-menu-item"
              onClick={() => {
                navigator.clipboard.writeText(String(contextMenu.value))
                closeContextMenu()
              }}
            >
              Değeri kopyala
            </button>
          </div>
        </>
      )}

      <div className="results-table-scroll">
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
                  title={`Sıralamak için tıklayın${isActive ? ` (${sortDir === 'asc' ? 'artan' : sortDir === 'desc' ? 'azalan' : ''})` : ''}`}
                >
                  {col.name}
                  {arrow}
                </th>
              )
            })}
          </tr>
        </thead>
        <tbody>
          {sortedRows.map(({ row, originalIndex }, rowIdx) => {
            const rowHasAnomaly = (anomalies ?? []).some((a) => a.row_index === originalIndex)
            return (
              <tr key={rowIdx} className={rowHasAnomaly ? 'results-row--anomaly' : undefined}>
                {row.map((cell, colIdx) => {
                  const colName = columns[colIdx]?.name ?? ''
                  const isAnomaly = anomalyCells.has(`${originalIndex}:${colName}`)
                  return (
                    <td
                      key={colIdx}
                      className={isAnomaly ? 'results-cell--anomaly' : undefined}
                      title={isAnomaly ? 'IQR tabanlı aykırı değer' : undefined}
                      onContextMenu={(e) => handleContextMenu(e, colName, String(cell))}
                    >
                      <span
                        className={onCellClick ? 'cell-drillable' : ''}
                        onClick={() => onCellClick?.(colName, String(cell))}
                        style={{ cursor: onCellClick ? 'pointer' : 'default' }}
                      >
                        {formatResultCell(cell, colName, { question })}
                      </span>
                    </td>
                  )
                })}
              </tr>
            )
          })}
        </tbody>
      </table>
      </div>

      <div className="result-footer">
        <span>
          {rowCount} satır
          {durationMs !== undefined ? ` · ${durationMs} ms` : ''}
        </span>
        {sortDir && (
          <span>
            Sıralama: {columns[sortColIdx!]?.name} ({sortDir === 'asc' ? 'artan' : 'azalan'})
          </span>
        )}
      </div>
    </div>
  )
}
