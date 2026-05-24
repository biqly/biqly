import { useEffect, useState } from 'react'
import { runMetadataDescribeDirect, type DescribeResult } from '../../api/metadataDescribe'
import { useT } from '../../i18n'
import type { AIRuntimeSettings } from '../../types/ai'
import type { ColumnRow, TableRow } from '../../types/semantic'
import { ErrorAlert } from '../ui/ErrorAlert'
import { ModelBadgeRow } from '../ui/ModelBadgeRow'

interface MetadataDescribeModalProps {
  table: TableRow
  datasourceId: string
  columns: ColumnRow[]
  aiRuntime: AIRuntimeSettings | null
  apiError: string | null
  runDescribeJob: (
    request: {
      datasource_id: string
      schema: string
      table: string
      sample_size: number
      auto_apply: boolean
    },
    onError: (message: string) => void,
  ) => Promise<DescribeResult | 'fallback' | null>
  patchDescription: (kind: 'table' | 'column', id: string, description: string) => Promise<void>
  onClose: () => void
  onApplied: (table: TableRow) => void
}

export function MetadataDescribeModal({
  table,
  datasourceId,
  columns,
  aiRuntime,
  apiError,
  runDescribeJob,
  patchDescription,
  onClose,
  onApplied,
}: MetadataDescribeModalProps) {
  const t = useT()
  const [form, setForm] = useState({ sample_size: 10, auto_apply: false })
  const [result, setResult] = useState<DescribeResult | null>(null)
  const [running, setRunning] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onClose])

  const runDescribe = async () => {
    setRunning(true)
    setError(null)
    const request = {
      datasource_id: datasourceId,
      schema: table.schema_name,
      table: table.table_name,
      sample_size: form.sample_size,
      auto_apply: form.auto_apply,
    }
    try {
      let res = await runDescribeJob(request, (message) => setError(message))
      if (res === 'fallback') {
        res = await runMetadataDescribeDirect(request)
      }
      if (res) {
        setResult(res)
        if (res.applied) onApplied(table)
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : t('metadata.bulk_network_error'))
    } finally {
      setRunning(false)
    }
  }

  const applySuggestion = async (kind: 'table' | 'column', name: string, description: string) => {
    if (kind === 'table') {
      await patchDescription('table', table.id, description)
    } else {
      const col = columns.find((c) => c.column_name === name)
      if (!col) return
      await patchDescription('column', col.id, description)
    }
  }

  return (
    <div
      className="modal-backdrop"
      role="presentation"
      onClick={(e) => { if (e.target === e.currentTarget) onClose() }}
    >
      <section className="modal-card" role="dialog" aria-modal="true" aria-labelledby="describe-title">
        <header className="modal-header">
          <div>
            <h2 id="describe-title">
              {t('metadata.describe_modal_title', { fqn: `${table.schema_name}.${table.table_name}` })}
            </h2>
            <ModelBadgeRow
              primaryLabel={t('metadata.describe_badge_label')}
              primaryModel={result?.model ?? aiRuntime?.llm_model}
              translationModel={
                result?.translation_applied
                  ? result?.translation_model
                  : aiRuntime?.translation_enabled
                    ? aiRuntime?.translation_model
                    : undefined
              }
            />
          </div>
          <button type="button" className="modal-close" aria-label={t('metadata.describe_close_aria')} onClick={onClose}>
            ×
          </button>
        </header>

        <div className="modal-body">
          <p style={{ color: 'var(--text-secondary)', margin: 0 }}>{t('metadata.describe_intro')}</p>

          {!result && (
            <>
              <div className="modal-form-row">
                <div className="form-group">
                  <label htmlFor="describe-sample-size">{t('metadata.describe_sample_size')}</label>
                  <input
                    id="describe-sample-size"
                    name="sample_size"
                    type="number"
                    min={1}
                    max={100}
                    value={form.sample_size}
                    onChange={(e) => setForm({ ...form, sample_size: Number(e.target.value) })}
                  />
                </div>
                <div className="form-group">
                  <label>{t('metadata.describe_options')}</label>
                  <div className="checkbox-row">
                    <input
                      id="auto-apply"
                      name="auto_apply"
                      type="checkbox"
                      checked={form.auto_apply}
                      onChange={(e) => setForm({ ...form, auto_apply: e.target.checked })}
                    />
                    <label htmlFor="auto-apply">{t('metadata.describe_auto_apply')}</label>
                  </div>
                </div>
              </div>
              <ErrorAlert error={error || apiError} />
              <div className="modal-actions">
                <button type="button" className="btn btn-ghost btn-sm" onClick={onClose}>
                  {t('metadata.bulk_cancel')}
                </button>
                <button type="button" className="btn btn-sm" onClick={() => void runDescribe()} disabled={running}>
                  {running ? t('metadata.describe_analyzing') : t('metadata.describe_generate')}
                </button>
              </div>
            </>
          )}

          {result && (
            <>
              {result.model && (
                <div style={{ fontSize: '0.8rem', color: 'var(--text-secondary)' }}>
                  {t('metadata.describe_model_line')} <code translate="no">{result.model}</code>
                  {result.translation_applied && result.translation_model ? (
                    <>{t('metadata.describe_translation_sep')} <code translate="no">{result.translation_model}</code></>
                  ) : null}
                </div>
              )}
              <p style={{ color: 'var(--text-secondary)', margin: 0 }}>
                {t('metadata.describe_rows_sampled', { n: result.sample_rows })}{' '}
                {result.applied
                  ? <span className="success">{t('metadata.describe_all_applied')}</span>
                  : t('metadata.describe_review_apply')}
              </p>
              {result.translation_error && (
                <p style={{ margin: 0, color: 'var(--error)' }}>
                  {t('metadata.describe_translation_failed')} {result.translation_error}
                </p>
              )}

              <div>
                <h3 style={{ marginBottom: '0.4rem' }}>{t('metadata.describe_section_table')}</h3>
                <div className="suggestion-block">
                  {result.description || <em style={{ color: 'var(--text-secondary)' }}>{t('metadata.describe_empty_paren')}</em>}
                </div>
                {!result.applied && result.description && (
                  <div className="modal-actions">
                    <button type="button" className="btn btn-sm" onClick={() => void applySuggestion('table', '', result.description)}>
                      {t('metadata.describe_apply_table')}
                    </button>
                  </div>
                )}
              </div>

              <div>
                <h3 style={{ marginBottom: '0.4rem' }}>{t('metadata.describe_section_columns')}</h3>
                <table className="results-table">
                  <thead>
                    <tr>
                      <th>{t('metadata.describe_col_column')}</th>
                      <th>{t('metadata.describe_col_suggestion')}</th>
                      {!result.applied && <th style={{ textAlign: 'right' }}>{t('metadata.describe_col_action')}</th>}
                    </tr>
                  </thead>
                  <tbody>
                    {result.columns.map((c) => (
                      <tr key={c.name}>
                        <td><code>{c.name}</code></td>
                        <td>{c.description || <em style={{ color: 'var(--text-secondary)' }}>{t('metadata.describe_empty_paren')}</em>}</td>
                        {!result.applied && (
                          <td className="actions">
                            {c.description && (
                              <button type="button" className="btn btn-sm" onClick={() => void applySuggestion('column', c.name, c.description)}>
                                {t('metadata.describe_apply')}
                              </button>
                            )}
                          </td>
                        )}
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>

              <div className="modal-actions">
                <button type="button" className="btn btn-ghost btn-sm" onClick={() => { setResult(null); onClose() }}>
                  {t('metadata.describe_close_footer')}
                </button>
              </div>
            </>
          )}
        </div>
      </section>
    </div>
  )
}
