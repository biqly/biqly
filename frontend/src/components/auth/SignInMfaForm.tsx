import type { RefObject, SubmitEvent } from 'react'

import type { TFunction } from '../../i18n'

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
      className="flex flex-col gap-4"
    >
      <h2 className="m-0 text-[1.25rem] font-bold text-foreground text-center mb-2">
        {t('mfa.login_title')}
      </h2>
      <p className="text-[0.85rem] text-foreground-muted text-center mb-6 px-2">
        {t('mfa.login_desc')}
      </p>

      {error && (
        <div
          className="p-[10px_12px] bg-error/8 border-l-[3px] border-error text-error text-[13px] rounded mb-2"
          role="alert"
          aria-live="assertive"
        >
          {error}
        </div>
      )}

      <div className="flex flex-col gap-1">
        <label className="text-[13px] font-medium text-foreground-muted" htmlFor="mfa-login-code">
          {t('mfa.label_code')}
        </label>
        <input
          id="mfa-login-code"
          type="text"
          pattern="[0-9]*"
          inputMode="numeric"
          maxLength={6}
          className={`w-full py-[10px] px-[14px] rounded-lg border border-border bg-[var(--bg-input,#ffffff)] text-foreground text-[14px] transition-all duration-250 focus:outline-none focus:border-accent focus:shadow-[0_0_0_3px_rgba(99,102,241,0.15)]`}
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
        className="flex items-center justify-center gap-2 w-full py-[11px] px-[16px] rounded-lg border-none bg-gradient-to-br from-accent to-[var(--accent-strong)] text-white text-[14px] font-semibold cursor-pointer transition-all duration-150 shadow-[0_4px_10px_rgba(99,102,241,0.2)] hover:opacity-95 hover:-translate-y-[1px] active:translate-y-0 disabled:opacity-60 disabled:cursor-not-allowed disabled:transform-none mt-4"
        disabled={mfaLoading || mfaCode.length !== 6}
      >
        {mfaLoading && (
          <div className="w-4 h-4 border-2 border-white/30 rounded-full border-t-white animate-spin" />
        )}
        {t('mfa.login_submit')}
      </button>

      <button
        type="button"
        className={`flex items-center justify-center gap-2 w-full py-[11px] px-[16px] rounded-lg cursor-pointer transition-all duration-150 mt-2 bg-transparent border border-border text-foreground-muted hover:bg-white/5 disabled:opacity-60 disabled:cursor-not-allowed`}
        onClick={onCancel}
        disabled={mfaLoading}
      >
        {t('mfa.login_cancel')}
      </button>
    </form>
  )
}
