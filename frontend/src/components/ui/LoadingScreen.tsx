import { useEffect, useState, useSyncExternalStore } from 'react'

import { useT } from '../../i18n'
import { LoadingIndicator } from './LoadingIndicator'
import {
  acquireCornerId,
  cornerOwnerId,
  registerCorner,
  shouldShowImmediately,
  subscribeCorner,
  unregisterCorner,
} from './loadingScreenRegistry'

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

  const [id] = useState(acquireCornerId)
  // Evaluated once at mount, before this instance registers: true when another
  // pill is already visible (Suspense fallback → data-load handoff).
  const [immediate] = useState(() => shouldShowImmediately())

  useEffect(() => {
    if (variant !== 'corner') {
      return
    }
    registerCorner(id)
    return () => unregisterCorner(id)
  }, [id, variant])

  const ownerId = useSyncExternalStore(subscribeCorner, cornerOwnerId, () => 0)

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

  // Only the owner renders the fixed pill (and the live region) — stacked
  // LoadingScreens otherwise pile identical pills onto the same corner and
  // announce duplicate statuses to screen readers.
  const isOwner = ownerId === id
  return (
    <div
      className="loading-screen"
      style={{ minHeight }}
      {...(isOwner ? { role: 'status', 'aria-live': 'polite', 'aria-label': displayLabel } : {})}
    >
      {isOwner ? (
        <LoadingIndicator
          label={displayLabel}
          className={`loading-indicator--fixed${immediate ? ' loading-indicator--fixed-immediate' : ''}`}
        />
      ) : null}
    </div>
  )
}
