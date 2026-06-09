import '../styles/glossary-enrich.css'

import { useT } from '../i18n'
import type { EnrichAnalyzeResult, EnrichApplyResult } from '../types/enrichContext'

interface EnrichSelection {
  selected: boolean
  value: string
}

interface GlossaryEnrichPanelProps {
  result: EnrichAnalyzeResult
  selections: Record<string, EnrichSelection>
  applyResult: EnrichApplyResult | null
  loading: boolean
  onClose: () => void
  onRerun: () => void
  onApply: () => void
  onSelectionChange: (gapId: string, patch: Partial<EnrichSelection>) => void
  onSelectAll: (selected: boolean) => void
}

export function GlossaryEnrichPanel({
  result,
  selections,
  applyResult,
  loading,
  onClose,
  onRerun,
  onApply,
  onSelectionChange,
  onSelectAll,
}: GlossaryEnrichPanelProps) {
  const t = useT()

  const suggestionsByGap = new Map(
    (result.suggestions ?? []).map((suggestion) => [suggestion.gap_id, suggestion.text]),
  )
  const applyableGaps = result.gaps.filter((gap) => gap.applyable)
  const selectedCount = applyableGaps.filter(
    (gap) => selections[gap.id]?.selected && selections[gap.id]?.value.trim(),
  ).length
  const allSelected =
    applyableGaps.length > 0 && applyableGaps.every((gap) => selections[gap.id]?.selected)

  return (
    <div className="card enrich-panel">
      <div className="enrich-panel__header">
        <div>
          <h3 className="enrich-panel__title">{t('glossary.enrich_context')}</h3>
          <p className="enrich-panel__subtitle">
            {result.gaps.length > 0
              ? t('glossary.enrich_context_gaps_found', { count: result.gaps.length })
              : t('glossary.enrich_context_no_gaps')}
            {result.model_name ? ` · ${result.model_name}` : ''}
            {result.sample_rows
              ? ` · ${t('glossary.enrich_context_sample_rows', { count: result.sample_rows })}`
              : ''}
          </p>
        </div>
        <div className="enrich-panel__actions">
          <button type="button" className="btn btn-sm btn-ghost" onClick={onClose}>
            {t('glossary.enrich_context_close')}
          </button>
          <button
            type="button"
            className="btn btn-sm btn-secondary"
            disabled={loading}
            onClick={onRerun}
          >
            {t('glossary.enrich_context_run')}
          </button>
          <button
            type="button"
            className="btn btn-sm btn-primary"
            disabled={loading || selectedCount === 0}
            onClick={onApply}
          >
            {selectedCount > 0
              ? t('glossary.enrich_context_apply_count', { count: selectedCount })
              : t('glossary.enrich_context_apply')}
          </button>
        </div>
      </div>

      {applyResult && (
        <div
          className={`enrich-panel__result${
            applyResult.errors && applyResult.errors.length > 0
              ? ' enrich-panel__result--error'
              : ''
          }`}
          role="status"
        >
          <div className="enrich-panel__result-summary">
            {t('glossary.enrich_context_applied', {
              applied: applyResult.applied,
              skipped: applyResult.skipped,
            })}
          </div>
          {applyResult.errors && applyResult.errors.length > 0 && (
            <>
              <div>
                {t('glossary.enrich_context_apply_failed', { count: applyResult.errors.length })}
              </div>
              <ul className="enrich-panel__result-errors">
                {applyResult.errors.map((err, idx) => (
                  <li key={idx}>{err}</li>
                ))}
              </ul>
            </>
          )}
        </div>
      )}

      {applyableGaps.length > 0 && (
        <div className="enrich-panel__bulk">
          <button
            type="button"
            className="btn btn-sm btn-ghost"
            disabled={loading || allSelected}
            onClick={() => onSelectAll(true)}
          >
            {t('glossary.enrich_context_select_all')}
          </button>
          <button
            type="button"
            className="btn btn-sm btn-ghost"
            disabled={loading || selectedCount === 0}
            onClick={() => onSelectAll(false)}
          >
            {t('glossary.enrich_context_clear')}
          </button>
          <span className="enrich-panel__bulk-count">
            {t('glossary.enrich_context_apply_count', { count: selectedCount })}
          </span>
        </div>
      )}

      {result.gaps.length > 0 && (
        <div className="table-wrap">
          <table className="results-table">
            <thead>
              <tr>
                <th scope="col" style={{ width: '2.5rem' }} />
                <th scope="col">{t('glossary.col_type')}</th>
                <th scope="col">{t('glossary.col_term')}</th>
                <th scope="col">{t('glossary.enrich_context_suggested')}</th>
              </tr>
            </thead>
            <tbody>
              {result.gaps.map((gap) => {
                const selection = selections[gap.id]
                const suggestion = suggestionsByGap.get(gap.id) ?? ''
                const currentValue = selection?.value ?? ''
                const canRestore = suggestion.length > 0 && currentValue !== suggestion
                return (
                  <tr key={gap.id}>
                    <td>
                      {gap.applyable ? (
                        <input
                          type="checkbox"
                          aria-label={t('glossary.enrich_context_select_gap', {
                            name: gap.summary,
                          })}
                          checked={selection?.selected ?? false}
                          onChange={(e) => {
                            onSelectionChange(gap.id, { selected: e.target.checked })
                          }}
                        />
                      ) : (
                        <span className="enrich-panel__muted">—</span>
                      )}
                    </td>
                    <td>
                      <code className="enrich-panel__kind">{gap.kind}</code>
                      {!gap.applyable && (
                        <div className="enrich-panel__muted">
                          {t('glossary.enrich_context_not_applyable')}
                        </div>
                      )}
                    </td>
                    <td>
                      <div className="enrich-panel__gap-summary">{gap.summary}</div>
                      {gap.detail && <div className="enrich-panel__gap-detail">{gap.detail}</div>}
                    </td>
                    <td>
                      {gap.applyable ? (
                        <div className="enrich-panel__value">
                          <textarea
                            className="input"
                            rows={2}
                            aria-label={t('glossary.enrich_context_value_label', {
                              name: gap.summary,
                            })}
                            value={currentValue}
                            onChange={(e) => {
                              onSelectionChange(gap.id, { value: e.target.value })
                            }}
                          />
                          <div className="enrich-panel__suggestion">
                            {suggestion ? (
                              <>
                                <span>
                                  {t('glossary.enrich_context_suggested')}: {suggestion}
                                </span>
                                {canRestore && (
                                  <button
                                    type="button"
                                    className="enrich-panel__restore"
                                    onClick={() => {
                                      onSelectionChange(gap.id, { value: suggestion })
                                    }}
                                  >
                                    {t('glossary.enrich_context_restore')}
                                  </button>
                                )}
                              </>
                            ) : (
                              <span>{t('glossary.enrich_context_no_suggestion')}</span>
                            )}
                          </div>
                        </div>
                      ) : (
                        <span className="enrich-panel__muted">—</span>
                      )}
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
