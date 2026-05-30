import { useMemo, useState, type MouseEvent, type KeyboardEvent } from 'react'
import { useT } from '../i18n'
import { formatResultCell } from '../utils/resultCellFormat'
import '../styles/table-results.css'
import type { ResultAnomaly } from '../types/ai'
import { buildAnomalyCellSet, isAnomalyCell } from './resultTable/anomalies'
import {
  buildContextMenuFromCellRect,
  buildContextMenuFromPointer,
  isContextMenuKey,
} from './resultTable/contextMenu'
import {
  ariaSortValue,
  cycleSortState,
  indexRows,
  sortArrow,
  sortIndexedRows,
  type SortDirection,
} from './resultTable/sort'

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
  const anomalyCells = useMemo(() => buildAnomalyCellSet(anomalies), [anomalies])
  const [sortColIdx, setSortColIdx] = useState<number | null>(null)
  const [sortDir, setSortDir] = useState<SortDirection>(null)
  const [contextMenu, setContextMenu] = useState<{
    x: number
    y: number
    colName: string
    value: string
  } | null>(null)

  const handleSort = (colIdx: number) => {
    const next = cycleSortState(sortColIdx, sortDir, colIdx)
    setSortColIdx(next.sortColIdx)
    setSortDir(next.sortDir)
  }

  const indexedRows = useMemo(() => indexRows(rows), [rows])

  const sortedRows = useMemo(
    () => sortIndexedRows(indexedRows, sortColIdx, sortDir),
    [indexedRows, sortColIdx, sortDir],
  )

  const closeContextMenu = () => setContextMenu(null)

  const handleContextMenu = (e: MouseEvent, colName: string, value: string) => {
    if (!onFilterByValue) return
    e.preventDefault()
    setContextMenu(buildContextMenuFromPointer(e.clientX, e.clientY, colName, value))
  }
  const handleCellKeyDown = (e: KeyboardEvent<HTMLElement>, colName: string, value: string) => {
    if (!onFilterByValue) return
    if (!isContextMenuKey(e.key, e.shiftKey)) return
    e.preventDefault()
    const rect = e.currentTarget.getBoundingClientRect()
    setContextMenu(buildContextMenuFromCellRect(rect, colName, value))
  }

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
                const arrow = sortArrow(sortColIdx, sortDir, colIdx)
                const ariaSort = ariaSortValue(sortColIdx, sortDir, colIdx)
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
                  const isAnomaly = isAnomalyCell(anomalyCells, originalIndex, colName)
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
