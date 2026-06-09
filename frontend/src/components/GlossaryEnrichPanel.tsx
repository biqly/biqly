import { useT } from '../i18n'
import type { EnrichAnalyzeResult } from '../types/enrichContext'

interface EnrichSelection {
  selected: boolean
  value: string
}

interface GlossaryEnrichPanelProps {
  result: EnrichAnalyzeResult
  selections: Record<string, EnrichSelection>
  loading: boolean
  onClose: () => void
  onRerun: () => void
  onApply: () => void
  onSelectionChange: (gapId: string, patch: Partial<EnrichSelection>) => void
}

export function GlossaryEnrichPanel({
  result,
  selections,
  loading,
  onClose,
  onRerun,
  onApply,
  onSelectionChange,
}: GlossaryEnrichPanelProps) {
  const t = useT()
  const hasSelected = Object.values(selections).some((s) => s.selected && s.value.trim())

  return (
    <div
      className="card"
      style={{
        marginBottom: '1.25rem',
        padding: '1rem',
        background: 'var(--surface-elevated, rgba(255,255,255,0.02))',
        border: '1px solid var(--border)',
      }}
    >
      <div
        style={{
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'flex-start',
          gap: '1rem',
          flexWrap: 'wrap',
          marginBottom: '0.75rem',
        }}
      >
        <div>
          <h3 style={{ margin: 0, fontSize: '1rem' }}>{t('glossary.enrich_context')}</h3>
          <p style={{ margin: '0.35rem 0 0', color: 'var(--text-secondary)', fontSize: '0.85rem' }}>
            {result.gaps.length > 0
              ? t('glossary.enrich_context_gaps_found', { count: result.gaps.length })
              : t('glossary.enrich_context_no_gaps')}
            {result.model_name ? ` · ${result.model_name}` : ''}
          </p>
        </div>
        <div style={{ display: 'flex', gap: '0.5rem', flexWrap: 'wrap' }}>
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
            disabled={loading || !hasSelected}
            onClick={onApply}
          >
            {t('glossary.enrich_context_apply')}
          </button>
        </div>
      </div>

      {result.gaps.length > 0 && (
        <div className="table-wrap">
          <table className="results-table">
            <thead>
              <tr>
                <th style={{ width: '2.5rem' }} />
                <th>{t('glossary.col_type')}</th>
                <th>{t('glossary.col_term')}</th>
                <th>{t('glossary.enrich_context_suggested')}</th>
              </tr>
            </thead>
            <tbody>
              {result.gaps.map((gap) => {
                const selection = selections[gap.id]
                return (
                  <tr key={gap.id}>
                    <td>
                      {gap.applyable ? (
                        <input
                          type="checkbox"
                          checked={selection?.selected ?? false}
                          onChange={(e) => {
                            onSelectionChange(gap.id, { selected: e.target.checked })
                          }}
                        />
                      ) : (
                        <span style={{ fontSize: '0.7rem', color: 'var(--text-muted)' }}>—</span>
                      )}
                    </td>
                    <td>
                      <code style={{ fontSize: '0.72rem' }}>{gap.kind}</code>
                      {!gap.applyable && (
                        <div style={{ fontSize: '0.7rem', color: 'var(--text-muted)' }}>
                          {t('glossary.enrich_context_not_applyable')}
                        </div>
                      )}
                    </td>
                    <td>
                      <div style={{ fontWeight: 600 }}>{gap.summary}</div>
                      {gap.detail && (
                        <div style={{ fontSize: '0.78rem', color: 'var(--text-secondary)' }}>
                          {gap.detail}
                        </div>
                      )}
                    </td>
                    <td>
                      {gap.applyable ? (
                        <textarea
                          className="input"
                          rows={2}
                          value={selection?.value ?? ''}
                          onChange={(e) => {
                            onSelectionChange(gap.id, { value: e.target.value })
                          }}
                        />
                      ) : (
                        <span style={{ color: 'var(--text-muted)' }}>—</span>
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
