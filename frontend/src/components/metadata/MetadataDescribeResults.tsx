import type { DescribeResult } from '../../api/metadataDescribe'
import type { useT } from '../../i18n'

export function MetadataDescribeResults({
  result,
  t,
  onApplyTable,
  onApplyColumn,
  onClose,
}: {
  result: DescribeResult
  t: ReturnType<typeof useT>
  onApplyTable: (description: string) => void
  onApplyColumn: (name: string, description: string) => void
  onClose: () => void
}) {
  return (
    <>
      {result.model && (
        <div style={{ fontSize: '0.8rem', color: 'var(--text-secondary)' }}>
          {t('metadata.describe_model_line')} <code translate="no">{result.model}</code>
          {result.translation_applied && result.translation_model ? (
            <>
              {t('metadata.describe_translation_sep')}{' '}
              <code translate="no">{result.translation_model}</code>
            </>
          ) : null}
        </div>
      )}
      <p style={{ color: 'var(--text-secondary)', margin: 0 }}>
        {t('metadata.describe_rows_sampled', { n: result.sample_rows })}{' '}
        {result.applied ? (
          <span className="success">{t('metadata.describe_all_applied')}</span>
        ) : (
          t('metadata.describe_review_apply')
        )}
      </p>
      {result.translation_error && (
        <p style={{ margin: 0, color: 'var(--error)' }}>
          {t('metadata.describe_translation_failed')} {result.translation_error}
        </p>
      )}

      <div>
        <h3 style={{ marginBottom: '0.4rem' }}>{t('metadata.describe_section_table')}</h3>
        <div className="suggestion-block">
          {result.description || (
            <em style={{ color: 'var(--text-secondary)' }}>{t('metadata.describe_empty_paren')}</em>
          )}
        </div>
        {!result.applied && result.description && (
          <div className="modal-actions">
            <button
              type="button"
              className="btn btn-sm"
              onClick={() => onApplyTable(result.description)}
            >
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
              {!result.applied && (
                <th style={{ textAlign: 'right' }}>{t('metadata.describe_col_action')}</th>
              )}
            </tr>
          </thead>
          <tbody>
            {result.columns.map((c) => (
              <tr key={c.name}>
                <td>
                  <code>{c.name}</code>
                </td>
                <td>
                  {c.description || (
                    <em style={{ color: 'var(--text-secondary)' }}>
                      {t('metadata.describe_empty_paren')}
                    </em>
                  )}
                </td>
                {!result.applied && (
                  <td className="actions">
                    {c.description && (
                      <button
                        type="button"
                        className="btn btn-sm"
                        onClick={() => onApplyColumn(c.name, c.description)}
                      >
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
        <button type="button" className="btn btn-ghost btn-sm" onClick={onClose}>
          {t('metadata.describe_close_footer')}
        </button>
      </div>
    </>
  )
}
