import { useT } from '../i18n'
import { LanguageSwitcher } from './ui/LanguageSwitcher'
import { ThemeToggle } from './ui/ThemeToggle'

interface SettingsProps {
  navigate?: (path: string) => void
}

export default function Settings({ navigate }: SettingsProps) {
  const t = useT()
  const goTo = (path: string) => {
    navigate?.(path)
  }

  return (
    <div className="page-stack">
      <section className="card card--elevated settings-prefs-card">
        <h2>{t('settings.language_section')}</h2>
        <p className="card-subtitle">{t('settings.language_hint')}</p>
        <div className="settings-control-row settings-control-row--wrap">
          <LanguageSwitcher />
        </div>
      </section>

      <section className="card card--elevated settings-prefs-card">
        <h2>{t('settings.theme_section')}</h2>
        <p className="card-subtitle">{t('settings.theme_hint')}</p>
        <div className="settings-control-row">
          <ThemeToggle />
        </div>
      </section>

      <section className="card card--elevated settings-prefs-card">
        <h2>{t('settings.prompt_templates_section')}</h2>
        <p className="card-subtitle">{t('settings.prompt_templates_hint')}</p>
        <div className="settings-control-row">
          <button type="button" className="btn btn-primary" onClick={() => goTo('/prompt-templates')}>
            {t('settings.prompt_templates_open')}
          </button>
        </div>
      </section>

      <section className="card card--elevated settings-prefs-card">
        <h2>{t('settings.time_grains_section')}</h2>
        <p className="card-subtitle">{t('settings.time_grains_hint')}</p>
        <div className="settings-control-row">
          <button type="button" className="btn btn-primary" onClick={() => goTo('/time-grains')}>
            {t('settings.time_grains_open')}
          </button>
        </div>
      </section>

      <p className="settings-footnote">{t('settings.persist_hint')}</p>
    </div>
  )
}
