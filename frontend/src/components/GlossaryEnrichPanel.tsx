import { useT } from '../i18n'
import { legacyButtonClass } from '../lib/buttonClasses'
import { legacyCardClass } from '../lib/cardClasses'
import { cn } from '../lib/cn'
import { legacyTableClass } from '../lib/tableClasses'
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
    <div
      className={legacyCardClass(
        'card border-border mb-5 rounded-lg border bg-(--surface-elevated,rgba(255,255,255,0.02)) p-4',
      )}
    >
      <div className="mb-3 flex flex-wrap items-start justify-between gap-4">
        <div>
          <h3 className="m-0 text-base">{t('glossary.enrich_context')}</h3>
          <p className="text-foreground-muted mt-[0.35rem] mr-0 mb-0 ml-0 text-[0.85rem]">
            {result.gaps.length > 0
              ? t('glossary.enrich_context_gaps_found', { count: result.gaps.length })
              : t('glossary.enrich_context_no_gaps')}
            {result.model_name ? ` · ${result.model_name}` : ''}
            {result.sample_rows
              ? ` · ${t('glossary.enrich_context_sample_rows', { count: result.sample_rows })}`
              : ''}
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <button
            type="button"
            className={legacyButtonClass('btn btn-sm btn-ghost')}
            onClick={onClose}
          >
            {t('glossary.enrich_context_close')}
          </button>
          <button
            type="button"
            className={legacyButtonClass('btn btn-sm btn-secondary')}
            disabled={loading}
            onClick={onRerun}
          >
            {t('glossary.enrich_context_run')}
          </button>
          <button
            type="button"
            className={legacyButtonClass('btn btn-sm btn-primary')}
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
          className={cn(
            'mb-3 rounded-lg border bg-(--surface-elevated,rgba(255,255,255,0.02)) px-3 py-[0.6rem] text-[0.85rem]',
            applyResult.errors && applyResult.errors.length > 0
              ? 'border-(--danger,#d9534f)'
              : 'border-border',
          )}
          role="status"
        >
          <div className="font-semibold">
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
              <ul className="mt-[0.4rem] mr-0 mb-0 ml-0 pl-[1.1rem] text-(--danger,#d9534f)">
                {applyResult.errors.map((err, idx) => (
                  <li key={idx}>{err}</li>
                ))}
              </ul>
            </>
          )}
        </div>
      )}

      {applyableGaps.length > 0 && (
        <div className="mb-3 flex items-center gap-2">
          <button
            type="button"
            className={legacyButtonClass('btn btn-sm btn-ghost')}
            disabled={loading || allSelected}
            onClick={() => onSelectAll(true)}
          >
            {t('glossary.enrich_context_select_all')}
          </button>
          <button
            type="button"
            className={legacyButtonClass('btn btn-sm btn-ghost')}
            disabled={loading || selectedCount === 0}
            onClick={() => onSelectAll(false)}
          >
            {t('glossary.enrich_context_clear')}
          </button>
          <span className="text-foreground-muted ml-auto text-[0.8rem]">
            {t('glossary.enrich_context_apply_count', { count: selectedCount })}
          </span>
        </div>
      )}

      {result.gaps.length > 0 && (
        <div className="table-wrap">
          <table className={legacyTableClass('results-table')}>
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
                        <span className="text-foreground-faint text-[0.7rem]">—</span>
                      )}
                    </td>
                    <td>
                      <code className="text-[0.72rem]">{gap.kind}</code>
                      {!gap.applyable && (
                        <div className="text-foreground-faint text-[0.7rem]">
                          {t('glossary.enrich_context_not_applyable')}
                        </div>
                      )}
                    </td>
                    <td>
                      <div className="font-semibold">{gap.summary}</div>
                      {gap.detail && (
                        <div className="text-foreground-muted text-[0.78rem]">{gap.detail}</div>
                      )}
                    </td>
                    <td>
                      {gap.applyable ? (
                        <div className="flex flex-col gap-[0.3rem]">
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
                          <div className="text-foreground-faint flex items-center gap-[0.4rem] text-[0.72rem]">
                            {suggestion ? (
                              <>
                                <span>
                                  {t('glossary.enrich_context_suggested')}: {suggestion}
                                </span>
                                {canRestore && (
                                  <button
                                    type="button"
                                    className="text-accent focus-visible:outline-accent cursor-pointer border-none bg-none p-0 text-[0.72rem] focus-visible:outline-2 focus-visible:outline-offset-2"
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
                        <span className="text-foreground-faint text-[0.7rem]">—</span>
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
