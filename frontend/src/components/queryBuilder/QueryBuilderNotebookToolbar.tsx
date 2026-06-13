import { noop } from '../../utils/constants'
import type { MetadataTFunction } from '../metadata/utils'
import {
  qbSqlToggleClass,
  qbToolbarBtnClass,
  qbToolbarBtnVariantClass,
  qbToolbarClass,
  qbVisualizeBtnClass,
  qbVisualizeContainerClass,
} from './queryBuilderClasses'

export function QueryBuilderNotebookToolbar({
  filtersActive,
  isSummarized,
  orderByActive,
  limit,
  mode,
  havingActive,
  windowActive,
  cteActive,
  onAddFilter,
  onToggleSummarize,
  onAddHaving,
  onAddWindowFunc,
  onAddCte,
  onPickFirstSort,
}: {
  filtersActive: boolean
  isSummarized: boolean
  orderByActive: boolean
  limit: number
  mode: string
  havingActive: boolean
  windowActive: boolean
  cteActive: boolean
  onAddFilter: () => void
  onToggleSummarize: () => void
  onAddHaving: () => void
  onAddWindowFunc: () => void
  onAddCte: () => void
  onPickFirstSort: () => void
}) {
  return (
    <div className={qbToolbarClass}>
      <button
        type="button"
        className={qbToolbarBtnVariantClass('filter', filtersActive)}
        onClick={onAddFilter}
      >
        + Filter
      </button>
      <button
        type="button"
        className={qbToolbarBtnVariantClass('summarize', isSummarized)}
        onClick={onToggleSummarize}
      >
        + Summarize
      </button>
      <button
        type="button"
        className={qbToolbarBtnVariantClass('sort', orderByActive)}
        onClick={onPickFirstSort}
      >
        + Sort
      </button>
      <button type="button" className={qbToolbarBtnVariantClass('limit', false)} onClick={noop}>
        Limit ({limit})
      </button>
      {mode === 'advanced' && (
        <>
          <button
            type="button"
            className={qbToolbarBtnVariantClass('advanced', havingActive)}
            onClick={onAddHaving}
          >
            + Having
          </button>
          <button
            type="button"
            className={qbToolbarBtnVariantClass('advanced', windowActive)}
            onClick={onAddWindowFunc}
          >
            + Window Func
          </button>
          <button
            type="button"
            className={qbToolbarBtnVariantClass('advanced', cteActive)}
            onClick={onAddCte}
          >
            + CTE
          </button>
        </>
      )}
    </div>
  )
}

export function QueryBuilderVisualizeFooter({
  show,
  loading,
  onRun,
  sqlVisible,
  onToggleSql,
  t,
}: {
  show: boolean
  loading: boolean
  onRun: () => void
  sqlVisible: boolean
  onToggleSql: () => void
  t: MetadataTFunction
}) {
  if (!show) {
    return null
  }
  const sqlBtnClass = sqlVisible
    ? `${qbToolbarBtnClass} border-accent bg-[color-mix(in_srgb,var(--accent)_8%,var(--bg-card-raised))] text-foreground shadow-[0_0_8px_var(--accent-glow)]`
    : qbToolbarBtnClass
  return (
    <div className={qbVisualizeContainerClass}>
      <button type="button" className={qbVisualizeBtnClass} onClick={onRun} disabled={loading}>
        {loading ? t('query_builder.running') : 'Visualize'}
      </button>
      <button
        type="button"
        className={`${sqlBtnClass} ${qbSqlToggleClass}`}
        onClick={onToggleSql}
        aria-pressed={sqlVisible}
      >
        <span aria-hidden="true">{'</>'}</span>{' '}
        {sqlVisible ? t('query_builder.hide_sql') : t('query_builder.show_sql')}
      </button>
    </div>
  )
}
