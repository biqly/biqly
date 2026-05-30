import { useT } from '../../i18n'

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
    navigator.clipboard.writeText(codes.join('\n'))
    alert(t('mfa.recovery_copied'))
  }

  if (variant === 'confirmation') {
    return (
      <div className="page-stack" style={{ gap: '1rem' }}>
        <div style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem' }}>
          <h4 style={{ margin: 0, color: 'var(--success)' }}>✔ {t('mfa.success_enabled')}</h4>
          <p style={{ margin: 0, fontSize: '0.85rem', color: 'var(--text-secondary)' }}>
            {t('mfa.recovery_desc')}
          </p>
        </div>
        <div className="recovery-codes-grid border-border" style={{ padding: '1rem' }}>
          {codes.map((code, index) => <div key={index}>{code}</div>)}
        </div>
        <div className="flex-gap-center-end" style={{ marginTop: '0.5rem' }}>
          <button type="button" className="btn btn-secondary btn-auto-width" onClick={copyCodes}>
            📋 {t('common.copy')}
          </button>
          <button type="button" className="btn btn-primary btn-auto-width" onClick={onDone}>
            {t('common.confirm_ok')}
          </button>
        </div>
      </div>
    )
  }

  return (
    <div className="recovery-codes-box border-border overflow-hidden card-lead-margin" style={{ padding: '1rem', backgroundColor: 'rgba(255, 255, 255, 0.02)' }}>
      <h4 style={{ margin: 0, fontSize: '0.9rem', color: 'var(--text)' }}>
        {t('mfa.recovery_title')}
      </h4>
      <p style={{ margin: '0.25rem 0 1rem 0', fontSize: '0.8rem', color: 'var(--text-secondary)' }}>
        {t('mfa.recovery_desc')}
      </p>
      <div className="recovery-codes-grid">
        {codes.map((code, index) => <div key={index}>{code}</div>)}
      </div>
      <button
        type="button"
        className="btn btn-sm btn-secondary"
        style={{ marginTop: '0.75rem', width: 'auto' }}
        onClick={copyCodes}
      >
        📋 {t('common.copy')}
      </button>
    </div>
  )
}
