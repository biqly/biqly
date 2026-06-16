import { useCallback, useEffect, useState } from 'react'

import { useAdminApi } from '../../hooks/useApi'
import { useT } from '../../i18n'
import { abRecommendationBannerClass, abRecommendationTitleClass } from '../../lib/feedbackClasses'
import { formHintClass } from '../../lib/formClasses'
import { legacyLayoutClass } from '../../lib/layoutClasses'
import { Button } from '../ui/Button'
import { LoadingScreen } from '../ui/LoadingScreen'
import type { Experiment } from './ABExperimentForm'
import { abExperimentStatusBadgeClass, abExperimentStatusLabel } from './abExperimentStatusBadge'
import {
  adminBtnGhostClass,
  adminCardClass,
  adminErrBoxClass,
  adminInputClass,
  adminPanelHeaderClass,
  adminThClass,
} from './adminClasses'
interface Variant {
  id?: string
  experiment_id: string
  name: string
  template_version: number
  traffic_pct: number
  is_control: boolean
}

interface ExperimentMetrics {
  experiment_id: string
  variant_id: string
  total_queries: number
  success_rate: number
  validator_pass_rate: number
  avg_confidence: number
  user_correction_rate: number
  positive_feedback_rate: number
  execution_success_rate: number
  avg_cost_usd: number
  avg_latency_ms: number
  stddev_cost_usd: number
  stddev_latency_ms: number
  total_tokens: number
}

interface Recommendation {
  winner_variant_id: string
  reason: string
  min_sample_reached: boolean
  sample_size: number
  significance: {
    is_significant: boolean
    p_value: number
    confidence: number
  }
}

interface TimeseriesDay {
  date: string
  metrics: ExperimentMetrics[] | null
}

interface ABExperimentDetailProps {
  experimentId: string
  onBack: () => void
  onEdit: () => void
}

function variantBarColor(v: Variant, index: number): string {
  if (v.is_control) {
    return 'bg-blue-500'
  }
  if (index % 3 === 0) {
    return 'bg-emerald-500'
  }
  if (index % 3 === 1) {
    return 'bg-purple-500'
  }
  return 'bg-amber-500'
}

interface ExperimentDetailHeaderProps {
  exp: Experiment
  isDraft: boolean
  isRunning: boolean
  isPaused: boolean
  totalTraffic: number
  controlCount: number
  onBack: () => void
  onEdit: () => void
  onStatusTransition: (status: string) => void | Promise<void>
}

function ExperimentDetailHeader({
  exp,
  isDraft,
  isRunning,
  isPaused,
  totalTraffic,
  controlCount,
  onBack,
  onEdit,
  onStatusTransition,
}: ExperimentDetailHeaderProps) {
  const t = useT()

  return (
    <>
      <button type="button" className={`${adminBtnGhostClass} mb-1`} onClick={onBack}>
        ← {t('admin.workspaces.back')}
      </button>

      <div className={adminPanelHeaderClass}>
        <div>
          <h2 style={{ margin: 0 }}>{exp.name}</h2>
          <p className={formHintClass}>{exp.description || t('metadata.no_description')}</p>
          <div className="text-foreground-muted mt-2 flex flex-wrap gap-4 text-sm">
            <span>
              <strong className="text-foreground">{t('admin.ab_experiments.col_template')}:</strong>{' '}
              {exp.template_name}
            </span>
            <span>
              <strong className="text-foreground">{t('admin.ab_experiments.col_locale')}:</strong>{' '}
              {exp.locale.toUpperCase()}
            </span>
            <span className="inline-flex items-center gap-2">
              <strong className="text-foreground">{t('admin.ab_experiments.col_status')}:</strong>
              <span className={abExperimentStatusBadgeClass(exp.status)}>
                <span className="size-1.5 rounded-full bg-current" aria-hidden="true" />
                {abExperimentStatusLabel(exp.status, t)}
              </span>
            </span>
          </div>
        </div>

        <div className="flex flex-wrap justify-end gap-2">
          {isDraft ? (
            <>
              <Button variant="secondary" autoWidth onClick={onEdit}>
                {t('common.edit')}
              </Button>
              <Button
                variant="primary"
                autoWidth
                onClick={() => void onStatusTransition('running')}
                disabled={totalTraffic !== 100 || controlCount !== 1}
              >
                {t('admin.ab_experiments.start_btn')}
              </Button>
            </>
          ) : null}

          {isRunning ? (
            <>
              <Button
                variant="secondary"
                autoWidth
                onClick={() => void onStatusTransition('paused')}
              >
                {t('admin.ab_experiments.pause_btn')}
              </Button>
              <Button
                variant="danger"
                autoWidth
                onClick={() => void onStatusTransition('completed')}
              >
                {t('admin.ab_experiments.complete_btn')}
              </Button>
            </>
          ) : null}

          {isPaused ? (
            <>
              <Button
                variant="primary"
                autoWidth
                onClick={() => void onStatusTransition('running')}
              >
                {t('admin.ab_experiments.resume_btn')}
              </Button>
              <Button
                variant="danger"
                autoWidth
                onClick={() => void onStatusTransition('completed')}
              >
                {t('admin.ab_experiments.complete_btn')}
              </Button>
            </>
          ) : null}
        </div>
      </div>
    </>
  )
}

