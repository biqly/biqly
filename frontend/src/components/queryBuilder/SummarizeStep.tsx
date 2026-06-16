import { useT } from '../../i18n'
import type { SemanticDimension, SemanticMetric } from '../../types/semantic'
import { Select } from '../ui/Select'
import { NotebookStep } from './NotebookStep'
import {
  qbAddBtnClass,
  qbSummarizeDividerClass,
  qbSummarizeSectionClass,
  qbSummarizeSplitClass,
  qbTagBase,
  qbTagBlueClass,
  qbTagCloseClass,
  qbTagGreenClass,
} from './queryBuilderClasses'
import type { SelectItem } from './types'
import { type dimOptionsForGroupRow, type metricFieldOptions } from './utils'

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
      <div className={qbSummarizeSplitClass}>
        {/* Aggregations */}
        <div className={qbSummarizeSectionClass}>
          {selectItems
            .filter((item) => item.type === 'metric')
            .map((item, aggIdx) => {
              const i = selectItems.indexOf(item)
              return (
                <div
                  key={item.id}
                  className={`${qbTagBase} ${qbTagGreenClass} flex items-center gap-1`}
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
                    className={qbTagCloseClass}
                    onClick={() => removeSelectItem(i)}
                    aria-label={t('query_builder.remove_aggregation_aria', { n: aggIdx + 1 })}
                  >
                    ×
                  </button>
                </div>
              )
            })}
          <button type="button" className={qbAddBtnClass} onClick={() => addMetricSelectItem('')}>
            +
          </button>
        </div>

        <div className={qbSummarizeDividerClass}>by</div>

        {/* Group by dimensions */}
        <div className={qbSummarizeSectionClass}>
          {groupBy.map((g, i) => (
            <div key={i} className={`${qbTagBase} ${qbTagBlueClass} flex items-center gap-1`}>
              <Select
                value={g}
                onChange={(v) => updateGroupByRow(i, v)}
                placeholder={t('query_builder.pick_dimension_placeholder')}
                options={dimOptionsForGroupRow(dimensions, groupBy, i)}
                size="sm"
              />
              <button
                type="button"
                className={qbTagCloseClass}
                onClick={() => removeGroupByRow(i)}
                aria-label={t('query_builder.remove_group_aria', { n: i + 1 })}
              >
                ×
              </button>
            </div>
          ))}
          <button type="button" className={qbAddBtnClass} onClick={addGroupByRow}>
            +
          </button>
        </div>
      </div>
    </NotebookStep>
  )
}
