import type { DescribeResult } from '../../api/metadataDescribe'
import type { useT } from '../../i18n'
import { buttonClass } from '../../lib/buttonClasses'
import { cn } from '../../lib/cn'
import { legacyFeedbackClass, suggestionBlockClass } from '../../lib/feedbackClasses'
import { modalActionsClass } from '../../lib/modalClasses'

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
        <p className="text-foreground-faint [&_code]:text-foreground-muted m-0 text-[0.76rem] [&_code]:font-mono [&_code]:text-[0.74rem]">
          {t('metadata.describe_model_line')} <code translate="no">{result.model}</code>
          {result.translation_applied && result.translation_model ? (
            <>
              {t('metadata.describe_translation_sep')}{' '}
              <code translate="no">{result.translation_model}</code>
            </>
          ) : null}
        </p>
      )}

      <div
        className={cn(
          'rounded-lg border px-3 py-2 text-[0.78rem] leading-snug',
          result.applied
            ? 'text-success border-[color-mix(in_srgb,var(--success)_25%,transparent)] bg-[color-mix(in_srgb,var(--success)_8%,transparent)]'
            : 'border-border bg-card-raised text-foreground-muted',
        )}
        role="status"
      >
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
        <h3 className="text-foreground m-0 mb-2 text-[0.86rem] font-[650] tracking-[-0.01em]">
          {t('metadata.describe_section_table')}
        </h3>
        <div className={suggestionBlockClass}>
          {result.description || (
            <em className="text-foreground-muted italic">{t('metadata.describe_empty_paren')}</em>
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
        <h3 className="text-foreground m-0 mb-2 text-[0.86rem] font-[650] tracking-[-0.01em]">
          {t('metadata.describe_section_columns')}
        </h3>
        <div className="border-border max-h-[min(40vh,18rem)] max-w-full overflow-x-auto overflow-y-auto rounded-lg border">
          <table
            className={cn(
              'text-caption w-full min-w-md table-fixed border-collapse',
              '[&_td]:px-3 [&_td]:py-2 [&_th]:px-3 [&_th]:py-2',
              '[&_thead_th]:text-left [&_thead_th]:text-[0.68rem] [&_thead_th]:font-bold [&_thead_th]:uppercase',
              '[&_thead_th]:text-foreground-muted [&_thead_th]:align-middle [&_thead_th]:tracking-[0.06em]',
              '[&_thead_th]:border-border-strong [&_thead_th]:border-b [&_thead_th]:bg-(--table-header-bg)',
              '[&_thead_th:last-child]:text-right',
              '[&_tbody_td]:border-border [&_tbody_td]:border-b [&_tbody_td]:align-top [&_tbody_td]:text-[0.82rem]',
              '[&_tbody_tr:last-child_td]:border-b-0',
              '[&_tbody_td:first-child]:w-[22%] [&_tbody_td:first-child_code]:text-[0.76rem] [&_tbody_td:first-child_code]:break-all',
              '[&_tbody_td:nth-child(2)]:wrap-anywhere',
              '[&_td.actions]:text-right [&_td.actions]:align-middle [&_td.actions]:whitespace-nowrap',
            )}
          >
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
                      <em className="text-foreground-muted italic">
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
