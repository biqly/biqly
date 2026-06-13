import { useState } from 'react'

import { useT } from '../../i18n'
import { legacyButtonClass } from '../../lib/buttonClasses'
import {
  metadataColNameBaseClass,
  metadataColNameCellClass,
  metadataColNameSuffixClass,
  metadataDisplayExprClass,
  metadataDisplayExprHintClass,
  metadataDisplayExprInputClass,
  metadataDisplayExprLabelClass,
  metadataDisplayExprRowClass,
  metadataDisplayExprSavedClass,
  metadataNestedCaptionClass,
  metadataNestedCellClass,
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
  onSaveDisplayExpression: (tab: TableRow, expr: string) => Promise<boolean>
}

/** Editor for the table's row display label (e.g. `author_name + " " + screen_name`). */
function DisplayExpressionEditor({
  table,
  onSave,
}: {
  table: TableRow
  onSave: (tab: TableRow, expr: string) => Promise<boolean>
}) {
  const t = useT()
  const original = table.display_expression ?? ''
  const [value, setValue] = useState(original)
  const [saving, setSaving] = useState(false)
  const [savedFlash, setSavedFlash] = useState(false)
  const inputId = `display-expr-${table.id}`
  const dirty = value.trim() !== original.trim()

  const save = async () => {
    setSaving(true)
    const ok = await onSave(table, value.trim())
    setSaving(false)
    if (ok) {
      setSavedFlash(true)
      window.setTimeout(() => setSavedFlash(false), 2000)
    }
  }

  return (
    <div className={metadataDisplayExprClass}>
      <label htmlFor={inputId} className={metadataDisplayExprLabelClass}>
        ✨ {t('metadata.display_expr_label')}
      </label>
      <div className={metadataDisplayExprRowClass}>
        <input
          id={inputId}
          type="text"
          className={metadataDisplayExprInputClass}
          value={value}
          onChange={(e) => setValue(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter' && dirty && !saving) {
              void save()
            }
          }}
          placeholder={t('metadata.display_expr_placeholder')}
          spellCheck={false}
        />
        <button
          type="button"
          className={legacyButtonClass('btn btn-sm')}
          disabled={!dirty || saving}
          onClick={() => {
            void save()
          }}
        >
          {saving ? t('common.saving') : t('common.save')}
        </button>
        {savedFlash && (
          <span className={metadataDisplayExprSavedClass} role="status">
            ✓ {t('metadata.display_expr_saved')}
          </span>
        )}
      </div>
      <small className={metadataDisplayExprHintClass}>{t('metadata.display_expr_hint')}</small>
    </div>
  )
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
  onSaveDisplayExpression,
}: MetadataColumnPanelProps) {
  const t = useT()

  return (
    <tr className={metadataNestedRowClass}>
      <td colSpan={4} className={metadataNestedCellClass}>
        <div className={metadataNestedPanelClass}>
          <DisplayExpressionEditor key={table.id} table={table} onSave={onSaveDisplayExpression} />
          <table className={resultsTableNestedClass()} lang={locale}>
            <caption className={metadataNestedCaptionClass}>
              {t('metadata.nested_columns_caption', {
                fqn: `${table.schema_name}.${table.table_name}`,
              })}
            </caption>
            <colgroup>
              <col className="metadata-ncol-name" />
              <col className="metadata-ncol-type" />
              <col className="metadata-ncol-desc" />
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
