import { useEffect, useState } from 'react'
import { useT, useLocale, localeLanguageTag } from '../i18n'
import { useAuth } from './auth/AuthProvider'
import {
  apiGetPasskeys,
  apiDeletePasskey,
  apiPasskeyRename,
  apiMFAStatus,
  apiMFAEnroll,
  apiMFAVerify,
  apiMFADisable,
  apiMFARegenerateRecovery,
} from '../api/auth'
import QRCode from 'qrcode'
import type { PasskeyInfo } from '../types/auth'
import { ConfirmDialog } from './ui/ConfirmDialog'
import { Modal } from './ui/Modal'
import { EmptyState } from './ui/EmptyState'
import { ErrorAlert } from './ui/ErrorAlert'
import { usePasskeyRegistration } from '../hooks/usePasskeyRegistration'

interface SettingsProps {
  navigate?: (path: string) => void
}

export default function Settings({ navigate }: SettingsProps) {
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
  const [mfaStatus, setMfaStatus] = useState<{ enabled: boolean; method?: string; verified_at?: string } | null>(null)
  const [mfaEnrollData, setMfaEnrollData] = useState<{ secret: string; otpauth_url: string; recovery_codes: string[] } | null>(null)
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
  } = usePasskeyRegistration(accessToken || '')

  useEffect(() => {
    if (registrationError) {
      setError(registrationError)
    }
  }, [registrationError])

  const goTo = (path: string) => {
    navigate?.(path)
  }

  const fetchPasskeys = async () => {
    if (!accessToken) return
    setLoading(true)
    setError(null)
    try {
      const data = await apiGetPasskeys(accessToken)
      setPasskeys(data || [])
    } catch (err: any) {
      setError(err.message || 'Failed to load passkeys')
    } finally {
      setLoading(false)
    }
  }

  const fetchMFAStatus = async () => {
    if (!accessToken) return
    try {
      const status = await apiMFAStatus(accessToken)
      setMfaStatus(status)
    } catch (err: any) {
      console.error('Failed to load MFA status', err)
    }
  }

  useEffect(() => {
    fetchPasskeys()
    fetchMFAStatus()
  }, [accessToken])

  const handleDeleteConfirm = async () => {
    if (!deleteTarget || !accessToken) return
    setDeleting(true)
    setError(null)
    setSuccessMessage(null)
    try {
      await apiDeletePasskey(accessToken, deleteTarget.id)
      setSuccessMessage(t('passkeys.success_delete'))
      setDeleteTarget(null)
      await fetchPasskeys()
    } catch (err: any) {
      setError(err.message || 'Failed to delete passkey')
    } finally {
      setDeleting(false)
    }
  }

  const openAddModal = () => {
    const defaultName = t('passkeys.title') + ' ' + new Date().toLocaleDateString(localeLanguageTag(locale))
    setNewPasskeyName(defaultName)
    setError(null)
    setRegistrationError(null)
    setSuccessMessage(null)
    setAddModalOpen(true)
  }

  const handleRegisterSubmit = async (e: React.SubmitEvent<HTMLFormElement>) => {
    e.preventDefault()
    if (!accessToken) return

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
    if (!renameTarget || !renamingName.trim() || renaming || !accessToken) return

    setRenaming(true)
    setError(null)
    setSuccessMessage(null)
    try {
      await apiPasskeyRename(accessToken, renameTarget.id, renamingName.trim())
      setSuccessMessage(t('passkeys.success_rename'))
      setRenameTarget(null)
      await fetchPasskeys()
    } catch (err: any) {
      setError(err.message || 'Failed to rename passkey')
    } finally {
      setRenaming(false)
    }
  }

  const handleMFAEnrollStart = async () => {
    if (!accessToken) return
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
    } catch (err: any) {
      setError(err.message || 'MFA enrollment failed')
    }
  }

  const handleMFAVerifySubmit = async (e: React.SubmitEvent<HTMLFormElement>) => {
    e.preventDefault()
    if (!accessToken || !mfaVerifyCode.trim() || mfaVerifying) return
    setMfaVerifying(true)
    setError(null)
    try {
      await apiMFAVerify(accessToken, mfaVerifyCode.trim())
      setSuccessMessage(t('mfa.success_enabled'))
      setMfaShowRecovery(true)
      await fetchMFAStatus()
    } catch (err: any) {
      setError(err.message || 'MFA verification failed')
    } finally {
      setMfaVerifying(false)
    }
  }

  const handleMFADisableSubmit = async (e: React.SubmitEvent<HTMLFormElement>) => {
    e.preventDefault()
    if (!accessToken || !mfaDisableCode.trim() || mfaDisabling) return
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
    } catch (err: any) {
      setError(err.message || 'Failed to disable MFA')
    } finally {
      setMfaDisabling(false)
    }
  }

  const handleMFARegenSubmit = async (e: React.SubmitEvent<HTMLFormElement>) => {
    e.preventDefault()
    if (!accessToken || !mfaRegenCode.trim() || mfaRegening) return
    setMfaRegening(true)
    setError(null)
    setSuccessMessage(null)
    try {
      const resp = await apiMFARegenerateRecovery(accessToken, mfaRegenCode.trim())
      setMfaNewRecoveryCodes(resp.recovery_codes)
      setSuccessMessage(t('mfa.success_regenerate'))
      setMfaRegenOpen(false)
      setMfaRegenCode('')
    } catch (err: any) {
      setError(err.message || 'Failed to regenerate recovery codes')
    } finally {
      setMfaRegening(false)
    }
  }

  return (
    <div className="page-stack">
      <section className="card card--elevated settings-prefs-card">
        <div className="card-intro card-intro--compact">
          <h2>{t('settings.prompt_templates_section')}</h2>
          <p className="card-lead card-lead--single-line" title={t('settings.prompt_templates_hint')}>
            {t('settings.prompt_templates_hint')}
          </p>
        </div>
        <div className="settings-control-row">
          <button type="button" className="btn btn-primary" onClick={() => goTo('/prompt-templates')}>
            {t('settings.prompt_templates_open')}
          </button>
        </div>
      </section>

      <section className="card card--elevated settings-prefs-card">
        <div className="card-intro card-intro--compact">
          <h2>{t('settings.time_grains_section')}</h2>
          <p className="card-lead card-lead--single-line" title={t('settings.time_grains_hint')}>
            {t('settings.time_grains_hint')}
          </p>
        </div>
        <div className="settings-control-row">
          <button type="button" className="btn btn-primary" onClick={() => goTo('/time-grains')}>
            {t('settings.time_grains_open')}
          </button>
        </div>
      </section>

      {/* Security & Passkey Management Section */}
      <section className="card card--elevated settings-prefs-card">
        <div className="card-intro card-intro--compact">
          <div className="card-header-row card-header-row--spaced">
            <h2>{t('passkeys.title')}</h2>
            <button
              type="button"
              className="btn btn-primary btn-sm btn-auto-width"
              onClick={openAddModal}
            >
              🔒 {t('passkeys.add_btn')}
            </button>
          </div>
          <p className="card-lead card-lead-margin">
            {t('passkeys.subtitle')}
          </p>
        </div>

        {error && (
          <div className="card-lead-margin">
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

        {loading ? (
          <div className="admin-center-container">
            <div className="spinner" style={{ width: '24px', height: '24px', borderTopColor: 'var(--accent)' }}></div>
          </div>
        ) : passkeys.length === 0 ? (
          <EmptyState
            title={t('passkeys.empty_title')}
            description={t('passkeys.empty_desc')}
          />
        ) : (
          <div className="results-table-scroll border-border overflow-hidden">
            <table className="results-table" style={{ margin: 0 }}>
              <thead>
                <tr>
                  <th>{t('passkeys.col_name')}</th>
                  <th>{t('passkeys.col_created')}</th>
                  <th>{t('passkeys.col_last_used')}</th>
                  <th className="actions" style={{ width: '80px', textAlign: 'right' }}></th>
                </tr>
              </thead>
              <tbody>
                {passkeys.map((pk) => (
                  <tr key={pk.id}>
                    <td style={{ fontWeight: 600 }}>
                      <span style={{ marginRight: '6px' }}>🔑</span> {pk.name}
                    </td>
                    <td>{new Date(pk.created_at).toLocaleString(localeLanguageTag(locale))}</td>
                    <td>
                      {pk.last_used_at
                        ? new Date(pk.last_used_at).toLocaleString(localeLanguageTag(locale))
                        : t('passkeys.never_used')}
                    </td>
                    <td className="actions">
                      <div className="flex-gap-center-end">
                        <button
                          type="button"
                          className="btn btn-sm btn-secondary btn-icon-only"
                          onClick={() => {
                            setRenameTarget(pk)
                            setRenamingName(pk.name)
                          }}
                        >
                          ✏️
                        </button>
                        <button
                          type="button"
                          className="btn btn-sm btn-danger-outline btn-icon-only"
                          onClick={() => setDeleteTarget(pk)}
                        >
                          🗑️
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>


      {/* Multi-Factor Authentication (2FA) Section */}
      <section className="card card--elevated settings-prefs-card">
        <div className="card-intro card-intro--compact">
          <div className="card-header-row card-header-row--spaced">
            <h2>{t('mfa.title')}</h2>
            {mfaStatus && (
              mfaStatus.enabled ? (
                <button
                  type="button"
                  className="btn btn-danger-outline btn-sm"
                  style={{ width: 'auto', margin: 0 }}
                  onClick={() => {
                    setMfaDisableCode('')
                    setError(null)
                    setSuccessMessage(null)
                    setMfaDisableOpen(true)
                  }}
                >
                  🔓 {t('mfa.disable_btn')}
                </button>
              ) : (
                <button
                  type="button"
                  className="btn btn-primary btn-sm btn-auto-width"
                  onClick={handleMFAEnrollStart}
                >
                  🔒 {t('mfa.enable_btn')}
                </button>
              )
            )}
          </div>
          <p className="card-lead card-lead-margin">
            {t('mfa.subtitle')}
          </p>
        </div>

        {!mfaStatus ? (
          <div className="admin-center-container">
            <div className="spinner" style={{ width: '24px', height: '24px', borderTopColor: 'var(--accent)' }}></div>
          </div>
        ) : !mfaStatus.enabled ? (
          <EmptyState
            title={t('mfa.empty_title')}
            description={t('mfa.empty_desc')}
          />
        ) : (
          <>
            <div className="results-table-scroll border-border overflow-hidden">
              <table className="results-table" style={{ margin: 0 }}>
                <thead>
                  <tr>
                    <th>{t('mfa.col_method')}</th>
                    <th>{t('mfa.col_status')}</th>
                    <th>{t('mfa.col_enabled_at')}</th>
                    <th className="actions" style={{ width: '80px', textAlign: 'right' }}></th>
                  </tr>
                </thead>
                <tbody>
                  <tr>
                    <td style={{ fontWeight: 600 }}>
                      <span style={{ marginRight: '6px' }}>📱</span> {t('mfa.method_totp')}
                    </td>
                    <td>
                      <span
                        className="badge badge-success admin-badge-active"
                        style={{ fontSize: '0.75rem' }}
                      >
                        {t('mfa.status_active')}
                      </span>
                    </td>
                    <td>
                      {mfaStatus.verified_at
                        ? new Date(mfaStatus.verified_at).toLocaleString(localeLanguageTag(locale))
                        : '-'}
                    </td>
                    <td className="actions">
                      <div className="flex-gap-center-end">
                        <button
                          type="button"
                          className="btn btn-sm btn-secondary btn-icon-only"
                          title={t('mfa.regenerate_recovery_btn')}
                          onClick={() => {
                            setMfaRegenCode('')
                            setError(null)
                            setSuccessMessage(null)
                            setMfaRegenOpen(true)
                          }}
                        >
                          🔄
                        </button>
                      </div>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>

            {/* Newly generated/regenerated recovery codes display */}
            {mfaNewRecoveryCodes && (
              <div className="recovery-codes-box border-border overflow-hidden card-lead-margin" style={{ padding: '1rem', backgroundColor: 'rgba(255, 255, 255, 0.02)' }}>
                <h4 style={{ margin: 0, fontSize: '0.9rem', color: 'var(--text)' }}>
                  {t('mfa.recovery_title')}
                </h4>
                <p style={{ margin: '0.25rem 0 1rem 0', fontSize: '0.8rem', color: 'var(--text-secondary)' }}>
                  {t('mfa.recovery_desc')}
                </p>
                <div className="recovery-codes-grid">
                  {mfaNewRecoveryCodes.map((code, idx) => (
                    <div key={idx}>{code}</div>
                  ))}
                </div>
                <button
                  type="button"
                  className="btn btn-sm btn-secondary"
                  style={{ marginTop: '0.75rem', width: 'auto' }}
                  onClick={() => {
                    navigator.clipboard.writeText(mfaNewRecoveryCodes.join('\n'))
                    alert(t('mfa.recovery_copied'))
                  }}
                >
                  📋 {t('common.copy')}
                </button>
              </div>
            )}
          </>
        )}
      </section>

      {/* Delete Confirmation Dialog */}
      <ConfirmDialog
        open={deleteTarget !== null}
        title={t('passkeys.delete_title')}
        message={t('passkeys.delete_confirm')}
        confirmLabel={deleting ? '...' : undefined}
        onConfirm={handleDeleteConfirm}
        onCancel={() => setDeleteTarget(null)}
      />

      {/* Register Passkey Prompt Name Modal */}
      <Modal
        open={addModalOpen}
        title={t('passkeys.modal_title')}
        subtitle={t('passkeys.modal_desc')}
        onClose={() => setAddModalOpen(false)}
      >
        <form onSubmit={handleRegisterSubmit} className="page-stack" style={{ gap: '1rem' }}>
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
              className="btn btn-primary btn-icon-only"
              style={{ display: 'inline-flex', alignItems: 'center', gap: '6px' }}
              disabled={registering || !newPasskeyName.trim()}
            >
              {registering ? (
                <>
                  <div className="spinner" style={{ width: '12px', height: '12px', borderTopColor: '#fff', margin: 0 }}></div>
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
        <form onSubmit={handleRenameSubmit} className="page-stack" style={{ gap: '1rem' }}>
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
              className="btn btn-primary btn-icon-only"
              style={{ display: 'inline-flex', alignItems: 'center', gap: '6px' }}
              disabled={renaming || !renamingName.trim()}
            >
              {renaming ? (
                <>
                  <div className="spinner" style={{ width: '12px', height: '12px', borderTopColor: '#fff', margin: 0 }}></div>
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
            <form onSubmit={handleMFAVerifySubmit} className="page-stack" style={{ gap: '1rem' }}>
              <div style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem' }}>
                <h4 style={{ margin: 0 }}>{t('mfa.step_scan')}</h4>
                <p style={{ margin: 0, fontSize: '0.85rem', color: 'var(--text-secondary)' }}>
                  {t('mfa.step_scan_desc')}
                </p>
              </div>

              {mfaQrCode && (
                <div className="mfa-qr-container">
                  <img src={mfaQrCode} alt="2FA QR Code" style={{ display: 'block', width: '180px', height: '180px' }} />
                </div>
              )}

              {mfaEnrollData && (
                <div style={{ fontSize: '0.85rem', backgroundColor: 'rgba(255,255,255,0.02)', padding: '0.75rem', borderRadius: '0.25rem', border: '1px dashed var(--border)' }}>
                  <strong>{t('mfa.step_manual')}</strong>
                  <div style={{ fontFamily: 'monospace', fontSize: '1rem', color: 'var(--accent)', marginTop: '0.25rem', letterSpacing: '1px' }}>
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

              <div className="form-group" style={{ margin: 0 }}>
                <label htmlFor="mfa-verify-input">{t('mfa.label_code')}</label>
                <input
                  id="mfa-verify-input"
                  type="text"
                  pattern="[0-9]*"
                  inputMode="numeric"
                  maxLength={6}
                  required
                  value={mfaVerifyCode}
                  onChange={(e) => setMfaVerifyCode(e.target.value.replace(/\D/g, ''))}
                  placeholder={t('mfa.placeholder_code')}
                  disabled={mfaVerifying}
                  autoFocus
                  className="mfa-otp-input"
                />
              </div>

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
            <div className="page-stack" style={{ gap: '1rem' }}>
              <div style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem' }}>
                <h4 style={{ margin: 0, color: 'var(--success)' }}>✔ {t('mfa.success_enabled')}</h4>
                <p style={{ margin: 0, fontSize: '0.85rem', color: 'var(--text-secondary)' }}>
                  {t('mfa.recovery_desc')}
                </p>
              </div>

              {mfaEnrollData && (
                <div className="recovery-codes-grid border-border" style={{ padding: '1rem' }}>
                  {mfaEnrollData.recovery_codes.map((code, idx) => (
                    <div key={idx}>{code}</div>
                  ))}
                </div>
              )}

              <div className="flex-gap-center-end" style={{ marginTop: '0.5rem' }}>
                <button
                  type="button"
                  className="btn btn-secondary btn-auto-width"
                  onClick={() => {
                    if (mfaEnrollData) {
                      navigator.clipboard.writeText(mfaEnrollData.recovery_codes.join('\n'))
                      alert(t('mfa.recovery_copied'))
                    }
                  }}
                >
                  📋 {t('common.copy')}
                </button>
                <button
                  type="button"
                  className="btn btn-primary btn-auto-width"
                  onClick={() => setMfaEnrollOpen(false)}
                >
                  {t('common.confirm_ok')}
                </button>
              </div>
            </div>
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
        <form onSubmit={handleMFADisableSubmit} className="page-stack" style={{ gap: '1rem' }}>
          <div className="form-group" style={{ margin: 0 }}>
            <label htmlFor="mfa-disable-input">{t('mfa.label_code')}</label>
            <input
              id="mfa-disable-input"
              type="text"
              pattern="[0-9]*"
              inputMode="numeric"
              maxLength={6}
              required
              value={mfaDisableCode}
              onChange={(e) => setMfaDisableCode(e.target.value.replace(/\D/g, ''))}
              placeholder={t('mfa.placeholder_code')}
              disabled={mfaDisabling}
              autoFocus
              className="mfa-otp-input"
            />
          </div>
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
        <form onSubmit={handleMFARegenSubmit} className="page-stack" style={{ gap: '1rem' }}>
          <div className="form-group" style={{ margin: 0 }}>
            <label htmlFor="mfa-regen-input">{t('mfa.label_code')}</label>
            <input
              id="mfa-regen-input"
              type="text"
              pattern="[0-9]*"
              inputMode="numeric"
              maxLength={6}
              required
              value={mfaRegenCode}
              onChange={(e) => setMfaRegenCode(e.target.value.replace(/\D/g, ''))}
              placeholder={t('mfa.placeholder_code')}
              disabled={mfaRegening}
              autoFocus
              className="mfa-otp-input"
            />
          </div>
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

      <p className="settings-footnote">{t('settings.persist_hint')}</p>
    </div>
  )
}

