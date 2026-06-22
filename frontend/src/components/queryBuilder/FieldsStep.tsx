import { useT } from '../../i18n'
import { cn } from '../../lib/cn'
import type { SemanticDimension, SemanticMetric } from '../../types/semantic'
import { Select } from '../ui/Select'
import { NotebookStep } from './NotebookStep'
import { qbAddBtnClass, qbTagBase, qbTagBlueClass, qbTagCloseClass } from './queryBuilderClasses'
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
        <div key={item.id} className={cn(qbTagBase, qbTagBlueClass, 'flex items-center gap-1')}>
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
            options={[
              ...(item.type === 'dimension'
                ? dimFieldOptions(dimensions)
                : metricFieldOptions(metrics)),
            ]}
            size="sm"
            searchable
          />
          <button
            type="button"
            className={qbTagCloseClass}
            onClick={() => removeSelectItem(i)}
            aria-label={t('query_builder.remove_field_aria', { n: i + 1 })}
          >
            ×
          </button>
        </div>
      ))}
      <button type="button" className={qbAddBtnClass} onClick={addSelectItem}>
        +
      </button>
    </NotebookStep>
  )
}
