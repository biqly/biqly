import { localeLanguageTag, useLocale, useT } from '../../i18n'
import { buttonClass } from '../../lib/buttonClasses'
import {
  cardClass,
  cardHeaderRowClass,
  cardIntroClass,
  cardIntroCompactClass,
  cardLeadClass,
} from '../../lib/cardClasses'
import { cn } from '../../lib/cn'
import { legacyFeedbackClass } from '../../lib/feedbackClasses'
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
    <section className={cn(cardClass({ elevated: true }), className)}>
      <div className={cn(cardIntroClass, cardIntroCompactClass)}>
        <div className={cn(cardHeaderRowClass, cardHeaderRowClass)}>
          <h2>{t('mfa.title')}</h2>
          {status &&
            (status.enabled ? (
              <button
                type="button"
                className={buttonClass('danger-outline', { size: 'sm' })}
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
                className={cn(buttonClass('primary', { size: 'sm' }), adminBtnAutoWidthClass)}
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
        <p className={cn(cardLeadClass, cardLeadMarginClass)}>{t('mfa.subtitle')}</p>
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
            className={`border-border bg-bg-secondary m-0 flex list-none flex-col gap-0 overflow-hidden rounded-lg border p-0`}
            role="list"
          >
            <li
              className={`border-border border-b px-4 py-[0.9rem] transition-colors duration-150 last:border-b-0 hover:bg-white/1.5`}
            >
              <div className="flex items-start justify-between gap-3">
                <div className="flex min-w-0 flex-wrap items-center gap-x-[0.55rem] gap-y-[0.4rem]">
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
                  <span className="text-[0.9rem] leading-[1.35] font-semibold wrap-break-word">
                    {t('mfa.method_totp')}
                  </span>
                  <span
                    className={`${legacyFeedbackClass('badge badge-success')} ${adminBadgeActiveClass}`}
                    style={{ fontSize: '0.75rem' }}
                  >
                    {t('mfa.status_active')}
                  </span>
                </div>
                <div className="flex shrink-0 items-center gap-[0.35rem]">
                  <button
                    type="button"
                    className={cn(buttonClass('secondary', { size: 'sm' }), adminBtnIconOnlyClass)}
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
              <p className="text-foreground-muted mt-[0.45rem] mr-0 mb-0 ml-0 text-[0.78rem] leading-[1.45]">
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
