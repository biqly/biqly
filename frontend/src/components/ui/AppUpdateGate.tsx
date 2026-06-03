import '../../styles/app-update.css'

import { useEffect, useMemo, useState } from 'react'

import { useT } from '../../i18n'
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
    <div className="app-update-banner" role="status" aria-live="polite">
      <div className="app-update-banner__left">
        <div className="app-update-banner__title">{title}</div>
        <div className="app-update-banner__body">{body}</div>
      </div>
      <div className="app-update-banner__right">
        <div className="app-update-banner__countdown">
          {t('app_update.reloading_in', { seconds: state.countdown })}
        </div>
        <button
          type="button"
          className="btn btn-sm"
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
