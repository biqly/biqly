import { type SubmitEvent } from 'react'
import { Link } from 'react-router-dom'

import type { useT } from '../../i18n'
import { legacyButtonClass } from '../../lib/buttonClasses'
import { legacyCardClass } from '../../lib/cardClasses'
import { cn } from '../../lib/cn'
import { legacyFeedbackClass } from '../../lib/feedbackClasses'
import { legacyFormClass } from '../../lib/formClasses'
import type { AuthUser } from '../../types/auth'
import { adminBtnAutoWidthClass } from '../admin/adminClasses'
import PasswordStrengthMeter from '../auth/PasswordStrengthMeter'
interface ProfileMessage {
  type: 'success' | 'error'
  text: string
}

export function AccountProfileHero({
  t,
  user,
  displayName,
  profileSaving,
  profileMessage,
  onDisplayNameChange,
  onProfileSubmit,
  onAvatarClick,
  onAvatarRemove,
  fileInputRef,
  onFileChange,
}: {
  t: ReturnType<typeof useT>
  user: AuthUser
  displayName: string
  profileSaving: boolean
  profileMessage: ProfileMessage | null
  onDisplayNameChange: (v: string) => void
  onProfileSubmit: (e: SubmitEvent<HTMLFormElement>) => void
  onAvatarClick: () => void
  onAvatarRemove: () => void
  fileInputRef: React.RefObject<HTMLInputElement | null>
  onFileChange: (e: React.ChangeEvent<HTMLInputElement>) => void
}) {
  const initials = (user.displayName ?? user.email).slice(0, 2).toUpperCase()

  return (
    <section
      className={legacyCardClass('card card--elevated flex flex-col gap-6')}
      aria-labelledby="settings-profile-heading"
    >
      <div className="flex flex-wrap items-center gap-4">
        <div className="relative h-13 w-13">
          <button
            type="button"
            className="group text-accent hover:border-accent relative grid h-full w-full cursor-pointer place-items-center overflow-hidden rounded-full border border-[color-mix(in_srgb,var(--accent)_35%,var(--border))] bg-[color-mix(in_srgb,var(--accent)_18%,transparent)] p-0 transition-all duration-200 hover:shadow-[0_0_12px_rgba(99,102,241,0.25)]"
            onClick={onAvatarClick}
            title={t('settings.profile_picture_change')}
          >
            {user.avatarUrl ? (
              <img src={user.avatarUrl} alt="" className="block h-full w-full object-cover" />
            ) : (
              <span className="text-[0.95rem] font-bold tracking-wider">{initials}</span>
            )}
            <div className="absolute inset-0 flex items-center justify-center bg-black/60 text-white opacity-0 transition-opacity duration-180 group-hover:opacity-100 group-focus-visible:opacity-100">
              <svg
                xmlns="http://www.w3.org/2000/svg"
                width="16"
                height="16"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="2.5"
                strokeLinecap="round"
                strokeLinejoin="round"
                className="lucide lucide-camera"
              >
                <path d="M14.5 4h-5L7 7H4a2 2 0 0 0-2 2v9a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2V9a2 2 0 0 0-2-2h-3l-2.5-3z" />
                <circle cx="12" cy="13" r="3" />
              </svg>
            </div>
          </button>
          {user.avatarUrl && (
            <button
              type="button"
              className={legacyCardClass(
                'bg-card border-border text-foreground-faint hover:text-error hover:border-error absolute -right-1 -bottom-1 flex h-[1.35rem] w-[1.35rem] cursor-pointer items-center justify-center rounded-full border shadow-[0_2px_6px_rgba(0,0,0,0.4)] transition-all duration-180 hover:scale-105 hover:bg-[color-mix(in_srgb,var(--error)_10%,var(--bg-card))]',
              )}
              onClick={onAvatarRemove}
              title={t('settings.profile_picture_remove')}
            >
              <svg
                xmlns="http://www.w3.org/2000/svg"
                width="10"
                height="10"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="2.5"
                strokeLinecap="round"
                strokeLinejoin="round"
                className="lucide lucide-trash-2"
              >
                <path d="M3 6h18" />
                <path d="M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6" />
                <path d="M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2" />
                <line x1="10" x2="10" y1="11" y2="17" />
                <line x1="14" x2="14" y1="11" y2="17" />
              </svg>
            </button>
          )}
          <input
            ref={fileInputRef}
            type="file"
            accept="image/*"
            style={{ display: 'none' }}
            onChange={onFileChange}
          />
        </div>
        <div>
          <h2 id="settings-profile-heading" className="m-0 text-[1.15rem]">
            {t('settings.profile_section')}
          </h2>
          <p className="text-foreground-muted mt-[0.35rem] mr-0 mb-0 ml-0 text-[0.875rem] leading-[1.45]">
            {t('settings.profile_hint')}
          </p>
        </div>
      </div>

      <form
        onSubmit={(e) => void onProfileSubmit(e)}
        className="flex flex-col gap-3"
        style={{ borderTop: 'none', paddingTop: 0 }}
      >
        <h3 className="m-0 text-[0.95rem] font-semibold">{t('settings.profile_name_title')}</h3>
        <div className={legacyFormClass('form-group')}>
          <label htmlFor="settings-display-name">{t('settings.profile_display_name')}</label>
          <input
            id="settings-display-name"
            type="text"
            value={displayName}
            onChange={(e) => onDisplayNameChange(e.target.value)}
            autoComplete="name"
            maxLength={120}
          />
        </div>
        <div className={legacyFormClass('form-group')}>
          <label htmlFor="settings-email-readonly">{t('settings.profile_email')}</label>
          <input
            id="settings-email-readonly"
            type="email"
            value={user.email}
            readOnly
            disabled
            className="disabled:cursor-not-allowed disabled:opacity-85"
          />
        </div>
        {profileMessage && (
          <p
            className={
              profileMessage.type === 'success'
                ? 'text-success m-0 text-[0.875rem]'
                : 'text-error m-0 text-[0.875rem]'
            }
          >
            {profileMessage.text}
          </p>
        )}
        <button
          type="submit"
          className={cn(legacyButtonClass('btn btn-primary btn-sm'), adminBtnAutoWidthClass)}
          disabled={profileSaving}
        >
          {profileSaving ? '...' : t('settings.profile_save')}
        </button>
      </form>
    </section>
  )
}

