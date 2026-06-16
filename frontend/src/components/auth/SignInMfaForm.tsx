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
      <h2 className="m-0 mb-2 text-center text-[1.25rem] font-bold text-foreground">
        {t('mfa.login_title')}
      </h2>
      <p className="mb-6 px-2 text-center text-[0.85rem] text-foreground-muted">
        {t('mfa.login_desc')}
      </p>

      {error && (
        <div
          className={legacyFeedbackClass(
            'p-[10px_12px] bg-error/8 border-l-[3px] border-error text-error text-[13px] rounded mb-2',
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
        className={`flex items-center justify-center gap-2 w-full py-2.75 px-4 rounded-lg cursor-pointer transition-all duration-150 mt-2 bg-transparent border border-border text-foreground-muted hover:bg-white/5 disabled:opacity-60 disabled:cursor-not-allowed`}
        onClick={onCancel}
        disabled={mfaLoading}
      >
        {t('mfa.login_cancel')}
      </button>
    </form>
  )
}
