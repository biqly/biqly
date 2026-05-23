import { useEffect, useState } from 'react'
import { useApi } from '../hooks/useApi'
import { useT } from '../i18n'
import { ErrorAlert } from './ui/ErrorAlert'

interface TimeGrain {
  grain: string
  suffix: string
  requires_time: boolean
  synonyms: string[]
  created_at?: string
  updated_at?: string
}

interface TimeGrainsProps {
  navigate?: (path: string) => void
}

export default function TimeGrains({ navigate }: TimeGrainsProps) {
  const t = useT()
  const { get, putData, loading, error } = useApi()
  const [grains, setGrains] = useState<TimeGrain[]>([])
  const [editingGrain, setEditingGrain] = useState<TimeGrain | null>(null)
  const [formSuffix, setFormSuffix] = useState('')
  const [formRequiresTime, setFormRequiresTime] = useState(false)
  const [formSynonyms, setFormSynonyms] = useState('')
  const [formError, setFormError] = useState<string | null>(null)
  const [successMessage, setSuccessMessage] = useState<string | null>(null)

  const fetchGrains = () => {
    get<TimeGrain[]>('/api/ai/settings/time-grains').then((data) => {
      if (data) {
        setGrains(data)
      }
    })
  }

  useEffect(() => {
    fetchGrains()
  }, [])

  const startEdit = (tg: TimeGrain) => {
    setEditingGrain(tg)
    setFormSuffix(tg.suffix)
    setFormRequiresTime(tg.requires_time)
    setFormSynonyms(tg.synonyms.join(', '))
    setFormError(null)
    setSuccessMessage(null)
  }

  const cancelEdit = () => {
    setEditingGrain(null)
    setFormError(null)
  }

  const handleSave = async () => {
    if (!editingGrain) return
    setFormError(null)
    setSuccessMessage(null)

    if (!formSuffix.trim()) {
      setFormError(t('time_grains.err_suffix_required') || 'Suffix is required')
      return
    }

    const cleanedSynonyms = formSynonyms
      .split(',')
      .map((s) => s.trim())
      .filter(Boolean)

    const payload = {
      suffix: formSuffix.trim(),
      requires_time: formRequiresTime,
      synonyms: cleanedSynonyms,
    }

    const res = await putData<{ status: string }>(
      `/api/ai/settings/time-grains/${editingGrain.grain}`,
      payload
    )

    if (res && res.status === 'ok') {
      setSuccessMessage(t('time_grains.success_save') || 'Time grain updated successfully.')
      setEditingGrain(null)
      fetchGrains()
    } else if (error) {
      setFormError(error)
    } else {
      setFormError(t('time_grains.error_save') || 'Failed to save time grain.')
    }
  }

  return (
    <div className="page-stack">
      <div className="card">
        <div className="card-header-row">
          <h2>{t('time_grains.title') || 'Time Grains'}</h2>
          <button
            type="button"
            className="btn-back"
            onClick={() => navigate?.('/settings')}
          >
            ← {t('time_grains.back_to_settings') || 'Back to Settings'}
          </button>
        </div>
        <p className="card-subtitle">
          {t('time_grains.subtitle') ||
            'Customize how the AI recognizes and handles time/date query grains (e.g., daily, monthly, yearly).'}
        </p>

        {error && <ErrorAlert error={error} />}

        {grains.length > 0 ? (
          <table className="results-table">
            <thead>
              <tr>
                <th>{t('time_grains.col_grain') || 'Time Grain'}</th>
                <th>{t('time_grains.col_suffix') || 'Suffix'}</th>
                <th>{t('time_grains.col_requires_time') || 'Requires Time Column'}</th>
                <th>{t('time_grains.col_synonyms') || 'Synonyms'}</th>
                <th className="actions">{t('common.actions') || 'Actions'}</th>
              </tr>
            </thead>
            <tbody>
              {grains.map((tg) => (
                <tr key={tg.grain}>
                  <td style={{ fontWeight: 600, color: 'var(--text-primary)' }}>{tg.grain}</td>
                  <td>
                    <code style={{ fontSize: '0.78rem', color: 'var(--accent)' }}>{tg.suffix}</code>
                  </td>
                  <td>
                    {tg.requires_time ? (
                      <span
                        style={{
                          display: 'inline-flex',
                          alignItems: 'center',
                          padding: '0.15rem 0.5rem',
                          background: 'rgba(16,185,129,0.1)',
                          color: 'var(--success)',
                          borderRadius: '999px',
                          fontSize: '0.72rem',
                          fontWeight: 500,
                        }}
                      >
                        {t('common.yes') || 'Yes'}
                      </span>
                    ) : (
                      <span
                        style={{
                          display: 'inline-flex',
                          alignItems: 'center',
                          padding: '0.15rem 0.5rem',
                          background: 'rgba(255,255,255,0.04)',
                          color: 'var(--text-muted)',
                          borderRadius: '999px',
                          fontSize: '0.72rem',
                        }}
                      >
                        {t('common.no') || 'No'}
                      </span>
                    )}
                  </td>
                  <td>
                    <div style={{ display: 'flex', flexWrap: 'wrap', gap: '0.3rem' }}>
                      {tg.synonyms.map((syn) => (
                        <span
                          key={syn}
                          style={{
                            display: 'inline-block',
                            padding: '0.15rem 0.5rem',
                            background: 'rgba(99,102,241,0.08)',
                            borderRadius: '0.3rem',
                            fontSize: '0.72rem',
                            color: 'var(--accent)',
                            border: '1px solid rgba(99,102,241,0.15)',
                          }}
                        >
                          {syn}
                        </span>
                      ))}
                    </div>
                  </td>
                  <td className="actions">
                    <button
                      type="button"
                      className="btn btn-sm btn-ghost"
                      onClick={() => startEdit(tg)}
                    >
                      {t('common.edit') || 'Edit'}
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        ) : (
          <div style={{ padding: '2rem', textAlign: 'center', color: 'var(--text-muted)' }}>
            {t('common.loading') || 'Loading…'}
          </div>
        )}
      </div>

      {successMessage && (
        <div
          style={{
            padding: '0.75rem 1rem',
            background: 'rgba(16,185,129,0.1)',
            border: '1px solid rgba(16,185,129,0.2)',
            borderRadius: '0.5rem',
            color: 'var(--success)',
            fontSize: '0.875rem',
          }}
        >
          {successMessage}
        </div>
      )}

      {/* Edit Modal */}
      {editingGrain && (
        <div className="modal-backdrop" onClick={cancelEdit}>
          <div className="modal-card" onClick={(e) => e.stopPropagation()}>
            <div className="modal-header">
              <h2>
                {t('time_grains.edit_title') || 'Edit Time Grain'}: {editingGrain.grain}
              </h2>
              <button className="modal-close" onClick={cancelEdit}>
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
                style={{ flexDirection: 'row', alignItems: 'flex-start', gap: '0.75rem', marginTop: '1rem' }}
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

              <div className="form-group" style={{ marginTop: '1.25rem' }}>
                <label htmlFor="tg-synonyms">
                  {t('time_grains.label_synonyms') || 'Synonyms (comma-separated)'}
                </label>
                <textarea
                  id="tg-synonyms"
                  value={formSynonyms}
                  onChange={(e) => setFormSynonyms(e.target.value)}
                  placeholder={
                    t('time_grains.placeholder_synonyms') || 'e.g., daily, per day, day'
                  }
                  rows={4}
                />
              </div>

              {formError && <ErrorAlert error={formError} />}
            </div>
            <div className="modal-actions">
              <button type="button" className="btn btn-ghost" onClick={cancelEdit}>
                {t('common.cancel') || 'Cancel'}
              </button>
              <button
                type="button"
                className="btn btn-primary"
                onClick={handleSave}
                disabled={loading}
              >
                {loading ? t('common.saving') || 'Saving…' : t('common.save') || 'Save'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