export function AccountEmailChangeSection({
  t,
  newEmail,
  emailSaving,
  emailMessage,
  onNewEmailChange,
  onSubmit,
}: {
  t: ReturnType<typeof useT>
  newEmail: string
  emailSaving: boolean
  emailMessage: ProfileMessage | null
  onNewEmailChange: (v: string) => void
  onSubmit: (e: SubmitEvent<HTMLFormElement>) => void
}) {
  return (
    <section
      className={legacyCardClass('card card--elevated flex flex-col gap-6')}
      aria-labelledby="settings-email-change-heading"
    >
      <form
        onSubmit={(e) => void onSubmit(e)}
        className="flex flex-col gap-3"
        style={{ borderTop: 'none', paddingTop: 0 }}
      >
        <h3 id="settings-email-change-heading" className="m-0 text-[0.95rem] font-semibold">
          {t('settings.profile_email_change_title')}
        </h3>
        <p className="text-foreground-muted m-0 text-[0.85rem] leading-[1.45]">
          {t('settings.profile_email_change_hint')}
        </p>
        <div className={legacyFormClass('form-group')}>
          <label htmlFor="settings-new-email">{t('settings.profile_new_email')}</label>
          <input
            id="settings-new-email"
            type="email"
            value={newEmail}
            onChange={(e) => onNewEmailChange(e.target.value)}
            autoComplete="email"
            placeholder={t('settings.profile_new_email_placeholder')}
          />
        </div>
        {emailMessage && (
          <p
            className={
              emailMessage.type === 'success'
                ? 'text-success m-0 text-[0.875rem]'
                : 'text-error m-0 text-[0.875rem]'
            }
          >
            {emailMessage.text}
          </p>
        )}
        <button
          type="submit"
          className={cn(legacyButtonClass('btn btn-secondary btn-sm'), adminBtnAutoWidthClass)}
          disabled={emailSaving || !newEmail.trim()}
        >
          {emailSaving ? '...' : t('settings.profile_email_request')}
        </button>
      </form>
    </section>
  )
}

