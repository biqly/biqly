import type { ReactElement } from 'react'
import { useTheme, type ThemeMode } from '../../theme'
import { useT } from '../../i18n'

const ICONS: Record<ThemeMode, ReactElement> = {
  system: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <rect x="3" y="4" width="18" height="13" rx="2" />
      <path d="M8 21h8M12 17v4" />
    </svg>
  ),
  light: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <circle cx="12" cy="12" r="4" />
      <path d="M12 2v2M12 20v2M4.93 4.93l1.41 1.41M17.66 17.66l1.41 1.41M2 12h2M20 12h2M4.93 19.07l1.41-1.41M17.66 6.34l1.41-1.41" />
    </svg>
  ),
  dark: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z" />
    </svg>
  ),
}

export function ThemeToggle() {
  const { mode, setMode } = useTheme()
  const t = useT()
  const modes: ThemeMode[] = ['system', 'light', 'dark']
  const labels: Record<ThemeMode, string> = {
    system: t('common.theme_system'),
    light: t('common.theme_light'),
    dark: t('common.theme_dark'),
  }
  return (
    <div className="theme-toggle" role="group" aria-label={t('common.theme')}>
      {modes.map((m) => {
        const active = m === mode
        return (
          <button
            key={m}
            type="button"
            className={`theme-toggle__btn${active ? ' theme-toggle__btn--active' : ''}`}
            onClick={() => setMode(m)}
            aria-pressed={active}
            aria-label={labels[m]}
            title={labels[m]}
          >
            {ICONS[m]}
          </button>
        )
      })}
    </div>
  )
}
