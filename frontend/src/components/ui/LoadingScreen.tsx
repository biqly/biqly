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
        className="relative flex-1 w-full flex flex-col items-center justify-center gap-4"
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
      className="relative flex-1 w-full"
      style={{ minHeight }}
      {...(isOwner ? { role: 'status', 'aria-live': 'polite', 'aria-label': displayLabel } : {})}
    >
      {isOwner ? (
        <LoadingIndicator
          label={displayLabel}
          className="fixed right-[max(1rem,env(safe-area-inset-right,0px))] bottom-[max(1rem,env(safe-area-inset-bottom,0px))] z-120 opacity-0 translate-y-2 scale-[0.98] animate-[loading-pill-in_0.22s_ease-out_0.25s_forwards] motion-reduce:translate-y-0 motion-reduce:animate-[loading-pill-fade_0.22s_ease-out_0.25s_forwards]"
          style={{ animationDelay: immediate ? '0s' : undefined }}
        />
      ) : null}
    </div>
  )
}
