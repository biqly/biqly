import { useT } from '../../i18n'
import { cn } from '../../lib/cn'
import type { SemanticDimension, SemanticMetric } from '../../types/semantic'
import { Select } from '../ui/Select'
import { NotebookStep } from './NotebookStep'
import {
  qbSummarizeAddClass,
  qbSummarizeDividerClass,
  qbSummarizeHeadingClass,
  qbSummarizeHintClass,
  qbSummarizeItemsClass,
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
      label={t('query_builder.summarize_label')}
      themeClass="summarize"
      onClose={onClear}
      closeTitle={t('common.cancel')}
      collapsible
      summary={t('query_builder.step_summary_count', {
        count: groupBy.length + selectItems.filter((i) => i.type === 'metric').length,
      })}
    >
      <div className={qbSummarizeSplitClass}>
        <section
          className={qbSummarizeSectionClass}
          aria-label={t('query_builder.aggregations_label')}
        >
          <header>
            <h3 className={qbSummarizeHeadingClass}>{t('query_builder.aggregations_label')}</h3>
            <p className={qbSummarizeHintClass}>{t('query_builder.aggregations_hint')}</p>
          </header>
          <div className={qbSummarizeItemsClass}>
            {selectItems
              .filter((item) => item.type === 'metric')
              .map((item, aggIdx) => {
                const i = selectItems.indexOf(item)
                return (
                  <div
                    key={item.id}
                    className={cn(qbTagBase, qbTagGreenClass, 'flex items-center gap-1')}
                  >
                    <Select
                      value={item.name}
                      onChange={(v) => updateSelectItem(i, 'name', v)}
                      placeholder={t('query_builder.pick_field_placeholder')}
                      options={metricFieldOptions(metrics)}
                      size="sm"
                      searchable
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
          </div>
          <button
            type="button"
            className={qbSummarizeAddClass}
            onClick={() => addMetricSelectItem('')}
          >
            <span aria-hidden="true">+</span>
            {t('query_builder.add_aggregation')}
          </button>
        </section>

        <div className={qbSummarizeDividerClass} aria-hidden="true">
          →
        </div>

        <section className={qbSummarizeSectionClass} aria-label={t('query_builder.group_by_label')}>
          <header>
            <h3 className={qbSummarizeHeadingClass}>{t('query_builder.group_by_label')}</h3>
            <p className={qbSummarizeHintClass}>{t('query_builder.group_by_hint')}</p>
          </header>
          <div className={qbSummarizeItemsClass}>
            {groupBy.map((g, i) => (
              <div key={i} className={cn(qbTagBase, qbTagBlueClass, 'flex items-center gap-1')}>
                <Select
                  value={g}
                  onChange={(v) => updateGroupByRow(i, v)}
                  placeholder={t('query_builder.pick_dimension_placeholder')}
                  options={dimOptionsForGroupRow(dimensions, groupBy, i)}
                  size="sm"
                  searchable
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
          </div>
          <button type="button" className={qbSummarizeAddClass} onClick={addGroupByRow}>
            <span aria-hidden="true">+</span>
            {t('query_builder.add_group_row')}
          </button>
        </section>
      </div>
    </NotebookStep>
  )
}