export function ABExperimentDetail({ experimentId, onBack, onEdit }: ABExperimentDetailProps) {
  const t = useT()
  const { get, postData, putData, deleteData } = useAdminApi()

  const [exp, setExp] = useState<Experiment | null>(null)
  const [variants, setVariants] = useState<Variant[]>([])
  const [metrics, setMetrics] = useState<ExperimentMetrics[]>([])
  const [recommendation, setRecommendation] = useState<Recommendation | null>(null)
  const [timeseries, setTimeseries] = useState<TimeseriesDay[]>([])

  // Form states for new variant creation
  const [newVarName, setNewVarName] = useState('')
  const [newVarVersion, setNewVarVersion] = useState(1)
  const [newVarTraffic, setNewVarTraffic] = useState(50)
  const [newVarIsControl, setNewVarIsControl] = useState(false)
  const [varError, setVarError] = useState<string | null>(null)

  const loadData = useCallback(async () => {
    const detail = await get<{ experiment: Experiment; variants: Variant[] }>(
      `/api/ai/ab-experiments/${experimentId}`,
    )
    if (detail) {
      setExp(detail.experiment)
      setVariants(detail.variants)

      // Only fetch analytics if experiment was/is running
      if (detail.experiment.status !== 'draft') {
        const stats = await get<ExperimentMetrics[]>(
          `/api/ai/ab-experiments/${experimentId}/metrics`,
        )
        setMetrics(stats ?? [])

        const rec = await get<Recommendation>(
          `/api/ai/ab-experiments/${experimentId}/recommendation`,
        )
        setRecommendation(rec ?? null)

        const history = await get<TimeseriesDay[]>(
          `/api/ai/ab-experiments/${experimentId}/timeseries`,
        )
        setTimeseries(history ?? [])
      }
    }
  }, [experimentId, get])

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void loadData()
  }, [loadData])

  const handleStatusTransition = async (status: string) => {
    const updated = await putData<Experiment>(`/api/ai/ab-experiments/${experimentId}/status`, {
      status,
    })
    if (updated) {
      void loadData()
    }
  }

  const handleAddVariant = async (e: React.FormEvent) => {
    e.preventDefault()
    setVarError(null)
    if (!newVarName.trim()) {
      return
    }

    const payload = {
      name: newVarName.trim(),
      template_version: newVarVersion,
      traffic_pct: newVarTraffic,
      is_control: newVarIsControl,
    }

    const res = await postData<Variant>(`/api/ai/ab-experiments/${experimentId}/variants`, payload)
    if (res) {
      setNewVarName('')
      setNewVarVersion(1)
      setNewVarTraffic(50)
      setNewVarIsControl(false)
      void loadData()
    } else {
      setVarError(t('common.error'))
    }
  }

  const handleDeleteVariant = async (variantId: string) => {
    if (!confirm(t('admin.ab_experiments.confirm_delete_variant'))) {
      return
    }
    const res = await deleteData<{ status: string }>(
      `/api/ai/ab-experiments/${experimentId}/variants/${variantId}`,
    )
    if (res) {
      void loadData()
    }
  }

  if (!exp) {
    return <LoadingScreen minHeight="240px" />
  }

  const isDraft = exp.status === 'draft'
  const isRunning = exp.status === 'running'
  const isPaused = exp.status === 'paused'

  const totalTraffic = variants.reduce((sum, v) => sum + v.traffic_pct, 0)
  const controlCount = variants.filter((v) => v.is_control).length
  const recommendationIsWorse =
    recommendation?.winner_variant_id === '' && recommendation.reason.includes('worse')

  return (
    <div className={legacyLayoutClass('page-stack')}>
      <ExperimentDetailHeader
        exp={exp}
        isDraft={isDraft}
        isRunning={isRunning}
        isPaused={isPaused}
        totalTraffic={totalTraffic}
        controlCount={controlCount}
        onBack={onBack}
        onEdit={onEdit}
        onStatusTransition={(status) => {
          void handleStatusTransition(status)
        }}
      />

      {/* Visual traffic split bar */}
      {variants.length > 0 && (
        <div className={adminCardClass}>
          <h2 className="m-0 border-none p-0 text-base font-semibold">
            {t('admin.ab_experiments.traffic_pct')} ({totalTraffic}%)
          </h2>
          <div className="bg-card-raised mt-3 mb-4 flex h-3 w-full overflow-hidden rounded-full">
            {variants.map((v, i) => (
              <div
                key={v.id ?? i}
                className={`h-full transition-[width] duration-300 ease-in-out ${variantBarColor(v, i)}`}
                style={{ width: `${v.traffic_pct}%` }}
                title={`${v.name}: ${v.traffic_pct}%`}
              />
            ))}
          </div>
          {isDraft && totalTraffic !== 100 && (
            <div
              className="alert alert-warning"
              role="alert"
              style={{ fontSize: 13, padding: '8px 12px' }}
            >
              {t('admin.ab_experiments.traffic_sum_hint')}
            </div>
          )}
        </div>
      )}

      {/* Main Grid content */}
      <div className="grid grid-cols-1 gap-6 lg:grid-cols-[2fr_1fr]">
        {/* Left column */}
        <div style={{ display: 'flex', flexDirection: 'column', gap: 24 }}>
          {/* Recommendation Banner */}
          {recommendation && (
            <div className={abRecommendationBannerClass(recommendationIsWorse)}>
              <div className={abRecommendationTitleClass(recommendationIsWorse)}>
                <span>💡</span>
                {t('admin.ab_experiments.recommendation_title')}
              </div>
              <div className="text-foreground text-sm leading-relaxed">{recommendation.reason}</div>
            </div>
          )}

          {/* Variants list card */}
          <div className={adminCardClass}>
            <h2 className={`border-border mt-0 mb-4 border-b pb-3 text-lg font-semibold`}>
              {t('admin.ab_experiments.variants_title')}
            </h2>
            <table className="w-full border-collapse text-left">
              <thead>
                <tr>
                  <th className={adminThClass}>{t('admin.ab_experiments.variant_name')}</th>
                  <th className={adminThClass}>{t('admin.ab_experiments.template_version')}</th>
                  <th className={adminThClass}>{t('admin.ab_experiments.traffic_pct')}</th>
                  <th className={adminThClass}>{t('admin.ab_experiments.is_control')}</th>
                  {isDraft && (
                    <th className={adminThClass}>{t('admin.ab_experiments.col_actions')}</th>
                  )}
                </tr>
              </thead>
              <tbody>
                {variants.map((v) => (
                  <tr key={v.id} className="hover:bg-(--control-hover-bg)">
                    <td className={`border-border border-b px-4 py-3 text-sm font-semibold`}>
                      {v.name}
                    </td>
                    <td className={`border-border border-b px-4 py-3 text-sm`}>
                      v{v.template_version}
                    </td>
                    <td className={`border-border border-b px-4 py-3 text-sm`}>{v.traffic_pct}%</td>
                    <td className={`border-border border-b px-4 py-3 text-sm`}>
                      {v.is_control ? '✅' : '—'}
                    </td>
                    {isDraft && (
                      <td className={`border-border border-b px-4 py-3 text-sm`}>
                        <Button
                          variant="danger-outline"
                          size="sm"
                          autoWidth
                          onClick={() => {
                            if (v.id) {
                              void handleDeleteVariant(v.id)
                            }
                          }}
                        >
                          {t('common.delete')}
                        </Button>
                      </td>
                    )}
                  </tr>
                ))}
              </tbody>
            </table>

            {/* Add Variant Form (Draft only) */}
            {isDraft && (
              <form onSubmit={(e) => void handleAddVariant(e)} style={{ marginTop: 24 }}>
                <div className="border-border bg-card-raised flex flex-col gap-3 rounded-lg border border-dashed p-4">
                  <h3 style={{ fontSize: 14, margin: '0 0 8px 0', fontWeight: 600 }}>
                    {t('admin.ab_experiments.add_variant')}
                  </h3>
                  {varError ? <div className={adminErrBoxClass}>{varError}</div> : null}
                  <div className="grid grid-cols-[2fr_1fr_1fr_auto] items-center gap-3">
                    <input
                      type="text"
                      className={adminInputClass}
                      placeholder={t('admin.ab_experiments.variant_name')}
                      value={newVarName}
                      onChange={(e) => setNewVarName(e.target.value)}
                      required
                    />
                    <input
                      type="number"
                      className={adminInputClass}
                      placeholder={t('admin.ab_experiments.template_version')}
                      value={newVarVersion}
                      onChange={(e) => setNewVarVersion(parseInt(e.target.value) || 1)}
                      min={1}
                      required
                    />
                    <input
                      type="number"
                      className={adminInputClass}
                      placeholder={t('admin.ab_experiments.traffic_pct')}
                      value={newVarTraffic}
                      onChange={(e) => setNewVarTraffic(parseInt(e.target.value) || 0)}
                      min={0}
                      max={100}
                      required
                    />
                    <label className="flex items-center gap-2 text-[13px]">
                      <input
                        type="checkbox"
                        checked={newVarIsControl}
                        onChange={(e) => setNewVarIsControl(e.target.checked)}
                      />
                      {t('admin.ab_experiments.is_control')}
                    </label>
                  </div>
                  <div style={{ display: 'flex', justifyContent: 'flex-end', marginTop: 8 }}>
                    <Button type="submit" variant="primary" size="sm" autoWidth>
                      {t('common.add')}
                    </Button>
                  </div>
                </div>
              </form>
            )}
          </div>

          {/* Metrics Comparison Card */}
          {!isDraft && metrics.length > 0 && (
            <div className={adminCardClass}>
              <h2 className={`border-border mt-0 mb-4 border-b pb-3 text-lg font-semibold`}>
                {t('admin.ab_experiments.metrics_title')}
              </h2>
              <table className="w-full border-collapse text-left">
                <thead>
                  <tr>
                    <th className={adminThClass}>Variant</th>
                    <th className={adminThClass}>{t('admin.ab_experiments.metric_queries')}</th>
                    <th className={adminThClass}>
                      {t('admin.ab_experiments.metric_success_rate')}
                    </th>
                    <th className={adminThClass}>{t('admin.ab_experiments.metric_latency')}</th>
                    <th className={adminThClass}>{t('admin.ab_experiments.metric_cost')}</th>
                    <th className={adminThClass}>{t('admin.ab_experiments.metric_tokens')}</th>
                  </tr>
                </thead>
                <tbody>
                  {metrics.map((m) => {
                    const variant = variants.find((v) => v.id === m.variant_id)
                    return (
                      <tr key={m.variant_id} className="hover:bg-(--control-hover-bg)">
                        <td className={`border-border border-b px-4 py-3 text-sm font-semibold`}>
                          {variant?.name ?? 'Unknown'}{' '}
                          {variant?.is_control && (
                            <span style={{ fontSize: 11, color: '#6b7280' }}>(Control)</span>
                          )}
                        </td>
                        <td className={`border-border border-b px-4 py-3 text-sm`}>
                          {m.total_queries}
                        </td>
                        <td className={`border-border border-b px-4 py-3 text-sm`}>
                          {(m.success_rate * 100).toFixed(1)}%
                        </td>
                        <td className={`border-border border-b px-4 py-3 text-sm`}>
                          {m.avg_latency_ms.toFixed(0)} ms
                        </td>
                        <td className={`border-border border-b px-4 py-3 text-sm`}>
                          ${m.avg_cost_usd.toFixed(4)}
                        </td>
                        <td className={`border-border border-b px-4 py-3 text-sm`}>
                          {m.total_tokens.toLocaleString()}
                        </td>
                      </tr>
                    )
                  })}
                </tbody>
              </table>
            </div>
          )}
        </div>

        {/* Right column (Summary & timeseries timeline overview) */}
        <div>
          {!isDraft && timeseries.length > 0 && (
            <div className={adminCardClass}>
              <h2 className={`border-border mt-0 mb-4 border-b pb-3 text-lg font-semibold`}>
                {t('admin.ab_experiments.timeseries_title')}
              </h2>
              <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
                {timeseries.map((day) => (
                  <div
                    key={day.date}
                    style={{
                      borderBottom: '1px solid var(--color-border-subtle, #f3f4f6)',
                      paddingBottom: 8,
                    }}
                  >
                    <div style={{ fontWeight: 600, fontSize: 13, marginBottom: 6 }}>{day.date}</div>
                    {day.metrics?.map((m) => {
                      const variant = variants.find((v) => v.id === m.variant_id)
                      return (
                        <div
                          key={m.variant_id}
                          style={{
                            display: 'flex',
                            justifyContent: 'space-between',
                            fontSize: 12,
                            color: '#4b5563',
                          }}
                        >
                          <span>{variant?.name}:</span>
                          <span>
                            {m.total_queries} Q · {(m.success_rate * 100).toFixed(0)}% SR
                          </span>
                        </div>
                      )
                    })}
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
