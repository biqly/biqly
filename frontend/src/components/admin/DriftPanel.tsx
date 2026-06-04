import '../../styles/drift.css'

import { useCallback, useEffect, useState } from 'react'

import { useApi } from '../../hooks/useApi'
import type { TranslationKey } from '../../i18n'
import { useT } from '../../i18n'

/* ------------------------------------------------------------------ */
/*  Types matching the backend drift_types.go API response             */
/* ------------------------------------------------------------------ */

export interface DriftItem {
  type: string
  field: string
  column_ref: string
  old_value?: string
  new_value?: string
  description: string
}

export interface DriftReport {
  id: string
  model_id: string
  datasource_id: string
  sync_event_id?: string
  severity: string
  drifts: DriftItem[]
  resolved: boolean
  resolved_by?: string
  resolved_at?: string
  detected_at: string
}

/* ------------------------------------------------------------------ */
/*  Severity helpers                                                    */
/* ------------------------------------------------------------------ */

const SEVERITY_ORDER: Record<string, number> = { critical: 0, warning: 1, info: 2 }

function highestSeverity(reports: DriftReport[]): string {
  let best = 'info'
  for (const r of reports) {
    if ((SEVERITY_ORDER[r.severity] ?? 3) < (SEVERITY_ORDER[best] ?? 3)) {
      best = r.severity
    }
  }
  return best
}

function driftTypeLabel(type: string, t: (key: TranslationKey) => string): string {
  const map: Record<string, TranslationKey> = {
    column_dropped: 'modeling.drift_type_column_dropped',
    column_added: 'modeling.drift_type_column_added',
    type_changed: 'modeling.drift_type_type_changed',
    table_dropped: 'modeling.drift_type_table_dropped',
    schema_dropped: 'modeling.drift_type_schema_dropped',
    join_broken: 'modeling.drift_type_join_broken',
    metric_broken: 'modeling.drift_type_metric_broken',
  }
  const key = map[type]
  return key ? t(key) : type
}

function severityLabel(sev: string, t: (key: TranslationKey) => string): string {
  const map: Record<string, TranslationKey> = {
    critical: 'modeling.drift_severity_critical',
    warning: 'modeling.drift_severity_warning',
    info: 'modeling.drift_severity_info',
  }
  const key = map[sev]
  return key ? t(key) : sev
}

/* ------------------------------------------------------------------ */
/*  DriftBadge — small inline indicator for model list                 */
/* ------------------------------------------------------------------ */

export function DriftBadge({ severity, count }: { severity: string; count: number }) {
  const t = useT()
  if (count === 0) {
    return null
  }
  return (
    <span
      className={`drift-badge drift-badge--${severity}`}
      title={t('modeling.drift_badge_title', { count })}
      aria-label={t('modeling.drift_badge_title', { count })}
    >
      <span className="drift-badge__dot" aria-hidden="true" />
      {count}
    </span>
  )
}

/* ------------------------------------------------------------------ */
/*  DriftBanner — alert bar for critical/warning drifts                */
/* ------------------------------------------------------------------ */

interface DriftBannerProps {
  reports: DriftReport[]
  onExpand: () => void
}

export function DriftBanner({ reports, onExpand }: DriftBannerProps) {
  const t = useT()
  if (reports.length === 0) {
    return null
  }

  const severity = highestSeverity(reports)
  const totalDrifts = reports.reduce((n, r) => n + r.drifts.length, 0)

  if (severity === 'info') {
    return null
  }

  return (
    <div className={`drift-banner drift-banner--${severity}`} role="alert">
      <span className="drift-banner__icon" aria-hidden="true">
        {severity === 'critical' ? '🔴' : '🟡'}
      </span>
      <span className="drift-banner__text">
        {t('modeling.drift_banner_text', { count: totalDrifts })}
      </span>
      <button
        type="button"
        className="btn btn-secondary drift-banner__action"
        onClick={onExpand}
      >
        {t('modeling.drift_view_details')}
      </button>
    </div>
  )
}

