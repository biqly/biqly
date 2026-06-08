import '../styles/tableBrowser.css'

import { useT } from '../i18n'
import { modelListHint, modelListLabel } from '../types/semantic'
import { formatResultCell } from '../utils/resultCellFormat'
import { TableBrowserModelContent } from './tableBrowser/TableBrowserModelContent'
import { useTableBrowserPage } from './tableBrowser/useTableBrowserPage'
import { LoadingScreen } from './ui/LoadingScreen'
import { Modal } from './ui/Modal'
import { Select } from './ui/Select'

const PREFERRED_TITLE_COLUMN_PATTERNS = [
  /^(title|name|label|subject|headline|heading|display_name)$/,
  /^(text|body|content|message|description|caption|summary)$/,
  /(_|^)(title|name|label|subject)$/,
  /(_|^)(text|body|content|message|description)$/,
]

const ID_COLUMN_PATTERNS = [/^(id|uuid|pk)$/, /(_|^)id$/]

function singularize(name: string): string {
  const n = name.toLowerCase()
  if (n.endsWith('ies')) {
    return `${n.slice(0, -3)}y`
  }
  if (n.endsWith('s') && !n.endsWith('ss')) {
    return n.slice(0, -1)
  }
  return n
}

function buildRowModalTitle(
  row: unknown[],
  columns: string[],
  fallback: string,
  tableKeyValue?: string | null,
): string {
  const stringValues: { name: string; value: string }[] = []
  for (let i = 0; i < columns.length; i++) {
    const v = row[i]
    if (v == null) {
      continue
    }
    const s = typeof v === 'string' ? v : typeof v === 'number' ? String(v) : ''
    const trimmed = s.trim()
    if (!trimmed) {
      continue
    }
    const colName = columns[i]
    if (!colName) {
      continue
    }
    stringValues.push({ name: colName.toLowerCase(), value: trimmed })
  }
  if (stringValues.length === 0) {
    return fallback
  }

  const truncate = (s: string) => (s.length > 80 ? `${s.slice(0, 77).trimEnd()}…` : s)

  for (const pattern of PREFERRED_TITLE_COLUMN_PATTERNS) {
    const hit = stringValues.find((c) => pattern.test(c.name))
    if (hit) {
      return truncate(hit.value)
    }
  }

  if (tableKeyValue) {
    const lastSegment = tableKeyValue.split('.').pop() ?? tableKeyValue
    const singular = singularize(lastSegment)
    const pkHit = stringValues.find(
      (c) =>
        c.name === `${singular}_id` ||
        c.name === `${lastSegment.toLowerCase()}_id` ||
        c.name === 'id',
    )
    if (pkHit) {
      return truncate(`${pkHit.name} ${pkHit.value}`)
    }
  }

  for (const pattern of ID_COLUMN_PATTERNS) {
    const hit = stringValues.find((c) => pattern.test(c.name))
    if (hit) {
      return truncate(`${hit.name} ${hit.value}`)
    }
  }
  return fallback
}

export default function TableBrowser() {
  const t = useT()
  const page = useTableBrowserPage()

  if (page.loading) {
    return <LoadingScreen minHeight="300px" />
  }

  return (
    <div className="card card--table-browser">
      <div className="table-browser-toolbar">
        <div className="table-browser-toolbar-field">
          <label htmlFor="table-browser-datasource" className="table-browser-toolbar-label">
            {t('saved_questions.label_select_datasource')}
          </label>
          <Select
            id="table-browser-datasource"
            value={page.datasourceId}
            onChange={page.setDatasourceId}
            options={page.datasources.map((d) => ({ value: d.id, label: d.name, hint: d.type }))}
          />
        </div>
        <div className="table-browser-toolbar-field">
          <label htmlFor="table-browser-model" className="table-browser-toolbar-label">
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
          <div className="table-browser-toolbar-field">
            <label htmlFor="table-browser-table" className="table-browser-toolbar-label">
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
          modelDetail={page.modelDetail}
          datasourceId={page.datasourceId}
          activeDimensions={page.activeDimensions}
          filters={page.filters}
          popoverOpen={page.popoverOpen}
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
          pageList={page.pageList}
          lastPageIndex={page.lastPageIndex}
          hasNext={page.hasNext}
          pageSizeOptions={page.pageSizeOptions}
          formatInt={page.formatInt}
          columnIndexByName={page.columnIndexByName}
          getDimensionLabel={page.getDimensionLabel}
          onOpenModeling={page.openModeling}
          onOpenEditFilter={page.handleOpenEditFilter}
          onRemoveFilter={page.handleRemoveFilter}
          onOpenAddFilter={page.handleOpenAddFilter}
          onClosePopover={() => page.setPopoverOpen(false)}
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

      <Modal
        open={page.detailRow != null && page.result?.columns != null}
        title={
          page.detailRow && page.result?.columns
            ? buildRowModalTitle(
                page.detailRow.row,
                page.result.columns.map((c) => c.name),
                t('table_browser.row_detail_title', {
                  n: page.formatInt(page.detailRow.displayIndex),
                }),
                page.selectedTableKey || page.modelDetail?.base_table,
              )
            : t('table_browser.row_detail')
        }
        subtitle={page.selectedTableKey || page.modelDetail?.base_table}
        onClose={() => page.setDetailRow(null)}
        bodyClassName="table-browser-detail-modal-body"
      >
        {page.detailRow && page.result?.columns && (
          <div
            className="table-browser-detail-grid"
            role="region"
            aria-label={t('table_browser.row_detail')}
          >
            {page.displayColumnNames.map((colName) => {
              const j = page.columnIndexByName.get(colName)
              const display = formatResultCell(
                j != null ? page.detailRow!.row[j] : null,
                colName,
                {},
              )
              return (
                <div key={colName} className="table-browser-detail-item">
                  <span className="table-browser-detail-label">
                    {page.getDimensionLabel(colName)}
                  </span>
                  <span className="table-browser-detail-value">{display}</span>
                </div>
              )
            })}
          </div>
        )}
      </Modal>
    </div>
  )
}
