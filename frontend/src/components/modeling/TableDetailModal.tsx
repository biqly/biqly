import { useCallback, useState } from 'react'

import type { TranslationKey } from '../../i18n'
import { buttonClass } from '../../lib/buttonClasses'
import {
  modelingDetailAliasClass,
  modelingDetailDescriptionClass,
  modelingDetailFieldNameClass,
  modelingDetailHeaderClass,
  modelingDetailInfoCellClass,
  modelingDetailInfoNameCellClass,
  modelingDetailInfoTableClass,
  modelingDetailInfoTableWrapClass,
  modelingDetailInfoTypeCellClass,
  modelingDetailMutedClass,
  modelingDetailPreviewHeaderClass,
  modelingDetailRootClass,
  modelingDetailTableCellClass,
  modelingDetailTableClass,
  modelingDetailTableHeaderClass,
  modelingDetailTableWrapClass,
  modelingDetailTitleClass,
  modelingTypeIconClass,
} from '../../lib/modelingClasses'
import type { ColumnRow, SemanticJoin, SemanticModelDetail, TableRow } from '../../types/semantic'
import { unknownToDisplayString } from '../../utils/formatters'
import {
  buildTableRowsUrl,
  tableRowsBody,
  type TableRowsResult,
} from '../tableBrowser/useTableBrowserQueryState'
import { Modal } from '../ui/Modal'
import { columnTypeIcon } from './columnTypeIcon'
import { columnOptions, relationshipLabel, tableKey } from './utils'

interface TableDetailModalProps {
  open: boolean
  table: TableRow | null
  model: SemanticModelDetail | null
  columns: ColumnRow[]
  datasourceId: string
  postData: <T>(url: string, body: unknown) => Promise<T | null>
  onClose: () => void
  onEdit: (table: TableRow) => void
  t: (key: TranslationKey, vars?: Record<string, string | number>) => string
}

const PREVIEW_LIMIT = 100

