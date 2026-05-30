import type { EvalRunSummary, EvalRunDetail } from '../../types/ai'
import { KPICard } from '../ui/KPICard'
import { getRateColor } from '../../utils/formatters'

interface EvalHistoryTabProps {
  runHistory: EvalRunSummary[]
  selectedRun: EvalRunDetail | null
  setSelectedRun: (r: EvalRunDetail | null) => void
  loadRunDetail: (id: string) => void
  localeTag: string
  t: any
}

export function EvalHistoryTab({
  runHistory,
  selectedRun,
  setSelectedRun,
  loadRunDetail,
  localeTag,
  t,
}: EvalHistoryTabProps) {
  if (!selectedRun) {
    return (
      <div className="card">
        <h3>{t('evaluation.history_title')}</h3>
        {runHistory.length === 0 ? (
          <p className="eval-empty-chart">{t('evaluation.history_empty')}</p>
        ) : (
          <table className="results-table">
            <thead>
              <tr>
                <th>{t('evaluation.hist_date')}</th>
                <th>{t('evaluation.hist_model')}</th>
                <th>{t('evaluation.hist_total')}</th>
                <th>{t('evaluation.hist_passed')}</th>
                <th>{t('evaluation.hist_failed')}</th>
                <th>{t('evaluation.hist_success_pct')}</th>
                <th>{t('evaluation.hist_prompt_version')}</th>
                <th>{t('evaluation.hist_detail')}</th>
              </tr>
            </thead>
            <tbody>
              {runHistory.map((r) => (
                <tr key={r.run_id}>
                  <td>{new Date(r.completed_at).toLocaleString(localeTag)}</td>
                  <td>{r.model}</td>
                  <td>{r.total_cases}</td>
                  <td style={{ color: 'var(--success)' }}>{r.passed}</td>
                  <td style={{ color: 'var(--error)' }}>{r.failed}</td>
                  <td
                    style={{
                      color: getRateColor(r.pass_rate * 100),
                      fontWeight: 700,
                    }}
                  >
                    {(r.pass_rate * 100).toFixed(0)}%
                  </td>
                  <td>v{r.prompt_template_bundle_version ?? 0}</td>
                  <td>
                    <button className="btn btn-sm" onClick={() => loadRunDetail(r.run_id)}>
                      {t('evaluation.detail_btn')}
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

  return (
    <>
      <div style={{ marginBottom: '0.5rem' }}>
        <button className="btn-back" onClick={() => setSelectedRun(null)}>
          {t('evaluation.back')}
        </button>
      </div>
      <div className="kpi-row">
        <KPICard label={t('evaluation.detail_kpi_total')} value={selectedRun.summary.total_cases} color="var(--accent)" />
        <KPICard label={t('evaluation.detail_kpi_passed')} value={selectedRun.summary.passed} color="var(--success)" />
        <KPICard label={t('evaluation.detail_kpi_failed')} value={selectedRun.summary.failed} color="var(--error)" />
        <KPICard label={t('evaluation.detail_kpi_rate')} value={`${(selectedRun.summary.pass_rate * 100).toFixed(0)}%`} color="var(--warning)" />
      </div>
      <div className="card">
        <h3>
          {t('evaluation.detail_cases_title', {
            model: selectedRun.summary.model,
            date: new Date(selectedRun.summary.completed_at).toLocaleString(localeTag),
          })}
        </h3>
        <table className="results-table eval-results-table">
          <thead>
            <tr>
              <th>{t('evaluation.col_scenario')}</th>
              <th>{t('evaluation.col_question')}</th>
              <th>{t('evaluation.col_status')}</th>
              <th>{t('evaluation.col_reason')}</th>
              <th>{t('evaluation.col_confidence')}</th>
              <th>{t('evaluation.col_latency')}</th>
            </tr>
          </thead>
          <tbody>
            {selectedRun.test_cases.map((tc) => (
              <tr key={tc.case_id}>
                <td>{tc.case_id}</td>
                <td>{tc.question}</td>
                <td>
                  <span className={`status-badge ${tc.match ? 'success' : 'error'}`}>
                    {tc.match ? t('evaluation.match_yes') : t('evaluation.match_no')}
                  </span>
                </td>
                <td
                  style={{
                    fontSize: '0.8rem',
                    maxWidth: 300,
                    overflow: 'hidden',
                    textOverflow: 'ellipsis',
                    whiteSpace: 'nowrap',
                  }}
                >
                  {tc.reason || t('common.em_dash')}
                </td>
                <td>{(tc.confidence * 100).toFixed(0)}%</td>
                <td>{t('evaluation.latency_ms', { ms: tc.latency_ms })}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </>
  )
}
