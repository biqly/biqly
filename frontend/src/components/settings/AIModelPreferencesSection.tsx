import { useCallback, useEffect, useMemo, useState } from 'react'

import type { AIPurpose } from '../../api/aiProviders'
import {
  deleteUserAIPreference,
  fetchUserAIModels,
  putUserAIPreferences,
  type SelectableAIModel,
} from '../../api/aiUserModels'
import { useToast } from '../../hooks/useToast'
import { useT } from '../../i18n'
import { buttonClass } from '../../lib/buttonClasses'
import { cardClass } from '../../lib/cardClasses'
import { cn } from '../../lib/cn'
import { friendlyErrorMessage } from '../../utils/error'
import { adminBtnAutoWidthClass } from '../admin/adminClasses'
import { useAuth } from '../auth/AuthProvider'
import { LoadingOverlay } from '../ui/LoadingOverlay'
import { Select } from '../ui/Select'

// Only the purposes that are resolved per-user at runtime and are safe to vary
// per user are offered here. embedding/translation write shared metadata and
// vectors (must stay consistent across users) and evaluation (judge) is an
// admin concern — those stay on the workspace/admin default. The backend
// rejects per-user preferences for any other purpose.
const PURPOSES: AIPurpose[] = ['query', 'describe']

const PURPOSE_ICONS: Record<AIPurpose, React.ReactNode> = {
  query: (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      width="16"
      height="16"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2.5"
      strokeLinecap="round"
      strokeLinejoin="round"
      className="lucide lucide-message-square-code"
      style={{ color: 'var(--accent)' }}
    >
      <path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z" />
      <path d="m10 8-2 2 2 2" />
      <path d="m14 8 2 2-2 2" />
    </svg>
  ),
  describe: (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      width="16"
      height="16"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2.5"
      strokeLinecap="round"
      strokeLinejoin="round"
      className="lucide lucide-file-text"
      style={{ color: '#10b981' }}
    >
      <path d="M15 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7Z" />
      <path d="M14 2v4a2 2 0 0 0 2 2h4" />
      <path d="M10 9H8" />
      <path d="M16 13H8" />
      <path d="M16 17H8" />
    </svg>
  ),
  embedding: (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      width="16"
      height="16"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2.5"
      strokeLinecap="round"
      strokeLinejoin="round"
      className="lucide lucide-database"
      style={{ color: '#f59e0b' }}
    >
      <circle cx="12" cy="5" r="3" />
      <path d="M3 5V19a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2V5" />
      <path d="M3 12h18" />
    </svg>
  ),
  translation: (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      width="16"
      height="16"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2.5"
      strokeLinecap="round"
      strokeLinejoin="round"
      className="lucide lucide-languages"
      style={{ color: '#3b82f6' }}
    >
      <path d="m5 8 6 6" />
      <path d="m4 14 6-6 2-3" />
      <path d="M2 5h12" />
      <path d="M7 2h1" />
      <path d="m22 22-5-10-5 10" />
      <path d="M14 18h6" />
    </svg>
  ),
  judge: (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      width="16"
      height="16"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2.5"
      strokeLinecap="round"
      strokeLinejoin="round"
      className="lucide lucide-gavel"
      style={{ color: '#ec4899' }}
    >
      <path d="m14 13-7.5 7.5c-.83.83-2.17.83-3 0 0 0 0 0 0 0a2.12 2.12 0 0 1 0-3L11 10" />
      <path d="m16 16 3-3" />
      <path d="m8 7 9 9" />
      <path d="m11 4 8 8" />
    </svg>
  ),
}

