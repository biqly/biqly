import { useEffect, useMemo, useState } from 'react'

import { useAdminApi } from '../../hooks/useApi'
import { useT } from '../../i18n'
import { formHintClass } from '../../lib/formClasses'
import { legacyLayoutClass } from '../../lib/layoutClasses'
import { Button } from '../ui/Button'
import { EmptyState } from '../ui/EmptyState'
import { LoadingOverlay } from '../ui/LoadingOverlay'
import { LoadingScreen } from '../ui/LoadingScreen'
import { Select } from '../ui/Select'
import type { Experiment } from './ABExperimentForm'
import { abExperimentStatusBadgeClass, abExperimentStatusLabel } from './abExperimentStatusBadge'
import {
  adminErrBoxClass,
  adminFormLabelClass,
  adminLabelTextClass,
  adminPanelHeaderClass,
  adminTableClass,
  adminTableContainerClass,
  adminTdClass,
  adminThClass,
  adminTheadRowClass,
  adminTrClass,
} from './adminClasses'

interface ABExperimentListProps {
  onSelect: (id: string) => void
  onCreate: () => void
}

function AbExperimentEmptyIcon() {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      width="40"
      height="40"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <path d="M9 3h6" />
      <path d="M10 3v4.5a4.5 4.5 0 0 0 9 0V3" />
      <path d="M7 14h10" />
      <path d="M8 14v7" />
      <path d="M16 14v7" />
      <path d="M6 21h12" />
    </svg>
  )
}

export function ABExperimentList({ onSelect, onCreate }: ABExperimentListProps) {
  const t = useT()
  const { get, loading, error } = useAdminApi()
  const [experiments, setExperiments] = useState<Experiment[]>([])
  const [statusFilter, setStatusFilter] = useState('')

  const statusOptions = useMemo(
    () => [
      { value: '', label: t('admin.filters.all') },
      { value: 'draft', label: t('admin.ab_experiments.status_draft') },
      { value: 'running', label: t('admin.ab_experiments.status_running') },
      { value: 'paused', label: t('admin.ab_experiments.status_paused') },
      { value: 'completed', label: t('admin.ab_experiments.status_completed') },
    ],
    [t],
  )

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

  const showInitialLoading = loading && experiments.length === 0
  const showEmpty = !loading && experiments.length === 0

  return (
    <div className={legacyLayoutClass('page-stack')}>
      <div className={adminPanelHeaderClass}>
        <div>
          <h2 style={{ margin: 0 }}>{t('admin.ab_experiments.title')}</h2>
          <p className={formHintClass}>{t('admin.ab_experiments.description')}</p>
        </div>
        {experiments.length > 0 && (
          <Button variant="primary" autoWidth onClick={onCreate}>
            {t('admin.ab_experiments.create_btn')}
          </Button>
        )}
      </div>

      {error ? (
        <div className={adminErrBoxClass} role="alert">
          {error}
        </div>
      ) : null}

      <label className={adminFormLabelClass} style={{ gap: 4, maxWidth: 240 }}>
        <span className={adminLabelTextClass}>{t('admin.ab_experiments.col_status')}</span>
        <Select
          value={statusFilter}
          onChange={setStatusFilter}
          options={statusOptions}
          ariaLabel={t('admin.ab_experiments.col_status')}
          size="sm"
        />
      </label>

      {showInitialLoading ? (
        <LoadingScreen minHeight="200px" />
      ) : showEmpty ? (
        <EmptyState
          title={t('admin.ab_experiments.empty_title')}
          description={t('admin.ab_experiments.empty')}
          icon={<AbExperimentEmptyIcon />}
          action={{ label: t('admin.ab_experiments.create_btn'), onClick: onCreate }}
        />
      ) : (
        <div className={adminTableContainerClass} style={{ position: 'relative' }}>
          <LoadingOverlay loading={loading} />
          <table className={adminTableClass}>
            <thead>
              <tr className={adminTheadRowClass}>
                <th className={adminThClass}>{t('admin.ab_experiments.col_name')}</th>
                <th className={adminThClass}>{t('admin.ab_experiments.col_template')}</th>
                <th className={adminThClass}>{t('admin.ab_experiments.col_locale')}</th>
                <th className={adminThClass}>{t('admin.ab_experiments.col_status')}</th>
                <th className={adminThClass}>{t('admin.ab_experiments.col_actions')}</th>
              </tr>
            </thead>
            <tbody>
              {experiments.map((exp) => (
                <tr key={exp.id} className={adminTrClass}>
                  <td className={`${adminTdClass} font-semibold`}>{exp.name}</td>
                  <td className={adminTdClass}>{exp.template_name}</td>
                  <td className={adminTdClass}>{exp.locale.toUpperCase()}</td>
                  <td className={adminTdClass}>
                    <span className={abExperimentStatusBadgeClass(exp.status)}>
                      <span className="size-1.5 rounded-full bg-current" aria-hidden="true" />
                      {abExperimentStatusLabel(exp.status, t)}
                    </span>
                  </td>
                  <td className={adminTdClass}>
                    <Button
                      variant="ghost"
                      size="sm"
                      autoWidth
                      onClick={() => exp.id && onSelect(exp.id)}
                    >
                      {t('admin.ab_experiments.view_btn')}
                    </Button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
