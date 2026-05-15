import { useI18n, type Locale } from '../../i18n'

const LOCALE_LABELS: Record<Locale, { label: string; short: string }> = {
  tr: { label: 'Türkçe', short: 'TR' },
  en: { label: 'English', short: 'EN' },
}

export function LanguageSwitcher() {
  const { locale, setLocale, supported, t } = useI18n()
  return (
    <div className="lang-switcher" role="group" aria-label={t('common.language')}>
      {supported.map((loc) => {
        const meta = LOCALE_LABELS[loc]
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
