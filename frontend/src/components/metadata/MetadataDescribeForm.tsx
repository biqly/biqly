import type { useT } from '../../i18n'
import { buttonClass } from '../../lib/buttonClasses'
import { cn } from '../../lib/cn'
import { modalActionsClass } from '../../lib/modalClasses'
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
      <p className="text-foreground-muted m-0 text-[0.78rem] leading-[1.45]">
        {t('metadata.describe_intro')}
      </p>
      <div className="grid grid-cols-1 items-stretch gap-[0.65rem] sm:grid-cols-2">
        <fieldset
          className={
            'border-border bg-card-raised m-0 min-w-0 rounded-lg border p-[0.55rem_0.65rem_0.65rem]'
          }
        >
          <legend className="text-foreground-faint px-1 py-0 text-[0.62rem] font-extrabold tracking-[0.07em] uppercase">
            {t('metadata.describe_sample_size')}
          </legend>
          <div className="mt-[0.4rem]">
            <input
              id="describe-sample-size"
              name="sample_size"
              type="number"
              min={1}
              max={100}
              className="w-17 p-[0.3rem_0.45rem] text-[0.8rem]"
              value={sampleSize}
              onChange={(e) => onSampleSizeChange(Number(e.target.value))}
              aria-label={t('metadata.describe_sample_size')}
            />
          </div>
        </fieldset>
        <fieldset
          className={
            'border-border bg-card-raised m-0 min-w-0 rounded-lg border p-[0.55rem_0.65rem_0.65rem]'
          }
        >
          <legend className="text-foreground-faint px-1 py-0 text-[0.62rem] font-extrabold tracking-[0.07em] uppercase">
            {t('metadata.describe_options')}
          </legend>
          <div className="mt-[0.4rem] flex items-center gap-[0.45rem]">
            <label
              className="text-foreground-muted m-0 inline-flex cursor-pointer items-center gap-[0.45rem] pb-[0.15rem] text-[0.78rem]"
              htmlFor="auto-apply"
            >
              <input
                id="auto-apply"
                name="auto_apply"
                type="checkbox"
                className="shrink-0"
                checked={autoApply}
                onChange={(e) => onAutoApplyChange(e.target.checked)}
              />
              <span>{t('metadata.describe_auto_apply')}</span>
            </label>
          </div>
        </fieldset>
      </div>
      <ErrorAlert error={error ?? apiError} />
      <div className={cn(modalActionsClass(), 'border-border mt-0 border-t pt-[0.85rem]')}>
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
      </div>
    </>
  )
}
