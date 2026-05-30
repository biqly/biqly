import { Select } from '../ui/Select'
import { NotebookStep } from './NotebookStep'
import type { SelectItem } from './types'

interface SummarizeStepProps {
  selectItems: SelectItem[]
  groupBy: string[]
  dimensions: any[]
  metrics: any[]
  updateSelectItem: (i: number, field: keyof SelectItem, v: string) => void
  removeSelectItem: (i: number) => void
  addMetricSelectItem: (v: string) => void
  updateGroupByRow: (i: number, v: string) => void
  removeGroupByRow: (i: number) => void
  addGroupByRow: () => void
  onClear: () => void
  metricFieldOptions: (metrics: any[]) => any[]
  dimOptionsForGroupRow: (dims: any[], groupBy: string[], i: number) => any[]
  t: any
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
  t,
}: SummarizeStepProps) {
  return (
    <NotebookStep
      label="Summarize"
      themeClass="summarize"
      onClose={onClear}
      closeTitle={t('common.cancel')}
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
          <button type="button" className="notebook-add-btn" onClick={() => addMetricSelectItem('')}>
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
    </NotebookStep>
  )
}
