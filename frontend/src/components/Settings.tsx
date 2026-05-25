import { useT } from '../i18n'

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
        <div className="card-intro card-intro--compact">
          <h2>{t('settings.prompt_templates_section')}</h2>
          <p className="card-lead card-lead--single-line" title={t('settings.prompt_templates_hint')}>
            {t('settings.prompt_templates_hint')}
          </p>
        </div>
        <div className="settings-control-row">
          <button type="button" className="btn btn-primary" onClick={() => goTo('/prompt-templates')}>
            {t('settings.prompt_templates_open')}
          </button>
        </div>
      </section>

      <section className="card card--elevated settings-prefs-card">
        <div className="card-intro card-intro--compact">
          <h2>{t('settings.time_grains_section')}</h2>
          <p className="card-lead card-lead--single-line" title={t('settings.time_grains_hint')}>
            {t('settings.time_grains_hint')}
          </p>
        </div>
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
