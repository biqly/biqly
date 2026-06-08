import { noop } from '../../utils/constants'
import type { MetadataTFunction } from '../metadata/utils'

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
    <div className="notebook-toolbar">
      <button
        type="button"
        className={`toolbar-btn toolbar-btn--filter ${filtersActive ? 'active' : ''}`}
        onClick={onAddFilter}
      >
        + Filter
      </button>
      <button
        type="button"
        className={`toolbar-btn toolbar-btn--summarize ${isSummarized ? 'active' : ''}`}
        onClick={onToggleSummarize}
      >
        + Summarize
      </button>
      <button
        type="button"
        className={`toolbar-btn toolbar-btn--sort ${orderByActive ? 'active' : ''}`}
        onClick={onPickFirstSort}
      >
        + Sort
      </button>
      <button type="button" className="toolbar-btn toolbar-btn--limit" onClick={noop}>
        Limit ({limit})
      </button>
      {mode === 'advanced' && (
        <>
          <button
            type="button"
            className={`toolbar-btn toolbar-btn--advanced ${havingActive ? 'active' : ''}`}
            onClick={onAddHaving}
          >
            + Having
          </button>
          <button
            type="button"
            className={`toolbar-btn toolbar-btn--advanced ${windowActive ? 'active' : ''}`}
            onClick={onAddWindowFunc}
          >
            + Window Func
          </button>
          <button
            type="button"
            className={`toolbar-btn toolbar-btn--advanced ${cteActive ? 'active' : ''}`}
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
  t,
}: {
  show: boolean
  loading: boolean
  onRun: () => void
  t: MetadataTFunction
}) {
  if (!show) {
    return null
  }
  return (
    <div className="visualize-btn-container">
      <button type="button" className="visualize-btn" onClick={onRun} disabled={loading}>
        {loading ? t('query_builder.running') : 'Visualize'}
      </button>
    </div>
  )
}
