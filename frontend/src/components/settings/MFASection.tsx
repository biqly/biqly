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
}

export function MFASection({
  status,
  recoveryCodes,
  onEnable,
  onDisable,
  onRegenerate,
}: MFASectionProps) {
  const t = useT()
  const [locale] = useLocale()

  return (
    <section className="card card--elevated settings-prefs-card">
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
                    <span className="badge badge-success admin-badge-active" style={{ fontSize: '0.75rem' }}>
                      {t('mfa.status_active')}
                    </span>
                  </td>
                  <td>
                    {status.verified_at
                      ? new Date(status.verified_at).toLocaleString(localeLanguageTag(locale))
                      : '-'}
                  </td>
                  <td className="actions">
                    <div className="flex-gap-center-end">
                      <button
                        type="button"
                        className="btn btn-sm btn-secondary btn-icon-only"
                        title={t('mfa.regenerate_recovery_btn')}
                        onClick={onRegenerate}
                      >
                        🔄
                      </button>
                    </div>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
          {recoveryCodes && <RecoveryCodesDisplay codes={recoveryCodes} />}
        </>
      )}
    </section>
  )
}
