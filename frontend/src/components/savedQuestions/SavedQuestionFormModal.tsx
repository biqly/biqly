import { Modal } from '../ui/Modal'
import { Select } from '../ui/Select'
import { ErrorAlert } from '../ui/ErrorAlert'
import type { Datasource } from '../../types/metadata'
import type { SavedQuestionSemanticModel, SavedQuestionFormState } from './types'

interface SavedQuestionFormModalProps {
  mode: 'new' | 'edit'
  open: boolean
  title: string
  formError: string | null
  datasources: Datasource[]
  semanticModels: SavedQuestionSemanticModel[]
  form: SavedQuestionFormState
  onChange: (patch: Partial<SavedQuestionFormState>) => void
  onClose: () => void
  onSave: () => void
  t: any
}

const DIALECTS = ['postgresql', 'mysql', 'sqlserver', 'clickhouse']
const LOCALES = ['', 'en', 'tr']

export function SavedQuestionFormModal({
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
  t,
}: SavedQuestionFormModalProps) {
  const id = (field: string) => `${mode}-${field}`

  return (
    <Modal open={open} title={title} onClose={onClose}>
      <div className="form-stack">
        {formError && <ErrorAlert error={formError} />}

        <div className="form-group">
          <label htmlFor={id('ds')}>{t('saved_questions.label_select_datasource')}</label>
          <Select
            id={id('ds')}
            value={form.datasourceId}
            onChange={(val) => onChange({ datasourceId: val })}
            options={datasources.map((d) => ({ value: d.id, label: d.name }))}
          />
        </div>

        <div className="form-group">
          <label htmlFor={id('model')}>{t('saved_questions.label_select_model')}</label>
          <Select
            id={id('model')}
            value={form.modelId}
            onChange={(val) => onChange({ modelId: val })}
            options={[
              { value: '', label: t('saved_questions.label_all_models') },
              ...semanticModels.map((m) => ({ value: m.id, label: m.label || m.name })),
            ]}
          />
        </div>

        <div className="form-group">
          <label htmlFor={id('name')}>{t('saved_questions.label_name')}</label>
          <input
            id={id('name')}
            value={form.name}
            onChange={(e) => onChange({ name: e.target.value })}
            placeholder="e.g. Sales by region"
            autoComplete="off"
          />
        </div>

        <div className="form-group">
          <label htmlFor={id('desc')}>{t('saved_questions.label_description')}</label>
          <textarea
            id={id('desc')}
            value={form.description}
            onChange={(e) => onChange({ description: e.target.value })}
            placeholder="e.g. Shows regional breakdown for orders"
            rows={2}
          />
        </div>

        <div className="form-group">
          <label htmlFor={id('question')}>{t('saved_questions.label_question')}</label>
          <textarea
            id={id('question')}
            value={form.question}
            onChange={(e) => onChange({ question: e.target.value })}
            placeholder="e.g. ne kadar sipariş aldık ülkelere göre?"
            rows={2}
          />
        </div>

        <div className="form-group">
          <label htmlFor={id('lq')}>{t('saved_questions.label_logical_query')}</label>
          <textarea
            id={id('lq')}
            value={form.logicalQuery}
            onChange={(e) => onChange({ logicalQuery: e.target.value })}
            placeholder='{ "select": ... }'
            rows={6}
            style={{ fontFamily: 'monospace' }}
          />
        </div>

        <div className="form-group">
          <label htmlFor={id('tags')}>{t('saved_questions.label_tags')}</label>
          <input
            id={id('tags')}
            value={form.tags}
            onChange={(e) => onChange({ tags: e.target.value })}
            placeholder="sales, region"
            autoComplete="off"
          />
        </div>

        <div className="form-group">
          <label htmlFor={id('dialect')}>{t('saved_questions.label_dialect')}</label>
          <Select
            id={id('dialect')}
            value={form.dialect}
            onChange={(val) => onChange({ dialect: val })}
            options={DIALECTS.map((d) => ({ value: d, label: d }))}
          />
        </div>

        <div className="form-group">
          <label htmlFor={id('locale')}>{t('saved_questions.label_locale')}</label>
          <Select
            id={id('locale')}
            value={form.locale}
            onChange={(val) => onChange({ locale: val })}
            options={LOCALES.map((l) => ({ value: l, label: l || 'Default' }))}
          />
        </div>

        <div className="form-group" style={{ flexDirection: 'row', alignItems: 'center', gap: '0.5rem' }}>
          <input
            type="checkbox"
            id={id('is-few-shot')}
            checked={form.isFewShot}
            onChange={(e) => onChange({ isFewShot: e.target.checked })}
          />
          <label htmlFor={id('is-few-shot')} style={{ margin: 0, cursor: 'pointer' }}>
            {t('saved_questions.label_is_few_shot')}
          </label>
        </div>

        <div className="modal-actions" style={{ display: 'flex', gap: '0.5rem', justifyContent: 'flex-end', marginTop: '1rem' }}>
          <button type="button" className="btn btn--neutral" onClick={onClose}>
            {t('saved_questions.btn_cancel')}
          </button>
          <button type="button" className="btn btn-primary" onClick={onSave}>
            {t('saved_questions.btn_save')}
          </button>
        </div>
      </div>
    </Modal>
  )
}
