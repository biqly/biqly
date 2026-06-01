import { useCallback, useEffect, useMemo, useState } from 'react'
import { useT } from '../../i18n'
import { useToast } from '../../hooks/useToast'
import { Select } from '../ui/Select'
import { LoadingOverlay } from '../ui/LoadingOverlay'
import type { AIPurpose } from '../../api/aiProviders'
import {
  deleteUserAIPreference,
  fetchUserAIModels,
  putUserAIPreferences,
  type SelectableAIModel,
} from '../../api/aiUserModels'

const PURPOSES: AIPurpose[] = ['query', 'describe', 'embedding', 'translation', 'judge']

export function AIModelPreferencesSection() {
  const t = useT()
  const toast = useToast()
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [dbManaged, setDbManaged] = useState(false)
  const [restricted, setRestricted] = useState(false)
  const [models, setModels] = useState<SelectableAIModel[]>([])
  const [choices, setChoices] = useState<Record<string, string>>({})

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const data = await fetchUserAIModels()
      setDbManaged(data.db_managed)
      setRestricted(data.restricted)
      setModels(data.models ?? [])
      setChoices(data.preferences ?? {})
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e))
    } finally {
      setLoading(false)
    }
  }, [toast])

  useEffect(() => {
    load()
  }, [load])

  const modelsByPurpose = useMemo(() => {
    const map: Record<string, SelectableAIModel[]> = {}
    for (const p of PURPOSES) map[p] = []
    for (const m of models) {
      const bucket = map[m.purpose] ?? (map[m.purpose] = [])
      bucket.push(m)
    }
    return map
  }, [models])

  if (!loading && !dbManaged) {
    return null
  }

  const handleSave = async () => {
    setSaving(true)
    try {
      const preferences = PURPOSES.flatMap((purpose) => {
        const modelID = choices[purpose]
        return modelID ? [{ purpose, model_id: modelID }] : []
      })
      const res = await putUserAIPreferences(preferences)
      setChoices(res.preferences ?? {})
      toast.success(t('settings.ai_models.saved'))
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e))
    } finally {
      setSaving(false)
    }
  }

  const handleClear = async (purpose: AIPurpose) => {
    setSaving(true)
    try {
      await deleteUserAIPreference(purpose)
      setChoices((prev) => {
        const next = { ...prev }
        delete next[purpose]
        return next
      })
      toast.success(t('settings.ai_models.cleared'))
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e))
    } finally {
      setSaving(false)
    }
  }

  return (
    <section
      className="card card--elevated settings-prefs-card settings-security-card ai-model-prefs"
      aria-labelledby="ai-models-prefs-heading"
    >
      <div className="settings-prefs-card__header">
        <div>
          <h2 id="ai-models-prefs-heading">{t('settings.ai_models.section')}</h2>
          <p>{t('settings.ai_models.hint')}</p>
          {restricted && <p className="ai-model-prefs__restricted">{t('settings.ai_models.restricted_hint')}</p>}
        </div>
      </div>

      <LoadingOverlay loading={loading}>
        <div className="ai-model-prefs__body">
          <div className="ai-model-prefs__grid">
            {PURPOSES.map((purpose) => {
              const options = modelsByPurpose[purpose] ?? []
              const value = choices[purpose] ?? ''
              return (
                <div key={purpose} className="ai-purpose-pref">
                  <div className="ai-purpose-pref__head">
                    <div>
                      <p className="ai-purpose-pref__title">{t(`admin.ai_providers.purposes.${purpose}`)}</p>
                      <p className="ai-purpose-pref__hint">{t(`settings.ai_models.purpose_hints.${purpose}`)}</p>
                    </div>
                    {value ? (
                      <button
                        type="button"
                        className="ai-purpose-pref__clear"
                        disabled={saving}
                        onClick={() => handleClear(purpose)}
                      >
                        {t('settings.ai_models.clear')}
                      </button>
                    ) : null}
                  </div>
                  <Select
                    value={value}
                    onChange={(v) => setChoices((prev) => ({ ...prev, [purpose]: v }))}
                    options={[
                      { value: '', label: t('settings.ai_models.use_default') },
                      ...options.map((m) => ({
                        value: m.id,
                        label: `${m.display_name} (${m.provider_name})`,
                      })),
                    ]}
                    disabled={saving || options.length === 0}
                  />
                  {options.length === 0 && (
                    <p className="ai-purpose-pref__empty-hint">{t('settings.ai_models.no_models_for_purpose')}</p>
                  )}
                </div>
              )
            })}
          </div>
          <div className="ai-model-prefs__footer">
            <button
              type="button"
              className="btn btn-primary btn-sm btn-auto-width"
              disabled={saving || loading}
              onClick={handleSave}
            >
              {saving ? '…' : t('settings.ai_models.save')}
            </button>
          </div>
        </div>
      </LoadingOverlay>
    </section>
  )
}
