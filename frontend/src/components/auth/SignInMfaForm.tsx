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
      className="auth-form"
    >
      <h2
        className="auth-title"
        style={{ fontSize: '1.25rem', marginBottom: '0.5rem', textAlign: 'center' }}
      >
        {t('mfa.login_title')}
      </h2>
      <p
        className="auth-subtitle"
        style={{
          fontSize: '0.85rem',
          marginBottom: '1.5rem',
          textAlign: 'center',
          padding: '0 0.5rem',
        }}
      >
        {t('mfa.login_desc')}
      </p>

      {error && (
        <div className="auth-error" role="alert" aria-live="assertive">
          {error}
        </div>
      )}

      <div className="form-group">
        <label className="form-label" htmlFor="mfa-login-code">
          {t('mfa.label_code')}
        </label>
        <input
          id="mfa-login-code"
          type="text"
          pattern="[0-9]*"
          inputMode="numeric"
          maxLength={6}
          className="form-input"
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
        className="auth-btn"
        disabled={mfaLoading || mfaCode.length !== 6}
        style={{ marginTop: '1rem' }}
      >
        {mfaLoading && <div className="spinner" />}
        {t('mfa.login_submit')}
      </button>

      <button
        type="button"
        className="auth-btn"
        style={{
          marginTop: '0.5rem',
          backgroundColor: 'transparent',
          border: '1px solid var(--border)',
          color: 'var(--text-secondary)',
        }}
        onClick={onCancel}
        disabled={mfaLoading}
      >
        {t('mfa.login_cancel')}
      </button>
    </form>
  )
}
