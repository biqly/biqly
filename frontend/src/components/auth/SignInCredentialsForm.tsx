import type { SubmitEvent } from 'react'

import { useAutofocus } from '../../hooks/useAutofocus'
import type { TFunction } from '../../i18n'
import { legacyCardClass } from '../../lib/cardClasses'
import { legacyFeedbackClass } from '../../lib/feedbackClasses'
export function SignInCredentialsForm({
  t,
  ldapEnabled,
  error,
  throttleMs,
  email,
  setEmail,
  password,
  setPassword,
  loading,
  passkeyLoading,
  onSubmit,
  onForgotPassword,
  onOAuth,
  onPasskeyLogin,
}: {
  t: TFunction
  ldapEnabled: boolean
  error: string | null
  throttleMs: number
  email: string
  setEmail: (value: string) => void
  password: string
  setPassword: (value: string) => void
  loading: boolean
  passkeyLoading: boolean
  onSubmit: (e: SubmitEvent<HTMLFormElement>) => void
  onForgotPassword: () => void
  onOAuth: (provider: string) => void
  onPasskeyLogin: () => void
}) {
  const emailRef = useAutofocus<HTMLInputElement>()

  return (
    <>
      <form
        onSubmit={(e) => {
          void onSubmit(e)
        }}
        className="flex flex-col gap-4"
      >
        {ldapEnabled && (
          <div
            className="p-[10px_12px] bg-accent/8 border-l-[3px] border-accent text-[13px] rounded text-foreground-muted mb-2"
            role="status"
            aria-live="polite"
          >
            {t('auth.ldap_hint')}
          </div>
        )}
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
        {throttleMs > 0 && (
          <div
            className={legacyFeedbackClass(
              'p-[10px_12px] bg-error/8 border-l-[3px] border-error text-error text-[13px] rounded mb-2',
            )}
            role="status"
            aria-live="polite"
          >
            {t('auth.login_throttled')} ({Math.ceil(throttleMs / 1000)}s)
          </div>
        )}

        <div className="flex flex-col gap-1">
          <label className="text-[13px] font-medium text-foreground-muted" htmlFor="email-input">
            {t('auth.email')}
          </label>
          <input
            ref={emailRef}
            id="email-input"
            type="email"
            className={`w-full py-[10px] px-[14px] rounded-lg border border-border bg-[var(--bg-input,#ffffff)] text-foreground text-[14px] transition-all duration-250 focus:outline-none focus:border-accent focus:shadow-[0_0_0_3px_rgba(99,102,241,0.15)]`}
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            required
            disabled={loading || passkeyLoading}
            autoComplete="username"
          />
        </div>

        <div className="flex flex-col gap-1">
          <label className="text-[13px] font-medium text-foreground-muted" htmlFor="password-input">
            {t('auth.password')}
          </label>
          <input
            id="password-input"
            type="password"
            className={`w-full py-[10px] px-[14px] rounded-lg border border-border bg-[var(--bg-input,#ffffff)] text-foreground text-[14px] transition-all duration-250 focus:outline-none focus:border-accent focus:shadow-[0_0_0_3px_rgba(99,102,241,0.15)]`}
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
            disabled={loading || passkeyLoading}
            autoComplete="current-password"
          />
        </div>

        <div className="flex items-center justify-between text-[13px]">
          <label className="flex items-center gap-1.5 cursor-pointer text-foreground-muted">
            <input
              type="checkbox"
              disabled={loading || passkeyLoading}
              className="cursor-pointer"
            />
            {t('auth.remember_me')}
          </label>
          <a
            href="/auth/forgot-password"
            className="text-accent no-underline font-medium hover:underline"
            onClick={(e) => {
              e.preventDefault()
              onForgotPassword()
            }}
          >
            {t('auth.forgot_password')}
          </a>
        </div>

        <button
          type="submit"
          className="flex items-center justify-center gap-2 w-full py-[11px] px-[16px] rounded-lg border-none bg-gradient-to-br from-accent to-[var(--accent-strong)] text-white text-[14px] font-semibold cursor-pointer transition-all duration-150 shadow-[0_4px_10px_rgba(99,102,241,0.2)] hover:opacity-95 hover:-translate-y-[1px] active:translate-y-0 disabled:opacity-60 disabled:cursor-not-allowed disabled:transform-none"
          disabled={loading || passkeyLoading || !email || !password || throttleMs > 0}
        >
          {loading && (
            <div className="w-4 h-4 border-2 border-white/30 rounded-full border-t-white animate-spin" />
          )}
          {t('auth.btn_signin')}
        </button>
      </form>

      <div className="flex items-center text-center text-foreground-muted text-[12px] my-4 before:content-[''] before:flex-1 before:border-b before:border-border before:mr-[0.5em] after:content-[''] after:flex-1 after:border-b after:border-border after:ml-[0.5em]">
        {t('auth.or')}
      </div>

      <div className="flex flex-col gap-2.5">
        <button
          type="button"
          className={legacyCardClass(
            'flex items-center justify-center gap-2.5 w-full py-[10px] px-[16px] rounded-lg border border-border bg-card text-foreground-muted text-[14px] font-medium cursor-pointer transition-colors duration-200 hover:bg-[var(--bg-hover,#f9fafb)] hover:border-foreground-muted dark:bg-white/[0.03] dark:border-white/[0.08] dark:text-[#e2e8f0] dark:hover:bg-white/[0.08] dark:hover:border-white/[0.2]',
          )}
          onClick={() => onOAuth('github')}
          disabled={loading || passkeyLoading}
        >
          <svg className="w-[18px] h-[18px]" viewBox="0 0 24 24" fill="currentColor">
            <path d="M12 .297c-6.63 0-12 5.373-12 12 0 5.303 3.438 9.8 8.205 11.385.6.113.82-.258.82-.577 0-.285-.01-1.04-.015-2.04-3.338.724-4.042-1.61-4.042-1.61C4.422 18.07 3.633 17.7 3.633 17.7c-1.087-.744.084-.729.084-.729 1.205.084 1.838 1.236 1.838 1.236 1.07 1.835 2.809 1.305 3.495.998.108-.776.417-1.305.76-1.605-2.665-.3-5.466-1.332-5.466-5.93 0-1.31.465-2.38 1.235-3.22-.135-.303-.54-1.523.105-3.176 0 0 1.005-.322 3.3 1.23.96-.267 1.98-.399 3-.405 1.02.006 2.04.138 3 .405 2.28-1.552 3.285-1.23 3.285-1.23.645 1.653.24 2.873.12 3.176.765.84 1.23 1.91 1.23 3.22 0 4.61-2.805 5.625-5.475 5.92.42.36.81 1.096.81 2.22 0 1.606-.015 2.896-.015 3.286 0 .315.21.69.825.57C20.565 22.092 24 17.592 24 12.297c0-6.627-5.373-12-12-12" />
          </svg>
          {t('auth.github_continue')}
        </button>

        <button
          type="button"
          className={legacyCardClass(
            'flex items-center justify-center gap-2.5 w-full py-[10px] px-[16px] rounded-lg border border-border bg-card text-foreground-muted text-[14px] font-medium cursor-pointer transition-colors duration-200 hover:bg-[var(--bg-hover,#f9fafb)] hover:border-foreground-muted dark:bg-white/[0.03] dark:border-white/[0.08] dark:text-[#e2e8f0] dark:hover:bg-white/[0.08] dark:hover:border-white/[0.2]',
          )}
          onClick={() => onOAuth('google')}
          disabled={loading || passkeyLoading}
        >
          <svg className="w-[18px] h-[18px]" viewBox="0 0 24 24">
            <path
              fill="#EA4335"
              d="M12 5.04c1.62 0 3.08.56 4.22 1.64l3.15-3.15C17.45 1.74 14.9 1 12 1 7.35 1 3.38 3.67 1.38 7.57l3.85 2.99c.96-2.88 3.66-4.52 6.77-4.52z"
            />
            <path
              fill="#4285F4"
              d="M23.49 12.27c0-.81-.07-1.59-.2-2.34H12v4.44h6.44c-.28 1.47-1.11 2.72-2.36 3.56l3.66 2.84c2.14-1.97 3.38-4.87 3.38-8.5z"
            />
            <path
              fill="#FBBC05"
              d="M5.23 14.44c-.24-.72-.38-1.49-.38-2.29s.14-1.57.38-2.29L1.38 6.86C.5 8.61 0 10.56 0 12.64s.5 4.03 1.38 5.78l3.85-2.99z"
            />
            <path
              fill="#34A853"
              d="M12 23c3.24 0 5.97-1.07 7.96-2.91l-3.66-2.84c-1.1.74-2.5 1.18-4.3 1.18-3.11 0-5.81-1.64-6.77-4.52L1.38 16.9C3.38 20.8 7.35 23 12 23z"
            />
          </svg>
          {t('auth.google_continue')}
        </button>

        <button
          type="button"
          className={`flex items-center justify-center gap-2.5 w-full py-[10px] px-[16px] rounded-lg border border-accent/30 bg-accent/[0.03] text-foreground-muted text-[14px] font-medium cursor-pointer transition-colors duration-200 hover:bg-accent/[0.08] hover:border-accent/50 dark:text-[#e2e8f0] ${passkeyLoading ? 'animate-pulse' : ''}`}
          onClick={() => {
            void onPasskeyLogin()
          }}
          disabled={loading || passkeyLoading}
        >
          🔑 {t('auth.passkey_continue')}
        </button>
      </div>
    </>
  )
}
