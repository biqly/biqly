import { useState } from 'react'
import { Bar } from 'recharts/es6/cartesian/Bar'
import { CartesianGrid } from 'recharts/es6/cartesian/CartesianGrid'
import { XAxis } from 'recharts/es6/cartesian/XAxis'
import { YAxis } from 'recharts/es6/cartesian/YAxis'
import { BarChart } from 'recharts/es6/chart/BarChart'
import { PieChart } from 'recharts/es6/chart/PieChart'
import { Cell } from 'recharts/es6/component/Cell'
import { ResponsiveContainer } from 'recharts/es6/component/ResponsiveContainer'
import { Tooltip as RechartsTooltip } from 'recharts/es6/component/Tooltip'
import { Pie } from 'recharts/es6/polar/Pie'

import type { TFunction } from '../../i18n'
import {
  chartAxisStroke,
  chartGridStroke,
  chartTooltipStyle,
  smallChartTick,
} from '../../utils/chartConfig'
import { ErrorAlert } from '../ui/ErrorAlert'
import { KPICard } from '../ui/KPICard'

interface EvalTestCase {
  id: string
  question: string
  status: 'pass' | 'fail'
  expected_logical_query: Record<string, unknown>
  got_logical_query: Record<string, unknown>
  confidence?: number
  error_message?: string
}

interface EvalRunTabProps {
  running: boolean
  runEvaluation: () => void
  showDemo: boolean
  runError: string | null
  activeData: {
    total: number
    passed: number
    failed: number
    pass_rate: number
    avg_confidence: number
    test_cases: EvalTestCase[]
  } | null
  pieData: { name: string; value: number; fill: string }[]
  trendData: { date: string; pass_rate: number; pass_rate_pct: number }[]
  t: TFunction
}

function DiffView({
  expected,
  got,
}: {
  expected: Record<string, unknown>
  got: Record<string, unknown>
}) {
  const expectedStr = JSON.stringify(expected, null, 2)
  const gotStr = JSON.stringify(got, null, 2)
  return (
    <div className="diff-view">
      <div className="diff-col">
        <div className="diff-col-header">Expected</div>
        <pre className="diff-pre">{expectedStr}</pre>
      </div>
      <div className="diff-col">
        <div className="diff-col-header">Actual</div>
        <pre className="diff-pre">{gotStr}</pre>
      </div>
    </div>
  )
}

function TestCaseRow({ tc, t }: { tc: EvalTestCase; t: TFunction }) {
  const [open, setOpen] = useState(false)
  const isFail = tc.status === 'fail'

  return (
    <>
      <tr>
        <td className="eval-tc-id">{tc.id}</td>
        <td className="eval-tc-question">{tc.question}</td>
        <td>
          <span className={`status-badge ${isFail ? 'error' : 'success'}`}>
            {isFail ? t('evaluation.status_fail_short') : t('evaluation.status_pass_short')}
          </span>
        </td>
        <td className="eval-tc-confidence">
          {tc.confidence !== undefined
            ? `${Math.round(tc.confidence * 100)}%`
            : t('common.em_dash')}
        </td>
        <td>
          {isFail && tc.error_message && (
            <span className="eval-error-hint">{tc.error_message}</span>
          )}
          <button className="btn btn-sm btn-ghost" onClick={() => setOpen(!open)}>
            {open ? t('evaluation.hide_diff') : t('evaluation.show_diff')}
          </button>
        </td>
      </tr>
      {open && (
        <tr>
          <td colSpan={5}>
            <DiffView expected={tc.expected_logical_query} got={tc.got_logical_query} />
          </td>
        </tr>
      )}
    </>
  )
}

