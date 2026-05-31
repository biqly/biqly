import { Select } from '../ui/Select'
import { NotebookStep } from './NotebookStep'
import { WINDOW_FUNC_OPTIONS } from './types'
import type { WindowFuncRow } from './types'

interface WindowFuncStepProps {
  windowFunctions: WindowFuncRow[]
  updateWindowFunc: (i: number, field: keyof WindowFuncRow, v: string) => void
  removeWindowFunc: (i: number) => void
  addWindowFunc: () => void
  onClear: () => void
  t: any
}

export function WindowFuncStep({
  windowFunctions,
  updateWindowFunc,
  removeWindowFunc,
  addWindowFunc,
  onClear,
  t,
}: WindowFuncStepProps) {
  if (windowFunctions.length === 0) return null

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
        <div
          key={i}
          className="notebook-tag notebook-tag--purple"
          style={{ display: 'flex', alignItems: 'center', gap: '0.25rem' }}
        >
          <Select
            value={w.func}
            onChange={(v) => updateWindowFunc(i, 'func', v)}
            options={WINDOW_FUNC_OPTIONS.map((opt) => ({ value: opt, label: opt }))}
            size="sm"
          />
          <input
            value={w.field}
            onChange={(e) => updateWindowFunc(i, 'field', e.target.value)}
            placeholder="field"
            style={{ width: '6rem' }}
          />
          <input
            value={w.partition_by}
            onChange={(e) => updateWindowFunc(i, 'partition_by', e.target.value)}
            placeholder="partition"
            style={{ width: '6rem' }}
          />
          <input
            value={w.order_by}
            onChange={(e) => updateWindowFunc(i, 'order_by', e.target.value)}
            placeholder="order"
            style={{ width: '6rem' }}
          />
          <button
            type="button"
            className="notebook-tag-close"
            onClick={() => removeWindowFunc(i)}
            aria-label="Remove Window Function"
          >
            ×
          </button>
        </div>
      ))}
      <button type="button" className="notebook-add-btn" onClick={addWindowFunc}>
        +
      </button>
    </NotebookStep>
  )
}
