import type { DragEvent } from 'react'

import type { useT } from '../../i18n'
import { buttonClass } from '../../lib/buttonClasses'
import { cn } from '../../lib/cn'
import { legacyFeedbackClass } from '../../lib/feedbackClasses'
import { legacyTableClass } from '../../lib/tableClasses'
import { formatResultCell } from '../../utils/resultCellFormat'
import { LoadingOverlay } from '../ui/LoadingOverlay'
import { PaginationControls } from '../ui/PaginationControls'
import { Select } from '../ui/Select'
import { TableBrowserCellValue } from './TableBrowserCellValue'
import {
  rowIndexNumberClass,
  tableBrowserAddFilterBtnClass,
  tableBrowserDataRowClass,
  tableBrowserDataRowTdClass,
  tableBrowserFilterBarClass,
  tableBrowserFilterTagClass,
  tableBrowserFilterTagCloseClass,
  tableBrowserIndexTdClass,
  tableBrowserIndexThClass,
  tableBrowserPageSizeClass,
  tableBrowserPageSizeLabelClass,
  tableBrowserPaginationClass,
  tableBrowserPaginationControlsClass,
  tableBrowserRangeClass,
  tableBrowserTableOverlayClass,
  tableBrowserTableShellClass,
  tableBrowserTableWrapClass,
  tableBrowserThClass,
  tableBrowserThFilterClass,
  tableBrowserThFilterIconClass,
  tableBrowserThGripClass,
  tableBrowserThInnerClass,
  tableBrowserThLabelClass,
  tableBrowserThSortClass,
  validationErrorBannerRowClass,
  validationErrorBannerTitleClass,
} from './tableBrowserClasses'
import type { TableBrowserFilter } from './tableBrowserFilterHandlers'
import { TableBrowserFilterPopover } from './TableBrowserFilterPopover'
import { formatTableBrowserFilterValue, tableBrowserOperatorLabel } from './tableBrowserFilterUtils'
import { buildTableBrowserRangeLabel } from './tableBrowserRangeLabel'
import type { TableRowsResult, TableSort } from './useTableBrowserQueryState'
function ValidationErrorBanner({
  error,
  t,
  onOpenModeling,
}: {
  error: string | null | undefined
  t: ReturnType<typeof useT>
  onOpenModeling: () => void
}) {
  if (!error) {
    return null
  }
  const isValidation = /validation failed/i.test(error)
  if (!isValidation) {
    return (
      <div className={legacyFeedbackClass('error')} role="alert">
        {error}
      </div>
    )
  }
  return (
    <div
      className={legacyFeedbackClass('error rounded-lg border border-red-500/20 bg-red-500/12 p-4')}
      role="alert"
    >
      <div className={validationErrorBannerRowClass}>
        <span className={validationErrorBannerTitleClass}>
          ⚠ {t('table_browser.validation_error_summary', { count: '1' })}
        </span>
        <button
          type="button"
          className={buttonClass('primary', { size: 'sm' })}
          onClick={onOpenModeling}
        >
          {t('table_browser.validation_error_open_modeling')}
        </button>
      </div>
    </div>
  )
}

