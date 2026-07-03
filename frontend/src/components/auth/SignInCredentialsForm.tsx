import type { SubmitEvent } from 'react'

import { useAutofocus } from '../../hooks/useAutofocus'
import type { TFunction } from '../../i18n'
import {
  authFieldClass,
  authFormClass,
  authInlineLinkClass,
  authInputClass,
  authLabelClass,
  authOAuthBtnClass,
  authSubmitBtnClass,
} from '../../lib/authClasses'
import { cn } from '../../lib/cn'
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
        className={authFormClass}
      >
        {ldapEnabled && (
          <div
            className="border-accent bg-accent/8 text-foreground-muted text-caption mb-2 rounded border-l-[3px] p-[10px_12px]"
            role="status"
            aria-live="polite"
          >
            {t('auth.ldap_hint')}
          </div>
        )}
        {error && (
          <div
            className={legacyFeedbackClass(
              'bg-error/8 border-error text-error text-caption mb-2 rounded border-l-[3px] p-[10px_12px]',
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
              'bg-error/8 border-error text-error text-caption mb-2 rounded border-l-[3px] p-[10px_12px]',
            )}
            role="status"
            aria-live="polite"
          >
            {t('auth.login_throttled')} ({Math.ceil(throttleMs / 1000)}s)
          </div>
        )}

        <div className={authFieldClass}>
          <label className={authLabelClass} htmlFor="email-input">
            {t('auth.email')}
          </label>
          <input
            ref={emailRef}
            id="email-input"
            type="email"
            className={authInputClass}
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            required
            disabled={loading || passkeyLoading}
            autoComplete="username"
            spellCheck={false}
          />
        </div>

        <div className={authFieldClass}>
          <label className={authLabelClass} htmlFor="password-input">
            {t('auth.password')}
          </label>
          <input
            id="password-input"
            type="password"
            className={authInputClass}
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
            disabled={loading || passkeyLoading}
            autoComplete="current-password"
          />
        </div>

        <div className="text-caption flex items-center justify-end">
          <a
            href="/auth/forgot-password"
            className={authInlineLinkClass}
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
          className={authSubmitBtnClass}
          disabled={loading || passkeyLoading || !email || !password || throttleMs > 0}
        >
          {loading && (
            <div className="h-4 w-4 animate-spin rounded-full border-2 border-white/30 border-t-white" />
          )}
          {t('auth.btn_signin')}
        </button>
      </form>

      <div className="text-foreground-muted before:border-border after:border-border my-4 flex items-center text-center text-xs before:mr-[0.5em] before:flex-1 before:border-b before:content-[''] after:ml-[0.5em] after:flex-1 after:border-b after:content-['']">
        {t('auth.or')}
      </div>

      <div className="flex flex-col gap-2.5">
        <button
          type="button"
          className={authOAuthBtnClass}
          onClick={() => onOAuth('github')}
          disabled={loading || passkeyLoading}
        >
          <svg className="h-4.5 w-4.5" viewBox="0 0 24 24" fill="currentColor">
            <path d="M12 .297c-6.63 0-12 5.373-12 12 0 5.303 3.438 9.8 8.205 11.385.6.113.82-.258.82-.577 0-.285-.01-1.04-.015-2.04-3.338.724-4.042-1.61-4.042-1.61C4.422 18.07 3.633 17.7 3.633 17.7c-1.087-.744.084-.729.084-.729 1.205.084 1.838 1.236 1.838 1.236 1.07 1.835 2.809 1.305 3.495.998.108-.776.417-1.305.76-1.605-2.665-.3-5.466-1.332-5.466-5.93 0-1.31.465-2.38 1.235-3.22-.135-.303-.54-1.523.105-3.176 0 0 1.005-.322 3.3 1.23.96-.267 1.98-.399 3-.405 1.02.006 2.04.138 3 .405 2.28-1.552 3.285-1.23 3.285-1.23.645 1.653.24 2.873.12 3.176.765.84 1.23 1.91 1.23 3.22 0 4.61-2.805 5.625-5.475 5.92.42.36.81 1.096.81 2.22 0 1.606-.015 2.896-.015 3.286 0 .315.21.69.825.57C20.565 22.092 24 17.592 24 12.297c0-6.627-5.373-12-12-12" />
          </svg>
          {t('auth.github_continue')}
        </button>

        <button
          type="button"
          className={authOAuthBtnClass}
          onClick={() => onOAuth('google')}
          disabled={loading || passkeyLoading}
        >
          <svg className="h-4.5 w-4.5" viewBox="0 0 24 24">
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
          className={cn(
            authOAuthBtnClass,
            'border-accent/30 bg-accent/3 hover:bg-accent/8 hover:border-accent/50',
            passkeyLoading && 'animate-pulse',
          )}
          onClick={() => {
            void onPasskeyLogin()
          }}
          disabled={loading || passkeyLoading}
        >
          <svg
            width="16"
            height="16"
            viewBox="0 0 24 24"
            aria-hidden="true"
            focusable="false"
            className="shrink-0"
          >
            <path
              fill="currentColor"
              d="M14 2a6 6 0 0 0-5.83 7.42L2 15.59V20a2 2 0 0 0 2 2h3v-2h2v-2h2l1.58-1.58A6 6 0 1 0 14 2Zm2.5 5.5a1.5 1.5 0 1 1 0-3 1.5 1.5 0 0 1 0 3Z"
            />
          </svg>
          {t('auth.passkey_continue')}
        </button>
      </div>
    </>
  )
}
