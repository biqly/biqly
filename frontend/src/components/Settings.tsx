import QRCode from 'qrcode'
import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'

import {
  apiDeletePasskey,
  apiGetPasskeys,
  apiMFADisable,
  apiMFAEnroll,
  apiMFARegenerateRecovery,
  apiMFAStatus,
  apiMFAVerify,
  apiPasskeyRename,
} from '../api/auth'
import { usePasskeyRegistration } from '../hooks/usePasskeyRegistration'
import { localeLanguageTag, useLocale, useT } from '../i18n'
import type { PasskeyInfo } from '../types/auth'
import { useAuth } from './auth/AuthProvider'
import { AccountProfileSection } from './settings/AccountProfileSection'
import { AIModelPreferencesSection } from './settings/AIModelPreferencesSection'
import { MFASection, type MFAStatus } from './settings/MFASection'
import { OTPCodeInput } from './settings/OTPCodeInput'
import { PasskeyTable } from './settings/PasskeyTable'
import { RecoveryCodesDisplay } from './settings/RecoveryCodesDisplay'
import { SettingsLinkCard } from './settings/SettingsLinkCard'
import { ConfirmDialog } from './ui/ConfirmDialog'
import { ErrorAlert } from './ui/ErrorAlert'
import { Modal } from './ui/Modal'

