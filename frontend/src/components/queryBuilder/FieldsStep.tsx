import { Select } from '../ui/Select'
import { NotebookStep } from './NotebookStep'
import type { SelectItem } from './types'

interface FieldsStepProps {
  selectItems: SelectItem[]
  dimensions: any[]
  metrics: any[]
  updateSelectItem: (i: number, field: keyof SelectItem, v: string) => void
  removeSelectItem: (i: number) => void
  addSelectItem: () => void
  dimFieldOptions: (dims: any[]) => any[]
  metricFieldOptions: (metrics: any[]) => any[]
  t: any
}

export function FieldsStep({
  selectItems,
  dimensions,
  metrics,
  updateSelectItem,
  removeSelectItem,
  addSelectItem,
  dimFieldOptions,
  metricFieldOptions,
  t,
}: FieldsStepProps) {
  return (
    <NotebookStep label="Fields" themeClass="fields">
      {selectItems.map((item, i) => (
        <div
          key={item.id}
          className="notebook-tag notebook-tag--blue"
          style={{ display: 'flex', alignItems: 'center', gap: '0.25rem' }}
        >
          <Select
            value={item.type}
            onChange={(v) => updateSelectItem(i, 'type', v)}
            options={[
              { value: 'dimension', label: t('query_builder.dimension') },
              { value: 'metric', label: t('query_builder.metric') },
            ]}
            size="sm"
          />
          <Select
            value={item.name}
            onChange={(v) => updateSelectItem(i, 'name', v)}
            placeholder={t('query_builder.pick_field_placeholder')}
            disabled={item.type === 'dimension' ? dimensions.length === 0 : metrics.length === 0}
            options={item.type === 'dimension' ? dimFieldOptions(dimensions) : metricFieldOptions(metrics)}
            size="sm"
          />
          <button
            type="button"
            className="notebook-tag-close"
            onClick={() => removeSelectItem(i)}
            aria-label="Remove Field"
          >
            ×
          </button>
        </div>
      ))}
      <button type="button" className="notebook-add-btn" onClick={addSelectItem}>
        +
      </button>
    </NotebookStep>
  )
}
