import { useState } from 'react'

import { useAdminApi } from '../../hooks/useApi'
import { useT } from '../../i18n'

export interface Experiment {
  id?: string
  name: string
  description: string
  template_name: string
  locale: string
  status?: string
}

interface ABExperimentFormProps {
  experiment?: Experiment | null
  onSave: (savedExp: Experiment) => void
  onCancel: () => void
}

const TEMPLATE_NAMES = [
  'system_rules',
  'output_format',
  'retry',
  'clarification',
  'ambiguity',
  'prompt_layout',
]

const LOCALES = ['en', 'tr']

export function ABExperimentForm({ experiment, onSave, onCancel }: ABExperimentFormProps) {
  const t = useT()
  const { postData, putData, error, loading } = useAdminApi()

  const [name, setName] = useState(experiment?.name ?? '')
  const [description, setDescription] = useState(experiment?.description ?? '')
  const [templateName, setTemplateName] = useState(experiment?.template_name ?? TEMPLATE_NAMES[0])
  const [locale, setLocale] = useState(experiment?.locale ?? LOCALES[0])

  const isEdit = !!experiment?.id

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!name.trim()) {
      return
    }

    const payload = {
      name: name.trim(),
      description: description.trim(),
      template_name: templateName,
      locale,
    }

    let result: Experiment | null = null
    if (isEdit && experiment.id) {
      // For updates, the API only takes name and description (active parameters cannot be modified)
      result = await putData<Experiment>(`/api/ai/ab-experiments/${experiment.id}`, {
        name: payload.name,
        description: payload.description,
      })
    } else {
      result = await postData<Experiment>('/api/ai/ab-experiments', payload)
    }

    if (result) {
      onSave(result)
    }
  }

  return (
    <div className={`bg-card border border-border rounded-lg p-6 shadow-card-sm`}>
      <h2 className={`text-lg font-semibold mt-0 mb-4 border-b border-border pb-3`}>
        {isEdit ? t('admin.ab_experiments.edit_title') : t('admin.ab_experiments.new_title')}
      </h2>

      <form className="flex flex-col gap-5" onSubmit={(e) => void handleSubmit(e)}>
        {error && (
          <div className="alert alert-danger" role="alert">
            {error}
          </div>
        )}

        <div className="flex flex-col gap-1.5">
          <label className="text-sm font-medium text-foreground-muted" htmlFor="exp-name">
            {t('admin.ab_experiments.fields.name')} *
          </label>
          <input
            id="exp-name"
            type="text"
            className={`px-3 py-2 border border-border rounded-md text-sm bg-[var(--input-bg)] text-inherit w-full focus:outline-none focus:border-accent focus:ring-2 focus:ring-[var(--control-focus-ring)]`}
            value={name}
            onChange={(e) => setName(e.target.value)}
            required
            disabled={loading}
          />
        </div>

        <div className="flex flex-col gap-1.5">
          <label className="text-sm font-medium text-foreground-muted" htmlFor="exp-description">
            {t('admin.ab_experiments.fields.description')}
          </label>
          <textarea
            id="exp-description"
            rows={3}
            className={`px-3 py-2 border border-border rounded-md text-sm bg-[var(--input-bg)] text-inherit w-full focus:outline-none focus:border-accent focus:ring-2 focus:ring-[var(--control-focus-ring)]`}
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            disabled={loading}
          />
        </div>

        {!isEdit && (
          <>
            <div className="flex flex-col gap-1.5">
              <label className="text-sm font-medium text-foreground-muted" htmlFor="exp-template">
                {t('admin.ab_experiments.fields.template_name')}
              </label>
              <select
                id="exp-template"
                className={`px-3 py-2 border border-border rounded-md text-sm bg-[var(--input-bg)] text-inherit w-full focus:outline-none focus:border-accent focus:ring-2 focus:ring-[var(--control-focus-ring)]`}
                value={templateName}
                onChange={(e) => setTemplateName(e.target.value)}
                disabled={loading}
              >
                {TEMPLATE_NAMES.map((name) => (
                  <option key={name} value={name}>
                    {name}
                  </option>
                ))}
              </select>
            </div>

            <div className="flex flex-col gap-1.5">
              <label className="text-sm font-medium text-foreground-muted" htmlFor="exp-locale">
                {t('admin.ab_experiments.fields.locale')}
              </label>
              <select
                id="exp-locale"
                className={`px-3 py-2 border border-border rounded-md text-sm bg-[var(--input-bg)] text-inherit w-full focus:outline-none focus:border-accent focus:ring-2 focus:ring-[var(--control-focus-ring)]`}
                value={locale}
                onChange={(e) => setLocale(e.target.value)}
                disabled={loading}
              >
                {LOCALES.map((loc) => (
                  <option key={loc} value={loc}>
                    {loc.toUpperCase()}
                  </option>
                ))}
              </select>
            </div>
          </>
        )}

        <div className="flex justify-end gap-3 mt-3">
          <button type="button" className="btn btn-secondary" onClick={onCancel} disabled={loading}>
            {t('common.cancel')}
          </button>
          <button type="submit" className="btn btn-primary" disabled={loading || !name.trim()}>
            {loading ? t('common.saving') : t('common.save')}
          </button>
        </div>
      </form>
    </div>
  )
}
