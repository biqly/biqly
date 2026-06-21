import { useT } from '../i18n'
import { buttonClass } from '../lib/buttonClasses'
import { legacyFormClass } from '../lib/formClasses'
import { modalActionsClass } from '../lib/modalClasses'
import { ErrorAlert } from './ui/ErrorAlert'
import { Modal } from './ui/Modal'

interface TimeGrain {
  grain: string
  suffix: string
  requires_time: boolean
  synonyms: string[]
}

export function TimeGrainsEditModal({
  editingGrain,
  formSuffix,
  setFormSuffix,
  formRequiresTime,
  setFormRequiresTime,
  formSynonyms,
  setFormSynonyms,
  formError,
  loading,
  onCancel,
  onSave,
}: {
  editingGrain: TimeGrain | null
  formSuffix: string
  setFormSuffix: (value: string) => void
  formRequiresTime: boolean
  setFormRequiresTime: (value: boolean) => void
  formSynonyms: string
  setFormSynonyms: (value: string) => void
  formError: string | null
  loading: boolean
  onCancel: () => void
  onSave: () => void
}) {
  const t = useT()
  if (!editingGrain) {
    return null
  }
  return (
    <Modal
      open
      title={`${t('time_grains.edit_title') || 'Edit Time Grain'}: ${editingGrain.grain}`}
      onClose={onCancel}
      bodyClassName="modal-body grid gap-3"
    >
      <div className={legacyFormClass('form-group')}>
        <label htmlFor="tg-suffix">{t('time_grains.label_suffix') || 'Suffix'}</label>
        <input
          id="tg-suffix"
          type="text"
          value={formSuffix}
          onChange={(e) => setFormSuffix(e.target.value)}
        />
      </div>
      <div
        className={legacyFormClass('form-group')}
        style={{
          flexDirection: 'row',
          alignItems: 'flex-start',
          gap: '0.75rem',
          marginTop: '1rem',
        }}
      >
        <input
          id="tg-requires-time"
          type="checkbox"
          checked={formRequiresTime}
          onChange={(e) => setFormRequiresTime(e.target.checked)}
          style={{ width: 'auto', marginTop: '0.2rem' }}
        />
        <div>
          <label htmlFor="tg-requires-time" style={{ marginBottom: '0.1rem', cursor: 'pointer' }}>
            {t('time_grains.label_requires_time') || 'Requires Time Column'}
          </label>
          <p style={{ fontSize: '0.78rem', color: 'var(--text-muted)', margin: 0 }}>
            {t('time_grains.label_requires_time_hint') ||
              'If checked, this grain only matches datetime/timestamp types.'}
          </p>
        </div>
      </div>
      <div className={legacyFormClass('form-group')} style={{ marginTop: '1.25rem' }}>
        <label htmlFor="tg-synonyms">
          {t('time_grains.label_synonyms') || 'Synonyms (comma-separated)'}
        </label>
        <textarea
          id="tg-synonyms"
          value={formSynonyms}
          onChange={(e) => setFormSynonyms(e.target.value)}
          placeholder={t('time_grains.placeholder_synonyms') || 'e.g., daily, per day, day'}
          rows={4}
        />
      </div>
      {formError && <ErrorAlert error={formError} />}
      <div className={modalActionsClass()}>
        <button type="button" className={buttonClass('ghost')} onClick={onCancel}>
          {t('common.cancel') || 'Cancel'}
        </button>
        <button
          type="button"
          className={buttonClass('primary')}
          onClick={onSave}
          disabled={loading}
        >
          {loading ? t('common.saving') || 'Saving…' : t('common.save') || 'Save'}
        </button>
      </div>
    </Modal>
  )
}
