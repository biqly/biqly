import { useT } from '../../i18n'
import { Select } from '../ui/Select'
import { NotebookStep } from './NotebookStep'
import { qbAddBtnClass, qbTagBase, qbTagCloseClass, qbTagPurpleClass } from './queryBuilderClasses'
import type { WindowFuncRow } from './types'
import { WINDOW_FUNC_OPTIONS } from './types'

interface WindowFuncStepProps {
  windowFunctions: WindowFuncRow[]
  updateWindowFunc: (i: number, field: keyof WindowFuncRow, v: string) => void
  removeWindowFunc: (i: number) => void
  addWindowFunc: () => void
  onClear: () => void
}

export function WindowFuncStep({
  windowFunctions,
  updateWindowFunc,
  removeWindowFunc,
  addWindowFunc,
  onClear,
}: WindowFuncStepProps) {
  const t = useT()
  if (windowFunctions.length === 0) {
    return null
  }

  return (
    <NotebookStep
      label="Window Func"
      themeClass="advanced"
      onClose={onClear}
      closeTitle={t('common.cancel')}
      collapsible
      defaultCollapsed
      summary={t('query_builder.step_summary_count', { count: windowFunctions.length })}
    >
      {windowFunctions.map((w, i) => (
        <div key={i} className={`${qbTagBase} ${qbTagPurpleClass} flex items-center gap-1`}>
          <Select
            value={w.func}
            onChange={(v) => updateWindowFunc(i, 'func', v)}
            options={WINDOW_FUNC_OPTIONS.map((opt) => ({ value: opt, label: opt }))}
            size="sm"
          />
          <input
            value={w.field}
            onChange={(e) => updateWindowFunc(i, 'field', e.target.value)}
            placeholder={t('query_builder.window_field_placeholder')}
            style={{ width: '6rem' }}
          />
          <input
            value={w.partition_by}
            onChange={(e) => updateWindowFunc(i, 'partition_by', e.target.value)}
            placeholder={t('query_builder.window_partition_placeholder')}
            style={{ width: '6rem' }}
          />
          <input
            value={w.order_by}
            onChange={(e) => updateWindowFunc(i, 'order_by', e.target.value)}
            placeholder={t('query_builder.window_order_placeholder')}
            style={{ width: '6rem' }}
          />
          <button
            type="button"
            className={qbTagCloseClass}
            onClick={() => removeWindowFunc(i)}
            aria-label={t('query_builder.remove_window_aria', { n: i + 1 })}
          >
            ×
          </button>
        </div>
      ))}
      <button type="button" className={qbAddBtnClass} onClick={addWindowFunc}>
        +
      </button>
    </NotebookStep>
  )
}
