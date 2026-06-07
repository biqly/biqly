import type { TFunction } from '../../i18n'
import { NotebookStep } from './NotebookStep'
import type { CTERow } from './types'

interface CteStepProps {
  ctes: CTERow[]
  updateCTE: (i: number, field: keyof CTERow, v: string) => void
  removeCTE: (i: number) => void
  addCTE: () => void
  onClear: () => void
  t: TFunction
}

export function CteStep({ ctes, updateCTE, removeCTE, addCTE, onClear, t }: CteStepProps) {
  if (ctes.length === 0) {
    return null
  }

  return (
    <NotebookStep
      label="CTEs"
      themeClass="advanced"
      onClose={onClear}
      closeTitle={t('common.cancel')}
      collapsible
      defaultCollapsed
      summary={t('query_builder.step_summary_count', { count: ctes.length })}
    >
      {ctes.map((c, i) => (
        <div
          key={i}
          className="notebook-tag notebook-tag--purple"
          style={{
            display: 'flex',
            flexDirection: 'column',
            gap: '0.25rem',
            alignItems: 'flex-start',
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: '0.25rem', width: '100%' }}>
            <input
              value={c.name}
              onChange={(e) => updateCTE(i, 'name', e.target.value)}
              placeholder="CTE Name"
              style={{ width: '8rem' }}
            />
            <button
              type="button"
              className="notebook-tag-close"
              onClick={() => removeCTE(i)}
              aria-label="Remove CTE"
            >
              ×
            </button>
          </div>
          <textarea
            value={c.query}
            onChange={(e) => updateCTE(i, 'query', e.target.value)}
            placeholder="CTE query JSON"
            rows={2}
            style={{
              background: 'var(--bg-primary)',
              border: '1px solid var(--border-strong)',
              color: 'var(--text-primary)',
              borderRadius: '0.25rem',
              width: '12rem',
              padding: '0.25rem',
              fontSize: '0.74rem',
            }}
          />
        </div>
      ))}
      <button type="button" className="notebook-add-btn" onClick={addCTE}>
        +
      </button>
    </NotebookStep>
  )
}
