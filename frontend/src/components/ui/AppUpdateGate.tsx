import { useEffect, useMemo, useState } from 'react'

import { useT } from '../../i18n'
import { buttonClass } from '../../lib/buttonClasses'
import {
  canReloadForUpdate,
  isLikelyAssetLoadFailure,
  markReloadForUpdate,
  startIndexPoll,
  type UpdateTrigger,
} from '../../utils/appUpdate'

interface GateState {
  visible: boolean
  trigger: UpdateTrigger
  countdown: number
}

const DEFAULT_COUNTDOWN = 3

export function AppUpdateGate() {
  const t = useT()
  const [state, setState] = useState<GateState | null>(null)

  const title = useMemo(() => {
    if (!state) {
      return ''
    }
    return state.trigger === 'chunk-error'
      ? t('app_update.recover_title')
      : t('app_update.available_title')
  }, [state, t])

  const body = useMemo(() => {
    if (!state) {
      return ''
    }
    return state.trigger === 'chunk-error'
      ? t('app_update.recover_body')
      : t('app_update.available_body')
  }, [state, t])

  const scheduleReload = (trigger: UpdateTrigger) => {
    if (!canReloadForUpdate()) {
      return
    }
    setState({ visible: true, trigger, countdown: DEFAULT_COUNTDOWN })
  }

  useEffect(() => {
    const stopPoll = startIndexPoll(() => scheduleReload('new-version'))
    return () => stopPoll()
  }, [])

  useEffect(() => {
    const onRejection = (e: PromiseRejectionEvent) => {
      if (!isLikelyAssetLoadFailure(e.reason)) {
        return
      }
      scheduleReload('chunk-error')
    }
    const onError = (e: ErrorEvent) => {
      if (!isLikelyAssetLoadFailure(e.error ?? e.message)) {
        return
      }
      scheduleReload('chunk-error')
    }
    window.addEventListener('unhandledrejection', onRejection)
    window.addEventListener('error', onError)
    return () => {
      window.removeEventListener('unhandledrejection', onRejection)
      window.removeEventListener('error', onError)
    }
  }, [])

  useEffect(() => {
    if (!state?.visible) {
      return
    }
    if (state.countdown <= 0) {
      markReloadForUpdate()
      window.location.reload()
      return
    }
    const id = window.setTimeout(
      () => setState((prev) => (prev ? { ...prev, countdown: prev.countdown - 1 } : prev)),
      1000,
    )
    return () => window.clearTimeout(id)
  }, [state])

  if (!state?.visible) {
    return null
  }

  return (
    <div
      className={
        'fixed right-4 bottom-4 left-4 z-2000 flex items-center justify-between gap-3.5 rounded-xl border border-[color-mix(in_srgb,var(--accent)_35%,var(--border))] bg-[color-mix(in_srgb,var(--bg-card)_92%,var(--accent)_8%)] p-[12px_14px] shadow-[0_14px_42px_rgba(0,0,0,0.55)] backdrop-blur-[10px] max-[520px]:right-3 max-[520px]:bottom-3 max-[520px]:left-3 max-[520px]:flex-col max-[520px]:items-start'
      }
      role="status"
      aria-live="polite"
    >
      <div className="flex min-w-0 flex-col gap-0.5">
        <div className="text-foreground text-[0.9rem] font-bold">{title}</div>
        <div className="text-foreground-muted text-[0.82rem] leading-[1.35]">{body}</div>
      </div>
      <div className="flex shrink-0 items-center gap-2.5 max-[520px]:w-full max-[520px]:justify-between">
        <div className="text-foreground-muted text-[0.78rem] whitespace-nowrap">
          {t('app_update.reloading_in', { seconds: state.countdown })}
        </div>
        <button
          type="button"
          className={buttonClass('secondary', { size: 'sm' })}
          onClick={() => {
            markReloadForUpdate()
            window.location.reload()
          }}
        >
          {t('app_update.reload_now')}
        </button>
      </div>
    </div>
  )
}
