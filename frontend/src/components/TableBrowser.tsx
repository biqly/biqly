import { useMemo } from 'react'

import { useT } from '../i18n'
import { modelListHint, modelListLabel } from '../types/semantic'
import {
  tableBrowserCardClass,
  tableBrowserToolbarClass,
  tableBrowserToolbarFieldClass,
  tableBrowserToolbarLabelClass,
} from './tableBrowser/tableBrowserClasses'
import { TableBrowserModelContent } from './tableBrowser/TableBrowserModelContent'
import { TableBrowserRowModal } from './tableBrowser/TableBrowserRowModal'
import { useTableBrowserPage } from './tableBrowser/useTableBrowserPage'
import { LoadingScreen } from './ui/LoadingScreen'
import { Select } from './ui/Select'

export default function TableBrowser() {
  const t = useT()
  const page = useTableBrowserPage()

  const modalFrame = useMemo(() => {
    const cols = page.result?.columns
    if (!page.detailRow || !cols) {
      return null
    }
    return {
      kind: 'row' as const,
      schema: page.selectedSchema,
      table: page.selectedTable,
      columns: cols.map((c) => c.name),
      row: page.detailRow.row,
    }
  }, [page.detailRow, page.result, page.selectedSchema, page.selectedTable])

  if (page.loading) {
    return <LoadingScreen minHeight="300px" />
  }

  return (
    <div className={tableBrowserCardClass}>
      <div className={tableBrowserToolbarClass}>
        <div className={tableBrowserToolbarFieldClass}>
          <label htmlFor="table-browser-datasource" className={tableBrowserToolbarLabelClass}>
            {t('saved_questions.label_select_datasource')}
          </label>
          <Select
            id="table-browser-datasource"
            value={page.datasourceId}
            onChange={page.setDatasourceId}
            options={page.datasources.map((d) => ({ value: d.id, label: d.name, hint: d.type }))}
          />
        </div>
        <div className={tableBrowserToolbarFieldClass}>
          <label htmlFor="table-browser-model" className={tableBrowserToolbarLabelClass}>
            {t('saved_questions.label_select_model')}
          </label>
          <Select
            id="table-browser-model"
            value={page.modelId}
            onChange={page.setModelId}
            disabled={!page.datasourceId || page.models.length === 0}
            placeholder={t('query_builder.placeholder_pick_model')}
            options={page.models.map((m) => ({
              value: m.id,
              label: modelListLabel(m),
              hint: modelListHint(m),
            }))}
          />
        </div>
        {page.modelDetail && page.tableOptions.length > 0 && (
          <div className={tableBrowserToolbarFieldClass}>
            <label htmlFor="table-browser-table" className={tableBrowserToolbarLabelClass}>
              {t('table_browser.label_select_table')}
            </label>
            <Select
              id="table-browser-table"
              value={page.selectedTableKey}
              onChange={page.setSelectedTableKey}
              options={page.tableOptions}
            />
          </div>
        )}
      </div>

      {page.modelDetail ? (
        <TableBrowserModelContent
          t={page.t}
          browserFields={page.browserFields}
          filters={page.filters}
          popoverOpen={page.popoverOpen}
          popoverAnchorEl={page.popoverAnchorEl}
          popoverField={page.popoverField}
          popoverOperator={page.popoverOperator}
          popoverChips={page.popoverChips}
          chipInputText={page.chipInputText}
          popoverCaseSensitive={page.popoverCaseSensitive}
          editingFilterId={page.editingFilterId}
          operatorLabels={page.operatorLabels}
          operatorOptions={page.operatorOptions}
          filterFieldOpts={page.filterFieldOpts}
          error={page.error}
          showTablePanel={page.showTablePanel}
          showInitialPlaceholder={page.showInitialPlaceholder}
          result={page.result}
          fetching={page.fetching}
          displayColumnNames={page.displayColumnNames}
          dragColumn={page.dragColumn}
          dropTargetColumn={page.dropTargetColumn}
          page={page.page}
          pageSize={page.pageSize}
          rangeStart={page.rangeStart}
          rangeEnd={page.rangeEnd}
          rowCount={page.rowCount}
          totalRows={page.totalRows}
          totalPages={page.totalPages}
          sort={page.sort}
          pageSizeOptions={page.pageSizeOptions}
          formatInt={page.formatInt}
          columnIndexByName={page.columnIndexByName}
          getDimensionLabel={page.getDimensionLabel}
          onOpenModeling={page.openModeling}
          onOpenEditFilter={page.handleOpenEditFilter}
          onRemoveFilter={page.handleRemoveFilter}
          onOpenAddFilter={page.handleOpenAddFilter}
          onClosePopover={page.handleCloseFilterPopover}
          onOperatorChange={page.setPopoverOperator}
          onFieldChange={page.setPopoverField}
          onChipInputChange={page.setChipInputText}
          onAddChip={page.handleAddChip}
          onRemoveChip={page.handleRemoveChip}
          onCaseSensitiveChange={page.setPopoverCaseSensitive}
          onSaveFilter={page.handleSaveFilter}
          onColumnDragStart={page.handleColumnDragStart}
          onColumnDragOver={page.handleColumnDragOver}
          onColumnDrop={page.handleColumnDrop}
          onColumnDragEnd={page.handleColumnDragEnd}
          onToggleSort={page.toggleSort}
          onRowClick={(rowIndex, row) =>
            page.setDetailRow({ displayIndex: page.page * page.pageSize + rowIndex + 1, row })
          }
          onPageSizeChange={(size) => {
            page.setPageSize(size)
            page.goToPage(0)
          }}
          onGoToPage={page.goToPage}
        />
      ) : (
        <p style={{ color: 'var(--text-muted)', fontSize: '0.85rem' }}>
          {t('table_browser.select_model')}
        </p>
      )}

      <TableBrowserRowModal
        open={modalFrame != null}
        onClose={() => page.setDetailRow(null)}
        datasourceId={page.datasourceId}
        joins={page.modelDetail?.joins ?? []}
        baseSchema={page.modelDetail?.base_schema ?? ''}
        displayExpressionByTable={page.displayExpressionByTable}
        initialFrame={modalFrame}
        fallbackTitle={
          page.detailRow
            ? t('table_browser.row_detail_title', {
              n: page.formatInt(page.detailRow.displayIndex),
            })
            : t('table_browser.row_detail')
        }
        postData={page.postData}
        t={page.t}
        formatInt={page.formatInt}
      />
    </div>
  )
}