export function AccountPasswordSection({
  t,
  hasPassword,
  currentPassword,
  newPassword,
  confirmPassword,
  passwordSaving,
  passwordMessage,
  onCurrentPasswordChange,
  onNewPasswordChange,
  onConfirmPasswordChange,
  onValidityChange,
  onSubmit,
  onForgotPassword,
}: {
  t: ReturnType<typeof useT>
  hasPassword: boolean
  currentPassword: string
  newPassword: string
  confirmPassword: string
  passwordSaving: boolean
  passwordMessage: ProfileMessage | null
  onCurrentPasswordChange: (v: string) => void
  onNewPasswordChange: (v: string) => void
  onConfirmPasswordChange: (v: string) => void
  onValidityChange: (info: { valid: boolean }) => void
  onSubmit: (e: SubmitEvent<HTMLFormElement>) => void
  onForgotPassword: () => void
}) {
  return (
    <section
      className={legacyCardClass('card card--elevated flex flex-col gap-6')}
      aria-labelledby="settings-password-heading"
    >
      <form
        onSubmit={(e) => void onSubmit(e)}
        className="flex flex-col gap-3"
        style={{ borderTop: 'none', paddingTop: 0 }}
      >
        <h3 id="settings-password-heading" className="m-0 text-[0.95rem] font-semibold">
          {t('settings.profile_password_title')}
        </h3>
        {!hasPassword ? (
          <p className="text-foreground-muted m-0 text-[0.85rem] leading-[1.45]">
            {t('settings.profile_no_password')}
          </p>
        ) : (
          <>
            <div className={legacyFormClass('form-group')}>
              <label htmlFor="settings-current-password">
                {t('settings.profile_current_password')}
              </label>
              <input
                id="settings-current-password"
                type="password"
                value={currentPassword}
                onChange={(e) => onCurrentPasswordChange(e.target.value)}
                autoComplete="current-password"
              />
            </div>
            <div className={legacyFormClass('form-group')}>
              <label htmlFor="settings-new-password">{t('settings.profile_new_password')}</label>
              <input
                id="settings-new-password"
                type="password"
                value={newPassword}
                onChange={(e) => onNewPasswordChange(e.target.value)}
                autoComplete="new-password"
              />
              <PasswordStrengthMeter password={newPassword} onValidityChange={onValidityChange} />
            </div>
            <div className={legacyFormClass('form-group')}>
              <label htmlFor="settings-confirm-password">
                {t('settings.profile_confirm_password')}
              </label>
              <input
                id="settings-confirm-password"
                type="password"
                value={confirmPassword}
                onChange={(e) => onConfirmPasswordChange(e.target.value)}
                autoComplete="new-password"
              />
            </div>
            {passwordMessage && (
              <p
                className={
                  passwordMessage.type === 'success'
                    ? 'text-success m-0 text-[0.875rem]'
                    : 'text-error m-0 text-[0.875rem]'
                }
              >
                {passwordMessage.text}
              </p>
            )}
            <div className="flex flex-wrap items-center gap-x-4 gap-y-3">
              <button
                type="submit"
                className={cn(legacyButtonClass('btn btn-primary btn-sm'), adminBtnAutoWidthClass)}
                disabled={passwordSaving || !currentPassword || !newPassword || !confirmPassword}
              >
                {passwordSaving ? '...' : t('settings.profile_password_save')}
              </button>
              <Link
                to="/auth/forgot-password"
                className="text-accent text-[0.875rem] no-underline hover:underline"
                onClick={onForgotPassword}
              >
                {t('settings.profile_forgot_password')}
              </Link>
            </div>
          </>
        )}
      </form>
    </section>
  )
}

export function AccountMfaBypassSection({
  t,
  bypassGenerating,
  bypassCode,
  bypassError,
  onGenerate,
}: {
  t: ReturnType<typeof useT>
  bypassGenerating: boolean
  bypassCode: string | null
  bypassError: string | null
  onGenerate: () => void
}) {
  return (
    <section
      className={legacyCardClass('card card--elevated flex flex-col gap-6')}
      style={{
        borderColor: 'color-mix(in srgb, var(--warning, #f59e0b) 20%, var(--border))',
        background:
          'linear-gradient(180deg, var(--bg-card) 0%, color-mix(in srgb, var(--warning, #f59e0b) 3%, var(--bg-card)) 100%)',
      }}
      aria-labelledby="settings-support-heading"
    >
      <div className="flex flex-col gap-3" style={{ borderTop: 'none', paddingTop: 0 }}>
        <h3 id="settings-support-heading" className="m-0 text-[0.95rem] font-semibold">
          {t('settings.profile_mfa_bypass_title')}
        </h3>
        <p className="text-foreground-muted m-0 text-[0.85rem] leading-[1.45]">
          {t('settings.profile_mfa_bypass_hint')}
        </p>
        <button
          type="button"
          className={cn(legacyButtonClass('btn btn-secondary btn-sm'), adminBtnAutoWidthClass)}
          onClick={onGenerate}
          disabled={bypassGenerating}
          style={{ display: 'inline-flex', alignItems: 'center', gap: '6px' }}
        >
          {t('settings.profile_mfa_bypass_btn')}
        </button>
        {bypassError && (
          <p className={legacyFeedbackClass('text-error m-0 text-[0.875rem]')}>{bypassError}</p>
        )}
        {bypassCode && (
          <div className="mt-1 flex flex-wrap items-center gap-2">
            <code
              className={`border-border rounded-[0.35rem] border border-dashed bg-white/3 px-3 py-2 font-mono text-[1rem] tracking-wider`}
            >
              {bypassCode}
            </code>
            <button
              type="button"
              className={cn(legacyButtonClass('btn btn-secondary btn-sm'), adminBtnAutoWidthClass)}
              style={{ display: 'inline-flex', alignItems: 'center', gap: '6px' }}
              onClick={() => void navigator.clipboard.writeText(bypassCode)}
            >
              {t('admin.user_detail.copy')}
            </button>
          </div>
        )}
      </div>
    </section>
  )
}
