import type { RefObject, SubmitEvent } from 'react'

import type { TFunction } from '../../i18n'
import {
  authFieldClass,
  authFormClass,
  authInputClass,
  authLabelClass,
  authSubmitBtnClass,
} from '../../lib/authClasses'
import { legacyFeedbackClass } from '../../lib/feedbackClasses'
export function SignInMfaForm({
  t,
  error,
  mfaCode,
  setMfaCode,
  mfaLoading,
  mfaCodeRef,
  onSubmit,
  onCancel,
}: {
  t: TFunction
  error: string | null
  mfaCode: string
  setMfaCode: (value: string) => void
  mfaLoading: boolean
  mfaCodeRef: RefObject<HTMLInputElement | null>
  onSubmit: (e: SubmitEvent<HTMLFormElement>) => void
  onCancel: () => void
}) {
  return (
    <form
      onSubmit={(e) => {
        void onSubmit(e)
      }}
      className={authFormClass}
    >
      <h2 className="text-foreground m-0 mb-2 text-center text-[1.25rem] font-bold">
        {t('mfa.login_title')}
      </h2>
      <p className="text-foreground-muted mb-6 px-2 text-center text-[0.85rem]">
        {t('mfa.login_desc')}
      </p>

      {error && (
        <div
          className={legacyFeedbackClass(
            'bg-error/8 border-error text-error mb-2 rounded border-l-[3px] p-[10px_12px] text-[13px]',
          )}
          role="alert"
          aria-live="assertive"
        >
          {error}
        </div>
      )}

      <div className={authFieldClass}>
        <label className={authLabelClass} htmlFor="mfa-login-code">
          {t('mfa.label_code')}
        </label>
        <input
          id="mfa-login-code"
          type="text"
          pattern="[0-9]*"
          inputMode="numeric"
          maxLength={6}
          className={authInputClass}
          value={mfaCode}
          onChange={(e) => setMfaCode(e.target.value.replace(/\D/g, ''))}
          required
          disabled={mfaLoading}
          ref={mfaCodeRef}
          style={{ letterSpacing: '4px', textAlign: 'center', fontSize: '1.25rem' }}
        />
      </div>

      <button
        type="submit"
        className={authSubmitBtnClass}
        disabled={mfaLoading || mfaCode.length !== 6}
      >
        {mfaLoading && (
          <div className="h-4 w-4 animate-spin rounded-full border-2 border-white/30 border-t-white" />
        )}
        {t('mfa.login_submit')}
      </button>

      <button
        type="button"
        className={`border-border text-foreground-muted mt-2 flex w-full cursor-pointer items-center justify-center gap-2 rounded-lg border bg-transparent px-4 py-2.75 transition-all duration-150 hover:bg-white/5 disabled:cursor-not-allowed disabled:opacity-60`}
        onClick={onCancel}
        disabled={mfaLoading}
      >
        {t('mfa.login_cancel')}
      </button>
    </form>
  )
}
