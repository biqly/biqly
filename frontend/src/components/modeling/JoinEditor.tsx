import type { TranslationKey } from '../../i18n'
import type { ColumnRow } from '../../types/semantic'
import { Select, type SelectOption } from '../ui/Select'
import type { JoinForm } from './types'
import { formatDataType } from './utils'

type Translate = (key: TranslationKey, vars?: Record<string, string | number>) => string

interface JoinEditorProps {
  open: boolean
  onToggle: () => void
  joinForm: JoinForm
  onChange: (patch: Partial<JoinForm>) => void
  tableOptions: SelectOption[]
  fromColumns: ColumnRow[]
  toColumns: ColumnRow[]
  fromColumnOptions: SelectOption[]
  toColumnOptions: SelectOption[]
  fromColumnValue: string
  toColumnValue: string
  selectedFromColumn: ColumnRow | null
  canSave: boolean
  saving: boolean
  loading: boolean
  onSave: () => void
  t: Translate
}

export function JoinEditor({
  open,
  onToggle,
  joinForm,
  onChange,
  tableOptions,
  fromColumns,
  toColumns,
  fromColumnOptions,
  toColumnOptions,
  fromColumnValue,
  toColumnValue,
  selectedFromColumn,
  canSave,
  saving,
  loading,
  onSave,
  t,
}: JoinEditorProps) {
  return (
    <aside
      className={`modeling-editor ${open ? '' : 'modeling-side--collapsed'}`}
      aria-label={t('modeling.relationship_editor_aria')}
    >
      <button
        type="button"
        className="modeling-side-toggle modeling-side-toggle--right"
        onClick={onToggle}
        title={open ? t('modeling.collapse_panel') : t('modeling.expand_panel')}
      >
        {open ? '›' : '‹'}
      </button>
      <div className="modeling-side-body">
        <div>
          <span className="modeling-kicker">{t('modeling.manual_relationship')}</span>
          <h2>{t('modeling.manual_title')}</h2>
          <p>{t('modeling.manual_desc')}</p>
        </div>
        <div className="form-group">
          <label>{t('modeling.source_table')}</label>
          <Select
            name="fromTable"
            value={joinForm.fromTable}
            onChange={(value) => onChange({ fromTable: value })}
            placeholder={t('modeling.table_placeholder')}
            header={t('modeling.source_table')}
            options={tableOptions}
          />
        </div>
        <div className="form-group">
          <label>{t('modeling.source_column')}</label>
          <Select
            name="fromColumn"
            value={fromColumnValue}
            onChange={(value) => onChange({ fromColumn: value })}
            placeholder={
              fromColumns.length === 0 ? t('modeling.no_columns') : t('modeling.column_placeholder')
            }
            header={t('modeling.source_column')}
            options={fromColumnOptions}
            disabled={!joinForm.fromTable || fromColumns.length === 0}
          />
        </div>
        <div className="form-group">
          <label>{t('modeling.target_table')}</label>
          <Select
            name="toTable"
            value={joinForm.toTable}
            onChange={(value) => onChange({ toTable: value })}
            placeholder={t('modeling.table_placeholder')}
            header={t('modeling.target_table')}
            options={tableOptions}
          />
        </div>
        <div className="form-group">
          <label>{t('modeling.target_column')}</label>
          <Select
            name="toColumn"
            value={toColumnValue}
            onChange={(value) => onChange({ toColumn: value })}
            placeholder={
              toColumns.length === 0
                ? t('modeling.no_compatible_columns')
                : t('modeling.column_placeholder')
            }
            header={t('modeling.target_column')}
            options={toColumnOptions}
            disabled={!joinForm.toTable || toColumns.length === 0}
          />
          {selectedFromColumn && (
            <small className="modeling-type-hint">
              {t('modeling.compatible_columns_hint', {
                type: formatDataType(t, selectedFromColumn.data_type),
              })}
            </small>
          )}
        </div>
        <div className="modeling-editor-grid">
          <div className="form-group">
            <label>{t('modeling.join_type_label')}</label>
            <Select
              value={joinForm.joinType}
              options={[
                { value: 'LEFT', label: 'LEFT' },
                { value: 'INNER', label: 'INNER' },
                { value: 'RIGHT', label: 'RIGHT' },
              ]}
              onChange={(v) => onChange({ joinType: v })}
            />
          </div>
          <div className="form-group">
            <label>{t('modeling.cardinality')}</label>
            <Select
              value={joinForm.relationship}
              options={[
                { value: 'many_to_one', label: 'many_to_one' },
                { value: 'one_to_many', label: 'one_to_many' },
                { value: 'one_to_one', label: 'one_to_one' },
                { value: 'many_to_many', label: 'many_to_many' },
              ]}
              onChange={(v) => onChange({ relationship: v })}
            />
          </div>
        </div>
        <button
          className="btn btn-primary"
          type="button"
          onClick={onSave}
          disabled={!canSave || saving || loading}
        >
          {saving ? t('common.saving') : t('modeling.add_relationship')}
        </button>
      </div>
    </aside>
  )
}
