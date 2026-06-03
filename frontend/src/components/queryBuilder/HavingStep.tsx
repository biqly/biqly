import { Select } from '../ui/Select'
import { NotebookStep } from './NotebookStep'
import type { HavingRow } from './types'

interface HavingStepProps {
  having: HavingRow[]
  metricOptsHaving: { value: string; label: string }[]
  updateHaving: (i: number, field: keyof HavingRow, v: string) => void
  removeHaving: (i: number) => void
  addHaving: () => void
  onClear: () => void
  t: any
}

export function HavingStep({
  having,
  metricOptsHaving,
  updateHaving,
  removeHaving,
  addHaving,
  onClear,
  t,
}: HavingStepProps) {
  if (having.length === 0) {
    return null
  }

  return (
    <NotebookStep
      label="Having"
      themeClass="advanced"
      onClose={onClear}
      closeTitle={t('common.cancel')}
      collapsible
      defaultCollapsed
      summary={t('query_builder.step_summary_count', { count: having.length })}
    >
      {having.map((h, i) => (
        <div
          key={i}
          className="notebook-tag notebook-tag--purple"
          style={{ display: 'flex', alignItems: 'center', gap: '0.25rem' }}
        >
          <Select
            value={h.field}
            onChange={(v) => updateHaving(i, 'field', v)}
            placeholder={t('query_builder.pick_metric_having')}
            options={metricOptsHaving}
            size="sm"
          />
          <Select
            value={h.operator}
            onChange={(v) => updateHaving(i, 'operator', v)}
            options={[
              { value: 'gt', label: '>' },
              { value: 'gte', label: '>=' },
              { value: 'lt', label: '<' },
              { value: 'lte', label: '<=' },
              { value: 'eq', label: '=' },
              { value: 'neq', label: '!=' },
            ]}
            size="sm"
          />
          <input
            value={h.value}
            onChange={(e) => updateHaving(i, 'value', e.target.value)}
            placeholder={t('query_builder.value_placeholder')}
            autoComplete="off"
            style={{ width: '6rem' }}
          />
          <button
            type="button"
            className="notebook-tag-close"
            onClick={() => removeHaving(i)}
            aria-label="Remove Having Constraint"
          >
            ×
          </button>
        </div>
      ))}
      <button type="button" className="notebook-add-btn" onClick={addHaving}>
        +
      </button>
    </NotebookStep>
  )
}
