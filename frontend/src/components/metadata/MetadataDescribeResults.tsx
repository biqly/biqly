import type { DescribeResult } from '../../api/metadataDescribe'
import type { useT } from '../../i18n'
import { buttonClass } from '../../lib/buttonClasses'
import { legacyFeedbackClass, suggestionBlockClass } from '../../lib/feedbackClasses'
import {
  modalActionsClass,
  modalDescribeEmptyEmClass,
  modalDescribeMetaLineClass,
  modalDescribeResultsScrollClass,
  modalDescribeResultsTableClass,
  modalDescribeSectionTitleClass,
  modalDescribeStatusBannerClass,
} from '../../lib/modalClasses'

export function MetadataDescribeResults({
  result,
  t,
  onApplyTable,
  onApplyColumn,
}: {
  result: DescribeResult
  t: ReturnType<typeof useT>
  onApplyTable: (description: string) => void
  onApplyColumn: (name: string, description: string) => void
}) {
  return (
    <>
      {result.model && (
        <p className={modalDescribeMetaLineClass}>
          {t('metadata.describe_model_line')} <code translate="no">{result.model}</code>
          {result.translation_applied && result.translation_model ? (
            <>
              {t('metadata.describe_translation_sep')}{' '}
              <code translate="no">{result.translation_model}</code>
            </>
          ) : null}
        </p>
      )}

      <div className={modalDescribeStatusBannerClass(result.applied)} role="status">
        {t('metadata.describe_rows_sampled', { n: result.sample_rows })}{' '}
        {result.applied ? (
          <span className={legacyFeedbackClass('success')}>
            {t('metadata.describe_all_applied')}
          </span>
        ) : (
          t('metadata.describe_review_apply')
        )}
      </div>

      {result.translation_error && (
        <p className="text-error m-0 text-[0.78rem]" role="alert">
          {t('metadata.describe_translation_failed')} {result.translation_error}
        </p>
      )}

      <section>
        <h3 className={modalDescribeSectionTitleClass}>{t('metadata.describe_section_table')}</h3>
        <div className={suggestionBlockClass}>
          {result.description || (
            <em className={modalDescribeEmptyEmClass}>{t('metadata.describe_empty_paren')}</em>
          )}
        </div>
        {!result.applied && result.description && (
          <div className={modalActionsClass()}>
            <button
              type="button"
              className={buttonClass('secondary', { size: 'sm' })}
              onClick={() => onApplyTable(result.description)}
            >
              {t('metadata.describe_apply_table')}
            </button>
          </div>
        )}
      </section>

      <section className="min-w-0">
        <h3 className={modalDescribeSectionTitleClass}>{t('metadata.describe_section_columns')}</h3>
        <div className={modalDescribeResultsScrollClass}>
          <table className={modalDescribeResultsTableClass}>
            <thead>
              <tr>
                <th scope="col">{t('metadata.describe_col_column')}</th>
                <th scope="col">{t('metadata.describe_col_suggestion')}</th>
                {!result.applied && <th scope="col">{t('metadata.describe_col_action')}</th>}
              </tr>
            </thead>
            <tbody>
              {result.columns.map((c) => (
                <tr key={c.name}>
                  <td>
                    <code translate="no">{c.name}</code>
                  </td>
                  <td>
                    {c.description || (
                      <em className={modalDescribeEmptyEmClass}>
                        {t('metadata.describe_empty_paren')}
                      </em>
                    )}
                  </td>
                  {!result.applied && (
                    <td className="actions">
                      {c.description ? (
                        <button
                          type="button"
                          className={buttonClass('secondary', { size: 'sm' })}
                          onClick={() => onApplyColumn(c.name, c.description)}
                        >
                          {t('metadata.describe_apply')}
                        </button>
                      ) : null}
                    </td>
                  )}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>
    </>
  )
}