export function TableBrowserModelContent({
  t,
  browserFields,
  filters,
  popoverOpen,
  popoverAnchorEl,
  popoverField,
  popoverOperator,
  popoverChips,
  chipInputText,
  popoverCaseSensitive,
  editingFilterId,
  operatorLabels,
  operatorOptions,
  filterFieldOpts,
  error,
  showTablePanel,
  showInitialPlaceholder,
  result,
  fetching,
  displayColumnNames,
  dragColumn,
  dropTargetColumn,
  page,
  pageSize,
  rangeStart,
  rangeEnd,
  rowCount,
  totalRows,
  totalPages,
  sort,
  pageSizeOptions,
  formatInt,
  columnIndexByName,
  getDimensionLabel,
  onOpenModeling,
  onOpenEditFilter,
  onRemoveFilter,
  onOpenAddFilter,
  onClosePopover,
  onOperatorChange,
  onFieldChange,
  onChipInputChange,
  onAddChip,
  onRemoveChip,
  onCaseSensitiveChange,
  onSaveFilter,
  onColumnDragStart,
  onColumnDragOver,
  onColumnDrop,
  onColumnDragEnd,
  onToggleSort,
  onRowClick,
  onPageSizeChange,
  onGoToPage,
}: {
  t: ReturnType<typeof useT>
  browserFields: { name: string }[]
  filters: TableBrowserFilter[]
  popoverOpen: boolean
  popoverAnchorEl: HTMLElement | null
  popoverField: string
  popoverOperator: string
  popoverChips: string[]
  chipInputText: string
  popoverCaseSensitive: boolean
  editingFilterId: string | null
  operatorLabels: Record<string, string>
  operatorOptions: { value: string; label: string }[]
  filterFieldOpts: { value: string; label: string }[]
  error: string | null
  showTablePanel: boolean
  showInitialPlaceholder: boolean
  result: TableRowsResult | null
  fetching: boolean
  displayColumnNames: string[]
  dragColumn: string | null
  dropTargetColumn: string | null
  page: number
  pageSize: number
  rangeStart: number
  rangeEnd: number
  rowCount: number
  totalRows: number | null
  totalPages: number | null
  sort: TableSort | null
  pageSizeOptions: { value: string; label: string }[]
  formatInt: (n: number) => string
  columnIndexByName: Map<string, number>
  getDimensionLabel: (name: string) => string
  onOpenModeling: () => void
  onOpenEditFilter: (filter: TableBrowserFilter, anchorEl?: HTMLElement | null) => void
  onRemoveFilter: (id: string) => void
  onOpenAddFilter: (defaultField?: string, anchorEl?: HTMLElement | null) => void
  onClosePopover: () => void
  onOperatorChange: (op: string) => void
  onFieldChange: (field: string) => void
  onChipInputChange: (text: string) => void
  onAddChip: (text: string) => void
  onRemoveChip: (index: number) => void
  onCaseSensitiveChange: (checked: boolean) => void
  onSaveFilter: () => void
  onColumnDragStart: (colName: string) => (e: DragEvent) => void
  onColumnDragOver: (colName: string) => (e: DragEvent) => void
  onColumnDrop: (colName: string) => (e: DragEvent) => void
  onColumnDragEnd: () => void
  onToggleSort: (colName: string) => void
  onRowClick: (rowIndex: number, row: unknown[]) => void
  onPageSizeChange: (size: number) => void
  onGoToPage: (page: number) => void
}) {
  const rangeLabel = buildTableBrowserRangeLabel(
    t,
    formatInt,
    rowCount,
    rangeStart,
    rangeEnd,
    totalRows,
  )

  if (browserFields.length === 0) {
    return (
      <p style={{ color: 'var(--text-muted)', fontSize: '0.85rem' }}>
        {t('table_browser.no_columns_for_table')}
      </p>
    )
  }

  return (
    <>
      <div className={tableBrowserFilterBarClass}>
        {filters.map((f) => (
          <span
            key={f.id}
            className={tableBrowserFilterTagClass}
            style={{ cursor: 'pointer' }}
            onClick={(e) => onOpenEditFilter(f, e.currentTarget)}
          >
            {getDimensionLabel(f.field)} {tableBrowserOperatorLabel(f.operator, operatorLabels)}{' '}
            {formatTableBrowserFilterValue(f.value)}
            <button
              type="button"
              className={tableBrowserFilterTagCloseClass}
              onClick={(e) => {
                e.stopPropagation()
                onRemoveFilter(f.id)
              }}
              aria-label={t('table_browser.remove_filter')}
            >
              ×
            </button>
          </span>
        ))}
        <button
          type="button"
          className={tableBrowserAddFilterBtnClass}
          onClick={(e) => onOpenAddFilter(undefined, e.currentTarget)}
          title={t('table_browser.add_filter')}
        >
          <svg
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="2.5"
            strokeLinecap="round"
            strokeLinejoin="round"
            style={{ width: '0.85rem', height: '0.85rem' }}
          >
            <line x1="12" y1="5" x2="12" y2="19" />
            <line x1="5" y1="12" x2="19" y2="12" />
          </svg>
          {t('table_browser.filter')}
        </button>
      </div>

      {popoverOpen && (
        <TableBrowserFilterPopover
          t={t}
          popoverField={popoverField}
          popoverOperator={popoverOperator}
          popoverChips={popoverChips}
          chipInputText={chipInputText}
          popoverCaseSensitive={popoverCaseSensitive}
          editingFilterId={editingFilterId}
          operatorOptions={operatorOptions}
          filterFieldOpts={filterFieldOpts}
          getDimensionLabel={getDimensionLabel}
          anchorEl={popoverAnchorEl}
          onClose={onClosePopover}
          onOperatorChange={onOperatorChange}
          onFieldChange={onFieldChange}
          onChipInputChange={onChipInputChange}
          onAddChip={onAddChip}
          onRemoveChip={onRemoveChip}
          onCaseSensitiveChange={onCaseSensitiveChange}
          onSave={onSaveFilter}
        />
      )}

      <ValidationErrorBanner error={error} t={t} onOpenModeling={onOpenModeling} />

      {showTablePanel && displayColumnNames.length > 0 && (
        <div className={tableBrowserTableShellClass}>
          <LoadingOverlay
            loading={fetching}
            label={
              showInitialPlaceholder ? t('table_browser.loading') : t('table_browser.loading_page')
            }
            className={tableBrowserTableOverlayClass}
          >
            <div
              className={cn(
                tableBrowserTableWrapClass,
                fetching && !showInitialPlaceholder && 'pointer-events-none opacity-55 blur-[2px]',
              )}
            >
              <table
                className={legacyTableClass(
                  'results-table mt-0! w-full border-collapse text-left text-sm max-[899px]:min-w-xl max-[680px]:min-w-lg',
                )}
              >
                <thead>
                  <tr>
                    <th scope="col" className={tableBrowserIndexThClass}></th>
                    {displayColumnNames.map((colName) => {
                      const sorted = sort?.column === colName ? sort.dir : null
                      return (
                        <th
                          key={colName}
                          scope="col"
                          draggable={!fetching}
                          aria-sort={
                            sorted ? (sorted === 'asc' ? 'ascending' : 'descending') : undefined
                          }
                          className={cn(
                            tableBrowserThClass,
                            dragColumn === colName && 'opacity-45',
                            dropTargetColumn === colName && 'shadow-[inset_0_-2px_0_var(--accent)]',
                          )}
                          onDragStart={onColumnDragStart(colName)}
                          onDragOver={onColumnDragOver(colName)}
                          onDrop={onColumnDrop(colName)}
                          onDragEnd={onColumnDragEnd}
                          onClick={() => !fetching && onToggleSort(colName)}
                          title={t('table_browser.sort_hint')}
                        >
                          <span className={tableBrowserThInnerClass}>
                            <span
                              className={tableBrowserThGripClass}
                              aria-hidden="true"
                              title={t('table_browser.drag_column')}
                              onClick={(e) => e.stopPropagation()}
                            >
                              ⋮⋮
                            </span>
                            <span className={tableBrowserThLabelClass}>
                              {getDimensionLabel(colName)}
                            </span>
                            {sorted && (
                              <span className={tableBrowserThSortClass} aria-hidden="true">
                                {sorted === 'asc' ? '↑' : '↓'}
                              </span>
                            )}
                            <button
                              type="button"
                              className={tableBrowserThFilterClass}
                              aria-label={t('table_browser.filter_column_aria', {
                                column: colName,
                              })}
                              title={t('table_browser.filter_by_column', { column: colName })}
                              onClick={(e) => {
                                e.stopPropagation()
                                if (!fetching) {
                                  onOpenAddFilter(colName, e.currentTarget)
                                }
                              }}
                            >
                              <svg
                                viewBox="0 0 16 16"
                                aria-hidden="true"
                                className={tableBrowserThFilterIconClass}
                                fill="none"
                                stroke="currentColor"
                                strokeWidth="1.5"
                                strokeLinecap="round"
                                strokeLinejoin="round"
                              >
                                <path d="M2 3h12L9 9v4l-2 1V9L2 3z" />
                              </svg>
                            </button>
                          </span>
                        </th>
                      )
                    })}
                  </tr>
                </thead>
                <tbody>
                  {(result?.rows ?? []).map((row, i) => (
                    <tr
                      key={i}
                      className={cn(tableBrowserDataRowClass, fetching && 'pointer-events-none')}
                      onClick={() => {
                        if (!fetching) {
                          onRowClick(i, row)
                        }
                      }}
                    >
                      <td className={tableBrowserIndexTdClass}>
                        <span className={rowIndexNumberClass}>{page * pageSize + i + 1}</span>
                      </td>
                      {displayColumnNames.map((colName) => {
                        const j = columnIndexByName.get(colName)
                        const cell = j != null ? row[j] : null
                        const display = formatResultCell(cell, colName, {})
                        return (
                          <td key={colName} className={tableBrowserDataRowTdClass}>
                            <TableBrowserCellValue value={display} />
                          </td>
                        )
                      })}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </LoadingOverlay>

          <div
            className={cn(
              tableBrowserPaginationClass,
              (fetching || showInitialPlaceholder) && 'opacity-72',
            )}
          >
            <span className={tableBrowserRangeClass}>
              {showInitialPlaceholder ? t('table_browser.loading') : rangeLabel}
            </span>
            <div className={tableBrowserPaginationControlsClass}>
              <div className={tableBrowserPageSizeClass}>
                <span className={tableBrowserPageSizeLabelClass}>
                  {t('table_browser.rows_per_page')}
                </span>
                <Select
                  value={String(pageSize)}
                  onChange={(v) => onPageSizeChange(Number(v))}
                  options={pageSizeOptions}
                  size="sm"
                  disabled={showInitialPlaceholder}
                />
              </div>
              <PaginationControls
                currentPage={page + 1}
                totalPages={totalPages ?? page + 1}
                onPageChange={(p) => onGoToPage(p - 1)}
                disabled={fetching || showInitialPlaceholder}
                formatNumber={formatInt}
              />
            </div>
          </div>
        </div>
      )}
    </>
  )
}
