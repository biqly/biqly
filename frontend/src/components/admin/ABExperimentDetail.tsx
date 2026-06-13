import { useCallback, useEffect, useState } from 'react'

import { useAdminApi } from '../../hooks/useApi'
import { useT } from '../../i18n'
import { legacyButtonClass } from '../../lib/buttonClasses'
import { legacyCardClass } from '../../lib/cardClasses'
import { abRecommendationBannerClass, abRecommendationTitleClass } from '../../lib/feedbackClasses'
import type { Experiment } from './ABExperimentForm'
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

function experimentStatusBadgeClass(status: Experiment['status']): string {
  switch (status) {
    case 'draft':
      return 'bg-[#f3f4f6] text-[#374151] dark:bg-zinc-800 dark:text-zinc-300'
    case 'running':
      return 'bg-[#ecfdf5] text-[#065f46] dark:bg-emerald-950/30 dark:text-emerald-400'
    case 'paused':
      return 'bg-[#fffbeb] text-[#92400e] dark:bg-amber-950/30 dark:text-amber-400'
    case 'completed':
      return 'bg-[#eff6ff] text-[#1e40af] dark:bg-blue-950/30 dark:text-blue-400'
    default:
      return ''
  }
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
      <button
        type="button"
        className="inline-flex items-center gap-2 bg-none border-none text-accent text-sm font-medium cursor-pointer p-0 mb-2 transition-colors duration-200 hover:text-accent-hover"
        onClick={onBack}
      >
        {t('admin.workspaces.back')}
      </button>

      <div className={`flex justify-between items-center gap-4 border-b border-border pb-4`}>
        <div className="flex flex-col gap-1">
          <h1 className="text-2xl font-semibold m-0">{exp.name}</h1>
          <p className="text-sm text-foreground-muted m-0 max-w-[720px]">
            {exp.description || t('metadata.no_description')}
          </p>
          <div style={{ display: 'flex', gap: 16, marginTop: 8 }}>
            <span>
              <strong>{t('admin.ab_experiments.col_template')}:</strong> {exp.template_name}
            </span>
            <span>
              <strong>{t('admin.ab_experiments.col_locale')}:</strong> {exp.locale.toUpperCase()}
            </span>
            <span>
              <strong>{t('admin.ab_experiments.col_status')}:</strong>{' '}
              <span
                className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium capitalize ${experimentStatusBadgeClass(exp.status)}`}
              >
                <span className="w-1.5 h-1.5 rounded-full bg-current" />
                {exp.status}
              </span>
            </span>
          </div>
        </div>

        <div className="flex justify-end gap-3 mt-0" style={{ marginTop: 0 }}>
          {isDraft && (
            <>
              <button
                type="button"
                className={legacyButtonClass('btn btn-secondary')}
                onClick={onEdit}
              >
                {t('common.edit')}
              </button>
              <button
                type="button"
                className={legacyButtonClass('btn btn-primary')}
                onClick={() => void onStatusTransition('running')}
                disabled={totalTraffic !== 100 || controlCount !== 1}
              >
                {t('admin.ab_experiments.start_btn')}
              </button>
            </>
          )}

          {isRunning && (
            <>
              <button
                type="button"
                className={legacyButtonClass('btn btn-secondary')}
                onClick={() => void onStatusTransition('paused')}
              >
                {t('admin.ab_experiments.pause_btn')}
              </button>
              <button
                type="button"
                className={legacyButtonClass('btn btn-danger')}
                onClick={() => void onStatusTransition('completed')}
              >
                {t('admin.ab_experiments.complete_btn')}
              </button>
            </>
          )}

          {isPaused && (
            <>
              <button
                type="button"
                className={legacyButtonClass('btn btn-primary')}
                onClick={() => void onStatusTransition('running')}
              >
                {t('admin.ab_experiments.resume_btn')}
              </button>
              <button
                type="button"
                className={legacyButtonClass('btn btn-danger')}
                onClick={() => void onStatusTransition('completed')}
              >
                {t('admin.ab_experiments.complete_btn')}
              </button>
            </>
          )}
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
    return <div style={{ padding: 24, textAlign: 'center' }}>{t('common.loading')}</div>
  }

  const isDraft = exp.status === 'draft'
  const isRunning = exp.status === 'running'
  const isPaused = exp.status === 'paused'

  const totalTraffic = variants.reduce((sum, v) => sum + v.traffic_pct, 0)
  const controlCount = variants.filter((v) => v.is_control).length
  const recommendationIsWorse =
    recommendation?.winner_variant_id === '' && recommendation.reason.includes('worse')

  return (
    <div className="flex flex-col gap-6 p-6 text-foreground font-[var(--font-family,inherit)]">
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
        <div
          className={legacyCardClass('bg-card border border-border rounded-lg p-6 shadow-card-sm')}
        >
          <h2 className="text-base font-semibold m-0 p-0 border-none">
            {t('admin.ab_experiments.traffic_pct')} ({totalTraffic}%)
          </h2>
          <div
            className={legacyCardClass(
              'flex h-3 w-full rounded-full overflow-hidden mt-3 mb-4 bg-card-raised',
            )}
          >
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
      <div className="grid grid-cols-1 lg:grid-cols-[2fr_1fr] gap-6">
        {/* Left column */}
        <div style={{ display: 'flex', flexDirection: 'column', gap: 24 }}>
          {/* Recommendation Banner */}
          {recommendation && (
            <div className={abRecommendationBannerClass(recommendationIsWorse)}>
              <div className={abRecommendationTitleClass(recommendationIsWorse)}>
                <span>💡</span>
                {t('admin.ab_experiments.recommendation_title')}
              </div>
              <div className="text-sm text-foreground leading-relaxed">{recommendation.reason}</div>
            </div>
          )}

          {/* Variants list card */}
          <div
            className={legacyCardClass(
              'bg-card border border-border rounded-lg p-6 shadow-card-sm',
            )}
          >
            <h2 className={`text-lg font-semibold mt-0 mb-4 border-b border-border pb-3`}>
              {t('admin.ab_experiments.variants_title')}
            </h2>
            <table className="w-full border-collapse text-left">
              <thead>
                <tr>
                  <th
                    className={legacyCardClass(
                      'px-4 py-3 border-b border-border text-sm font-medium text-foreground-muted bg-card-raised',
                    )}
                  >
                    {t('admin.ab_experiments.variant_name')}
                  </th>
                  <th
                    className={legacyCardClass(
                      'px-4 py-3 border-b border-border text-sm font-medium text-foreground-muted bg-card-raised',
                    )}
                  >
                    {t('admin.ab_experiments.template_version')}
                  </th>
                  <th
                    className={legacyCardClass(
                      'px-4 py-3 border-b border-border text-sm font-medium text-foreground-muted bg-card-raised',
                    )}
                  >
                    {t('admin.ab_experiments.traffic_pct')}
                  </th>
                  <th
                    className={legacyCardClass(
                      'px-4 py-3 border-b border-border text-sm font-medium text-foreground-muted bg-card-raised',
                    )}
                  >
                    {t('admin.ab_experiments.is_control')}
                  </th>
                  {isDraft && (
                    <th
                      className={legacyCardClass(
                        'px-4 py-3 border-b border-border text-sm font-medium text-foreground-muted bg-card-raised',
                      )}
                    >
                      {t('admin.ab_experiments.col_actions')}
                    </th>
                  )}
                </tr>
              </thead>
              <tbody>
                {variants.map((v) => (
                  <tr key={v.id} className="hover:bg-[var(--control-hover-bg)]">
                    <td className={`px-4 py-3 border-b border-border text-sm font-semibold`}>
                      {v.name}
                    </td>
                    <td className={`px-4 py-3 border-b border-border text-sm`}>
                      v{v.template_version}
                    </td>
                    <td className={`px-4 py-3 border-b border-border text-sm`}>{v.traffic_pct}%</td>
                    <td className={`px-4 py-3 border-b border-border text-sm`}>
                      {v.is_control ? '✅' : '—'}
                    </td>
                    {isDraft && (
                      <td className={`px-4 py-3 border-b border-border text-sm`}>
                        <button
                          type="button"
                          className={legacyButtonClass('btn btn-secondary btn-sm')}
                          style={{ color: '#ef4444' }}
                          onClick={() => {
                            if (v.id) {
                              void handleDeleteVariant(v.id)
                            }
                          }}
                        >
                          {t('common.delete')}
                        </button>
                      </td>
                    )}
                  </tr>
                ))}
              </tbody>
            </table>

            {/* Add Variant Form (Draft only) */}
            {isDraft && (
              <form onSubmit={(e) => void handleAddVariant(e)} style={{ marginTop: 24 }}>
                <div
                  className={legacyCardClass(
                    'border border-dashed border-border-strong rounded-lg p-4 bg-card-raised flex flex-col gap-3',
                  )}
                >
                  <h3 style={{ fontSize: 14, margin: '0 0 8px 0', fontWeight: 600 }}>
                    {t('admin.ab_experiments.add_variant')}
                  </h3>
                  {varError && <div className="alert alert-danger">{varError}</div>}
                  <div className="grid grid-cols-[2fr_1fr_1fr_auto] gap-3 items-center">
                    <input
                      type="text"
                      className={`px-3 py-2 border border-border rounded-md text-sm bg-[var(--input-bg)] text-inherit w-full focus:outline-none focus:border-accent focus:ring-2 focus:ring-[var(--control-focus-ring)]`}
                      placeholder={t('admin.ab_experiments.variant_name')}
                      value={newVarName}
                      onChange={(e) => setNewVarName(e.target.value)}
                      required
                    />
                    <input
                      type="number"
                      className={`px-3 py-2 border border-border rounded-md text-sm bg-[var(--input-bg)] text-inherit w-full focus:outline-none focus:border-accent focus:ring-2 focus:ring-[var(--control-focus-ring)]`}
                      placeholder={t('admin.ab_experiments.template_version')}
                      value={newVarVersion}
                      onChange={(e) => setNewVarVersion(parseInt(e.target.value) || 1)}
                      min={1}
                      required
                    />
                    <input
                      type="number"
                      className={`px-3 py-2 border border-border rounded-md text-sm bg-[var(--input-bg)] text-inherit w-full focus:outline-none focus:border-accent focus:ring-2 focus:ring-[var(--control-focus-ring)]`}
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
                    <button type="submit" className={legacyButtonClass('btn btn-primary btn-sm')}>
                      {t('common.add')}
                    </button>
                  </div>
                </div>
              </form>
            )}
          </div>

          {/* Metrics Comparison Card */}
          {!isDraft && metrics.length > 0 && (
            <div
              className={legacyCardClass(
                'bg-card border border-border rounded-lg p-6 shadow-card-sm',
              )}
            >
              <h2 className={`text-lg font-semibold mt-0 mb-4 border-b border-border pb-3`}>
                {t('admin.ab_experiments.metrics_title')}
              </h2>
              <table className="w-full border-collapse text-left">
                <thead>
                  <tr>
                    <th
                      className={legacyCardClass(
                        'px-4 py-3 border-b border-border text-sm font-medium text-foreground-muted bg-card-raised',
                      )}
                    >
                      Variant
                    </th>
                    <th
                      className={legacyCardClass(
                        'px-4 py-3 border-b border-border text-sm font-medium text-foreground-muted bg-card-raised',
                      )}
                    >
                      {t('admin.ab_experiments.metric_queries')}
                    </th>
                    <th
                      className={legacyCardClass(
                        'px-4 py-3 border-b border-border text-sm font-medium text-foreground-muted bg-card-raised',
                      )}
                    >
                      {t('admin.ab_experiments.metric_success_rate')}
                    </th>
                    <th
                      className={legacyCardClass(
                        'px-4 py-3 border-b border-border text-sm font-medium text-foreground-muted bg-card-raised',
                      )}
                    >
                      {t('admin.ab_experiments.metric_latency')}
                    </th>
                    <th
                      className={legacyCardClass(
                        'px-4 py-3 border-b border-border text-sm font-medium text-foreground-muted bg-card-raised',
                      )}
                    >
                      {t('admin.ab_experiments.metric_cost')}
                    </th>
                    <th
                      className={legacyCardClass(
                        'px-4 py-3 border-b border-border text-sm font-medium text-foreground-muted bg-card-raised',
                      )}
                    >
                      {t('admin.ab_experiments.metric_tokens')}
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {metrics.map((m) => {
                    const variant = variants.find((v) => v.id === m.variant_id)
                    return (
                      <tr key={m.variant_id} className="hover:bg-[var(--control-hover-bg)]">
                        <td className={`px-4 py-3 border-b border-border text-sm font-semibold`}>
                          {variant?.name ?? 'Unknown'}{' '}
                          {variant?.is_control && (
                            <span style={{ fontSize: 11, color: '#6b7280' }}>(Control)</span>
                          )}
                        </td>
                        <td className={`px-4 py-3 border-b border-border text-sm`}>
                          {m.total_queries}
                        </td>
                        <td className={`px-4 py-3 border-b border-border text-sm`}>
                          {(m.success_rate * 100).toFixed(1)}%
                        </td>
                        <td className={`px-4 py-3 border-b border-border text-sm`}>
                          {m.avg_latency_ms.toFixed(0)} ms
                        </td>
                        <td className={`px-4 py-3 border-b border-border text-sm`}>
                          ${m.avg_cost_usd.toFixed(4)}
                        </td>
                        <td className={`px-4 py-3 border-b border-border text-sm`}>
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
            <div
              className={legacyCardClass(
                'bg-card border border-border rounded-lg p-6 shadow-card-sm',
              )}
            >
              <h2 className={`text-lg font-semibold mt-0 mb-4 border-b border-border pb-3`}>
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
