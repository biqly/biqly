import { LOCALE_OPTIONS, useI18n } from '../../i18n'
import {
  segmentedControlBtnClass,
  segmentedControlShellClass,
} from '../../lib/headerControlClasses'

export function LanguageSwitcher() {
  const { locale, setLocale, supported, t } = useI18n()
  return (
    <div className={segmentedControlShellClass} role="group" aria-label={t('common.language')}>
      {supported.map((loc) => {
        const meta = LOCALE_OPTIONS[loc]
        const active = loc === locale
        return (
          <button
            key={loc}
            type="button"
            className={segmentedControlBtnClass(active)}
            aria-pressed={active}
            aria-label={meta.label}
            onClick={() => setLocale(loc)}
          >
            {meta.short}
          </button>
        )
      })}
    </div>
  )
}
