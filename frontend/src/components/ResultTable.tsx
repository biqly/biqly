import { type KeyboardEvent, type MouseEvent, useMemo, useState } from 'react'

import { useToast } from '../hooks/useToast'
import { localeLanguageTag, useLocale, useT } from '../i18n'
import { buttonClass } from '../lib/buttonClasses'
import { cn } from '../lib/cn'
import { cellDrillableClass } from '../lib/tableClasses'
import type { QueryColumnFormat, ResultAnomaly } from '../types/ai'
import { downloadCsv } from '../utils/exportCsv'
import { formatResultCell } from '../utils/resultCellFormat'
import { formatGrainValue } from './aiQuery/grainLabels'
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
  type SortDirection,
  sortIndexedRows,
} from './resultTable/sort'

interface ResultTableProps {
  columns: { name: string; type?: string; format?: QueryColumnFormat }[]
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
  const [locale] = useLocale()
  const localeTag = localeLanguageTag(locale)
  const toast = useToast()
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

  const handleExport = () => {
    const exportRows = sortedRows.map(({ row }) => row)
    if (exportRows.length === 0) {
      toast.warning(t('result_table.export_empty'))
      return
    }
    downloadCsv(columns, exportRows, question ? `biqly-${question}` : 'biqly-export')
    toast.success(t('result_table.export_success', { count: exportRows.length }))
  }

  const closeContextMenu = () => setContextMenu(null)

  const handleContextMenu = (e: MouseEvent, colName: string, value: string) => {
    if (!onFilterByValue) {
      return
    }
    e.preventDefault()
    setContextMenu(buildContextMenuFromPointer(e.clientX, e.clientY, colName, value))
  }
  const handleCellKeyDown = (e: KeyboardEvent<HTMLElement>, colName: string, value: string) => {
    if (!onFilterByValue) {
      return
    }
    if (!isContextMenuKey(e.key, e.shiftKey)) {
      return
    }
    e.preventDefault()
    const rect = e.currentTarget.getBoundingClientRect()
    setContextMenu(buildContextMenuFromCellRect(rect, colName, value))
  }

  const handleGlobalClick = () => closeContextMenu()
  const handleKeyDown = (e: KeyboardEvent) => {
    if (e.key === 'Escape') {
      closeContextMenu()
    }
  }

  return (
    <div onKeyDown={handleKeyDown} style={{ position: 'relative' }}>
      {contextMenu && (
        <>
          <div style={{ position: 'fixed', inset: 0, zIndex: 999 }} onClick={handleGlobalClick} />
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
              {t('result_table.filter_by_value', {
                column: contextMenu.colName,
                value: contextMenu.value,
              })}
            </button>
            <button
              className="context-menu-item"
              onClick={() => {
                void navigator.clipboard.writeText(String(contextMenu.value))
                closeContextMenu()
              }}
            >
              {t('result_table.copy_value')}
            </button>
          </div>
        </>
      )}

      <div className="custom-scrollbar-thin mt-4 max-w-full overflow-x-auto overscroll-x-contain">
        <table className="mt-4 w-full min-w-2xl border-collapse text-[0.9rem] max-[720px]:text-[0.82rem]">
          <thead>
            <tr>
              {columns.map((col, colIdx) => {
                const isActive = sortColIdx === colIdx
                const arrow = sortArrow(sortColIdx, sortDir, colIdx)
                const ariaSort = ariaSortValue(sortColIdx, sortDir, colIdx)
                return (
                  <th
                    key={col.name}
                    className={`border-border hover:text-foreground sticky top-0 z-2 border-b bg-(--table-header-bg) p-[0.75rem_0.9rem] pt-[0.85rem] pb-[0.85rem] text-left align-middle font-['Plus_Jakarta_Sans',sans-serif] text-[0.7rem] font-bold tracking-wider text-(--table-header-fg) uppercase shadow-[0_1px_0_var(--table-header-shadow-line)] transition-colors duration-150 max-[720px]:p-[0.55rem_0.6rem]`}
                    aria-sort={ariaSort}
                    title={t('result_table.sort_hint', {
                      direction:
                        isActive && sortDir
                          ? t(
                              sortDir === 'asc'
                                ? 'result_table.sort_asc'
                                : 'result_table.sort_desc',
                            )
                          : '',
                    })}
                  >
                    <button
                      type="button"
                      className="font-inherit inline-flex w-full cursor-pointer items-center gap-[0.35rem] border-0 bg-transparent p-0 text-inherit"
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
                <tr
                  key={rowIdx}
                  className={`border-border hover:text-foreground group border-b last:border-b-0 odd:bg-(--table-stripe-odd) even:bg-(--table-stripe-even) hover:bg-(--table-stripe-hover)`}
                >
                  {row.map((cell, colIdx) => {
                    const colName = columns[colIdx]?.name ?? ''
                    const isAnomaly = isAnomalyCell(anomalyCells, originalIndex, colName)
                    // Month/quarter grains carry an integer ordinal: relabel the
                    // display only; sort, drill-down and export keep the raw value.
                    const grainLabel = formatGrainValue(columns[colIdx]?.format, cell, localeTag)
                    return (
                      <td
                        key={colIdx}
                        className={cn(
                          'border-border text-foreground-muted border-b p-[0.75rem_0.9rem] text-left align-middle text-[0.86rem] leading-[1.4] transition-colors duration-150 max-[720px]:p-[0.55rem_0.6rem]',
                          isAnomaly &&
                            'text-foreground! bg-[color-mix(in_srgb,var(--warning)_4%,transparent)] font-semibold shadow-[inset_0_0_0_1px_color-mix(in_srgb,var(--warning)_40%,transparent)]',
                        )}
                        title={isAnomaly ? anomalyTitle : undefined}
                        onContextMenu={(e) => handleContextMenu(e, colName, String(cell))}
                        onKeyDown={(e) => handleCellKeyDown(e, colName, String(cell))}
                        tabIndex={onFilterByValue ? 0 : undefined}
                      >
                        <span
                          className={onCellClick ? cellDrillableClass : ''}
                          onClick={() => onCellClick?.(colName, String(cell))}
                        >
                          {grainLabel ??
                            formatResultCell(cell, colName, {
                              question,
                              columnFormat: columns[colIdx]?.format,
                            })}
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

      <div className="text-foreground-muted flex items-center justify-between pt-[0.4rem] text-[0.75rem] max-[720px]:flex-wrap max-[720px]:gap-2">
        <span>
          {t('result_table.row_count', { count: rowCount })}
          {durationMs !== undefined ? ` · ${durationMs} ms` : ''}
        </span>
        {sortDir && (
          <span>
            {t('result_table.sorting', {
              column: columns[sortColIdx!]?.name ?? '',
              direction:
                sortDir === 'asc' ? t('result_table.sort_asc') : t('result_table.sort_desc'),
            })}
          </span>
        )}
        <button
          type="button"
          className={buttonClass('secondary', {
            size: 'sm',
            autoWidth: true,
            className: 'ml-auto! text-[0.78rem]',
          })}
          onClick={handleExport}
          disabled={rows.length === 0}
        >
          {t('result_table.export_csv')}
        </button>
      </div>
    </div>
  )
}
