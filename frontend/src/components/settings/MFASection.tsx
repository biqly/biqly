import { localeLanguageTag, useLocale, useT } from '../../i18n'
import { formatDateTime } from '../../utils/formatters'
import {
  adminBadgeActiveClass,
  adminBtnAutoWidthClass,
  adminBtnIconOnlyClass,
  adminCenterContainerClass,
  cardLeadMarginClass,
} from '../admin/adminClasses'
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
    <section className={['card card--elevated', className].filter(Boolean).join(' ')}>
      <div className="card-intro card-intro--compact">
        <div className="card-header-row card-header-row--spaced">
          <h2>{t('mfa.title')}</h2>
          {status &&
            (status.enabled ? (
              <button
                type="button"
                className="btn btn-danger-outline btn-sm"
                style={{
                  width: 'auto',
                  margin: 0,
                  display: 'inline-flex',
                  alignItems: 'center',
                  gap: '6px',
                }}
                onClick={onDisable}
              >
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  width="14"
                  height="14"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  className="lucide lucide-unlock"
                >
                  <rect width="18" height="11" x="3" y="11" rx="2" ry="2" />
                  <path d="M7 11V7a5 5 0 0 1 9.9-1" />
                </svg>
                {t('mfa.disable_btn')}
              </button>
            ) : (
              <button
                type="button"
                className={`btn btn-primary btn-sm ${adminBtnAutoWidthClass}`}
                style={{ display: 'inline-flex', alignItems: 'center', gap: '6px' }}
                onClick={onEnable}
              >
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  width="14"
                  height="14"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  className="lucide lucide-lock"
                >
                  <rect width="18" height="11" x="3" y="11" rx="2" ry="2" />
                  <path d="M7 11V7a5 5 0 0 1 10 0v4" />
                </svg>
                {t('mfa.enable_btn')}
              </button>
            ))}
        </div>
        <p className={`card-lead ${cardLeadMarginClass}`}>{t('mfa.subtitle')}</p>
      </div>

      {!status ? (
        <div className={adminCenterContainerClass}>
          <div
            className="spinner"
            style={{ width: '24px', height: '24px', borderTopColor: 'var(--accent)' }}
          ></div>
        </div>
      ) : !status.enabled ? (
        <EmptyState title={t('mfa.empty_title')} description={t('mfa.empty_desc')} />
      ) : (
        <>
          <ul
            className={`flex flex-col gap-0 m-0 p-0 list-none border border-border rounded-lg overflow-hidden bg-bg-secondary`}
            role="list"
          >
            <li
              className={`py-[0.9rem] px-4 border-b border-border transition-colors duration-150 hover:bg-white/[0.015] last:border-b-0`}
            >
              <div className="flex items-start justify-between gap-3">
                <div className="flex flex-wrap items-center gap-y-[0.4rem] gap-x-[0.55rem] min-w-0">
                  <span
                    className="shrink-0 text-[1rem] leading-[1.2]"
                    aria-hidden
                    style={{ display: 'inline-flex', alignItems: 'center' }}
                  >
                    <svg
                      xmlns="http://www.w3.org/2000/svg"
                      width="18"
                      height="18"
                      viewBox="0 0 24 24"
                      fill="none"
                      stroke="currentColor"
                      strokeWidth="2"
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      className="lucide lucide-smartphone"
                      style={{ color: 'var(--accent)' }}
                    >
                      <rect width="14" height="20" x="5" y="2" rx="2" ry="2" />
                      <path d="M12 18h.01" />
                    </svg>
                  </span>
                  <span className="text-[0.9rem] font-semibold leading-[1.35] break-words">
                    {t('mfa.method_totp')}
                  </span>
                  <span
                    className={`badge badge-success ${adminBadgeActiveClass}`}
                    style={{ fontSize: '0.75rem' }}
                  >
                    {t('mfa.status_active')}
                  </span>
                </div>
                <div className="flex shrink-0 items-center gap-[0.35rem]">
                  <button
                    type="button"
                    className={`btn btn-sm btn-secondary ${adminBtnIconOnlyClass}`}
                    title={t('mfa.regenerate_recovery_btn')}
                    onClick={onRegenerate}
                    style={{
                      display: 'inline-flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                    }}
                  >
                    <svg
                      xmlns="http://www.w3.org/2000/svg"
                      width="14"
                      height="14"
                      viewBox="0 0 24 24"
                      fill="none"
                      stroke="currentColor"
                      strokeWidth="2"
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      className="lucide lucide-refresh-cw"
                    >
                      <path d="M21 12a9 9 0 0 0-9-9 9.75 9.75 0 0 0-6.74 2.74L3 8" />
                      <path d="M3 3v5h5" />
                      <path d="M3 12a9 9 0 0 0 9 9 9.75 9.75 0 0 0 6.74-2.74L21 16" />
                      <path d="M16 16h5v5" />
                    </svg>
                  </button>
                </div>
              </div>
              <p className="mt-[0.45rem] mr-0 mb-0 ml-0 text-foreground-muted text-[0.78rem] leading-[1.45]">
                {t('mfa.col_enabled_at')}:{' '}
                {status.verified_at
                  ? formatDateTime(status.verified_at, localeLanguageTag(locale))
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