/* ------------------------------------------------------------------ */
/*  DriftPanel — full detail panel with resolve actions                 */
/* ------------------------------------------------------------------ */

interface DriftPanelProps {
  modelId: string
  /** If true, the panel is rendered in expanded state by default. */
  defaultOpen?: boolean
}

export function DriftPanel({ modelId, defaultOpen = false }: DriftPanelProps) {
  const t = useT()
  const { get, postData } = useApi()
  const [reports, setReports] = useState<DriftReport[]>([])
  const [open, setOpen] = useState(defaultOpen)
  const [resolving, setResolving] = useState<Set<string>>(new Set())

  const load = useCallback(() => {
    if (!modelId) {
      return
    }
    void get<DriftReport[]>(`/api/semantic/models/${modelId}/drift`).then((data) => {
      setReports(data ?? [])
    })
  }, [modelId, get])

  useEffect(() => {
    load()
  }, [load])

  const resolve = useCallback(
    async (reportId: string) => {
      setResolving((prev) => new Set(prev).add(reportId))
      await postData(`/api/drift/${reportId}/resolve`, {})
      setResolving((prev) => {
        const next = new Set(prev)
        next.delete(reportId)
        return next
      })
      load()
    },
    [postData, load],
  )

  if (reports.length === 0) {
    return null
  }

  const severity = highestSeverity(reports)
  const totalDrifts = reports.reduce((n, r) => n + r.drifts.length, 0)

  return (
    <div className="drift-panel" id="drift-panel">
      <button
        type="button"
        className="drift-panel__header"
        onClick={() => setOpen((v) => !v)}
        aria-expanded={open}
        aria-controls="drift-panel-body"
      >
        <span className="drift-panel__title">
          <DriftBadge severity={severity} count={totalDrifts} />
          {t('modeling.drift_panel_title')}
        </span>
        <span className={`drift-panel__chevron ${open ? 'drift-panel__chevron--open' : ''}`}>
          ▶
        </span>
      </button>

      {open && (
        <div className="drift-panel__body" id="drift-panel-body">
          {reports.map((report) => (
            <div key={report.id}>
              <div className="drift-report__heading">
                <span>{severityLabel(report.severity, t)}</span>
                <time dateTime={report.detected_at}>
                  {new Date(report.detected_at).toLocaleString()}
                </time>
              </div>
              <table className="drift-table" aria-label={t('modeling.drift_table_aria')}>
                <thead>
                  <tr>
                    <th>{t('modeling.drift_col_type')}</th>
                    <th>{t('modeling.drift_col_field')}</th>
                    <th>{t('modeling.drift_col_ref')}</th>
                    <th>{t('modeling.drift_col_description')}</th>
                    <th>{t('modeling.drift_col_action')}</th>
                  </tr>
                </thead>
                <tbody>
                  {report.drifts.map((item, idx) => (
                    <tr key={`${report.id}-${idx}`}>
                      <td>
                        <span className="drift-table__severity">
                          <span
                            className={`drift-table__severity-dot drift-table__severity-dot--${report.severity}`}
                            aria-hidden="true"
                          />
                          {driftTypeLabel(item.type, t)}
                        </span>
                      </td>
                      <td className="drift-table__field">{item.field || '—'}</td>
                      <td className="drift-table__field">{item.column_ref || '—'}</td>
                      <td className="drift-table__desc">{item.description}</td>
                      <td>
                        {idx === 0 && (
                          <button
                            type="button"
                            className="drift-table__resolve-btn"
                            disabled={resolving.has(report.id)}
                            onClick={() => void resolve(report.id)}
                            title={t('modeling.drift_resolve_title')}
                          >
                            {resolving.has(report.id)
                              ? t('common.saving')
                              : t('modeling.drift_resolve')}
                          </button>
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
