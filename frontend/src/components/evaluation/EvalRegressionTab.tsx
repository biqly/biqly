import type { TFunction } from '../../i18n'
import { legacyButtonClass } from '../../lib/buttonClasses'
import { legacyCardClass } from '../../lib/cardClasses'
import { cn } from '../../lib/cn'
import {
  evalStatusBadgeClass,
  evalStatusFailBadgeClass,
  evalStatusPassBadgeClass,
} from '../../lib/feedbackClasses'
import { legacyTableClass } from '../../lib/tableClasses'
import type { EvalRunSummary, RegressionReport } from '../../types/ai'
import { formatDateTime } from '../../utils/formatters'
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
      <div className={legacyCardClass('card')}>
        <h3>{t('evaluation.regression_title')}</h3>
        <div className="flex flex-wrap items-end gap-4">
          <div className="min-w-56 flex-1">
            <label className="text-caption text-foreground-muted mb-1 block">
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
                hint: formatDateTime(r.completed_at, localeTag),
              }))}
            />
          </div>
          <div className="min-w-56 flex-1">
            <label className="text-caption text-foreground-muted mb-1 block">
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
                hint: formatDateTime(r.completed_at, localeTag),
              }))}
            />
          </div>
          <button
            className={legacyButtonClass('btn btn-sm btn-primary')}
            onClick={runRegression}
            disabled={!configured || !baselineId || !currentId || regressionLoading}
          >
            {regressionLoading ? t('evaluation.comparing') : t('evaluation.compare')}
          </button>
        </div>
      </div>

      {regression && (
        <>
          <div className="grid grid-cols-[repeat(auto-fit,minmax(200px,1fr))] gap-4">
            <div className={cn(legacyCardClass('card'), 'border-error mb-0')}>
              <p className="text-foreground-muted text-caption">
                {t('evaluation.reg_new_failures')}
              </p>
              <p className="text-error text-3xl font-bold">{regression.new_failures.length}</p>
            </div>
            <div className={cn(legacyCardClass('card'), 'border-success mb-0')}>
              <p className="text-foreground-muted text-caption">{t('evaluation.reg_fixed')}</p>
              <p className="text-success text-3xl font-bold">{regression.fixed_failures.length}</p>
            </div>
            <div className={cn(legacyCardClass('card'), 'border-warning mb-0')}>
              <p className="text-foreground-muted text-caption">{t('evaluation.reg_changed')}</p>
              <p className="text-warning text-3xl font-bold">{regression.changed_cases.length}</p>
            </div>
          </div>

          {regression.new_failures.length > 0 && (
            <div className={legacyCardClass('card')}>
              <h3 className="text-error">{t('evaluation.reg_new_failures_heading')}</h3>
              <table className={legacyTableClass('results-table')}>
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
                        <span className={evalStatusPassBadgeClass}>
                          {t('evaluation.match_yes')}
                        </span>
                      </td>
                      <td>
                        <span className={evalStatusFailBadgeClass}>{t('evaluation.match_no')}</span>
                      </td>
                      <td className="text-caption">{c.is_reason}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}

          {regression.fixed_failures.length > 0 && (
            <div className={legacyCardClass('card')}>
              <h3 className="text-success">{t('evaluation.reg_fixed_heading')}</h3>
              <table className={legacyTableClass('results-table')}>
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
                        <span className={evalStatusFailBadgeClass}>{t('evaluation.match_no')}</span>
                      </td>
                      <td>
                        <span className={evalStatusPassBadgeClass}>
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
            <div className={legacyCardClass('card')}>
              <h3 className="text-warning">{t('evaluation.reg_changed_heading')}</h3>
              <table className={legacyTableClass('results-table')}>
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
                        <span className={evalStatusBadgeClass(c.was_match)}>
                          {c.was_match ? t('evaluation.match_yes') : t('evaluation.match_no')}
                        </span>
                      </td>
                      <td>
                        <span className={evalStatusBadgeClass(c.is_match)}>
                          {c.is_match ? t('evaluation.match_yes') : t('evaluation.match_no')}
                        </span>
                      </td>
                      <td className="text-caption">{c.was_reason || t('common.em_dash')}</td>
                      <td className="text-caption">{c.is_reason || t('common.em_dash')}</td>
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
