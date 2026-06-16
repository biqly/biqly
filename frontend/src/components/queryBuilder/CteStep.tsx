import { useT } from '../../i18n'
import { NotebookStep } from './NotebookStep'
import { qbAddBtnClass, qbTagBase, qbTagCloseClass, qbTagPurpleClass } from './queryBuilderClasses'
import type { CTERow } from './types'

interface CteStepProps {
  ctes: CTERow[]
  updateCTE: (i: number, field: keyof CTERow, v: string) => void
  removeCTE: (i: number) => void
  addCTE: () => void
  onClear: () => void
}

export function CteStep({ ctes, updateCTE, removeCTE, addCTE, onClear }: CteStepProps) {
  const t = useT()
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
        <div key={i} className={`${qbTagBase} ${qbTagPurpleClass} flex flex-col items-start gap-1`}>
          <div className="flex w-full items-center gap-1">
            <input
              value={c.name}
              onChange={(e) => updateCTE(i, 'name', e.target.value)}
              placeholder={t('query_builder.cte_name_placeholder')}
              style={{ width: '8rem' }}
            />
            <button
              type="button"
              className={qbTagCloseClass}
              onClick={() => removeCTE(i)}
              aria-label={t('query_builder.remove_cte_aria', { n: i + 1 })}
            >
              ×
            </button>
          </div>
          <textarea
            value={c.query}
            onChange={(e) => updateCTE(i, 'query', e.target.value)}
            placeholder={t('query_builder.cte_json_placeholder')}
            rows={2}
            className="border-border-strong bg-canvas text-foreground w-48 rounded border p-1 text-[0.74rem]"
          />
        </div>
      ))}
      <button type="button" className={qbAddBtnClass} onClick={addCTE}>
        +
      </button>
    </NotebookStep>
  )
}
