import type { TFunction } from '../../i18n'
import { Select } from '../ui/Select'
import { NotebookStep } from './NotebookStep'
import type { FilterRow } from './types'

interface FilterStepProps {
  filters: FilterRow[]
  filterFieldOpts: { value: string; label: string; hint: string }[]
  updateFilter: (i: number, field: keyof FilterRow, v: string) => void
  removeFilter: (i: number) => void
  addFilter: () => void
  onClear: () => void
  t: TFunction
}

export function FilterStep({
  filters,
  filterFieldOpts,
  updateFilter,
  removeFilter,
  addFilter,
  onClear,
  t,
}: FilterStepProps) {
  if (filters.length === 0) {
    return null
  }

  return (
    <NotebookStep
      label="Filter"
      themeClass="filter"
      onClose={onClear}
      closeTitle={t('common.cancel')}
      collapsible
      summary={t('query_builder.step_summary_count', { count: filters.length })}
    >
      {filters.map((f, i) => {
        const selectedOpt = filterFieldOpts.find((opt) => opt.value === f.field)
        const isTextField = selectedOpt?.hint === 'text'
        return (
          <div
            key={f.id}
            className="notebook-tag notebook-tag--purple"
            style={{ display: 'flex', alignItems: 'center', gap: '0.25rem' }}
          >
            <Select
              value={f.field}
              onChange={(v) => updateFilter(i, 'field', v)}
              placeholder={t('query_builder.pick_field_placeholder')}
              disabled={filterFieldOpts.length === 0}
              options={filterFieldOpts}
              size="sm"
            />
            <Select
              value={f.operator}
              onChange={(v) => updateFilter(i, 'operator', v)}
              options={[
                { value: 'eq', label: '=' },
                { value: 'neq', label: '!=' },
                { value: 'gt', label: '>' },
                { value: 'gte', label: '>=' },
                { value: 'lt', label: '<' },
                { value: 'lte', label: '<=' },
                { value: 'contains', label: t('query_builder.op_contains') },
                { value: 'in', label: t('query_builder.op_in') },
                { value: 'between', label: t('query_builder.op_between') },
                { value: 'is_null', label: t('query_builder.op_is_null') || 'is null' },
                { value: 'is_not_null', label: t('query_builder.op_is_not_null') || 'is not null' },
                ...(isTextField
                  ? [
                      { value: 'is_empty', label: t('query_builder.op_is_empty') || 'is empty' },
                      {
                        value: 'is_not_empty',
                        label: t('query_builder.op_is_not_empty') || 'is not empty',
                      },
                    ]
                  : []),
              ]}
              size="sm"
            />
            {!['is_null', 'is_not_null', 'is_empty', 'is_not_empty'].includes(f.operator) && (
              <input
                value={f.value}
                onChange={(e) => updateFilter(i, 'value', e.target.value)}
                placeholder={t('query_builder.value_placeholder')}
                autoComplete="off"
                style={{ width: '7rem' }}
              />
            )}
            <button
              type="button"
              className="notebook-tag-close"
              onClick={() => removeFilter(i)}
              aria-label="Remove Filter"
            >
              ×
            </button>
          </div>
        )
      })}
      <button type="button" className="notebook-add-btn" onClick={addFilter}>
        +
      </button>
    </NotebookStep>
  )
}
