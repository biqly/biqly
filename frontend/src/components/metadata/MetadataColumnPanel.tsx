import { useT } from '../../i18n'
import {
  metadataColNameBaseClass,
  metadataColNameCellClass,
  metadataColNameSuffixClass,
  metadataNestedCaptionClass,
  metadataNestedCellClass,
  metadataNestedColDescClass,
  metadataNestedColNameClass,
  metadataNestedColTypeClass,
  metadataNestedPanelClass,
  metadataNestedRowClass,
  resultsTableNestedClass,
} from '../../lib/tableClasses'
import type { ColumnRow, TableRow } from '../../types/semantic'
import { MetadataDescriptionCell } from './MetadataDescriptionCell'
import type { MetadataEditingState } from './utils'
import { columnKeySuffix } from './utils'

interface MetadataColumnPanelProps {
  table: TableRow
  columns: ColumnRow[]
  locale: string
  editing: MetadataEditingState | null
  onStartEdit: (column: ColumnRow) => void
  onEditChange: (columnId: string, value: string) => void
  onSave: () => void
  onCancelEdit: () => void
  onSaveDisplayExpression?: (tab: TableRow, expr: string) => Promise<boolean>
}

export function MetadataColumnPanel({
  table,
  columns,
  locale,
  editing,
  onStartEdit,
  onEditChange,
  onSave,
  onCancelEdit,
  onSaveDisplayExpression: _onSaveDisplayExpression,
}: MetadataColumnPanelProps) {
  const t = useT()

  return (
    <tr className={metadataNestedRowClass}>
      <td colSpan={4} className={metadataNestedCellClass}>
        <div className={metadataNestedPanelClass}>
          <table className={resultsTableNestedClass()} lang={locale}>
            <caption className={metadataNestedCaptionClass}>
              {t('metadata.nested_columns_caption', {
                fqn: `${table.schema_name}.${table.table_name}`,
              })}
            </caption>
            <colgroup>
              <col className={metadataNestedColNameClass} />
              <col className={metadataNestedColTypeClass} />
              <col className={metadataNestedColDescClass} />
            </colgroup>
            <thead>
              <tr>
                <th scope="col">{t('metadata.col_column_name')}</th>
                <th scope="col" className="metadata-col-type">
                  {t('metadata.col_data_type')}
                </th>
                <th scope="col">{t('metadata.col_column_desc')}</th>
              </tr>
            </thead>
            <tbody>
              {columns.map((c) => {
                const keySuffix = columnKeySuffix(c, t)
                const fkMultiline = !!(
                  c.is_foreign_key &&
                  c.referenced_table &&
                  c.referenced_column
                )
                return (
                  <tr key={c.id}>
                    <td className={metadataColNameCellClass}>
                      <span className={metadataColNameBaseClass}>{c.column_name}</span>
                      {keySuffix && (
                        <span className={metadataColNameSuffixClass(fkMultiline)}>
                          {fkMultiline ? `(${keySuffix})` : ` (${keySuffix})`}
                        </span>
                      )}
                    </td>
                    <td className="metadata-col-type">
                      {c.data_type}
                      {c.nullable ? '' : t('metadata.not_null_suffix')}
                    </td>
                    <MetadataDescriptionCell
                      kind="column"
                      entityId={c.id}
                      description={c.description}
                      editing={editing}
                      placeholder={t('metadata.placeholder_double_click')}
                      onStartEdit={() => onStartEdit(c)}
                      onChange={(value) => onEditChange(c.id, value)}
                      onSave={onSave}
                      onCancel={onCancelEdit}
                    />
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      </td>
    </tr>
  )
}
