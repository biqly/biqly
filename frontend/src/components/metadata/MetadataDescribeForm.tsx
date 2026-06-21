import type { useT } from '../../i18n'
import { buttonClass } from '../../lib/buttonClasses'
import {
  checkboxRowClass,
  modalActionsBorderedClass,
  modalDescribeFieldsetClass,
  modalDescribeLegendClass,
  modalDescribeSampleInputClass,
} from '../../lib/modalClasses'
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
      <div className="grid grid-cols-1 items-stretch gap-3 sm:grid-cols-2">
        <fieldset className={modalDescribeFieldsetClass}>
          <legend className={modalDescribeLegendClass}>{t('metadata.describe_sample_size')}</legend>
          <div className="mt-2">
            <input
              id="describe-sample-size"
              name="sample_size"
              type="number"
              min={1}
              max={100}
              className={modalDescribeSampleInputClass}
              value={sampleSize}
              onChange={(e) => onSampleSizeChange(Number(e.target.value))}
              aria-label={t('metadata.describe_sample_size')}
            />
          </div>
        </fieldset>
        <fieldset className={modalDescribeFieldsetClass}>
          <legend className={modalDescribeLegendClass}>{t('metadata.describe_options')}</legend>
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
        </fieldset>
      </div>
      <ErrorAlert error={error ?? apiError} />
      <footer className={modalActionsBorderedClass()}>
        <button
          type="button"
          className={buttonClass('ghost', { size: 'sm' })}
          onClick={onClose}
          disabled={running}
        >
          {t('metadata.bulk_cancel')}
        </button>
        <button
          type="button"
          className={buttonClass('secondary', { size: 'sm' })}
          onClick={onRun}
          disabled={running}
        >
          {running ? t('metadata.describe_analyzing') : t('metadata.describe_generate')}
        </button>
      </footer>
    </>
  )
}
