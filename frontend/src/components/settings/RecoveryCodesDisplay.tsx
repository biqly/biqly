import { useT } from '../../i18n'
import { buttonClass } from '../../lib/buttonClasses'
import { cn } from '../../lib/cn'
import { legacyLayoutClass } from '../../lib/layoutClasses'
import {
  adminBtnAutoWidthClass,
  adminFlexGapCenterEndClass,
  cardLeadMarginClass,
  recoveryCodesGridClass,
} from '../admin/adminClasses'

interface RecoveryCodesDisplayProps {
  codes: string[]
  variant?: 'inline' | 'confirmation'
  onDone?: () => void
}

export function RecoveryCodesDisplay({
  codes,
  variant = 'inline',
  onDone,
}: RecoveryCodesDisplayProps) {
  const t = useT()
  const copyCodes = () => {
    void navigator.clipboard.writeText(codes.join('\n'))
    alert(t('mfa.recovery_copied'))
  }

  if (variant === 'confirmation') {
    return (
      <div className={cn(legacyLayoutClass('page-stack'), 'gap-4')}>
        <div className="flex flex-col gap-2">
          <h4 className="text-success m-0">✔ {t('mfa.success_enabled')}</h4>
          <p className="text-foreground-muted m-0 text-sm">{t('mfa.recovery_desc')}</p>
        </div>
        <div className={cn(recoveryCodesGridClass, 'border-border border p-4')}>
          {codes.map((code, index) => (
            <div key={index}>{code}</div>
          ))}
        </div>
        <div className={cn(adminFlexGapCenterEndClass, 'mt-2')}>
          <button
            type="button"
            className={cn(buttonClass('secondary'), adminBtnAutoWidthClass)}
            onClick={copyCodes}
          >
            📋 {t('common.copy')}
          </button>
          <button
            type="button"
            className={cn(buttonClass('primary'), adminBtnAutoWidthClass)}
            onClick={onDone}
          >
            {t('common.confirm_ok')}
          </button>
        </div>
      </div>
    )
  }

  return (
    <div
      className={cn(
        'border-border bg-card-raised overflow-hidden rounded-lg border p-4',
        cardLeadMarginClass,
      )}
    >
      <h4 className="text-foreground m-0 text-sm font-semibold">{t('mfa.recovery_title')}</h4>
      <p className="text-foreground-muted mt-1 mb-4 text-xs">{t('mfa.recovery_desc')}</p>
      <div className={recoveryCodesGridClass}>
        {codes.map((code, index) => (
          <div key={index}>{code}</div>
        ))}
      </div>
      <button
        type="button"
        className={cn(buttonClass('secondary', { size: 'sm' }), 'mt-3 w-auto')}
        onClick={copyCodes}
      >
        📋 {t('common.copy')}
      </button>
    </div>
  )
}
