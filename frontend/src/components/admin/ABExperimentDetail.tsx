import { useCallback, useEffect, useState } from 'react'

import { useAdminApi } from '../../hooks/useApi'
import { useT } from '../../i18n'
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

export function ABExperimentDetail({ experimentId, onBack, onEdit }: ABExperimentDetailProps) {
  const t = useT()
  const { get, postData, putData, deleteData, loading } = useAdminApi()

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
  const isCompleted = exp.status === 'completed'

  const totalTraffic = variants.reduce((sum, v) => sum + v.traffic_pct, 0)
  const controlCount = variants.filter((v) => v.is_control).length

  return (
    <div className="ab-experiment">
      <button type="button" className="ab-experiment__back-btn" onClick={onBack}>
        {t('admin.workspaces.back')}
      </button>

      <div className="ab-experiment__header">
        <div className="ab-experiment__title-group">
          <h1 className="ab-experiment__title">{exp.name}</h1>
          <p className="ab-experiment__desc">{exp.description || t('metadata.no_description')}</p>
          <div style={{ display: 'flex', gap: 16, marginTop: 8 }}>
            <span>
              <strong>{t('admin.ab_experiments.col_template')}:</strong> {exp.template_name}
            </span>
            <span>
              <strong>{t('admin.ab_experiments.col_locale')}:</strong> {exp.locale.toUpperCase()}
            </span>
            <span>
              <strong>{t('admin.ab_experiments.col_status')}:</strong>{' '}
              <span className={`ab-status-badge ab-status-badge--${exp.status}`}>
                <span className="ab-status-badge__dot" />
                {exp.status}
              </span>
            </span>
          </div>
        </div>

        <div className="ab-form__actions" style={{ marginTop: 0 }}>
          {isDraft && (
            <>
              <button type="button" className="btn btn-secondary" onClick={onEdit}>
                {t('common.edit')}
              </button>
              <button
                type="button"
                className="btn btn-primary"
                onClick={() => void handleStatusTransition('running')}
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
                className="btn btn-secondary"
                onClick={() => void handleStatusTransition('paused')}
              >
                {t('admin.ab_experiments.pause_btn')}
              </button>
              <button
                type="button"
                className="btn btn-danger"
                onClick={() => void handleStatusTransition('completed')}
              >
                {t('admin.ab_experiments.complete_btn')}
              </button>
            </>
          )}

          {isPaused && (
            <>
              <button
                type="button"
                className="btn btn-primary"
                onClick={() => void handleStatusTransition('running')}
              >
                {t('admin.ab_experiments.resume_btn')}
              </button>
              <button
                type="button"
                className="btn btn-danger"
                onClick={() => void handleStatusTransition('completed')}
              >
                {t('admin.ab_experiments.complete_btn')}
              </button>
            </>
          )}
        </div>
      </div>

      {/* Visual traffic split bar */}
      {variants.length > 0 && (
        <div className="ab-experiment-card">
          <h2
            className="ab-experiment-card__title"
            style={{ fontSize: 16, border: 'none', margin: 0, padding: 0 }}
          >
            {t('admin.ab_experiments.traffic_pct')} ({totalTraffic}%)
          </h2>
          <div className="ab-traffic-bar">
            {variants.map((v, i) => (
              <div
                key={v.id ?? i}
                className={`ab-traffic-bar__segment ab-traffic-bar__segment--${
                  v.is_control ? 'control' : `treatment-${i}`
                }`}
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
      <div className="ab-experiment__content-grid">
        {/* Left column */}
        <div style={{ display: 'flex', flexDirection: 'column', gap: 24 }}>
          {/* Recommendation Banner */}
          {recommendation && (
            <div
              className={`ab-recommendation ${
                recommendation.winner_variant_id === '' && recommendation.reason.includes('worse')
                  ? 'ab-recommendation--warning'
                  : ''
              }`}
            >
              <div className="ab-recommendation__header">
                <span>💡</span>
                {t('admin.ab_experiments.recommendation_title')}
              </div>
              <div className="ab-recommendation__reason">{recommendation.reason}</div>
            </div>
          )}

          {/* Variants list card */}
          <div className="ab-experiment-card">
            <h2 className="ab-experiment-card__title">
              {t('admin.ab_experiments.variants_title')}
            </h2>
            <table className="ab-experiment-table">
              <thead>
                <tr>
                  <th>{t('admin.ab_experiments.variant_name')}</th>
                  <th>{t('admin.ab_experiments.template_version')}</th>
                  <th>{t('admin.ab_experiments.traffic_pct')}</th>
                  <th>{t('admin.ab_experiments.is_control')}</th>
                  {isDraft && <th>{t('admin.ab_experiments.col_actions')}</th>}
                </tr>
              </thead>
              <tbody>
                {variants.map((v) => (
                  <tr key={v.id}>
                    <td className="ab-experiment-table__cell-bold">{v.name}</td>
                    <td>v{v.template_version}</td>
                    <td>{v.traffic_pct}%</td>
                    <td>{v.is_control ? '✅' : '—'}</td>
                    {isDraft && (
                      <td>
                        <button
                          type="button"
                          className="btn btn-secondary btn-sm"
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
                <div className="ab-variant-builder">
                  <h3 style={{ fontSize: 14, margin: '0 0 8px 0', fontWeight: 600 }}>
                    {t('admin.ab_experiments.add_variant')}
                  </h3>
                  {varError && <div className="alert alert-danger">{varError}</div>}
                  <div className="ab-variant-builder__row">
                    <input
                      type="text"
                      className="ab-form__input"
                      placeholder={t('admin.ab_experiments.variant_name')}
                      value={newVarName}
                      onChange={(e) => setNewVarName(e.target.value)}
                      required
                    />
                    <input
                      type="number"
                      className="ab-form__input"
                      placeholder={t('admin.ab_experiments.template_version')}
                      value={newVarVersion}
                      onChange={(e) => setNewVarVersion(parseInt(e.target.value) || 1)}
                      min={1}
                      required
                    />
                    <input
                      type="number"
                      className="ab-form__input"
                      placeholder={t('admin.ab_experiments.traffic_pct')}
                      value={newVarTraffic}
                      onChange={(e) => setNewVarTraffic(parseInt(e.target.value) || 0)}
                      min={0}
                      max={100}
                      required
                    />
                    <label className="ab-variant-builder__control-checkbox">
                      <input
                        type="checkbox"
                        checked={newVarIsControl}
                        onChange={(e) => setNewVarIsControl(e.target.checked)}
                      />
                      {t('admin.ab_experiments.is_control')}
                    </label>
                  </div>
                  <div style={{ display: 'flex', justifyContent: 'flex-end', marginTop: 8 }}>
                    <button type="submit" className="btn btn-primary btn-sm">
                      {t('common.add')}
                    </button>
                  </div>
                </div>
              </form>
            )}
          </div>

          {/* Metrics Comparison Card */}
          {!isDraft && metrics.length > 0 && (
            <div className="ab-experiment-card">
              <h2 className="ab-experiment-card__title">
                {t('admin.ab_experiments.metrics_title')}
              </h2>
              <table className="ab-experiment-table">
                <thead>
                  <tr>
                    <th>Variant</th>
                    <th>{t('admin.ab_experiments.metric_queries')}</th>
                    <th>{t('admin.ab_experiments.metric_success_rate')}</th>
                    <th>{t('admin.ab_experiments.metric_latency')}</th>
                    <th>{t('admin.ab_experiments.metric_cost')}</th>
                    <th>{t('admin.ab_experiments.metric_tokens')}</th>
                  </tr>
                </thead>
                <tbody>
                  {metrics.map((m) => {
                    const variant = variants.find((v) => v.id === m.variant_id)
                    return (
                      <tr key={m.variant_id}>
                        <td className="ab-experiment-table__cell-bold">
                          {variant?.name ?? 'Unknown'}{' '}
                          {variant?.is_control && (
                            <span style={{ fontSize: 11, color: '#6b7280' }}>(Control)</span>
                          )}
                        </td>
                        <td>{m.total_queries}</td>
                        <td>{(m.success_rate * 100).toFixed(1)}%</td>
                        <td>{m.avg_latency_ms.toFixed(0)} ms</td>
                        <td>${m.avg_cost_usd.toFixed(4)}</td>
                        <td>{m.total_tokens.toLocaleString()}</td>
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
            <div className="ab-experiment-card">
              <h2 className="ab-experiment-card__title">
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
