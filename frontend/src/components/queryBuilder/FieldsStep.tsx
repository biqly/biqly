import { useT } from '../../i18n'
import type { SemanticDimension, SemanticMetric } from '../../types/semantic'
import { Select } from '../ui/Select'
import { NotebookStep } from './NotebookStep'
import type { SelectItem } from './types'
import type { dimFieldOptions, metricFieldOptions } from './utils'

type FieldSelectOption = ReturnType<typeof dimFieldOptions>[number]

interface FieldsStepProps {
  selectItems: SelectItem[]
  dimensions: SemanticDimension[]
  metrics: SemanticMetric[]
  updateSelectItem: (i: number, field: keyof SelectItem, v: string) => void
  removeSelectItem: (i: number) => void
  addSelectItem: () => void
  dimFieldOptions: (dims: SemanticDimension[]) => FieldSelectOption[]
  metricFieldOptions: (metrics: SemanticMetric[]) => ReturnType<typeof metricFieldOptions>
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
}: FieldsStepProps) {
  const t = useT()
  return (
    <NotebookStep label="Dimensions" themeClass="fields">
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
            options={
              item.type === 'dimension' ? dimFieldOptions(dimensions) : metricFieldOptions(metrics)
            }
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
