import type { DragEvent } from 'react'

import type { useT } from '../../i18n'
import type { SemanticDimension, SemanticModelDetail } from '../../types/semantic'
import { formatResultCell } from '../../utils/resultCellFormat'
import { LoadingOverlay } from '../ui/LoadingOverlay'
import { Select } from '../ui/Select'
import type { TableBrowserFilter } from './tableBrowserFilterHandlers'
import { TableBrowserFilterPopover } from './TableBrowserFilterPopover'
import { formatTableBrowserFilterValue, tableBrowserOperatorLabel } from './tableBrowserFilterUtils'
import { buildTableBrowserRangeLabel } from './tableBrowserRangeLabel'

interface QueryBuilderResult {
  columns?: { name: string; type?: string }[]
  rows?: unknown[][]
}

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
  modelDetail,
  datasourceId,
  activeDimensions,
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
  pageList,
  lastPageIndex,
  hasNext,
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
  onRowClick,
  onPageSizeChange,
  onGoToPage,
}: {
  t: ReturnType<typeof useT>
  modelDetail: SemanticModelDetail
  datasourceId: string
  activeDimensions: SemanticDimension[]
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
  result: QueryBuilderResult | null
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
  pageList: (number | 'gap')[] | null
  lastPageIndex: number | null
  hasNext: boolean
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

  if (activeDimensions.length === 0) {
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
                      {displayColumnNames.map((colName) => (
                        <th
                          key={colName}
                          scope="col"
                          draggable={!fetching}
                          className={`table-browser-th th-clickable${dragColumn === colName ? ' is-dragging' : ''}${dropTargetColumn === colName ? ' is-drop-target' : ''}`}
                          onDragStart={onColumnDragStart(colName)}
                          onDragOver={onColumnDragOver(colName)}
                          onDrop={onColumnDrop(colName)}
                          onDragEnd={onColumnDragEnd}
                          onClick={() => !fetching && onOpenAddFilter(colName)}
                          title={t('table_browser.filter_by_column', { column: colName })}
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
                            <span className="th-chevron">▼</span>
                          </span>
                        </th>
                      ))}
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
                            <td key={colName} title={display}>
                              {display}
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
                <nav
                  className="table-browser-page-nav"
                  aria-label={t('table_browser.pagination_nav')}
                >
                  <button
                    type="button"
                    className="table-browser-page-btn table-browser-page-btn--icon"
                    disabled={page === 0 || fetching}
                    onClick={() => onGoToPage(0)}
                    title={t('table_browser.first_page')}
                    aria-label={t('table_browser.first_page')}
                  >
                    «
                  </button>
                  <button
                    type="button"
                    className="table-browser-page-btn table-browser-page-btn--icon"
                    disabled={page === 0 || fetching}
                    onClick={() => onGoToPage(page - 1)}
                    title={t('table_browser.prev_page')}
                    aria-label={t('table_browser.prev_page')}
                  >
                    ‹
                  </button>
                  {pageList ? (
                    <div className="table-browser-page-list" role="list">
                      {pageList.map((token, idx) =>
                        token === 'gap' ? (
                          <span
                            key={`gap-${idx}`}
                            className="table-browser-page-gap"
                            aria-hidden="true"
                          >
                            …
                          </span>
                        ) : (
                          <button
                            key={token}
                            type="button"
                            role="listitem"
                            className={`table-browser-page-num-btn${token === page ? ' is-active' : ''}`}
                            disabled={fetching || token === page}
                            onClick={() => onGoToPage(token)}
                            aria-label={t('table_browser.go_to_page', { page: token + 1 })}
                            aria-current={token === page ? 'page' : undefined}
                          >
                            {formatInt(token + 1)}
                          </button>
                        ),
                      )}
                    </div>
                  ) : (
                    <span className="table-browser-page-num">
                      {t('table_browser.page_number', { page: formatInt(page + 1) })}
                    </span>
                  )}
                  <button
                    type="button"
                    className="table-browser-page-btn table-browser-page-btn--icon"
                    disabled={!hasNext || fetching}
                    onClick={() => onGoToPage(page + 1)}
                    title={t('table_browser.next_page')}
                    aria-label={t('table_browser.next_page')}
                  >
                    ›
                  </button>
                  <button
                    type="button"
                    className="table-browser-page-btn table-browser-page-btn--icon"
                    disabled={lastPageIndex == null || page >= lastPageIndex || fetching}
                    onClick={() => lastPageIndex != null && onGoToPage(lastPageIndex)}
                    title={t('table_browser.last_page')}
                    aria-label={t('table_browser.last_page')}
                  >
                    »
                  </button>
                </nav>
              </div>
            </div>
          ) : null}
        </>
      )}
    </>
  )
}
