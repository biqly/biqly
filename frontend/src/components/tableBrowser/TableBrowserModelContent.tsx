import type { DragEvent } from 'react'

import type { useT } from '../../i18n'
import { formatResultCell } from '../../utils/resultCellFormat'
import { LoadingOverlay } from '../ui/LoadingOverlay'
import { PaginationControls } from '../ui/PaginationControls'
import { Select } from '../ui/Select'
import { TableBrowserCellValue } from './TableBrowserCellValue'
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
      <div className="error" role="alert">
        {error}
      </div>
    )
  }
  return (
    <div className="error validation-error-banner" role="alert">
      <div className="validation-error-banner__row">
        <span className="validation-error-banner__title">
          ⚠ {t('table_browser.validation_error_summary', { count: '1' })}
        </span>
        <button type="button" className="btn btn-sm btn-primary" onClick={onOpenModeling}>
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
  onOpenEditFilter: (filter: TableBrowserFilter) => void
  onRemoveFilter: (id: string) => void
  onOpenAddFilter: (defaultField?: string) => void
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
      <div className="table-browser-filter-bar">
        {filters.map((f) => (
          <span
            key={f.id}
            className="table-browser-filter-tag"
            style={{ cursor: 'pointer' }}
            onClick={() => onOpenEditFilter(f)}
          >
            {getDimensionLabel(f.field)} {tableBrowserOperatorLabel(f.operator, operatorLabels)}{' '}
            {formatTableBrowserFilterValue(f.value)}
            <button
              type="button"
              className="table-browser-filter-tag-close"
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
          className="table-browser-add-filter-btn"
          onClick={() => onOpenAddFilter()}
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
      </div>

      <ValidationErrorBanner error={error} t={t} onOpenModeling={onOpenModeling} />

      {showTablePanel && (
        <>
          {showInitialPlaceholder ? (
            <div
              className="table-browser-table-placeholder"
              role="status"
              aria-live="polite"
              aria-busy="true"
            >
              <span className="loading-overlay-spinner" aria-hidden="true" />
              <span>{t('table_browser.loading')}</span>
            </div>
          ) : result?.columns ? (
            <LoadingOverlay
              loading={fetching}
              label={t('table_browser.loading_page')}
              className="table-browser-table-overlay"
            >
              <div className={`table-browser-table-wrap${fetching ? ' is-blurred' : ''}`}>
                <table className="results-table table-browser-grid">
                  <thead>
                    <tr>
                      <th scope="col" className="table-browser-col-index"></th>
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
                            className={`table-browser-th th-clickable${dragColumn === colName ? ' is-dragging' : ''}${dropTargetColumn === colName ? ' is-drop-target' : ''}`}
                            onDragStart={onColumnDragStart(colName)}
                            onDragOver={onColumnDragOver(colName)}
                            onDrop={onColumnDrop(colName)}
                            onDragEnd={onColumnDragEnd}
                            onClick={() => !fetching && onToggleSort(colName)}
                            title={t('table_browser.sort_hint')}
                          >
                            <span className="table-browser-th-inner">
                              <span
                                className="table-browser-th-grip"
                                aria-hidden="true"
                                title={t('table_browser.drag_column')}
                                onClick={(e) => e.stopPropagation()}
                              >
                                ⋮⋮
                              </span>
                              <span className="table-browser-th-label">
                                {getDimensionLabel(colName)}
                              </span>
                              {sorted && (
                                <span className="table-browser-th-sort" aria-hidden="true">
                                  {sorted === 'asc' ? '↑' : '↓'}
                                </span>
                              )}
                              <button
                                type="button"
                                className="table-browser-th-filter"
                                aria-label={t('table_browser.filter_column_aria', {
                                  column: colName,
                                })}
                                title={t('table_browser.filter_by_column', { column: colName })}
                                onClick={(e) => {
                                  e.stopPropagation()
                                  if (!fetching) {
                                    onOpenAddFilter(colName)
                                  }
                                }}
                              >
                                ▼
                              </button>
                            </span>
                          </th>
                        )
                      })}
                    </tr>
                  </thead>
                  <tbody>
                    {(result.rows ?? []).map((row, i) => (
                      <tr
                        key={i}
                        className={`table-browser-data-row${fetching ? ' is-disabled' : ''}`}
                        onClick={() => {
                          if (!fetching) {
                            onRowClick(i, row)
                          }
                        }}
                      >
                        <td className="table-browser-col-index">
                          <span className="row-index-number">{page * pageSize + i + 1}</span>
                        </td>
                        {displayColumnNames.map((colName) => {
                          const j = columnIndexByName.get(colName)
                          const cell = j != null ? row[j] : null
                          const display = formatResultCell(cell, colName, {})
                          return (
                            <td key={colName}>
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
          ) : null}

          {result?.columns?.length ? (
            <div className={`table-browser-pagination${fetching ? ' is-loading' : ''}`}>
              <span className="table-browser-range">{rangeLabel}</span>
              <div className="table-browser-pagination-controls">
                <div className="table-browser-page-size">
                  <span className="table-browser-page-size-label">
                    {t('table_browser.rows_per_page')}
                  </span>
                  <Select
                    value={String(pageSize)}
                    onChange={(v) => onPageSizeChange(Number(v))}
                    options={pageSizeOptions}
                    size="sm"
                  />
                </div>
                <PaginationControls
                  currentPage={page + 1}
                  totalPages={totalPages ?? page + 1}
                  onPageChange={(p) => onGoToPage(p - 1)}
                  disabled={fetching}
                  formatNumber={formatInt}
                />
              </div>
            </div>
          ) : null}
        </>
      )}
    </>
  )
}
