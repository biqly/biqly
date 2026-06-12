import { useT } from '../../i18n'
import type { SemanticDimension, SemanticMetric } from '../../types/semantic'
import { Select } from '../ui/Select'
import { NotebookStep } from './NotebookStep'
import type { SelectItem } from './types'
import { type dimOptionsForGroupRow, getFieldLabel, type metricFieldOptions } from './utils'

interface SummarizeStepProps {
  selectItems: SelectItem[]
  groupBy: string[]
  dimensions: SemanticDimension[]
  metrics: SemanticMetric[]
  updateSelectItem: (i: number, field: keyof SelectItem, v: string) => void
  removeSelectItem: (i: number) => void
  addMetricSelectItem: (v: string) => void
  updateGroupByRow: (i: number, v: string) => void
  removeGroupByRow: (i: number) => void
  addGroupByRow: () => void
  onClear: () => void
  metricFieldOptions: (metrics: SemanticMetric[]) => ReturnType<typeof metricFieldOptions>
  dimOptionsForGroupRow: (
    dims: SemanticDimension[],
    groupBy: string[],
    i: number,
  ) => ReturnType<typeof dimOptionsForGroupRow>
  fieldLabelMode: 'technical' | 'human'
  setGroupBy: (v: string[]) => void
}

export function SummarizeStep({
  selectItems,
  groupBy,
  dimensions,
  metrics,
  updateSelectItem,
  removeSelectItem,
  addMetricSelectItem,
  updateGroupByRow,
  removeGroupByRow,
  addGroupByRow,
  onClear,
  metricFieldOptions,
  dimOptionsForGroupRow,
  fieldLabelMode,
  setGroupBy,
}: SummarizeStepProps) {
  const t = useT()
  return (
    <NotebookStep
      label="Summarize"
      themeClass="summarize"
      onClose={onClear}
      closeTitle={t('common.cancel')}
      collapsible
      summary={t('query_builder.step_summary_count', {
        count: groupBy.length + selectItems.filter((i) => i.type === 'metric').length,
      })}
    >
      <div className="notebook-summarize-split">
        {/* Aggregations */}
        <div className="notebook-summarize-section">
          {selectItems
            .filter((item) => item.type === 'metric')
            .map((item) => {
              const i = selectItems.indexOf(item)
              return (
                <div
                  key={item.id}
                  className="notebook-tag notebook-tag--green"
                  style={{ display: 'flex', alignItems: 'center', gap: '0.25rem' }}
                >
                  <Select
                    value={item.name}
                    onChange={(v) => updateSelectItem(i, 'name', v)}
                    placeholder={t('query_builder.pick_field_placeholder')}
                    options={metricFieldOptions(metrics)}
                    size="sm"
                  />
                  <button
                    type="button"
                    className="notebook-tag-close"
                    onClick={() => removeSelectItem(i)}
                    aria-label="Remove Aggregation"
                  >
                    ×
                  </button>
                </div>
              )
            })}
          <button
            type="button"
            className="notebook-add-btn"
            onClick={() => addMetricSelectItem('')}
          >
            +
          </button>
        </div>

        <div className="notebook-summarize-divider">by</div>

        {/* Group by dimensions */}
        <div className="notebook-summarize-section">
          {groupBy.map((g, i) => (
            <div
              key={i}
              className="notebook-tag notebook-tag--blue"
              style={{ display: 'flex', alignItems: 'center', gap: '0.25rem' }}
            >
              <Select
                value={g}
                onChange={(v) => updateGroupByRow(i, v)}
                placeholder={t('query_builder.pick_dimension_placeholder')}
                options={dimOptionsForGroupRow(dimensions, groupBy, i)}
                size="sm"
              />
              <button
                type="button"
                className="notebook-tag-close"
                onClick={() => removeGroupByRow(i)}
                aria-label="Remove Grouping"
              >
                ×
              </button>
            </div>
          ))}
          <button type="button" className="notebook-add-btn" onClick={addGroupByRow}>
            +
          </button>
        </div>
      </div>
      <div className="notebook-summarize-available">
        <div className="notebook-summarize-available__title">
          {t('query_builder.available_columns') || 'Available Columns (Dimensions)'}
        </div>
        <div className="notebook-summarize-available__list">
          {dimensions.map((d) => {
            const isSelected = groupBy.includes(d.name)
            return (
              <button
                key={d.name}
                type="button"
                className={`notebook-dimension-badge ${isSelected ? 'notebook-dimension-badge--active' : ''}`}
                onClick={() => {
                  if (isSelected) {
                    setGroupBy(groupBy.filter((g) => g !== d.name))
                  } else {
                    setGroupBy([...groupBy.filter(Boolean), d.name])
                  }
                }}
              >
                {getFieldLabel(d.name, d.label, fieldLabelMode)}
              </button>
            )
          })}
        </div>
      </div>
    </NotebookStep>
  )
}
