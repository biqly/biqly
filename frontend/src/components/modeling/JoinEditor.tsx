import type { TranslationKey } from '../../i18n'
import { legacyButtonClass } from '../../lib/buttonClasses'
import { legacyCardClass } from '../../lib/cardClasses'
import { cn } from '../../lib/cn'
import { modelingFormGroupClass } from '../../lib/formClasses'
import {
  modelingEditorClass,
  modelingEditorSideBodyClass,
  modelingKickerClass,
  modelingSideToggleClass,
  modelingTypeHintClass,
} from '../../lib/modelingClasses'
import type { ColumnRow } from '../../types/semantic'
import { joinTypeHintKey } from '../ui/joinType'
import { JoinTypeIcon } from '../ui/JoinTypeIcon'
import { Select, type SelectOption } from '../ui/Select'
import type { JoinForm } from './types'
import { formatDataType } from './utils'
const JOIN_TYPES = ['LEFT', 'INNER', 'RIGHT'] as const

const CARDINALITY_OPTIONS = [
  { value: 'many_to_one', label: 'N:1 · many_to_one' },
  { value: 'one_to_many', label: '1:N · one_to_many' },
  { value: 'one_to_one', label: '1:1 · one_to_one' },
  { value: 'many_to_many', label: 'N:N · many_to_many' },
]

function shortTableName(qualified: string): string {
  const idx = qualified.lastIndexOf('.')
  return idx === -1 ? qualified : qualified.slice(idx + 1)
}

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
  const previewHintKey = joinTypeHintKey(joinForm.joinType)
  return (
    <aside
      className={modelingEditorClass(open)}
      aria-label={t('modeling.relationship_editor_aria')}
    >
      <button
        type="button"
        className={modelingSideToggleClass('right')}
        onClick={onToggle}
        title={open ? t('modeling.collapse_panel') : t('modeling.expand_panel')}
      >
        {open ? '›' : '‹'}
      </button>
      <div className={modelingEditorSideBodyClass}>
        <div>
          <span className={modelingKickerClass}>{t('modeling.manual_relationship')}</span>
          <h2>{t('modeling.manual_title')}</h2>
          <p>{t('modeling.manual_desc')}</p>
        </div>
        <div className={modelingFormGroupClass}>
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
        <div className={modelingFormGroupClass}>
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
        <div className={modelingFormGroupClass}>
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
        <div className={modelingFormGroupClass}>
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
            <small className={modelingTypeHintClass}>
              {t('modeling.compatible_columns_hint', {
                type: formatDataType(t, selectedFromColumn.data_type),
              })}
            </small>
          )}
        </div>
        <div className={modelingFormGroupClass}>
          <label id="join-type-label">{t('modeling.join_type_label')}</label>
          <div className="flex gap-[0.35rem]" role="radiogroup" aria-labelledby="join-type-label">
            {JOIN_TYPES.map((jt) => {
              const hintKey = joinTypeHintKey(jt)
              const isActive = joinForm.joinType === jt
              return (
                <button
                  key={jt}
                  type="button"
                  role="radio"
                  aria-checked={isActive}
                  className={cn(
                    legacyCardClass(
                      'bg-card-raised focus-visible:outline-accent inline-flex flex-1 cursor-pointer items-center justify-center gap-[0.35rem] rounded-lg border px-2 py-[0.45rem] text-[0.74rem] font-semibold transition-[border-color,color,background] duration-120 ease-in-out focus-visible:outline-2 focus-visible:outline-offset-2',
                    ),
                    isActive
                      ? 'border-accent text-accent bg-[color-mix(in_srgb,var(--accent)_10%,transparent)]'
                      : 'border-border text-foreground-muted hover:border-accent hover:text-foreground',
                  )}
                  title={hintKey ? t(hintKey) : undefined}
                  onClick={() => onChange({ joinType: jt })}
                >
                  <JoinTypeIcon type={jt} />
                  <span>{jt}</span>
                </button>
              )
            })}
          </div>
        </div>
        <div className={modelingFormGroupClass}>
          <label>{t('modeling.cardinality')}</label>
          <Select
            value={joinForm.relationship}
            options={CARDINALITY_OPTIONS}
            onChange={(v) => onChange({ relationship: v as JoinForm['relationship'] })}
          />
        </div>
        {joinForm.fromTable && joinForm.toTable && (
          <div
            className="border-border-strong grid gap-[0.45rem] rounded-lg border border-dashed bg-[color-mix(in_srgb,var(--accent)_4%,transparent)] px-3 py-[0.65rem]"
            aria-live="polite"
          >
            <div className="flex flex-wrap items-center justify-center gap-2">
              <span
                className={legacyCardClass(
                  'text-foreground bg-card-raised border-border max-w-36 overflow-hidden rounded-[0.35rem] border px-2 py-[0.2rem] font-mono text-[0.76rem] text-ellipsis whitespace-nowrap',
                )}
              >
                {shortTableName(joinForm.fromTable)}
              </span>
              <span
                className="text-accent inline-flex"
                title={previewHintKey ? t(previewHintKey) : undefined}
              >
                <JoinTypeIcon type={joinForm.joinType} size={22} />
              </span>
              <span
                className={legacyCardClass(
                  'text-foreground bg-card-raised border-border max-w-36 overflow-hidden rounded-[0.35rem] border px-2 py-[0.2rem] font-mono text-[0.76rem] text-ellipsis whitespace-nowrap',
                )}
              >
                {shortTableName(joinForm.toTable)}
              </span>
            </div>
            {fromColumnValue && toColumnValue && (
              <code className="text-foreground-muted block text-center text-[0.7rem] wrap-anywhere">
                ON {shortTableName(joinForm.fromTable)}.{fromColumnValue} ={' '}
                {shortTableName(joinForm.toTable)}.{toColumnValue}
              </code>
            )}
          </div>
        )}
        <button
          className={legacyButtonClass('btn btn-primary')}
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
