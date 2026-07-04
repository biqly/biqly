import type { TFunction } from '../../i18n'
import { buttonClass } from '../../lib/buttonClasses'
import { formStackClass, legacyFormClass } from '../../lib/formClasses'
import { modalActionsClass } from '../../lib/modalClasses'
import type { Datasource } from '../../types/metadata'
import { ErrorAlert } from '../ui/ErrorAlert'
import { Modal } from '../ui/Modal'
import { Select } from '../ui/Select'
import type { SkillFormState } from './types'

interface SkillSemanticModel {
  id: string
  name: string
  label?: string | null
  status?: string
}

interface SkillFormModalProps {
  mode: 'new' | 'edit'
  open: boolean
  title: string
  formError: string | null
  datasources: Datasource[]
  semanticModels: SkillSemanticModel[]
  form: SkillFormState
  onChange: (patch: Partial<SkillFormState>) => void
  onClose: () => void
  onSave: () => void
  saving?: boolean
  t: TFunction
}

export function SkillFormModal({
  mode,
  open,
  title,
  formError,
  datasources,
  semanticModels,
  form,
  onChange,
  onClose,
  onSave,
  saving = false,
  t,
}: SkillFormModalProps) {
  const id = (field: string) => `skill-${mode}-${field}`

  return (
    <Modal open={open} title={title} onClose={onClose}>
      <div className={formStackClass}>
        {formError && <ErrorAlert error={formError} />}

        <div className={legacyFormClass('form-group')}>
          <label htmlFor={id('ds')}>{t('skills.label_select_datasource')}</label>
          <Select
            id={id('ds')}
            value={form.datasourceId}
            onChange={(val) => onChange({ datasourceId: val })}
            options={datasources.map((d) => ({ value: d.id, label: d.name }))}
          />
        </div>

        <div className={legacyFormClass('form-group')}>
          <label htmlFor={id('model')}>{t('skills.label_select_model')}</label>
          <Select
            id={id('model')}
            value={form.modelId}
            onChange={(val) => onChange({ modelId: val })}
            options={[
              { value: '', label: t('skills.label_all_models') },
              ...semanticModels.map((m) => ({ value: m.id, label: m.label ?? m.name })),
            ]}
          />
        </div>

        <div className={legacyFormClass('form-group')}>
          <label htmlFor={id('name')}>{t('skills.label_name')}</label>
          <input
            id={id('name')}
            value={form.name}
            onChange={(e) => onChange({ name: e.target.value })}
            placeholder={t('skills.placeholder_name')}
            autoComplete="off"
          />
        </div>

        <div className={legacyFormClass('form-group')}>
          <label htmlFor={id('desc')}>{t('skills.label_description')}</label>
          <textarea
            id={id('desc')}
            value={form.description}
            onChange={(e) => onChange({ description: e.target.value })}
            placeholder={t('skills.placeholder_description')}
            rows={2}
          />
        </div>

        <div className={legacyFormClass('form-group')}>
          <label htmlFor={id('question')}>{t('skills.label_question')}</label>
          <textarea
            id={id('question')}
            value={form.question}
            onChange={(e) => onChange({ question: e.target.value })}
            placeholder={t('skills.placeholder_question')}
            rows={2}
          />
        </div>

        <div className={legacyFormClass('form-group')}>
          <label htmlFor={id('lq')}>{t('skills.label_logical_query')}</label>
          <textarea
            id={id('lq')}
            value={form.logicalQuery}
            onChange={(e) => onChange({ logicalQuery: e.target.value })}
            placeholder='{ "select": ... }'
            rows={6}
            style={{ fontFamily: 'monospace' }}
          />
        </div>

        <div className={legacyFormClass('form-group')}>
          <label htmlFor={id('params')}>{t('skills.label_parameters')}</label>
          <textarea
            id={id('params')}
            value={form.parameters}
            onChange={(e) => onChange({ parameters: e.target.value })}
            placeholder='[{ "name": "country", "label": "Country", "required": true }]'
            rows={4}
            style={{ fontFamily: 'monospace' }}
          />
          <p className={legacyFormClass('form-hint')}>{t('skills.parameters_hint')}</p>
        </div>

        <div className={legacyFormClass('form-group')}>
          <label htmlFor={id('tags')}>{t('skills.label_tags')}</label>
          <input
            id={id('tags')}
            value={form.tags}
            onChange={(e) => onChange({ tags: e.target.value })}
            placeholder="finance, monthly"
            autoComplete="off"
          />
        </div>

        <div className={modalActionsClass()}>
          <button type="button" className={buttonClass('ghost')} onClick={onClose}>
            {t('skills.btn_cancel')}
          </button>
          <button
            type="button"
            className={buttonClass('primary')}
            onClick={onSave}
            disabled={saving}
          >
            {t('skills.btn_save')}
          </button>
        </div>
      </div>
    </Modal>
  )
}
