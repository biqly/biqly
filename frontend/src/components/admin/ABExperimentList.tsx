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
    <div className={`bg-card border border-border rounded-lg p-6 shadow-card-sm`}>
      <div className={`flex justify-between items-center gap-4 border-b border-border pb-4`}>
        <div className="flex flex-col gap-1">
          <h1 className="text-2xl font-semibold m-0">{t('admin.ab_experiments.title')}</h1>
          <p className="text-sm text-foreground-muted m-0 max-w-[720px]">
            {t('admin.ab_experiments.description')}
          </p>
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
        <span className="text-sm font-medium text-foreground-muted">
          {t('admin.filters.apply')}:
        </span>
        <select
          className={`px-3 py-2 border border-border rounded-md text-sm bg-[var(--input-bg)] text-inherit w-full focus:outline-none focus:border-accent focus:ring-2 focus:ring-[var(--control-focus-ring)]`}
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
        <table className="w-full border-collapse text-left">
          <thead>
            <tr>
              <th
                className={`px-4 py-3 border-b border-border text-sm font-medium text-foreground-muted bg-card-raised`}
              >
                {t('admin.ab_experiments.col_name')}
              </th>
              <th
                className={`px-4 py-3 border-b border-border text-sm font-medium text-foreground-muted bg-card-raised`}
              >
                {t('admin.ab_experiments.col_template')}
              </th>
              <th
                className={`px-4 py-3 border-b border-border text-sm font-medium text-foreground-muted bg-card-raised`}
              >
                {t('admin.ab_experiments.col_locale')}
              </th>
              <th
                className={`px-4 py-3 border-b border-border text-sm font-medium text-foreground-muted bg-card-raised`}
              >
                {t('admin.ab_experiments.col_status')}
              </th>
              <th
                className={`px-4 py-3 border-b border-border text-sm font-medium text-foreground-muted bg-card-raised`}
              >
                {t('admin.ab_experiments.col_actions')}
              </th>
            </tr>
          </thead>
          <tbody>
            {experiments.map((exp) => (
              <tr key={exp.id} className="hover:bg-[var(--control-hover-bg)]">
                <td className={`px-4 py-3 border-b border-border text-sm font-semibold`}>
                  {exp.name}
                </td>
                <td className={`px-4 py-3 border-b border-border text-sm`}>{exp.template_name}</td>
                <td className={`px-4 py-3 border-b border-border text-sm`}>
                  {exp.locale.toUpperCase()}
                </td>
                <td className={`px-4 py-3 border-b border-border text-sm`}>
                  <span
                    className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium capitalize ${
                      exp.status === 'draft'
                        ? 'bg-[#f3f4f6] text-[#374151] dark:bg-zinc-800 dark:text-zinc-300'
                        : exp.status === 'running'
                          ? 'bg-[#ecfdf5] text-[#065f46] dark:bg-emerald-950/30 dark:text-emerald-400'
                          : exp.status === 'paused'
                            ? 'bg-[#fffbeb] text-[#92400e] dark:bg-amber-950/30 dark:text-amber-400'
                            : exp.status === 'completed'
                              ? 'bg-[#eff6ff] text-[#1e40af] dark:bg-blue-950/30 dark:text-blue-400'
                              : ''
                    }`}
                  >
                    <span className="w-1.5 h-1.5 rounded-full bg-current" />
                    {exp.status}
                  </span>
                </td>
                <td className={`px-4 py-3 border-b border-border text-sm`}>
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
