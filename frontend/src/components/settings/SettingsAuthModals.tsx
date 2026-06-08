import type { useT } from '../../i18n'
import type { PasskeyInfo } from '../../types/auth'
import { ConfirmDialog } from '../ui/ConfirmDialog'
import { Modal } from '../ui/Modal'
import { OTPCodeInput } from './OTPCodeInput'
import { RecoveryCodesDisplay } from './RecoveryCodesDisplay'

export function SettingsAuthModals({
  t,
  deleteTarget,
  deleting,
  onDeleteConfirm,
  onDeleteCancel,
  addModalOpen,
  newPasskeyName,
  registering,
  onAddModalClose,
  onPasskeyNameChange,
  onRegisterSubmit,
  renameTarget,
  renamingName,
  renaming,
  onRenameModalClose,
  onRenamingNameChange,
  onRenameSubmit,
  mfaEnrollOpen,
  mfaShowRecovery,
  mfaQrCode,
  mfaEnrollData,
  mfaVerifyCode,
  mfaVerifying,
  onMfaEnrollClose,
  onMfaVerifyCodeChange,
  onMfaVerifySubmit,
  mfaDisableOpen,
  mfaDisableCode,
  mfaDisabling,
  onMfaDisableClose,
  onMfaDisableCodeChange,
  onMfaDisableSubmit,
  mfaRegenOpen,
  mfaRegenCode,
  mfaRegening,
  onMfaRegenClose,
  onMfaRegenCodeChange,
  onMfaRegenSubmit,
}: {
  t: ReturnType<typeof useT>
  deleteTarget: PasskeyInfo | null
  deleting: boolean
  onDeleteConfirm: () => void
  onDeleteCancel: () => void
  addModalOpen: boolean
  newPasskeyName: string
  registering: boolean
  onAddModalClose: () => void
  onPasskeyNameChange: (name: string) => void
  onRegisterSubmit: (e: React.SubmitEvent<HTMLFormElement>) => void
  renameTarget: PasskeyInfo | null
  renamingName: string
  renaming: boolean
  onRenameModalClose: () => void
  onRenamingNameChange: (name: string) => void
  onRenameSubmit: (e: React.SubmitEvent<HTMLFormElement>) => void
  mfaEnrollOpen: boolean
  mfaShowRecovery: boolean
  mfaQrCode: string
  mfaEnrollData: { secret: string; recovery_codes: string[] } | null
  mfaVerifyCode: string
  mfaVerifying: boolean
  onMfaEnrollClose: () => void
  onMfaVerifyCodeChange: (code: string) => void
  onMfaVerifySubmit: (e: React.SubmitEvent<HTMLFormElement>) => void
  mfaDisableOpen: boolean
  mfaDisableCode: string
  mfaDisabling: boolean
  onMfaDisableClose: () => void
  onMfaDisableCodeChange: (code: string) => void
  onMfaDisableSubmit: (e: React.SubmitEvent<HTMLFormElement>) => void
  mfaRegenOpen: boolean
  mfaRegenCode: string
  mfaRegening: boolean
  onMfaRegenClose: () => void
  onMfaRegenCodeChange: (code: string) => void
  onMfaRegenSubmit: (e: React.SubmitEvent<HTMLFormElement>) => void
}) {
  return (
    <>
      <ConfirmDialog
        open={deleteTarget !== null}
        title={t('passkeys.delete_title')}
        message={t('passkeys.delete_confirm')}
        confirmLabel={deleting ? '...' : undefined}
        onConfirm={onDeleteConfirm}
        onCancel={onDeleteCancel}
      />

      <Modal
        open={addModalOpen}
        title={t('passkeys.modal_title')}
        subtitle={t('passkeys.modal_desc')}
        onClose={onAddModalClose}
      >
        <form onSubmit={onRegisterSubmit} className="page-stack" style={{ gap: '1rem' }}>
          <div className="form-group" style={{ margin: 0 }}>
            <label htmlFor="passkey-name">{t('passkeys.modal_label_name')}</label>
            <input
              id="passkey-name"
              type="text"
              required
              value={newPasskeyName}
              onChange={(e) => onPasskeyNameChange(e.target.value)}
              placeholder={t('passkeys.modal_placeholder_name')}
              disabled={registering}
            />
          </div>
          <div className="flex-gap-center-end" style={{ marginTop: '0.5rem' }}>
            <button
              type="button"
              className="btn btn-secondary btn-auto-width"
              onClick={onAddModalClose}
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
              {registering ? `${t('passkeys.modal_submit')}...` : t('passkeys.modal_submit')}
            </button>
          </div>
        </form>
      </Modal>

      <Modal
        open={renameTarget !== null}
        title={t('passkeys.rename_title')}
        subtitle={t('passkeys.rename_desc')}
        onClose={onRenameModalClose}
      >
        <form onSubmit={onRenameSubmit} className="page-stack" style={{ gap: '1rem' }}>
          <div className="form-group" style={{ margin: 0 }}>
            <label htmlFor="rename-passkey-name">{t('passkeys.modal_label_name')}</label>
            <input
              id="rename-passkey-name"
              type="text"
              required
              value={renamingName}
              onChange={(e) => onRenamingNameChange(e.target.value)}
              placeholder={t('passkeys.modal_placeholder_name')}
              disabled={renaming}
            />
          </div>
          <div className="flex-gap-center-end" style={{ marginTop: '0.5rem' }}>
            <button
              type="button"
              className="btn btn-secondary btn-auto-width"
              onClick={onRenameModalClose}
              disabled={renaming}
            >
              {t('common.confirm_cancel')}
            </button>
            <button
              type="submit"
              className="btn btn-primary btn-auto-width"
              disabled={renaming || !renamingName.trim()}
            >
              {renaming ? `${t('passkeys.modal_submit')}...` : t('passkeys.modal_submit')}
            </button>
          </div>
        </form>
      </Modal>

      <Modal
        open={mfaEnrollOpen}
        title={t('mfa.modal_enroll_title')}
        subtitle={t('mfa.modal_enroll_desc')}
        onClose={onMfaEnrollClose}
      >
        <div className="page-stack" style={{ gap: '1.5rem' }}>
          {!mfaShowRecovery ? (
            <form onSubmit={onMfaVerifySubmit} className="page-stack" style={{ gap: '1rem' }}>
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
                onChange={onMfaVerifyCodeChange}
                disabled={mfaVerifying}
              />
              <div className="flex-gap-center-end" style={{ marginTop: '0.5rem' }}>
                <button
                  type="button"
                  className="btn btn-secondary btn-auto-width"
                  onClick={onMfaEnrollClose}
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
                onDone={onMfaEnrollClose}
              />
            )
          )}
        </div>
      </Modal>

      <Modal
        open={mfaDisableOpen}
        title={t('mfa.disable_title')}
        subtitle={t('mfa.disable_desc')}
        onClose={onMfaDisableClose}
      >
        <form onSubmit={onMfaDisableSubmit} className="page-stack" style={{ gap: '1rem' }}>
          <OTPCodeInput
            id="mfa-disable-input"
            value={mfaDisableCode}
            onChange={onMfaDisableCodeChange}
            disabled={mfaDisabling}
          />
          <div className="flex-gap-center-end" style={{ marginTop: '0.5rem' }}>
            <button
              type="button"
              className="btn btn-secondary btn-auto-width"
              onClick={onMfaDisableClose}
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

      <Modal
        open={mfaRegenOpen}
        title={t('mfa.regenerate_recovery_btn')}
        subtitle={t('mfa.disable_desc')}
        onClose={onMfaRegenClose}
      >
        <form onSubmit={onMfaRegenSubmit} className="page-stack" style={{ gap: '1rem' }}>
          <OTPCodeInput
            id="mfa-regen-input"
            value={mfaRegenCode}
            onChange={onMfaRegenCodeChange}
            disabled={mfaRegening}
          />
          <div className="flex-gap-center-end" style={{ marginTop: '0.5rem' }}>
            <button
              type="button"
              className="btn btn-secondary btn-auto-width"
              onClick={onMfaRegenClose}
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
    </>
  )
}