export function EvalRunTab({
  running,
  runEvaluation,
  showDemo,
  runError,
  activeData,
  pieData,
  trendData,
  t,
}: EvalRunTabProps) {
  return (
    <>
      <div className="card">
        <div className="card-header-row">
          <h2>{t('evaluation.panel_title')}</h2>
          <div style={{ display: 'flex', gap: '0.5rem' }}>
            <button className="btn btn-sm btn-primary" onClick={runEvaluation} disabled={running}>
              {running ? t('evaluation.running') : t('evaluation.run_submit')}
            </button>
          </div>
        </div>

        {showDemo && <div className="demo-banner">{t('evaluation.demo_banner')}</div>}

        <ErrorAlert error={runError} />
      </div>

      {activeData && (
        <>
          {/* KPI Cards */}
          <div className="kpi-row">
            <KPICard
              label={t('evaluation.kpi_total_scenarios')}
              value={activeData.total}
              color="var(--accent)"
            />
            <KPICard
              label={t('evaluation.kpi_pass_rate')}
              value={`${Math.round(activeData.pass_rate * 100)}%`}
              color="var(--success)"
            />
            <KPICard
              label={t('evaluation.kpi_failed_scenarios')}
              value={activeData.failed}
              color="var(--error)"
            />
            <KPICard
              label={t('evaluation.kpi_avg_confidence')}
              value={`${Math.round(activeData.avg_confidence * 100)}%`}
              color="var(--warning)"
            />
          </div>

          {/* Charts */}
          <div className="eval-charts-row">
            <div className="card">
              <h3>{t('evaluation.chart_pass_distribution')}</h3>
              <div className="chart-container" style={{ height: 240 }}>
                <ResponsiveContainer width="100%" height="100%">
                  <PieChart>
                    <Pie
                      data={pieData}
                      dataKey="value"
                      nameKey="name"
                      cx="50%"
                      cy="50%"
                      outerRadius={80}
                      label={({ name, value }) => `${name}: ${value}`}
                    >
                      {pieData.map((entry, i) => (
                        <Cell key={i} fill={entry.fill} />
                      ))}
                    </Pie>
                    <RechartsTooltip contentStyle={chartTooltipStyle} />
                  </PieChart>
                </ResponsiveContainer>
              </div>
            </div>

            <div className="card">
              <h3>{t('evaluation.chart_accuracy_trend')}</h3>
              {trendData.length > 0 ? (
                <div className="chart-container" style={{ height: 240 }}>
                  <ResponsiveContainer width="100%" height="100%">
                    <BarChart data={trendData}>
                      <CartesianGrid strokeDasharray="3 3" stroke={chartGridStroke} />
                      <XAxis dataKey="date" stroke={chartAxisStroke} tick={smallChartTick} />
                      <YAxis stroke={chartAxisStroke} domain={[0, 100]} tick={smallChartTick} />
                      <RechartsTooltip
                        contentStyle={chartTooltipStyle}
                        formatter={(v) => {
                          if (typeof v === 'number' || typeof v === 'string') {
                            return `${v}%`
                          }
                          return ''
                        }}
                      />
                      <Bar dataKey="pass_rate_pct" fill="#3b82f6" radius={[4, 4, 0, 0]} />
                    </BarChart>
                  </ResponsiveContainer>
                </div>
              ) : (
                <p className="eval-empty-chart">{t('evaluation.no_trend_data')}</p>
              )}
            </div>
          </div>

          {/* Test Cases Table */}
          <div className="card">
            <h3>{t('evaluation.test_cases_title')}</h3>
            <table className="results-table eval-results-table">
              <thead>
                <tr>
                  <th style={{ width: '80px' }}>ID</th>
                  <th>{t('evaluation.col_question')}</th>
                  <th style={{ width: '100px' }}>{t('evaluation.col_status')}</th>
                  <th style={{ width: '110px' }}>{t('evaluation.col_confidence')}</th>
                  <th style={{ width: '140px' }}></th>
                </tr>
              </thead>
              <tbody>
                {activeData.test_cases.map((tc) => (
                  <TestCaseRow key={tc.id} tc={tc} t={t} />
                ))}
              </tbody>
            </table>
          </div>
        </>
      )}
    </>
  )
}
