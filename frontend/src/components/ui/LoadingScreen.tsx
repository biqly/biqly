import { useT } from '../../i18n'
import { LoadingIndicator } from './LoadingIndicator'

interface LoadingScreenProps {
  label?: string
  minHeight?: string
  /** corner: fixed bottom-right (default). center: inline for compact areas like auth card. */
  variant?: 'corner' | 'center'
}

export function LoadingScreen({
  label,
  minHeight = '40vh',
  variant = 'corner',
}: LoadingScreenProps) {
  const t = useT()
  const displayLabel = label ?? t('common.loading')

  if (variant === 'center') {
    return (
      <div
        className="loading-screen loading-screen--center"
        style={{ minHeight }}
        role="status"
        aria-live="polite"
      >
        <LoadingIndicator label={displayLabel} />
      </div>
    )
  }

  return (
    <div
      className="loading-screen"
      style={{ minHeight }}
      role="status"
      aria-live="polite"
      aria-label={displayLabel}
    >
      <LoadingIndicator label={displayLabel} className="loading-indicator--fixed" />
    </div>
  )
}
