import type { TFunction } from '../../i18n'
import type { EvalRunSummary, RegressionReport } from '../../types/ai'
import { Select } from '../ui/Select'

interface EvalRegressionTabProps {
  runHistory: EvalRunSummary[]
  baselineId: string
  currentId: string
  setBaselineId: (v: string) => void
  setCurrentId: (v: string) => void
  regression: RegressionReport | null
  regressionLoading: boolean
  runRegression: () => void
  configured: boolean
  localeTag: string
  t: TFunction
}

export function EvalRegressionTab({
  runHistory,
  baselineId,
  currentId,
  setBaselineId,
  setCurrentId,
  regression,
  regressionLoading,
  runRegression,
  configured,
  localeTag,
  t,
}: EvalRegressionTabProps) {
  return (
    <>
      <div className="card">
        <h3>{t('evaluation.regression_title')}</h3>
        <div style={{ display: 'flex', gap: '1rem', alignItems: 'flex-end', flexWrap: 'wrap' }}>
          <div style={{ flex: 1, minWidth: '14rem' }}>
            <label
              style={{
                display: 'block',
                fontSize: '0.8rem',
                color: 'var(--text-secondary)',
                marginBottom: '0.3rem',
              }}
            >
              {t('evaluation.baseline_label')}
            </label>
            <Select
              value={baselineId}
              onChange={setBaselineId}
              placeholder={t('evaluation.placeholder_select')}
              header={t('evaluation.runs_header')}
              options={runHistory.map((r) => ({
                value: r.run_id,
                label: `${r.model} ${t('common.em_dash')} ${(r.pass_rate * 100).toFixed(0)}%`,
                hint: new Date(r.completed_at).toLocaleString(localeTag),
              }))}
            />
          </div>
          <div style={{ flex: 1, minWidth: '14rem' }}>
            <label
              style={{
                display: 'block',
                fontSize: '0.8rem',
                color: 'var(--text-secondary)',
                marginBottom: '0.3rem',
              }}
            >
              {t('evaluation.current_label')}
            </label>
            <Select
              value={currentId}
              onChange={setCurrentId}
              placeholder={t('evaluation.placeholder_select')}
              header={t('evaluation.runs_header')}
              options={runHistory.map((r) => ({
                value: r.run_id,
                label: `${r.model} ${t('common.em_dash')} ${(r.pass_rate * 100).toFixed(0)}%`,
                hint: new Date(r.completed_at).toLocaleString(localeTag),
              }))}
            />
          </div>
          <button
            className="btn btn-sm btn-primary"
            onClick={runRegression}
            disabled={!configured || !baselineId || !currentId || regressionLoading}
          >
            {regressionLoading ? t('evaluation.comparing') : t('evaluation.compare')}
          </button>
        </div>
      </div>

      {regression && (
        <>
          <div
            style={{
              display: 'grid',
              gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))',
              gap: '1rem',
            }}
          >
            <div className="card" style={{ borderColor: 'var(--error)', marginBottom: 0 }}>
              <p style={{ color: 'var(--text-secondary)', fontSize: '0.8rem' }}>
                {t('evaluation.reg_new_failures')}
              </p>
              <p style={{ fontSize: '2rem', fontWeight: 700, color: 'var(--error)' }}>
                {regression.new_failures.length}
              </p>
            </div>
            <div className="card" style={{ borderColor: 'var(--success)', marginBottom: 0 }}>
              <p style={{ color: 'var(--text-secondary)', fontSize: '0.8rem' }}>
                {t('evaluation.reg_fixed')}
              </p>
              <p style={{ fontSize: '2rem', fontWeight: 700, color: 'var(--success)' }}>
                {regression.fixed_failures.length}
              </p>
            </div>
            <div className="card" style={{ borderColor: 'var(--warning)', marginBottom: 0 }}>
              <p style={{ color: 'var(--text-secondary)', fontSize: '0.8rem' }}>
                {t('evaluation.reg_changed')}
              </p>
              <p style={{ fontSize: '2rem', fontWeight: 700, color: 'var(--warning)' }}>
                {regression.changed_cases.length}
              </p>
            </div>
          </div>

          {regression.new_failures.length > 0 && (
            <div className="card">
              <h3 style={{ color: 'var(--error)' }}>{t('evaluation.reg_new_failures_heading')}</h3>
              <table className="results-table">
                <thead>
                  <tr>
                    <th>{t('evaluation.col_scenario')}</th>
                    <th>{t('evaluation.col_question')}</th>
                    <th>{t('evaluation.reg_col_before')}</th>
                    <th>{t('evaluation.reg_col_after')}</th>
                    <th>{t('evaluation.reg_col_why')}</th>
                  </tr>
                </thead>
                <tbody>
                  {regression.new_failures.map((c) => (
                    <tr key={c.case_id}>
                      <td>{c.case_id}</td>
                      <td>{c.question}</td>
                      <td>
                        <span className="inline-block text-[0.72rem] font-bold uppercase tracking-normal px-2 py-0.5 rounded-full bg-[rgba(52,211,153,0.12)] text-success">
                          {t('evaluation.match_yes')}
                        </span>
                      </td>
                      <td>
                        <span className="inline-block text-[0.72rem] font-bold uppercase tracking-normal px-2 py-0.5 rounded-full bg-[rgba(251,113,133,0.12)] text-error">
                          {t('evaluation.match_no')}
                        </span>
                      </td>
                      <td style={{ fontSize: '0.8rem' }}>{c.is_reason}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}

          {regression.fixed_failures.length > 0 && (
            <div className="card">
              <h3 style={{ color: 'var(--success)' }}>{t('evaluation.reg_fixed_heading')}</h3>
              <table className="results-table">
                <thead>
                  <tr>
                    <th>{t('evaluation.col_scenario')}</th>
                    <th>{t('evaluation.col_question')}</th>
                    <th>{t('evaluation.reg_col_before')}</th>
                    <th>{t('evaluation.reg_col_after')}</th>
                  </tr>
                </thead>
                <tbody>
                  {regression.fixed_failures.map((c) => (
                    <tr key={c.case_id}>
                      <td>{c.case_id}</td>
                      <td>{c.question}</td>
                      <td>
                        <span className="inline-block text-[0.72rem] font-bold uppercase tracking-normal px-2 py-0.5 rounded-full bg-[rgba(251,113,133,0.12)] text-error">
                          {t('evaluation.match_no')}
                        </span>
                      </td>
                      <td>
                        <span className="inline-block text-[0.72rem] font-bold uppercase tracking-normal px-2 py-0.5 rounded-full bg-[rgba(52,211,153,0.12)] text-success">
                          {t('evaluation.match_yes')}
                        </span>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}

          {regression.changed_cases.length > 0 && (
            <div className="card">
              <h3 style={{ color: 'var(--warning)' }}>{t('evaluation.reg_changed_heading')}</h3>
              <table className="results-table">
                <thead>
                  <tr>
                    <th>{t('evaluation.col_scenario')}</th>
                    <th>{t('evaluation.col_question')}</th>
                    <th>{t('evaluation.reg_col_before')}</th>
                    <th>{t('evaluation.reg_col_after')}</th>
                    <th>{t('evaluation.reg_col_prev_why')}</th>
                    <th>{t('evaluation.reg_col_next_why')}</th>
                  </tr>
                </thead>
                <tbody>
                  {regression.changed_cases.map((c) => (
                    <tr key={c.case_id}>
                      <td>{c.case_id}</td>
                      <td>{c.question}</td>
                      <td>
                        <span
                          className={`inline-block text-[0.72rem] font-bold uppercase tracking-normal px-2 py-0.5 rounded-full ${
                            c.was_match
                              ? 'bg-[rgba(52,211,153,0.12)] text-success'
                              : 'bg-[rgba(251,113,133,0.12)] text-error'
                          }`}
                        >
                          {c.was_match ? t('evaluation.match_yes') : t('evaluation.match_no')}
                        </span>
                      </td>
                      <td>
                        <span
                          className={`inline-block text-[0.72rem] font-bold uppercase tracking-normal px-2 py-0.5 rounded-full ${
                            c.is_match
                              ? 'bg-[rgba(52,211,153,0.12)] text-success'
                              : 'bg-[rgba(251,113,133,0.12)] text-error'
                          }`}
                        >
                          {c.is_match ? t('evaluation.match_yes') : t('evaluation.match_no')}
                        </span>
                      </td>
                      <td style={{ fontSize: '0.8rem' }}>{c.was_reason || t('common.em_dash')}</td>
                      <td style={{ fontSize: '0.8rem' }}>{c.is_reason || t('common.em_dash')}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </>
      )}
    </>
  )
}
