import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'

import {
  getPublicDashboard,
  type PublicDashboard,
  runPublicWidget,
} from '../../api/publicDashboard'
import { useT } from '../../i18n/hooks'
import { cardClass } from '../../lib/cardClasses'
import { cn } from '../../lib/cn'
import { type DashboardWidget, DashboardWidgetRenderer } from '../dashboard/DashboardWidgetRenderer'

function widgetHeightPx(h: DashboardWidget['h']): number {
  if (typeof h === 'number') {
    return h
  }
  return h === 'small' ? 220 : h === 'large' ? 520 : 360
}

export default function PublicDashboardPage() {
  const t = useT()
  const { token = '' } = useParams()
  const [dash, setDash] = useState<PublicDashboard | null>(null)
  const [state, setState] = useState<'loading' | 'ready' | 'error'>('loading')
  // Reset to loading when the route token changes, derived during render
  // (not inside the effect below) so setState never fires synchronously
  // from the effect body — see DashboardWidgetRenderer's
  // useResetStateOnDepsChange for the same pattern.
  const [prevToken, setPrevToken] = useState(token)
  if (token !== prevToken) {
    setPrevToken(token)
    setState('loading')
    setDash(null)
  }

  useEffect(() => {
    let active = true
    getPublicDashboard(token)
      .then((d) => {
        if (active) {
          setDash(d)
          setState('ready')
        }
      })
      .catch(() => {
        if (active) {
          setState('error')
        }
      })
    return () => {
      active = false
    }
  }, [token])

  if (state === 'loading') {
    return (
      <div className="text-foreground-faint flex min-h-screen items-center justify-center">
        {t('publicDashboard.loading')}
      </div>
    )
  }
  if (state === 'error' || !dash) {
    return (
      <main className="flex min-h-screen flex-col items-center justify-center gap-2 p-8 text-center">
        <h1 className="text-xl font-bold">{t('publicDashboard.not_found_title')}</h1>
        <p className="text-foreground-faint">{t('publicDashboard.not_found_desc')}</p>
      </main>
    )
  }

  return (
    <main className="bg-background text-foreground min-h-screen p-6">
      <header className="mb-6">
        <h1 className="text-[1.8rem] font-bold">{dash.name}</h1>
        {dash.description && (
          <p className="text-foreground-faint mt-0.5 text-[0.9rem]">{dash.description}</p>
        )}
      </header>
      <div className="grid grid-cols-12 gap-6">
        {dash.widgets.map((widget) => (
          <div
            key={widget.id}
            className={cn(cardClass({ elevated: true }), 'relative flex flex-col p-5')}
            style={{
              gridColumn: `span ${widget.w || 6}`,
              minHeight: `${widgetHeightPx(widget.h)}px`,
            }}
          >
            <div className="mb-3 flex items-center justify-between border-b border-(--border-light,#f1f5f9) pb-1">
              <h2 className="text-foreground m-0 text-base font-semibold">{widget.title}</h2>
            </div>
            <div className="flex-1 overflow-hidden">
              <DashboardWidgetRenderer
                widget={widget}
                fetchData={(w) => runPublicWidget(token, w.id)}
              />
            </div>
          </div>
        ))}
      </div>
      <footer className="text-foreground-faint mt-8 text-center text-xs">
        {t('publicDashboard.powered_by')} <span className="font-semibold">biqly</span>
      </footer>
    </main>
  )
}
