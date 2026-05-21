import { LOCALE_OPTIONS, useI18n } from '../../i18n'

export function LanguageSwitcher() {
  const { locale, setLocale, supported, t } = useI18n()
  return (
    <div className="lang-switcher" role="group" aria-label={t('common.language')}>
      {supported.map((loc) => {
        const meta = LOCALE_OPTIONS[loc]
        const active = loc === locale
        return (
          <button
            key={loc}
            type="button"
            className={`lang-switcher__btn${active ? ' lang-switcher__btn--active' : ''}`}
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
