import { useCallback, useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'

import { useApi } from '../hooks/useApi'
import { useT } from '../i18n'
import { TimeGrainsEditModal } from './TimeGrainsEditModal'
import { TimeGrainsTable } from './TimeGrainsTable'
import { ErrorAlert } from './ui/ErrorAlert'
import { LoadingScreen } from './ui/LoadingScreen'

interface TimeGrain {
  grain: string
  suffix: string
  requires_time: boolean
  synonyms: string[]
  created_at?: string
  updated_at?: string
}

export default function TimeGrains() {
  const navigate = useNavigate()
  const t = useT()
  const { get, putData, loading, error } = useApi()
  const [grains, setGrains] = useState<TimeGrain[]>([])
  const [initLoading, setInitLoading] = useState(true)
  const [editingGrain, setEditingGrain] = useState<TimeGrain | null>(null)
  const [formSuffix, setFormSuffix] = useState('')
  const [formRequiresTime, setFormRequiresTime] = useState(false)
  const [formSynonyms, setFormSynonyms] = useState('')
  const [formError, setFormError] = useState<string | null>(null)
  const [successMessage, setSuccessMessage] = useState<string | null>(null)

  const fetchGrains = useCallback(() => {
    get<TimeGrain[]>('/api/ai/settings/time-grains')
      .then((data) => {
        if (data) {
          setGrains(data)
        }
        setInitLoading(false)
      })
      .catch(() => {
        setInitLoading(false)
      })
  }, [get])

  useEffect(() => {
    fetchGrains()
  }, [fetchGrains])

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
    if (!editingGrain) {
      return
    }
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
      payload,
    )

    if (res?.status === 'ok') {
      setSuccessMessage(t('time_grains.success_save') || 'Time grain updated successfully.')
      setEditingGrain(null)
      fetchGrains()
    } else if (error) {
      setFormError(error)
    } else {
      setFormError(t('time_grains.error_save') || 'Failed to save time grain.')
    }
  }

  if (initLoading && grains.length === 0) {
    return <LoadingScreen minHeight="300px" />
  }

  return (
    <div className="page-stack">
      <div className="card">
        <div className="card-intro">
          <div className="card-header-row">
            <h2>{t('time_grains.title') || 'Time Grains'}</h2>
            <button
              type="button"
              className="btn-back"
              onClick={() => {
                void navigate('/settings')
              }}
            >
              ← {t('time_grains.back_to_settings') || 'Back to Settings'}
            </button>
          </div>
          <p
            className="card-lead card-lead--single-line"
            title={
              t('time_grains.subtitle') ||
              'Customize how the AI recognizes and handles time/date query grains (e.g., daily, monthly, yearly).'
            }
          >
            {t('time_grains.subtitle') ||
              'Customize how the AI recognizes and handles time/date query grains (e.g., daily, monthly, yearly).'}
          </p>
        </div>

        {error && <ErrorAlert error={error} />}

        <TimeGrainsTable grains={grains} loading={loading} onEdit={startEdit} t={t} />
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

      <TimeGrainsEditModal
        editingGrain={editingGrain}
        formSuffix={formSuffix}
        setFormSuffix={setFormSuffix}
        formRequiresTime={formRequiresTime}
        setFormRequiresTime={setFormRequiresTime}
        formSynonyms={formSynonyms}
        setFormSynonyms={setFormSynonyms}
        formError={formError}
        loading={loading}
        onCancel={cancelEdit}
        onSave={() => {
          void handleSave()
        }}
        t={t}
      />
    </div>
  )
}
