import type { useT } from '../../i18n'
import { legacyButtonClass } from '../../lib/buttonClasses'
import { legacyFormClass } from '../../lib/formClasses'
import { checkboxRowClass, modalActionsClass, modalFormRowClass } from '../../lib/modalClasses'
import { ErrorAlert } from '../ui/ErrorAlert'

export function MetadataDescribeForm({
  t,
  sampleSize,
  autoApply,
  running,
  error,
  apiError,
  onSampleSizeChange,
  onAutoApplyChange,
  onClose,
  onRun,
}: {
  t: ReturnType<typeof useT>
  sampleSize: number
  autoApply: boolean
  running: boolean
  error: string | null
  apiError: string | null
  onSampleSizeChange: (size: number) => void
  onAutoApplyChange: (checked: boolean) => void
  onClose: () => void
  onRun: () => void
}) {
  return (
    <>
      <div className={modalFormRowClass()}>
        <div className={legacyFormClass('form-group')}>
          <label htmlFor="describe-sample-size">{t('metadata.describe_sample_size')}</label>
          <input
            id="describe-sample-size"
            name="sample_size"
            type="number"
            min={1}
            max={100}
            value={sampleSize}
            onChange={(e) => onSampleSizeChange(Number(e.target.value))}
          />
        </div>
        <div className={legacyFormClass('form-group')}>
          <label>{t('metadata.describe_options')}</label>
          <div className={checkboxRowClass()}>
            <input
              id="auto-apply"
              name="auto_apply"
              type="checkbox"
              checked={autoApply}
              onChange={(e) => onAutoApplyChange(e.target.checked)}
            />
            <label htmlFor="auto-apply">{t('metadata.describe_auto_apply')}</label>
          </div>
        </div>
      </div>
      <ErrorAlert error={error ?? apiError} />
      <div className={modalActionsClass()}>
        <button
          type="button"
          className={legacyButtonClass('btn btn-ghost btn-sm')}
          onClick={onClose}
        >
          {t('metadata.bulk_cancel')}
        </button>
        <button
          type="button"
          className={legacyButtonClass('btn btn-sm')}
          onClick={onRun}
          disabled={running}
        >
          {running ? t('metadata.describe_analyzing') : t('metadata.describe_generate')}
        </button>
      </div>
    </>
  )
}
