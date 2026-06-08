import { ErrorAlert } from './ui/ErrorAlert'
import type { TFunction } from '../i18n'

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
  t,
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
  t: TFunction
}) {
  if (!editingGrain) {
    return null
  }
  return (
    <div className="modal-backdrop" onClick={onCancel}>
      <div className="modal-card" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h2>
            {t('time_grains.edit_title') || 'Edit Time Grain'}: {editingGrain.grain}
          </h2>
          <button className="modal-close" onClick={onCancel}>
            ×
          </button>
        </div>
        <div className="modal-body">
          <div className="form-group">
            <label htmlFor="tg-suffix">{t('time_grains.label_suffix') || 'Suffix'}</label>
            <input
              id="tg-suffix"
              type="text"
              value={formSuffix}
              onChange={(e) => setFormSuffix(e.target.value)}
            />
          </div>
          <div
            className="form-group"
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
              <label
                htmlFor="tg-requires-time"
                style={{ marginBottom: '0.1rem', cursor: 'pointer' }}
              >
                {t('time_grains.label_requires_time') || 'Requires Time Column'}
              </label>
              <p style={{ fontSize: '0.78rem', color: 'var(--text-muted)', margin: 0 }}>
                {t('time_grains.label_requires_time_hint') ||
                  'If checked, this grain only matches datetime/timestamp types.'}
              </p>
            </div>
          </div>
          <div className="form-group" style={{ marginTop: '1.25rem' }}>
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
        </div>
        <div className="modal-actions">
          <button type="button" className="btn btn-ghost" onClick={onCancel}>
            {t('common.cancel') || 'Cancel'}
          </button>
          <button type="button" className="btn btn-primary" onClick={onSave} disabled={loading}>
            {loading ? t('common.saving') || 'Saving…' : t('common.save') || 'Save'}
          </button>
        </div>
      </div>
    </div>
  )
}
