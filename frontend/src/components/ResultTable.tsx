import { useMemo, useState, type MouseEvent, type KeyboardEvent } from 'react'
import { useT } from '../i18n'
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
  const t = useT()
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

  const sortedRows = useMemo(() => {
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
  }, [indexedRows, sortColIdx, sortDir])

  const closeContextMenu = () => setContextMenu(null)

  const handleContextMenu = (e: MouseEvent, colName: string, value: string) => {
    if (!onFilterByValue) return
    e.preventDefault()
    setContextMenu({ x: e.clientX, y: e.clientY, colName, value: String(value ?? '') })
  }
  const handleCellKeyDown = (e: KeyboardEvent<HTMLElement>, colName: string, value: string) => {
    if (!onFilterByValue) return
    if (e.key !== 'ContextMenu' && !(e.shiftKey && e.key === 'F10')) return
    e.preventDefault()
    const rect = e.currentTarget.getBoundingClientRect()
    setContextMenu({ x: rect.left, y: rect.bottom, colName, value: String(value ?? '') })
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
              {t('result_table.filter_by_value', { column: contextMenu.colName, value: contextMenu.value })}
            </button>
            <button
              className="context-menu-item"
              onClick={() => {
                navigator.clipboard.writeText(String(contextMenu.value))
                closeContextMenu()
              }}
            >
              {t('result_table.copy_value')}
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
                const arrow = isActive && sortDir ? (sortDir === 'asc' ? '↑' : '↓') : ''
                const ariaSort = isActive && sortDir
                  ? sortDir === 'asc'
                    ? 'ascending'
                    : 'descending'
                  : 'none'
                return (
                  <th
                    key={col.name}
                    className="sortable"
                    aria-sort={ariaSort}
                    title={t('result_table.sort_hint', {
                      direction: isActive && sortDir
                        ? t(sortDir === 'asc' ? 'result_table.sort_asc' : 'result_table.sort_desc')
                        : '',
                    })}
                  >
                    <button
                      type="button"
                      className="results-table-sort-button"
                      onClick={() => handleSort(colIdx)}
                    >
                      <span>{col.name}</span>
                      {arrow && <span aria-hidden="true">{arrow}</span>}
                    </button>
                  </th>
                )
              })}
            </tr>
          </thead>
          <tbody>
            {sortedRows.map(({ row, originalIndex }, rowIdx) => {
              const anomalyTitle = t('ai_query.anomalies_title')
              return (
              <tr key={rowIdx}>
                {row.map((cell, colIdx) => {
                  const colName = columns[colIdx]?.name ?? ''
                  const isAnomaly = anomalyCells.has(`${originalIndex}:${colName}`)
                  return (
                    <td
                      key={colIdx}
                      className={isAnomaly ? 'results-cell--anomaly' : undefined}
                      title={isAnomaly ? anomalyTitle : undefined}
                      onContextMenu={(e) => handleContextMenu(e, colName, String(cell))}
                      onKeyDown={(e) => handleCellKeyDown(e, colName, String(cell))}
                      tabIndex={onFilterByValue ? 0 : undefined}
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
            )})}
          </tbody>
        </table>
      </div>

      <div className="result-footer">
        <span>
          {t('result_table.row_count', { count: rowCount })}
          {durationMs !== undefined ? ` · ${durationMs} ms` : ''}
        </span>
        {sortDir && (
          <span>
            {t('result_table.sorting', {
              column: columns[sortColIdx!]?.name ?? '',
              direction: sortDir === 'asc' ? t('result_table.sort_asc') : t('result_table.sort_desc'),
            })}
          </span>
        )}
      </div>
    </div>
  )
}
