import type { useT } from '../../i18n'
import { buttonClass } from '../../lib/buttonClasses'
import { cn } from '../../lib/cn'
import { legacyFormClass } from '../../lib/formClasses'
import { legacyLayoutClass } from '../../lib/layoutClasses'
import type { PasskeyInfo } from '../../types/auth'
import {
  adminBtnAutoWidthClass,
  adminFlexGapCenterEndClass,
  mfaQrContainerClass,
} from '../admin/adminClasses'
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
        <form onSubmit={onRegisterSubmit} className={cn(legacyLayoutClass('page-stack'), 'gap-4')}>
          <div className={cn(legacyFormClass('form-group'), 'm-0')}>
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
          <div className={cn(adminFlexGapCenterEndClass, 'mt-2')}>
            <button
              type="button"
              className={cn(buttonClass('secondary'), adminBtnAutoWidthClass)}
              onClick={onAddModalClose}
              disabled={registering}
            >
              {t('common.confirm_cancel')}
            </button>
            <button
              type="submit"
              className={cn(
                buttonClass('primary'),
                adminBtnAutoWidthClass,
                'inline-flex items-center gap-1.5',
              )}
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
        <form onSubmit={onRenameSubmit} className={cn(legacyLayoutClass('page-stack'), 'gap-4')}>
          <div className={cn(legacyFormClass('form-group'), 'm-0')}>
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
          <div className={cn(adminFlexGapCenterEndClass, 'mt-2')}>
            <button
              type="button"
              className={cn(buttonClass('secondary'), adminBtnAutoWidthClass)}
              onClick={onRenameModalClose}
              disabled={renaming}
            >
              {t('common.confirm_cancel')}
            </button>
            <button
              type="submit"
              className={cn(buttonClass('primary'), adminBtnAutoWidthClass)}
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
        <div className={cn(legacyLayoutClass('page-stack'), 'gap-6')}>
          {!mfaShowRecovery ? (
            <form
              onSubmit={onMfaVerifySubmit}
              className={cn(legacyLayoutClass('page-stack'), 'gap-4')}
            >
              <div className="flex flex-col gap-2">
                <h4 className="m-0">{t('mfa.step_scan')}</h4>
                <p className="text-caption text-foreground-muted m-0">{t('mfa.step_scan_desc')}</p>
              </div>
              {mfaQrCode && (
                <div className={mfaQrContainerClass}>
                  <img src={mfaQrCode} alt="2FA QR Code" className="block h-45 w-45" />
                </div>
              )}
              {mfaEnrollData && (
                <div className="text-caption border-border rounded border border-dashed bg-white/2 p-3">
                  <strong>{t('mfa.step_manual')}</strong>
                  <div className="text-accent mt-1 font-mono text-base tracking-wider">
                    {mfaEnrollData.secret}
                  </div>
                </div>
              )}
              <hr className="border-border my-2 border-0 border-t" />
              <div className="flex flex-col gap-2">
                <h4 className="m-0">{t('mfa.step_verify')}</h4>
                <p className="text-caption text-foreground-muted m-0">
                  {t('mfa.step_verify_desc')}
                </p>
              </div>
              <OTPCodeInput
                id="mfa-verify-input"
                value={mfaVerifyCode}
                onChange={onMfaVerifyCodeChange}
                disabled={mfaVerifying}
              />
              <div className={cn(adminFlexGapCenterEndClass, 'mt-2')}>
                <button
                  type="button"
                  className={cn(buttonClass('secondary'), adminBtnAutoWidthClass)}
                  onClick={onMfaEnrollClose}
                  disabled={mfaVerifying}
                >
                  {t('common.confirm_cancel')}
                </button>
                <button
                  type="submit"
                  className={cn(buttonClass('primary'), adminBtnAutoWidthClass)}
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
        <form
          onSubmit={onMfaDisableSubmit}
          className={cn(legacyLayoutClass('page-stack'), 'gap-4')}
        >
          <OTPCodeInput
            id="mfa-disable-input"
            value={mfaDisableCode}
            onChange={onMfaDisableCodeChange}
            disabled={mfaDisabling}
          />
          <div className={cn(adminFlexGapCenterEndClass, 'mt-2')}>
            <button
              type="button"
              className={cn(buttonClass('secondary'), adminBtnAutoWidthClass)}
              onClick={onMfaDisableClose}
              disabled={mfaDisabling}
            >
              {t('common.confirm_cancel')}
            </button>
            <button
              type="submit"
              className={cn(buttonClass('danger'), adminBtnAutoWidthClass)}
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
        <form onSubmit={onMfaRegenSubmit} className={cn(legacyLayoutClass('page-stack'), 'gap-4')}>
          <OTPCodeInput
            id="mfa-regen-input"
            value={mfaRegenCode}
            onChange={onMfaRegenCodeChange}
            disabled={mfaRegening}
          />
          <div className={cn(adminFlexGapCenterEndClass, 'mt-2')}>
            <button
              type="button"
              className={cn(buttonClass('secondary'), adminBtnAutoWidthClass)}
              onClick={onMfaRegenClose}
              disabled={mfaRegening}
            >
              {t('common.confirm_cancel')}
            </button>
            <button
              type="submit"
              className={cn(buttonClass('primary'), adminBtnAutoWidthClass)}
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
