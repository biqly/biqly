import { useState } from 'react'

import { useAdminApi } from '../../hooks/useApi'
import { useT } from '../../i18n'
import { formHintClass } from '../../lib/formClasses'
import { legacyLayoutClass } from '../../lib/layoutClasses'
import { Button } from '../ui/Button'
import { Select } from '../ui/Select'
import {
  adminCardClass,
  adminErrBoxClass,
  adminFormLabelClass,
  adminInputClass,
  adminLabelTextClass,
  adminPanelHeaderClass,
} from './adminClasses'

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
  const [templateName, setTemplateName] = useState(
    experiment?.template_name ?? TEMPLATE_NAMES[0] ?? 'system_rules',
  )
  const [locale, setLocale] = useState(experiment?.locale ?? LOCALES[0] ?? 'en')

  const isEdit = !!experiment?.id

  const templateOptions = TEMPLATE_NAMES.map((value) => ({ value, label: value }))
  const localeOptions = LOCALES.map((value) => ({
    value,
    label: value.toUpperCase(),
  }))

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
    <div className={legacyLayoutClass('page-stack')}>
      <div className={adminPanelHeaderClass}>
        <div>
          <h2 style={{ margin: 0 }}>
            {isEdit ? t('admin.ab_experiments.edit_title') : t('admin.ab_experiments.new_title')}
          </h2>
          <p className={formHintClass}>{t('admin.ab_experiments.description')}</p>
        </div>
      </div>

      <div className={adminCardClass}>
        <form className="flex flex-col gap-5" onSubmit={(e) => void handleSubmit(e)}>
          {error ? (
            <div className={adminErrBoxClass} role="alert">
              {error}
            </div>
          ) : null}

          <label className={adminFormLabelClass} htmlFor="exp-name">
            <span className={adminLabelTextClass}>{t('admin.ab_experiments.fields.name')} *</span>
            <input
              id="exp-name"
              type="text"
              className={adminInputClass}
              value={name}
              onChange={(e) => setName(e.target.value)}
              required
              disabled={loading}
            />
          </label>

          <label className={adminFormLabelClass} htmlFor="exp-description">
            <span className={adminLabelTextClass}>
              {t('admin.ab_experiments.fields.description')}
            </span>
            <textarea
              id="exp-description"
              rows={3}
              className={adminInputClass}
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              disabled={loading}
            />
          </label>

          {!isEdit ? (
            <>
              <label className={adminFormLabelClass}>
                <span className={adminLabelTextClass}>
                  {t('admin.ab_experiments.fields.template_name')}
                </span>
                <Select
                  value={templateName}
                  onChange={setTemplateName}
                  options={templateOptions}
                  disabled={loading}
                  ariaLabel={t('admin.ab_experiments.fields.template_name')}
                />
              </label>

              <label className={adminFormLabelClass}>
                <span className={adminLabelTextClass}>
                  {t('admin.ab_experiments.fields.locale')}
                </span>
                <Select
                  value={locale}
                  onChange={setLocale}
                  options={localeOptions}
                  disabled={loading}
                  ariaLabel={t('admin.ab_experiments.fields.locale')}
                />
              </label>
            </>
          ) : null}

          <div className="mt-1 flex justify-end gap-3">
            <Button variant="secondary" autoWidth onClick={onCancel} disabled={loading}>
              {t('common.cancel')}
            </Button>
            <Button type="submit" variant="primary" autoWidth disabled={loading || !name.trim()}>
              {loading ? t('common.saving') : t('common.save')}
            </Button>
          </div>
        </form>
      </div>
    </div>
  )
}