export function AIModelPreferencesSection() {
  const t = useT()
  const toast = useToast()
  const { accessToken } = useAuth()
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [dbManaged, setDbManaged] = useState(false)
  const [restricted, setRestricted] = useState(false)
  const [models, setModels] = useState<SelectableAIModel[]>([])
  const [choices, setChoices] = useState<Record<string, string>>({})

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const data = await fetchUserAIModels(accessToken ?? undefined)
      setDbManaged(data.db_managed)
      setRestricted(data.restricted)
      setModels(data.models)
      setChoices(data.preferences)
    } catch (e) {
      toast.error(friendlyErrorMessage(t, e))
    } finally {
      setLoading(false)
    }
  }, [toast, accessToken, t])

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load()
  }, [load])

  const modelsByPurpose = useMemo(() => {
    const map: Record<string, SelectableAIModel[]> = {}
    for (const p of PURPOSES) {
      map[p] = []
    }
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
      const res = await putUserAIPreferences(preferences, accessToken ?? undefined)
      setChoices(res.preferences)
      toast.success(t('settings.ai_models.saved'))
    } catch (e) {
      toast.error(friendlyErrorMessage(t, e))
    } finally {
      setSaving(false)
    }
  }

  const handleClear = async (purpose: AIPurpose) => {
    setSaving(true)
    try {
      await deleteUserAIPreference(purpose, accessToken ?? undefined)
      setChoices((prev) => {
        const next = { ...prev }
        delete next[purpose]
        return next
      })
      toast.success(t('settings.ai_models.cleared'))
    } catch (e) {
      toast.error(friendlyErrorMessage(t, e))
    } finally {
      setSaving(false)
    }
  }

  return (
    <section
      className={cardClass({ className: 'mb-0', elevated: true })}
      aria-labelledby="ai-models-prefs-heading"
    >
      <div className="mb-4 flex flex-wrap items-start justify-between gap-x-4 gap-y-3">
        <div>
          <h2 id="ai-models-prefs-heading" className="m-0">
            {t('settings.ai_models.section')}
          </h2>
          <p className="text-foreground-muted mt-[0.35rem] mr-0 mb-0 ml-0 max-w-2xl flex-[1_1_100%] text-[0.875rem] leading-[1.45]">
            {t('settings.ai_models.hint')}
          </p>
          {restricted && (
            <p className="text-foreground-muted text-caption m-0 mt-[0.35rem] max-w-2xl leading-[1.45]">
              {t('settings.ai_models.restricted_hint')}
            </p>
          )}
        </div>
      </div>

      <LoadingOverlay loading={loading}>
        <div className="p-0">
          <div className="grid grid-cols-1 gap-3 min-[1100px]:grid-cols-3 sm:grid-cols-2">
            {PURPOSES.map((purpose) => {
              const options = modelsByPurpose[purpose] ?? []
              const value = choices[purpose] ?? ''
              return (
                <div
                  key={purpose}
                  className={
                    'border-border bg-card-raised flex min-h-29.5 flex-col gap-2 rounded-[10px] border px-3.5 py-3'
                  }
                >
                  <div className="flex items-start justify-between gap-2">
                    <div>
                      <p className="text-foreground text-caption m-0 inline-flex items-center gap-2 leading-[1.3] font-semibold">
                        {PURPOSE_ICONS[purpose]}
                        {t(`admin.ai_providers.purposes.${purpose}`)}
                      </p>
                      <p className="text-foreground-faint mt-1 mb-0 text-[0.72rem] leading-[1.35]">
                        {t(`settings.ai_models.purpose_hints.${purpose}`)}
                      </p>
                    </div>
                    {value ? (
                      <button
                        type="button"
                        className="text-foreground-muted hover:enabled:text-foreground shrink-0 cursor-pointer rounded-md border-0 bg-transparent px-2 py-0.5 text-[0.72rem] font-semibold hover:enabled:bg-white/5 disabled:cursor-not-allowed disabled:opacity-45"
                        disabled={saving}
                        onClick={() => {
                          void handleClear(purpose)
                        }}
                      >
                        {t('settings.ai_models.clear')}
                      </button>
                    ) : null}
                  </div>
                  <div className="w-full [&_.ui-select]:w-full">
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
                  {options.length === 0 && (
                    <p className="text-warning m-0 text-[0.72rem]">
                      {t('settings.ai_models.no_models_for_purpose')}
                    </p>
                  )}
                </div>
              )
            })}
          </div>
          <p className="text-foreground-faint mt-1 mb-0 text-[0.72rem] leading-[1.35]">
            {t('settings.ai_models.admin_managed_note')}
          </p>
          <div className={`border-border mt-5 flex justify-end border-t pt-5`}>
            <button
              type="button"
              className={cn(buttonClass('primary', { size: 'sm' }), adminBtnAutoWidthClass)}
              disabled={saving || loading}
              onClick={() => {
                void handleSave()
              }}
            >
              {saving ? '…' : t('settings.ai_models.save')}
            </button>
          </div>
        </div>
      </LoadingOverlay>
    </section>
  )
}