export default function Settings() {
  const navigate = useNavigate()
  const t = useT()
  const [locale] = useLocale()
  const { accessToken } = useAuth()

  const [passkeys, setPasskeys] = useState<PasskeyInfo[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [successMessage, setSuccessMessage] = useState<string | null>(null)

  // Deletion States
  const [deleteTarget, setDeleteTarget] = useState<PasskeyInfo | null>(null)
  const [deleting, setDeleting] = useState(false)

  // Registration States
  const [addModalOpen, setAddModalOpen] = useState(false)
  const [newPasskeyName, setNewPasskeyName] = useState('')

  // Renaming States
  const [renameTarget, setRenameTarget] = useState<PasskeyInfo | null>(null)
  const [renamingName, setRenamingName] = useState('')
  const [renaming, setRenaming] = useState(false)

  // Multi-Factor Authentication (MFA) States
  const [mfaStatus, setMfaStatus] = useState<MFAStatus | null>(null)
  const [mfaEnrollData, setMfaEnrollData] = useState<{
    secret: string
    otpauth_url: string
    recovery_codes: string[]
  } | null>(null)
  const [mfaQrCode, setMfaQrCode] = useState('')
  const [mfaEnrollOpen, setMfaEnrollOpen] = useState(false)
  const [mfaVerifyCode, setMfaVerifyCode] = useState('')
  const [mfaVerifying, setMfaVerifying] = useState(false)
  const [mfaShowRecovery, setMfaShowRecovery] = useState(false)
  const [mfaDisableOpen, setMfaDisableOpen] = useState(false)
  const [mfaDisableCode, setMfaDisableCode] = useState('')
  const [mfaDisabling, setMfaDisabling] = useState(false)
  const [mfaRegenOpen, setMfaRegenOpen] = useState(false)
  const [mfaRegenCode, setMfaRegenCode] = useState('')
  const [mfaRegening, setMfaRegening] = useState(false)
  const [mfaNewRecoveryCodes, setMfaNewRecoveryCodes] = useState<string[] | null>(null)
  const {
    registering,
    error: registrationError,
    setError: setRegistrationError,
    registerPasskey,
  } = usePasskeyRegistration(accessToken ?? '')

  useEffect(() => {
    if (registrationError) {
      setError(registrationError)
    }
  }, [registrationError])

  const goTo = (path: string) => {
    void navigate(path)
  }

  const fetchPasskeys = async () => {
    if (!accessToken) {
      return
    }
    setLoading(true)
    setError(null)
    try {
      const data = await apiGetPasskeys(accessToken)
      setPasskeys(data || [])
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to load passkeys')
    } finally {
      setLoading(false)
    }
  }

  const fetchMFAStatus = async () => {
    if (!accessToken) {
      return
    }
    try {
      const status = await apiMFAStatus(accessToken)
      setMfaStatus(status)
    } catch (err: unknown) {
      console.error('Failed to load MFA status', err)
    }
  }

  useEffect(() => {
    void fetchPasskeys()
    void fetchMFAStatus()
  }, [accessToken])

  const handleDeleteConfirm = async () => {
    if (!deleteTarget || !accessToken) {
      return
    }
    setDeleting(true)
    setError(null)
    setSuccessMessage(null)
    try {
      await apiDeletePasskey(accessToken, deleteTarget.id)
      setSuccessMessage(t('passkeys.success_delete'))
      setDeleteTarget(null)
      await fetchPasskeys()
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to delete passkey')
    } finally {
      setDeleting(false)
    }
  }

  const openAddModal = () => {
    const defaultName =
      t('passkeys.title') + ' ' + new Date().toLocaleDateString(localeLanguageTag(locale))
    setNewPasskeyName(defaultName)
    setError(null)
    setRegistrationError(null)
    setSuccessMessage(null)
    setAddModalOpen(true)
  }

  const handleRegisterSubmit = async (e: React.SubmitEvent<HTMLFormElement>) => {
    e.preventDefault()
    if (!accessToken) {
      return
    }

    setError(null)
    setRegistrationError(null)
    setSuccessMessage(null)

    const ok = await registerPasskey(newPasskeyName)
    if (ok) {
      setSuccessMessage(t('passkeys.success_register'))
      setAddModalOpen(false)
      await fetchPasskeys()
    }
  }

  const handleRenameSubmit = async (e: React.SubmitEvent<HTMLFormElement>) => {
    e.preventDefault()
    if (!renameTarget || !renamingName.trim() || renaming || !accessToken) {
      return
    }

    setRenaming(true)
    setError(null)
    setSuccessMessage(null)
    try {
      await apiPasskeyRename(accessToken, renameTarget.id, renamingName.trim())
      setSuccessMessage(t('passkeys.success_rename'))
      setRenameTarget(null)
      await fetchPasskeys()
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to rename passkey')
    } finally {
      setRenaming(false)
    }
  }

  const handleMFAEnrollStart = async () => {
    if (!accessToken) {
      return
    }
    setError(null)
    setSuccessMessage(null)
    setMfaNewRecoveryCodes(null)
    try {
      const enroll = await apiMFAEnroll(accessToken)
      setMfaEnrollData(enroll)
      setMfaVerifyCode('')
      setMfaShowRecovery(false)

      const qrDataUrl = await QRCode.toDataURL(enroll.otpauth_url)
      setMfaQrCode(qrDataUrl)

      setMfaEnrollOpen(true)
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'MFA enrollment failed')
    }
  }

  const handleMFAVerifySubmit = async (e: React.SubmitEvent<HTMLFormElement>) => {
    e.preventDefault()
    if (!accessToken || !mfaVerifyCode.trim() || mfaVerifying) {
      return
    }
    setMfaVerifying(true)
    setError(null)
    try {
      await apiMFAVerify(accessToken, mfaVerifyCode.trim())
      setSuccessMessage(t('mfa.success_enabled'))
      setMfaShowRecovery(true)
      await fetchMFAStatus()
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'MFA verification failed')
    } finally {
      setMfaVerifying(false)
    }
  }

  const handleMFADisableSubmit = async (e: React.SubmitEvent<HTMLFormElement>) => {
    e.preventDefault()
    if (!accessToken || !mfaDisableCode.trim() || mfaDisabling) {
      return
    }
    setMfaDisabling(true)
    setError(null)
    setSuccessMessage(null)
    try {
      await apiMFADisable(accessToken, mfaDisableCode.trim())
      setSuccessMessage(t('mfa.success_disabled'))
      setMfaDisableOpen(false)
      setMfaDisableCode('')
      setMfaNewRecoveryCodes(null)
      await fetchMFAStatus()
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to disable MFA')
    } finally {
      setMfaDisabling(false)
    }
  }

  const handleMFARegenSubmit = async (e: React.SubmitEvent<HTMLFormElement>) => {
    e.preventDefault()
    if (!accessToken || !mfaRegenCode.trim() || mfaRegening) {
      return
    }
    setMfaRegening(true)
    setError(null)
    setSuccessMessage(null)
    try {
      const resp = await apiMFARegenerateRecovery(accessToken, mfaRegenCode.trim())
      setMfaNewRecoveryCodes(resp.recovery_codes)
      setSuccessMessage(t('mfa.success_regenerate'))
      setMfaRegenOpen(false)
      setMfaRegenCode('')
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to regenerate recovery codes')
    } finally {
      setMfaRegening(false)
    }
  }

  const openMFADisableModal = () => {
    setMfaDisableCode('')
    setError(null)
    setSuccessMessage(null)
    setMfaDisableOpen(true)
  }

  const openMFARegenModal = () => {
    setMfaRegenCode('')
    setError(null)
    setSuccessMessage(null)
    setMfaRegenOpen(true)
  }

  return (
    <div className="settings-page">
      {(error ?? successMessage) && (
        <div className="settings-alerts">
          {error && (
            <div className="card-lead-margin" style={{ margin: 0 }}>
              <ErrorAlert error={error} />
            </div>
          )}
          {successMessage && (
            <div className="admin-alert-success">
              <div>🎉 {successMessage}</div>
              <button
                type="button"
                className="admin-alert-close-btn"
                onClick={() => setSuccessMessage(null)}
              >
                ×
              </button>
            </div>
          )}
        </div>
      )}

      <div className="settings-layout">
        <aside
          className="settings-layout__profile"
          aria-labelledby="settings-profile-group-heading"
        >
          <h2 id="settings-profile-group-heading" className="settings-section-group__title">
            {t('settings.profile_group')}
          </h2>
          <AccountProfileSection />
        </aside>

        <div className="settings-layout__main">
          <section className="settings-section-group" aria-labelledby="settings-security-heading">
            <h2 id="settings-security-heading" className="settings-section-group__title">
              {t('settings.security_group')}
            </h2>
            <div className="settings-security-grid">
              <MFASection
                className="settings-security-card"
                status={mfaStatus}
                recoveryCodes={mfaNewRecoveryCodes}
                onEnable={() => {
                  void handleMFAEnrollStart()
                }}
                onDisable={openMFADisableModal}
                onRegenerate={openMFARegenModal}
              />

              <section
                className="card card--elevated settings-prefs-card settings-security-card"
                aria-labelledby="passkeys-heading"
              >
                <div className="settings-prefs-card__header">
                  <div>
                    <h2 id="passkeys-heading">{t('passkeys.title')}</h2>
                    <p>{t('passkeys.subtitle')}</p>
                  </div>
                  <button
                    type="button"
                    className="btn btn-primary btn-sm btn-auto-width"
                    onClick={openAddModal}
                    style={{ display: 'inline-flex', alignItems: 'center', gap: '6px' }}
                  >
                    <svg
                      xmlns="http://www.w3.org/2000/svg"
                      width="14"
                      height="14"
                      viewBox="0 0 24 24"
                      fill="none"
                      stroke="currentColor"
                      strokeWidth="2.5"
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      className="lucide lucide-fingerprint"
                    >
                      <path d="M2 12a10 10 0 0 1 13-9.54" />
                      <path d="M22 12c0 2.2-1.4 4.5-2.5 6.5" />
                      <path d="M12 12.5a3.5 3.5 0 1 0 7 0c0-1.8-1.5-3.5-3.5-3.5" />
                      <path d="M9.5 12a6.5 6.5 0 0 1 13 0c0 3.2-2.2 6.1-3.6 8.5" />
                      <path d="M5.8 12a9.4 9.4 0 0 1 17.6-.8" />
                      <path d="M8 15a4.5 4.5 0 0 0 9 0" />
                    </svg>
                    {t('passkeys.add_btn')}
                  </button>
                </div>

                <div className="settings-passkeys-scroll-wrapper">
                  <PasskeyTable
                    passkeys={passkeys}
                    loading={loading}
                    locale={locale}
                    onRename={(passkey) => {
                      setRenameTarget(passkey)
                      setRenamingName(passkey.name)
                    }}
                    onDelete={setDeleteTarget}
                  />
                </div>
              </section>
            </div>
          </section>

          <section className="settings-section-group" aria-labelledby="settings-config-heading">
            <h2 id="settings-config-heading" className="settings-section-group__title">
              {t('settings.configuration_group')}
            </h2>
            <AIModelPreferencesSection />
            <div className="settings-link-grid">
              <SettingsLinkCard
                title={t('settings.prompt_templates_section')}
                description={t('settings.prompt_templates_hint')}
                icon={
                  <svg
                    xmlns="http://www.w3.org/2000/svg"
                    width="22"
                    height="22"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="2"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    className="lucide lucide-file-text"
                    style={{ color: 'var(--accent)' }}
                  >
                    <path d="M15 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7Z" />
                    <path d="M14 2v4a2 2 0 0 0 2 2h4" />
                    <path d="M10 9H8" />
                    <path d="M16 13H8" />
                    <path d="M16 17H8" />
                  </svg>
                }
                action={
                  <button
                    type="button"
                    className="btn btn-primary btn-sm btn-auto-width"
                    onClick={() => goTo('/prompt-templates')}
                  >
                    {t('settings.prompt_templates_open')}
                  </button>
                }
              />
              <SettingsLinkCard
                title={t('settings.time_grains_section')}
                description={t('settings.time_grains_hint')}
                icon={
                  <svg
                    xmlns="http://www.w3.org/2000/svg"
                    width="22"
                    height="22"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="2"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    className="lucide lucide-calendar-days"
                    style={{ color: 'var(--success)' }}
                  >
                    <rect width="18" height="18" x="3" y="4" rx="2" ry="2" />
                    <line x1="16" x2="16" y1="2" y2="6" />
                    <line x1="8" x2="8" y1="2" y2="6" />
                    <line x1="3" x2="21" y1="10" y2="10" />
                    <path d="M8 14h.01" />
                    <path d="M12 14h.01" />
                    <path d="M16 14h.01" />
                    <path d="M8 18h.01" />
                    <path d="M12 18h.01" />
                    <path d="M16 18h.01" />
                  </svg>
                }
                action={
                  <button
                    type="button"
                    className="btn btn-primary btn-sm btn-auto-width"
                    onClick={() => goTo('/time-grains')}
                  >
                    {t('settings.time_grains_open')}
                  </button>
                }
              />
              <SettingsLinkCard
                title={t('settings.ai_config_section')}
                description={t('settings.ai_config_hint')}
                icon={
                  <svg
                    xmlns="http://www.w3.org/2000/svg"
                    width="22"
                    height="22"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="2"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    className="lucide lucide-cpu"
                    style={{ color: 'var(--warning)' }}
                  >
                    <rect width="16" height="16" x="4" y="4" rx="2" />
                    <rect width="6" height="6" x="9" y="9" rx="1" />
                    <path d="M9 1v3" />
                    <path d="M15 1v3" />
                    <path d="M9 20v3" />
                    <path d="M15 20v3" />
                    <path d="M20 9h3" />
                    <path d="M20 15h3" />
                    <path d="M1 9h3" />
                    <path d="M1 15h3" />
                  </svg>
                }
                action={
                  <button
                    type="button"
                    className="btn btn-primary btn-sm btn-auto-width"
                    onClick={() => goTo('/admin?tab=ai_providers')}
                  >
                    {t('settings.ai_config_open')}
                  </button>
                }
              />
            </div>
          </section>

          <p className="settings-footnote">{t('settings.persist_hint')}</p>
        </div>
      </div>

      {/* Delete Confirmation Dialog */}
      <ConfirmDialog
        open={deleteTarget !== null}
        title={t('passkeys.delete_title')}
        message={t('passkeys.delete_confirm')}
        confirmLabel={deleting ? '...' : undefined}
        onConfirm={() => {
          void handleDeleteConfirm()
        }}
        onCancel={() => setDeleteTarget(null)}
      />

      {/* Register Passkey Prompt Name Modal */}
      <Modal
        open={addModalOpen}
        title={t('passkeys.modal_title')}
        subtitle={t('passkeys.modal_desc')}
        onClose={() => setAddModalOpen(false)}
      >
        <form
          onSubmit={(e) => {
            void handleRegisterSubmit(e)
          }}
          className="page-stack"
          style={{ gap: '1rem' }}
        >
          <div className="form-group" style={{ margin: 0 }}>
            <label htmlFor="passkey-name">{t('passkeys.modal_label_name')}</label>
            <input
              id="passkey-name"
              type="text"
              required
              value={newPasskeyName}
              onChange={(e) => setNewPasskeyName(e.target.value)}
              placeholder={t('passkeys.modal_placeholder_name')}
              disabled={registering}
              autoFocus
            />
          </div>
          <div className="flex-gap-center-end" style={{ marginTop: '0.5rem' }}>
            <button
              type="button"
              className="btn btn-secondary btn-auto-width"
              onClick={() => setAddModalOpen(false)}
              disabled={registering}
            >
              {t('common.confirm_cancel')}
            </button>
            <button
              type="submit"
              className="btn btn-primary btn-auto-width"
              style={{ display: 'inline-flex', alignItems: 'center', gap: '6px' }}
              disabled={registering || !newPasskeyName.trim()}
            >
              {registering ? (
                <>
                  <div
                    className="spinner"
                    style={{ width: '12px', height: '12px', borderTopColor: '#fff', margin: 0 }}
                  ></div>
                  {t('passkeys.modal_submit')}...
                </>
              ) : (
                t('passkeys.modal_submit')
              )}
            </button>
          </div>
        </form>
      </Modal>

      {/* Rename Passkey Modal */}
      <Modal
        open={renameTarget !== null}
        title={t('passkeys.rename_title')}
        subtitle={t('passkeys.rename_desc')}
        onClose={() => setRenameTarget(null)}
      >
        <form
          onSubmit={(e) => {
            void handleRenameSubmit(e)
          }}
          className="page-stack"
          style={{ gap: '1rem' }}
        >
          <div className="form-group" style={{ margin: 0 }}>
            <label htmlFor="rename-passkey-name">{t('passkeys.modal_label_name')}</label>
            <input
              id="rename-passkey-name"
              type="text"
              required
              value={renamingName}
              onChange={(e) => setRenamingName(e.target.value)}
              placeholder={t('passkeys.modal_placeholder_name')}
              disabled={renaming}
              autoFocus
            />
          </div>
          <div className="flex-gap-center-end" style={{ marginTop: '0.5rem' }}>
            <button
              type="button"
              className="btn btn-secondary btn-auto-width"
              onClick={() => setRenameTarget(null)}
              disabled={renaming}
            >
              {t('common.confirm_cancel')}
            </button>
            <button
              type="submit"
              className="btn btn-primary btn-auto-width"
              style={{ display: 'inline-flex', alignItems: 'center', gap: '6px' }}
              disabled={renaming || !renamingName.trim()}
            >
              {renaming ? (
                <>
                  <div
                    className="spinner"
                    style={{ width: '12px', height: '12px', borderTopColor: '#fff', margin: 0 }}
                  ></div>
                  {t('passkeys.modal_submit')}...
                </>
              ) : (
                t('passkeys.modal_submit')
              )}
            </button>
          </div>
        </form>
      </Modal>

      {/* 2FA Enrollment Modal */}
      <Modal
        open={mfaEnrollOpen}
        title={t('mfa.modal_enroll_title')}
        subtitle={t('mfa.modal_enroll_desc')}
        onClose={() => setMfaEnrollOpen(false)}
      >
        <div className="page-stack" style={{ gap: '1.5rem' }}>
          {!mfaShowRecovery ? (
            <form
              onSubmit={(e) => {
                void handleMFAVerifySubmit(e)
              }}
              className="page-stack"
              style={{ gap: '1rem' }}
            >
              <div style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem' }}>
                <h4 style={{ margin: 0 }}>{t('mfa.step_scan')}</h4>
                <p style={{ margin: 0, fontSize: '0.85rem', color: 'var(--text-secondary)' }}>
                  {t('mfa.step_scan_desc')}
                </p>
              </div>

              {mfaQrCode && (
                <div className="mfa-qr-container">
                  <img
                    src={mfaQrCode}
                    alt="2FA QR Code"
                    style={{ display: 'block', width: '180px', height: '180px' }}
                  />
                </div>
              )}

              {mfaEnrollData && (
                <div
                  style={{
                    fontSize: '0.85rem',
                    backgroundColor: 'rgba(255,255,255,0.02)',
                    padding: '0.75rem',
                    borderRadius: '0.25rem',
                    border: '1px dashed var(--border)',
                  }}
                >
                  <strong>{t('mfa.step_manual')}</strong>
                  <div
                    style={{
                      fontFamily: 'monospace',
                      fontSize: '1rem',
                      color: 'var(--accent)',
                      marginTop: '0.25rem',
                      letterSpacing: '1px',
                    }}
                  >
                    {mfaEnrollData.secret}
                  </div>
                </div>
              )}

              <hr style={{ border: 0, borderTop: '1px solid var(--border)', margin: '0.5rem 0' }} />

              <div style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem' }}>
                <h4 style={{ margin: 0 }}>{t('mfa.step_verify')}</h4>
                <p style={{ margin: 0, fontSize: '0.85rem', color: 'var(--text-secondary)' }}>
                  {t('mfa.step_verify_desc')}
                </p>
              </div>

              <OTPCodeInput
                id="mfa-verify-input"
                value={mfaVerifyCode}
                onChange={setMfaVerifyCode}
                disabled={mfaVerifying}
                autoFocus
              />

              <div className="flex-gap-center-end" style={{ marginTop: '0.5rem' }}>
                <button
                  type="button"
                  className="btn btn-secondary btn-auto-width"
                  onClick={() => setMfaEnrollOpen(false)}
                  disabled={mfaVerifying}
                >
                  {t('common.confirm_cancel')}
                </button>
                <button
                  type="submit"
                  className="btn btn-primary btn-auto-width"
                  disabled={mfaVerifying || mfaVerifyCode.length !== 6}
                >
                  {mfaVerifying ? '...' : t('mfa.verify_btn')}
                </button>
              </div>
            </form>
          ) : (
            mfaEnrollData && (
              <RecoveryCodesDisplay
                codes={mfaEnrollData.recovery_codes}
                variant="confirmation"
                onDone={() => setMfaEnrollOpen(false)}
              />
            )
          )}
        </div>
      </Modal>

      {/* 2FA Disable Modal */}
      <Modal
        open={mfaDisableOpen}
        title={t('mfa.disable_title')}
        subtitle={t('mfa.disable_desc')}
        onClose={() => setMfaDisableOpen(false)}
      >
        <form
          onSubmit={(e) => {
            void handleMFADisableSubmit(e)
          }}
          className="page-stack"
          style={{ gap: '1rem' }}
        >
          <OTPCodeInput
            id="mfa-disable-input"
            value={mfaDisableCode}
            onChange={setMfaDisableCode}
            disabled={mfaDisabling}
            autoFocus
          />
          <div className="flex-gap-center-end" style={{ marginTop: '0.5rem' }}>
            <button
              type="button"
              className="btn btn-secondary btn-auto-width"
              onClick={() => setMfaDisableOpen(false)}
              disabled={mfaDisabling}
            >
              {t('common.confirm_cancel')}
            </button>
            <button
              type="submit"
              className="btn btn-danger btn-auto-width"
              disabled={mfaDisabling || mfaDisableCode.length !== 6}
            >
              {mfaDisabling ? '...' : t('mfa.disable_submit')}
            </button>
          </div>
        </form>
      </Modal>

      {/* 2FA Regenerate Recovery Modal */}
      <Modal
        open={mfaRegenOpen}
        title={t('mfa.regenerate_recovery_btn')}
        subtitle={t('mfa.disable_desc')}
        onClose={() => setMfaRegenOpen(false)}
      >
        <form
          onSubmit={(e) => {
            void handleMFARegenSubmit(e)
          }}
          className="page-stack"
          style={{ gap: '1rem' }}
        >
          <OTPCodeInput
            id="mfa-regen-input"
            value={mfaRegenCode}
            onChange={setMfaRegenCode}
            disabled={mfaRegening}
            autoFocus
          />
          <div className="flex-gap-center-end" style={{ marginTop: '0.5rem' }}>
            <button
              type="button"
              className="btn btn-secondary btn-auto-width"
              onClick={() => setMfaRegenOpen(false)}
              disabled={mfaRegening}
            >
              {t('common.confirm_cancel')}
            </button>
            <button
              type="submit"
              className="btn btn-primary btn-auto-width"
              disabled={mfaRegening || mfaRegenCode.length !== 6}
            >
              {mfaRegening ? '...' : t('mfa.regenerate_recovery_btn')}
            </button>
          </div>
        </form>
      </Modal>
    </div>
  )
}
