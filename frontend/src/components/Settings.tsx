import { useEffect, useState } from 'react'
import { useT, useLocale, localeLanguageTag } from '../i18n'
import { useAuth } from './auth/AuthProvider'
import {
  apiGetPasskeys,
  apiDeletePasskey,
  apiPasskeyRegisterBegin,
  apiPasskeyRegisterFinish,
} from '../api/auth'
import { base64urlToBuffer, bufferToBase64url } from '../utils/webauthn'
import type { PasskeyInfo } from '../types/auth'
import { ConfirmDialog } from './ui/ConfirmDialog'
import { Modal } from './ui/Modal'
import { EmptyState } from './ui/EmptyState'
import { ErrorAlert } from './ui/ErrorAlert'

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
  const [registering, setRegistering] = useState(false)

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

  useEffect(() => {
    fetchPasskeys()
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
    setSuccessMessage(null)
    setAddModalOpen(true)
  }

  const handleRegisterSubmit = async (e: React.SubmitEvent<HTMLFormElement>) => {
    e.preventDefault()
    if (!accessToken) return

    const isSupported = window.PublicKeyCredential &&
      typeof window.PublicKeyCredential.isUserVerifyingPlatformAuthenticatorAvailable === 'function'

    if (!isSupported) {
      setError(t('passkeys.error_not_supported'))
      setAddModalOpen(false)
      return
    }

    setRegistering(true)
    setError(null)
    setSuccessMessage(null)

    try {
      // 1. Begin Registration on Backend
      const beginResp = await apiPasskeyRegisterBegin(accessToken)
      const publicKeyOptions = beginResp.publicKey || beginResp

      if (!publicKeyOptions) {
        throw new Error('Invalid options from server')
      }

      // Convert challenge, user id, and excluded credentials IDs to ArrayBuffer
      const options: CredentialCreationOptions = {
        publicKey: {
          ...publicKeyOptions,
          challenge: base64urlToBuffer(publicKeyOptions.challenge),
          user: {
            ...publicKeyOptions.user,
            id: base64urlToBuffer(publicKeyOptions.user.id),
          },
          excludeCredentials: publicKeyOptions.excludeCredentials?.map((cred: any) => ({
            ...cred,
            id: base64urlToBuffer(cred.id),
          })),
        },
      }

      // 2. Trigger browser's WebAuthn prompt
      const credential = await navigator.credentials.create(options)
      if (!credential) {
        throw new Error('No credential returned by browser')
      }

      // 3. Serialize response back to base64url
      const attestation = credential as PublicKeyCredential
      const response = attestation.response as AuthenticatorAttestationResponse
      const credentialJson = {
        id: attestation.id,
        rawId: bufferToBase64url(attestation.rawId),
        type: attestation.type,
        response: {
          clientDataJSON: bufferToBase64url(response.clientDataJSON),
          attestationObject: bufferToBase64url(response.attestationObject),
          transports: response.getTransports ? response.getTransports() : [],
        },
      }

      // 4. Finish registration on Backend
      await apiPasskeyRegisterFinish(accessToken, credentialJson, newPasskeyName.trim())
      setSuccessMessage(t('passkeys.success_register'))
      setAddModalOpen(false)
      await fetchPasskeys()
    } catch (err: any) {
      if (err.name !== 'NotAllowedError') {
        setError(err.message || 'Passkey registration failed')
      }
    } finally {
      setRegistering(false)
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
              className="btn btn-primary btn-sm"
              style={{ width: 'auto', margin: 0 }}
              onClick={openAddModal}
            >
              🔒 {t('passkeys.add_btn')}
            </button>
          </div>
          <p className="card-lead" style={{ margin: '0.5rem 0 1.25rem' }}>
            {t('passkeys.subtitle')}
          </p>
        </div>

        {error && (
          <div style={{ marginBottom: '1.25rem' }}>
            <ErrorAlert error={error} />
          </div>
        )}

        {successMessage && (
          <div
            style={{
              marginBottom: '1.25rem',
              border: '1px solid var(--success)',
              background: 'color-mix(in srgb, var(--success) 12%, transparent)',
              padding: '0.75rem 1rem',
              borderRadius: '0.5rem',
              color: 'var(--success)',
              display: 'flex',
              justifyContent: 'space-between',
              alignItems: 'center',
              fontSize: '0.82rem',
              fontWeight: 500,
            }}
          >
            <div>🎉 {successMessage}</div>
            <button
              type="button"
              style={{
                background: 'transparent',
                border: 0,
                color: 'inherit',
                cursor: 'pointer',
                fontSize: '1.2rem',
                padding: '0 0.25rem',
                lineHeight: 1,
              }}
              onClick={() => setSuccessMessage(null)}
            >
              ×
            </button>
          </div>
        )}

        {loading ? (
          <div style={{ display: 'flex', justifyContent: 'center', padding: '2.5rem' }}>
            <div className="spinner" style={{ width: '24px', height: '24px', borderTopColor: 'var(--accent)' }}></div>
          </div>
        ) : passkeys.length === 0 ? (
          <EmptyState
            title={t('passkeys.empty_title')}
            description={t('passkeys.empty_desc')}
          />
        ) : (
          <div className="results-table-scroll" style={{ border: '1px solid var(--border)', borderRadius: '0.5rem', overflow: 'hidden' }}>
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
                    <td className="actions" style={{ textAlign: 'right' }}>
                      <button
                        type="button"
                        className="btn btn-sm btn-danger-outline"
                        style={{ margin: 0, padding: '0.2rem 0.5rem', minHeight: 'auto', width: 'auto', display: 'inline-flex' }}
                        onClick={() => setDeleteTarget(pk)}
                      >
                        🗑️
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
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
          <div style={{ display: 'flex', gap: '0.5rem', justifyContent: 'flex-end', marginTop: '0.5rem' }}>
            <button
              type="button"
              className="btn btn-secondary"
              style={{ width: 'auto', margin: 0 }}
              onClick={() => setAddModalOpen(false)}
              disabled={registering}
            >
              {t('common.confirm_cancel')}
            </button>
            <button
              type="submit"
              className="btn btn-primary"
              style={{ width: 'auto', margin: 0, display: 'inline-flex', alignItems: 'center', gap: '6px' }}
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

      <p className="settings-footnote">{t('settings.persist_hint')}</p>
    </div>
  )
}
