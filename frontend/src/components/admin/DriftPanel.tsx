import { useCallback, useEffect, useState } from 'react'

import { useApi } from '../../hooks/useApi'
import type { TranslationKey } from '../../i18n'
import { useT } from '../../i18n'
import { legacyButtonClass } from '../../lib/buttonClasses'

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
      className={`inline-flex cursor-default items-center gap-1 rounded-full px-2 py-0.5 text-[0.75rem] leading-[1.4] font-semibold whitespace-nowrap ${badgeSeverityClasses[severity] ?? ''}`}
      title={t('modeling.drift_badge_title', { count })}
      aria-label={t('modeling.drift_badge_title', { count })}
    >
      <span
        className={`h-1.5 w-1.5 shrink-0 rounded-full ${dotSeverityClasses[severity] ?? ''}`}
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
      className={`animate-drift-banner-enter flex items-center gap-2.5 rounded-lg p-[8px_14px] text-[0.85rem] leading-normal ${bannerSeverityClasses[severity] ?? ''}`}
      role="alert"
    >
      <span className="shrink-0 text-[1rem]" aria-hidden="true">
        {severity === 'critical' ? '🔴' : '🟡'}
      </span>
      <span className="flex-1">{t('modeling.drift_banner_text', { count: totalDrifts })}</span>
      <button
        type="button"
        className={legacyButtonClass('btn btn-secondary shrink-0')}
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
    <div
      className="animate-drift-panel-enter mt-3 overflow-hidden rounded-xl border border-[hsl(220_13%_86%)] bg-white dark:border-[hsl(220_10%_24%)] dark:bg-[hsl(220_10%_12%)]"
      id="drift-panel"
    >
      <button
        type="button"
        className="font-inherit color-inherit flex w-full cursor-pointer items-center justify-between gap-2.5 border-0 bg-[hsl(220_14%_96%)] p-[10px_14px] text-left select-none hover:bg-[hsl(220_14%_93%)] focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-[hsl(220_70%_50%)] dark:bg-[hsl(220_10%_16%)] dark:hover:bg-[hsl(220_10%_20%)]"
        onClick={() => setOpen((v) => !v)}
        aria-expanded={open}
        aria-controls="drift-panel-body"
      >
        <span className="flex items-center gap-2 text-[0.875rem] font-semibold">
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
        <div className="px-3.5 pt-0 pb-3.5" id="drift-panel-body">
          {reports.map((report) => (
            <div key={report.id}>
              <div className="text-foreground-faint mx-0 mt-2.5 mb-0.5 flex items-center gap-2 text-[0.8rem] [&_time]:italic">
                <span>{severityLabel(report.severity, t)}</span>
                <time dateTime={report.detected_at}>
                  {new Date(report.detected_at).toLocaleString()}
                </time>
              </div>
              <table
                className="mt-2.5 w-full border-collapse text-[0.825rem]"
                aria-label={t('modeling.drift_table_aria')}
              >
                <thead>
                  <tr>
                    <th className="text-foreground-faint border-b border-[hsl(220_13%_86%)] p-[6px_10px] text-left text-[0.75rem] font-semibold tracking-wider uppercase dark:border-[hsl(220_10%_24%)]">
                      {t('modeling.drift_col_type')}
                    </th>
                    <th className="text-foreground-faint border-b border-[hsl(220_13%_86%)] p-[6px_10px] text-left text-[0.75rem] font-semibold tracking-wider uppercase dark:border-[hsl(220_10%_24%)]">
                      {t('modeling.drift_col_field')}
                    </th>
                    <th className="text-foreground-faint border-b border-[hsl(220_13%_86%)] p-[6px_10px] text-left text-[0.75rem] font-semibold tracking-wider uppercase dark:border-[hsl(220_10%_24%)]">
                      {t('modeling.drift_col_ref')}
                    </th>
                    <th className="text-foreground-faint border-b border-[hsl(220_13%_86%)] p-[6px_10px] text-left text-[0.75rem] font-semibold tracking-wider uppercase dark:border-[hsl(220_10%_24%)]">
                      {t('modeling.drift_col_description')}
                    </th>
                    <th className="text-foreground-faint border-b border-[hsl(220_13%_86%)] p-[6px_10px] text-left text-[0.75rem] font-semibold tracking-wider uppercase dark:border-[hsl(220_10%_24%)]">
                      {t('modeling.drift_col_action')}
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {report.drifts.map((item, idx) => (
                    <tr key={`${report.id}-${idx}`} className="group">
                      <td className="border-b border-[hsl(220_13%_92%)] p-[8px_10px] align-top group-last:border-b-0 dark:border-[hsl(220_10%_20%)]">
                        <span className="inline-flex items-center gap-1">
                          <span
                            className={`h-2 w-2 shrink-0 rounded-full ${dotSeverityClasses[report.severity] ?? ''}`}
                            aria-hidden="true"
                          />
                          {driftTypeLabel(item.type, t)}
                        </span>
                      </td>
                      <td className="border-b border-[hsl(220_13%_92%)] p-[8px_10px] align-top font-mono text-[0.8rem] group-last:border-b-0 dark:border-[hsl(220_10%_20%)]">
                        {item.field || '—'}
                      </td>
                      <td className="border-b border-[hsl(220_13%_92%)] p-[8px_10px] align-top font-mono text-[0.8rem] group-last:border-b-0 dark:border-[hsl(220_10%_20%)]">
                        {item.column_ref || '—'}
                      </td>
                      <td className="text-foreground-muted max-w-[320px] border-b border-[hsl(220_13%_92%)] p-[8px_10px] align-top group-last:border-b-0 dark:border-[hsl(220_10%_20%)]">
                        {item.description}
                      </td>
                      <td className="border-b border-[hsl(220_13%_92%)] p-[8px_10px] align-top group-last:border-b-0 dark:border-[hsl(220_10%_20%)]">
                        {idx === 0 && (
                          <button
                            type="button"
                            className="cursor-pointer rounded-md border border-[hsl(220_13%_86%)] bg-white p-[4px_10px] text-[0.75rem] text-[hsl(220_10%_20%)] transition-[background,border-color,color] duration-150 hover:border-[hsl(140_50%_40%)] hover:bg-[hsl(140_50%_94%)] hover:text-[hsl(140_50%_40%)] disabled:cursor-not-allowed disabled:opacity-50 dark:border-[hsl(220_10%_24%)] dark:bg-[hsl(220_10%_16%)] dark:text-[hsl(0_0%_85%)] dark:hover:border-[hsl(140_40%_35%)] dark:hover:bg-[hsl(140_30%_16%)] dark:hover:text-[hsl(140_60%_70%)]"
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
