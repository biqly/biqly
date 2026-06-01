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
    <section className="card card--elevated settings-prefs-card" aria-labelledby="ai-models-prefs-heading">
      <div className="settings-prefs-card__header">
        <div>
          <h2 id="ai-models-prefs-heading">{t('settings.ai_models.section')}</h2>
          <p>{t('settings.ai_models.hint')}</p>
          {restricted && (
            <p style={{ fontSize: 13, color: 'var(--text-secondary)', marginTop: 6 }}>
              {t('settings.ai_models.restricted_hint')}
            </p>
          )}
        </div>
        <button
          type="button"
          className="btn btn-primary btn-sm btn-auto-width"
          disabled={saving || loading}
          onClick={handleSave}
        >
          {saving ? '…' : t('settings.ai_models.save')}
        </button>
      </div>

      <LoadingOverlay loading={loading}>
        <div style={{ padding: '0 16px 16px', display: 'flex', flexDirection: 'column', gap: 14 }}>
          {PURPOSES.map((purpose) => {
            const options = modelsByPurpose[purpose] ?? []
            const value = choices[purpose] ?? ''
            return (
              <div key={purpose} style={{ display: 'flex', flexWrap: 'wrap', gap: 12, alignItems: 'flex-end' }}>
                <div style={{ flex: '1 1 200px', minWidth: 200 }}>
                  <label style={{ display: 'block', fontSize: 13, fontWeight: 600, marginBottom: 6 }}>
                    {t(`admin.ai_providers.purposes.${purpose}`)}
                  </label>
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
                </div>
                {value && (
                  <button
                    type="button"
                    className="btn btn-secondary btn-sm"
                    disabled={saving}
                    onClick={() => handleClear(purpose)}
                  >
                    {t('settings.ai_models.clear')}
                  </button>
                )}
              </div>
            )
          })}
        </div>
      </LoadingOverlay>
    </section>
  )
}
