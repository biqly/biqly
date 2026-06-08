import type { useT } from '../../i18n'
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
      <div className="modal-form-row">
        <div className="form-group">
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
        <div className="form-group">
          <label>{t('metadata.describe_options')}</label>
          <div className="checkbox-row">
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
      <div className="modal-actions">
        <button type="button" className="btn btn-ghost btn-sm" onClick={onClose}>
          {t('metadata.bulk_cancel')}
        </button>
        <button type="button" className="btn btn-sm" onClick={onRun} disabled={running}>
          {running ? t('metadata.describe_analyzing') : t('metadata.describe_generate')}
        </button>
      </div>
    </>
  )
}
