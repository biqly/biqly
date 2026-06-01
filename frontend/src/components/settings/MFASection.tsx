import { localeLanguageTag, useLocale, useT } from '../../i18n'
import { EmptyState } from '../ui/EmptyState'
import { RecoveryCodesDisplay } from './RecoveryCodesDisplay'

export interface MFAStatus {
  enabled: boolean
  method?: string
  verified_at?: string
}

interface MFASectionProps {
  status: MFAStatus | null
  recoveryCodes: string[] | null
  onEnable: () => void
  onDisable: () => void
  onRegenerate: () => void
  className?: string
}

export function MFASection({
  status,
  recoveryCodes,
  onEnable,
  onDisable,
  onRegenerate,
  className,
}: MFASectionProps) {
  const t = useT()
  const [locale] = useLocale()

  return (
    <section className={['card card--elevated settings-prefs-card', className].filter(Boolean).join(' ')}>
      <div className="card-intro card-intro--compact">
        <div className="card-header-row card-header-row--spaced">
          <h2>{t('mfa.title')}</h2>
          {status && (
            status.enabled ? (
              <button
                type="button"
                className="btn btn-danger-outline btn-sm"
                style={{ width: 'auto', margin: 0 }}
                onClick={onDisable}
              >
                🔓 {t('mfa.disable_btn')}
              </button>
            ) : (
              <button type="button" className="btn btn-primary btn-sm btn-auto-width" onClick={onEnable}>
                🔒 {t('mfa.enable_btn')}
              </button>
            )
          )}
        </div>
        <p className="card-lead card-lead-margin">{t('mfa.subtitle')}</p>
      </div>

      {!status ? (
        <div className="admin-center-container">
          <div className="spinner" style={{ width: '24px', height: '24px', borderTopColor: 'var(--accent)' }}></div>
        </div>
      ) : !status.enabled ? (
        <EmptyState title={t('mfa.empty_title')} description={t('mfa.empty_desc')} />
      ) : (
        <>
          <ul className="settings-security-list" role="list">
            <li className="settings-security-item">
              <div className="settings-security-item__row">
                <div className="settings-security-item__identity">
                  <span className="settings-security-item__icon" aria-hidden>
                    📱
                  </span>
                  <span className="settings-security-item__name">{t('mfa.method_totp')}</span>
                  <span className="badge badge-success admin-badge-active" style={{ fontSize: '0.75rem' }}>
                    {t('mfa.status_active')}
                  </span>
                </div>
                <div className="settings-security-item__actions">
                  <button
                    type="button"
                    className="btn btn-sm btn-secondary btn-icon-only"
                    title={t('mfa.regenerate_recovery_btn')}
                    onClick={onRegenerate}
                  >
                    🔄
                  </button>
                </div>
              </div>
              <p className="settings-security-item__detail">
                {t('mfa.col_enabled_at')}:{' '}
                {status.verified_at
                  ? new Date(status.verified_at).toLocaleString(localeLanguageTag(locale))
                  : '—'}
              </p>
            </li>
          </ul>
          {recoveryCodes && <RecoveryCodesDisplay codes={recoveryCodes} />}
        </>
      )}
    </section>
  )
}
