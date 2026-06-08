import { type SubmitEvent } from 'react'
import { Link } from 'react-router-dom'

import type { useT } from '../../i18n'
import type { AuthUser } from '../../types/auth'
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
      className="card card--elevated settings-profile-card"
      aria-labelledby="settings-profile-heading"
    >
      <div className="settings-profile-card__hero">
        <div className="settings-profile-avatar-container">
          <button
            type="button"
            className="settings-profile-avatar-button"
            onClick={onAvatarClick}
            title={t('settings.profile_picture_change')}
          >
            {user.avatarUrl ? (
              <img src={user.avatarUrl} alt="" className="settings-profile-avatar-img" />
            ) : (
              <span className="settings-profile-avatar-initials">{initials}</span>
            )}
            <div className="settings-profile-avatar-overlay">
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
              className="settings-profile-avatar-remove"
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
          <h2 id="settings-profile-heading" className="settings-profile-card__title">
            {t('settings.profile_section')}
          </h2>
          <p className="settings-profile-card__subtitle">{t('settings.profile_hint')}</p>
        </div>
      </div>

      <form
        onSubmit={(e) => void onProfileSubmit(e)}
        className="settings-profile-block"
        style={{ borderTop: 'none', paddingTop: 0 }}
      >
        <h3 className="settings-profile-block__title">{t('settings.profile_name_title')}</h3>
        <div className="form-group">
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
        <div className="form-group">
          <label htmlFor="settings-email-readonly">{t('settings.profile_email')}</label>
          <input
            id="settings-email-readonly"
            type="email"
            value={user.email}
            readOnly
            disabled
            className="input-readonly"
          />
        </div>
        {profileMessage && (
          <p
            className={
              profileMessage.type === 'success'
                ? 'settings-inline-success'
                : 'settings-inline-error'
            }
          >
            {profileMessage.text}
          </p>
        )}
        <button
          type="submit"
          className="btn btn-primary btn-sm btn-auto-width"
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
      className="card card--elevated settings-profile-card"
      aria-labelledby="settings-email-change-heading"
    >
      <form
        onSubmit={(e) => void onSubmit(e)}
        className="settings-profile-block"
        style={{ borderTop: 'none', paddingTop: 0 }}
      >
        <h3 id="settings-email-change-heading" className="settings-profile-block__title">
          {t('settings.profile_email_change_title')}
        </h3>
        <p className="settings-profile-block__hint">{t('settings.profile_email_change_hint')}</p>
        <div className="form-group">
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
              emailMessage.type === 'success' ? 'settings-inline-success' : 'settings-inline-error'
            }
          >
            {emailMessage.text}
          </p>
        )}
        <button
          type="submit"
          className="btn btn-secondary btn-sm btn-auto-width"
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
      className="card card--elevated settings-profile-card"
      aria-labelledby="settings-password-heading"
    >
      <form
        onSubmit={(e) => void onSubmit(e)}
        className="settings-profile-block"
        style={{ borderTop: 'none', paddingTop: 0 }}
      >
        <h3 id="settings-password-heading" className="settings-profile-block__title">
          {t('settings.profile_password_title')}
        </h3>
        {!hasPassword ? (
          <p className="settings-profile-block__hint">{t('settings.profile_no_password')}</p>
        ) : (
          <>
            <div className="form-group">
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
            <div className="form-group">
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
            <div className="form-group">
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
                    ? 'settings-inline-success'
                    : 'settings-inline-error'
                }
              >
                {passwordMessage.text}
              </p>
            )}
            <div className="settings-profile-actions">
              <button
                type="submit"
                className="btn btn-primary btn-sm btn-auto-width"
                disabled={passwordSaving || !currentPassword || !newPassword || !confirmPassword}
              >
                {passwordSaving ? '...' : t('settings.profile_password_save')}
              </button>
              <Link
                to="/auth/forgot-password"
                className="settings-profile-link"
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
      className="card card--elevated settings-profile-card settings-profile-card--support-card"
      aria-labelledby="settings-support-heading"
    >
      <div className="settings-profile-block" style={{ borderTop: 'none', paddingTop: 0 }}>
        <h3 id="settings-support-heading" className="settings-profile-block__title">
          {t('settings.profile_mfa_bypass_title')}
        </h3>
        <p className="settings-profile-block__hint">{t('settings.profile_mfa_bypass_hint')}</p>
        <button
          type="button"
          className="btn btn-secondary btn-sm btn-auto-width"
          onClick={onGenerate}
          disabled={bypassGenerating}
          style={{ display: 'inline-flex', alignItems: 'center', gap: '6px' }}
        >
          {t('settings.profile_mfa_bypass_btn')}
        </button>
        {bypassError && <p className="settings-inline-error">{bypassError}</p>}
        {bypassCode && (
          <div className="settings-bypass-code">
            <code>{bypassCode}</code>
            <button
              type="button"
              className="btn btn-secondary btn-sm btn-auto-width"
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