export function TableDetailModal({
  open,
  table,
  model,
  columns,
  datasourceId,
  postData,
  onClose,
  onEdit,
  t,
}: TableDetailModalProps) {
  const tableScope = table ? tableKey(table.schema_name, table.table_name) : ''
  const [previewState, setPreviewState] = useState<{
    key: string
    result: TableRowsResult | null
  }>({ key: '', result: null })
  const [previewingKey, setPreviewingKey] = useState('')
  const preview = previewState.key === tableScope ? previewState.result : null
  const previewing = previewingKey === tableScope

  const loadPreview = useCallback(async () => {
    if (!table) {
      return
    }
    const key = tableKey(table.schema_name, table.table_name)
    setPreviewingKey(key)
    try {
      const result = await postData<TableRowsResult>(
        buildTableRowsUrl(datasourceId, table.schema_name, table.table_name),
        tableRowsBody([], null, PREVIEW_LIMIT, 0),
      )
      setPreviewState({ key, result })
    } finally {
      setPreviewingKey('')
    }
  }, [datasourceId, postData, table])

  if (!open || !table) {
    return null
  }

  const key = tableKey(table.schema_name, table.table_name)
  const tableColumns = columnOptions(columns, key)
  const relationships: SemanticJoin[] = (model?.joins ?? []).filter((join) => {
    const from = tableKey(join.from_schema ?? model?.base_schema ?? '', join.from_table)
    const to = tableKey(join.to_schema ?? model?.base_schema ?? '', join.to_table)
    return from === key || to === key
  })
  const previewColumns = preview?.columns ?? []
  const previewRows = preview?.rows ?? []

  return (
    <Modal
      open={open}
      title={table.label ?? table.table_name}
      subtitle={`${table.schema_name}.${table.table_name}`}
      onClose={onClose}
      className="w-[min(100%,60rem)]"
    >
      <div className={modelingDetailRootClass}>
        <div className={modelingDetailHeaderClass}>
          <div className={modelingDetailAliasClass}>
            <span className={modelingDetailMutedClass}>{t('modeling.detail_alias')}</span>
            <span>{table.label ?? table.table_name}</span>
          </div>
          <button
            type="button"
            className={buttonClass('secondary', { size: 'sm', autoWidth: true })}
            onClick={() => onEdit(table)}
          >
            {t('modeling.detail_edit')}
          </button>
        </div>
        {table.description ? (
          <p className={modelingDetailDescriptionClass}>{table.description}</p>
        ) : null}

        <section>
          <h3 className={modelingDetailTitleClass}>
            {t('modeling.detail_columns')} ({tableColumns.length})
          </h3>
          <div className={modelingDetailInfoTableWrapClass}>
            <table className={modelingDetailInfoTableClass}>
              <thead>
                <tr>
                  <th className={modelingDetailTableHeaderClass}>
                    {t('modeling.detail_col_name')}
                  </th>
                  <th className={modelingDetailTableHeaderClass}>
                    {t('modeling.detail_col_type')}
                  </th>
                  <th className={modelingDetailTableHeaderClass}>
                    {t('modeling.detail_col_description')}
                  </th>
                </tr>
              </thead>
              <tbody>
                {tableColumns.map((column) => {
                  const icon = columnTypeIcon(column.data_type)
                  return (
                    <tr key={column.id}>
                      <td className={modelingDetailInfoNameCellClass}>{column.column_name}</td>
                      <td className={modelingDetailInfoTypeCellClass}>
                        <span className="inline-flex items-center gap-1.5">
                          <span className={modelingTypeIconClass} aria-hidden="true">
                            {icon.kind === 'timestamp' ? (
                              <svg
                                xmlns="http://www.w3.org/2000/svg"
                                viewBox="0 0 12 12"
                                fill="none"
                                stroke="currentColor"
                                strokeWidth="1.3"
                                strokeLinecap="round"
                                strokeLinejoin="round"
                                className="size-[0.65rem]"
                              >
                                <circle cx="6" cy="6" r="5" />
                                <polyline points="6,2.5 6,6 8.5,7.5" />
                              </svg>
                            ) : (
                              icon.glyph
                            )}
                          </span>
                          {column.data_type}
                        </span>
                      </td>
                      <td className={modelingDetailInfoCellClass}>{column.description}</td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        </section>

        {relationships.length > 0 ? (
          <section>
            <h3 className={modelingDetailTitleClass}>
              {t('modeling.detail_relationships')} ({relationships.length})
            </h3>
            <div className={modelingDetailInfoTableWrapClass}>
              <table className={modelingDetailInfoTableClass}>
                <thead>
                  <tr>
                    <th className={modelingDetailTableHeaderClass}>
                      {t('modeling.detail_rel_name')}
                    </th>
                    <th className={modelingDetailTableHeaderClass}>
                      {t('modeling.detail_rel_from')}
                    </th>
                    <th className={modelingDetailTableHeaderClass}>
                      {t('modeling.detail_rel_to')}
                    </th>
                    <th className={modelingDetailTableHeaderClass}>
                      {t('modeling.detail_rel_type')}
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {relationships.map((relationship) => (
                    <tr key={relationship.id}>
                      <td className={modelingDetailInfoNameCellClass}>{relationship.name}</td>
                      <td className={modelingDetailInfoNameCellClass}>{relationship.from_table}</td>
                      <td className={modelingDetailInfoNameCellClass}>{relationship.to_table}</td>
                      <td className={modelingDetailInfoTypeCellClass}>
                        {relationshipLabel(t, relationship.relationship)}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </section>
        ) : null}

        <section>
          <div className={modelingDetailPreviewHeaderClass}>
            <h3 className={modelingDetailFieldNameClass}>{t('modeling.detail_preview')}</h3>
            <button
              type="button"
              className={buttonClass('secondary', { size: 'sm', autoWidth: true })}
              onClick={() => void loadPreview()}
              disabled={previewing}
            >
              {previewing ? t('modeling.detail_preview_loading') : t('modeling.detail_preview_btn')}
            </button>
          </div>
          {previewRows.length > 0 ? (
            <div className={modelingDetailTableWrapClass}>
              <table className={modelingDetailTableClass}>
                <thead>
                  <tr>
                    {previewColumns.map((column) => (
                      <th key={column.name} className={modelingDetailTableHeaderClass}>
                        {column.name}
                      </th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {previewRows.map((row, rowIndex) => (
                    <tr key={rowIndex}>
                      {row.map((cell, cellIndex) => (
                        <td key={cellIndex} className={modelingDetailTableCellClass}>
                          {unknownToDisplayString(cell)}
                        </td>
                      ))}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ) : null}
        </section>
      </div>
    </Modal>
  )
}
