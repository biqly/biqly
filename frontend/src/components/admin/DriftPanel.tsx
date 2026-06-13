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

const badgeSeverityClasses: Record<string, string> = {
  critical:
    'bg-[hsl(0_70%_95%)] text-[hsl(0_70%_45%)] dark:bg-[hsl(0_40%_18%)] dark:text-[hsl(0_80%_72%)]',
  warning:
    'bg-[hsl(40_90%_92%)] text-[hsl(30_80%_40%)] dark:bg-[hsl(40_40%_18%)] dark:text-[hsl(40_80%_70%)]',
  info: 'bg-[hsl(210_70%_94%)] text-[hsl(210_60%_45%)] dark:bg-[hsl(210_30%_18%)] dark:text-[hsl(210_70%_72%)]',
}

const dotSeverityClasses: Record<string, string> = {
  critical: 'bg-[hsl(0_70%_45%)]',
  warning: 'bg-[hsl(30_80%_40%)]',
  info: 'bg-[hsl(210_60%_45%)]',
}

const bannerSeverityClasses: Record<string, string> = {
  critical:
    'bg-[hsl(0_70%_95%)] border border-[hsl(0_50%_85%)] text-[hsl(0_70%_45%)] dark:bg-[hsl(0_40%_14%)] dark:border-[hsl(0_40%_25%)] dark:text-[hsl(0_80%_72%)]',
  warning:
    'bg-[hsl(40_90%_92%)] border border-[hsl(40_50%_80%)] text-[hsl(30_80%_40%)] dark:bg-[hsl(40_40%_14%)] dark:border-[hsl(40_40%_25%)] dark:text-[hsl(40_80%_70%)]',
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
      className={`inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[0.75rem] font-semibold leading-[1.4] whitespace-nowrap cursor-default ${badgeSeverityClasses[severity] ?? ''}`}
      title={t('modeling.drift_badge_title', { count })}
      aria-label={t('modeling.drift_badge_title', { count })}
    >
      <span
        className={`w-1.5 h-1.5 rounded-full shrink-0 ${dotSeverityClasses[severity] ?? ''}`}
        aria-hidden="true"
      />
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
    <div
      className={`flex items-center gap-2.5 p-[8px_14px] rounded-lg text-[0.85rem] leading-normal animate-drift-banner-enter ${bannerSeverityClasses[severity] ?? ''}`}
      role="alert"
    >
      <span className="text-[1rem] shrink-0" aria-hidden="true">
        {severity === 'critical' ? '🔴' : '🟡'}
      </span>
      <span className="flex-1">{t('modeling.drift_banner_text', { count: totalDrifts })}</span>
      <button type="button" className="btn btn-secondary shrink-0" onClick={onExpand}>
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
    <div
      className="border border-[hsl(220_13%_86%)] dark:border-[hsl(220_10%_24%)] rounded-xl overflow-hidden bg-white dark:bg-[hsl(220_10%_12%)] mt-3 animate-drift-panel-enter"
      id="drift-panel"
    >
      <button
        type="button"
        className="flex items-center justify-between gap-2.5 p-[10px_14px] bg-[hsl(220_14%_96%)] dark:bg-[hsl(220_10%_16%)] hover:bg-[hsl(220_14%_93%)] dark:hover:bg-[hsl(220_10%_20%)] cursor-pointer select-none border-0 w-full text-left font-inherit color-inherit focus-visible:outline-2 focus-visible:outline-[hsl(220_70%_50%)] focus-visible:-outline-offset-2"
        onClick={() => setOpen((v) => !v)}
        aria-expanded={open}
        aria-controls="drift-panel-body"
      >
        <span className="flex items-center gap-2 font-semibold text-[0.875rem]">
          <DriftBadge severity={severity} count={totalDrifts} />
          {t('modeling.drift_panel_title')}
        </span>
        <span
          className={`text-[0.75rem] transition-transform duration-200 ease-out ${open ? 'rotate-90' : ''}`}
        >
          ▶
        </span>
      </button>

      {open && (
        <div className="px-3.5 pb-3.5 pt-0" id="drift-panel-body">
          {reports.map((report) => (
            <div key={report.id}>
              <div className="flex items-center gap-2 text-[0.8rem] text-foreground-faint mt-2.5 mx-0 mb-[2px] [&_time]:italic">
                <span>{severityLabel(report.severity, t)}</span>
                <time dateTime={report.detected_at}>
                  {new Date(report.detected_at).toLocaleString()}
                </time>
              </div>
              <table
                className="w-full border-collapse text-[0.825rem] mt-2.5"
                aria-label={t('modeling.drift_table_aria')}
              >
                <thead>
                  <tr>
                    <th className="text-left p-[6px_10px] font-semibold text-[0.75rem] uppercase tracking-wider text-foreground-faint border-b border-[hsl(220_13%_86%)] dark:border-[hsl(220_10%_24%)]">
                      {t('modeling.drift_col_type')}
                    </th>
                    <th className="text-left p-[6px_10px] font-semibold text-[0.75rem] uppercase tracking-wider text-foreground-faint border-b border-[hsl(220_13%_86%)] dark:border-[hsl(220_10%_24%)]">
                      {t('modeling.drift_col_field')}
                    </th>
                    <th className="text-left p-[6px_10px] font-semibold text-[0.75rem] uppercase tracking-wider text-foreground-faint border-b border-[hsl(220_13%_86%)] dark:border-[hsl(220_10%_24%)]">
                      {t('modeling.drift_col_ref')}
                    </th>
                    <th className="text-left p-[6px_10px] font-semibold text-[0.75rem] uppercase tracking-wider text-foreground-faint border-b border-[hsl(220_13%_86%)] dark:border-[hsl(220_10%_24%)]">
                      {t('modeling.drift_col_description')}
                    </th>
                    <th className="text-left p-[6px_10px] font-semibold text-[0.75rem] uppercase tracking-wider text-foreground-faint border-b border-[hsl(220_13%_86%)] dark:border-[hsl(220_10%_24%)]">
                      {t('modeling.drift_col_action')}
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {report.drifts.map((item, idx) => (
                    <tr key={`${report.id}-${idx}`} className="group">
                      <td className="p-[8px_10px] border-b border-[hsl(220_13%_92%)] dark:border-[hsl(220_10%_20%)] align-top group-last:border-b-0">
                        <span className="inline-flex items-center gap-1">
                          <span
                            className={`w-2 h-2 rounded-full shrink-0 ${dotSeverityClasses[report.severity] ?? ''}`}
                            aria-hidden="true"
                          />
                          {driftTypeLabel(item.type, t)}
                        </span>
                      </td>
                      <td className="p-[8px_10px] border-b border-[hsl(220_13%_92%)] dark:border-[hsl(220_10%_20%)] align-top group-last:border-b-0 font-mono text-[0.8rem]">
                        {item.field || '—'}
                      </td>
                      <td className="p-[8px_10px] border-b border-[hsl(220_13%_92%)] dark:border-[hsl(220_10%_20%)] align-top group-last:border-b-0 font-mono text-[0.8rem]">
                        {item.column_ref || '—'}
                      </td>
                      <td className="p-[8px_10px] border-b border-[hsl(220_13%_92%)] dark:border-[hsl(220_10%_20%)] align-top group-last:border-b-0 text-foreground-muted max-w-[320px]">
                        {item.description}
                      </td>
                      <td className="p-[8px_10px] border-b border-[hsl(220_13%_92%)] dark:border-[hsl(220_10%_20%)] align-top group-last:border-b-0">
                        {idx === 0 && (
                          <button
                            type="button"
                            className="p-[4px_10px] rounded-md border border-[hsl(220_13%_86%)] bg-white text-[hsl(220_10%_20%)] dark:border-[hsl(220_10%_24%)] dark:bg-[hsl(220_10%_16%)] dark:text-[hsl(0_0%_85%)] text-[0.75rem] cursor-pointer transition-[background,border-color,color] duration-150 hover:bg-[hsl(140_50%_94%)] hover:border-[hsl(140_50%_40%)] hover:text-[hsl(140_50%_40%)] dark:hover:bg-[hsl(140_30%_16%)] dark:hover:border-[hsl(140_40%_35%)] dark:hover:text-[hsl(140_60%_70%)] disabled:opacity-50 disabled:cursor-not-allowed"
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
