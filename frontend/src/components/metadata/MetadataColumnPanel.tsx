import { useT } from '../../i18n'
import type { ColumnRow, TableRow } from '../../types/semantic'
import { MetadataDescriptionCell } from './MetadataDescriptionCell'
import { columnKeySuffix } from './utils'
import type { MetadataEditingState } from './utils'

interface MetadataColumnPanelProps {
  table: TableRow
  columns: ColumnRow[]
  locale: string
  editing: MetadataEditingState | null
  onStartEdit: (column: ColumnRow) => void
  onEditChange: (columnId: string, value: string) => void
  onSave: () => void
  onCancelEdit: () => void
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
}: MetadataColumnPanelProps) {
  const t = useT()

  return (
    <tr className="metadata-nested-row">
      <td colSpan={4} className="metadata-nested-cell">
        <div className="metadata-nested-panel">
          <table className="results-table results-table--metadata-list results-table--nested" lang={locale}>
            <caption className="metadata-nested-caption">
              {t('metadata.nested_columns_caption', { fqn: `${table.schema_name}.${table.table_name}` })}
            </caption>
            <colgroup>
              <col className="metadata-ncol-name" />
              <col className="metadata-ncol-type" />
              <col className="metadata-ncol-desc" />
            </colgroup>
            <thead>
              <tr>
                <th scope="col">{t('metadata.col_column_name')}</th>
                <th scope="col" className="metadata-col-type">{t('metadata.col_data_type')}</th>
                <th scope="col">{t('metadata.col_column_desc')}</th>
              </tr>
            </thead>
            <tbody>
              {columns.map((c) => {
                const keySuffix = columnKeySuffix(c, t)
                const fkMultiline = !!(c.is_foreign_key && c.referenced_table && c.referenced_column)
                return (
                  <tr key={c.id}>
                    <td className="metadata-col-name-cell">
                      <span className="metadata-col-name-base">{c.column_name}</span>
                      {keySuffix && (
                        <span
                          className={
                            fkMultiline
                              ? 'metadata-col-name-suffix metadata-col-name-suffix--multiline'
                              : 'metadata-col-name-suffix'
                          }
                        >
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
