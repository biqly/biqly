import { useEffect, useState } from 'react'

import { useAdminApi } from '../../hooks/useApi'
import { useT } from '../../i18n'
import type { Experiment } from './ABExperimentForm'

interface ABExperimentListProps {
  onSelect: (id: string) => void
  onCreate: () => void
}

export function ABExperimentList({ onSelect, onCreate }: ABExperimentListProps) {
  const t = useT()
  const { get, loading, error } = useAdminApi()
  const [experiments, setExperiments] = useState<Experiment[]>([])
  const [statusFilter, setStatusFilter] = useState<string>('')

  useEffect(() => {
    const load = async () => {
      const url = statusFilter
        ? `/api/ai/ab-experiments?status=${statusFilter}`
        : '/api/ai/ab-experiments'
      const data = await get<Experiment[]>(url)
      setExperiments(data ?? [])
    }
    void load()
  }, [get, statusFilter])

  return (
    <div className="ab-experiment-card">
      <div className="ab-experiment__header">
        <div className="ab-experiment__title-group">
          <h1 className="ab-experiment__title">{t('admin.ab_experiments.title')}</h1>
          <p className="ab-experiment__desc">{t('admin.ab_experiments.description')}</p>
        </div>
        <button type="button" className="btn btn-primary" onClick={onCreate}>
          {t('admin.ab_experiments.create_btn')}
        </button>
      </div>

      {error && (
        <div className="alert alert-danger" role="alert">
          {error}
        </div>
      )}

      <div style={{ marginBottom: 16, display: 'flex', gap: 12, alignItems: 'center' }}>
        <span className="ab-form__label">{t('admin.filters.apply')}:</span>
        <select
          className="ab-form__select"
          style={{ width: 'auto' }}
          value={statusFilter}
          onChange={(e) => setStatusFilter(e.target.value)}
        >
          <option value="">{t('admin.filters.all')}</option>
          <option value="draft">{t('admin.ab_experiments.status_draft')}</option>
          <option value="running">{t('admin.ab_experiments.status_running')}</option>
          <option value="paused">{t('admin.ab_experiments.status_paused')}</option>
          <option value="completed">{t('admin.ab_experiments.status_completed')}</option>
        </select>
      </div>

      {loading && experiments.length === 0 ? (
        <div style={{ padding: 24, textAlign: 'center' }}>{t('common.loading')}</div>
      ) : experiments.length === 0 ? (
        <div style={{ padding: 24, textAlign: 'center', color: '#6b7280' }}>
          {t('admin.ab_experiments.empty')}
        </div>
      ) : (
        <table className="ab-experiment-table">
          <thead>
            <tr>
              <th>{t('admin.ab_experiments.col_name')}</th>
              <th>{t('admin.ab_experiments.col_template')}</th>
              <th>{t('admin.ab_experiments.col_locale')}</th>
              <th>{t('admin.ab_experiments.col_status')}</th>
              <th>{t('admin.ab_experiments.col_actions')}</th>
            </tr>
          </thead>
          <tbody>
            {experiments.map((exp) => (
              <tr key={exp.id}>
                <td className="ab-experiment-table__cell-bold">{exp.name}</td>
                <td>{exp.template_name}</td>
                <td>{exp.locale.toUpperCase()}</td>
                <td>
                  <span className={`ab-status-badge ab-status-badge--${exp.status}`}>
                    <span className="ab-status-badge__dot" />
                    {exp.status}
                  </span>
                </td>
                <td>
                  <button
                    type="button"
                    className="btn btn-secondary btn-sm"
                    onClick={() => exp.id && onSelect(exp.id)}
                  >
                    {t('common.edit')} / {t('admin.ai_history.detail')}
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  )
}
