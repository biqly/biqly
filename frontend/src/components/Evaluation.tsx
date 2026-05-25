import { useEffect, useMemo, useState } from 'react'
import '../styles/evaluation.css'
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  PieChart,
  Pie,
  Cell,
} from 'recharts'
import { useAdminApi } from '../hooks/useApi'
import { useQueryParam } from '../hooks/useQueryParam'
import { localeLanguageTag, useI18n, useT } from '../i18n'
import { chartAxisStroke, chartGridStroke, chartTooltipStyle, smallChartTick } from '../utils/chartConfig'
import { getRateColor } from '../utils/formatters'
import { ErrorAlert } from './ui/ErrorAlert'
import { KPICard } from './ui/KPICard'
import { Select } from './ui/Select'
import type { EvalRunSummary, EvalRunDetail, RegressionReport } from '../types/ai'

// ─── Types ─────────────────────────────────────────────────────────

interface EvalTestCase {
  id: string
  question: string
  status: 'pass' | 'fail'
  expected_logical_query: Record<string, unknown>
  got_logical_query: Record<string, unknown>
  confidence?: number
  error_message?: string
}

interface EvalRunResponse {
  total: number
  passed: number
  failed: number
  pass_rate: number
  avg_confidence: number
  test_cases: EvalTestCase[]
  accuracy_trend?: { date: string; pass_rate: number }[]
}

// Demo data shown when the API is not yet available
const DEMO_DATA: EvalRunResponse = {
  total: 42,
  passed: 34,
  failed: 8,
  pass_rate: 0.81,
  avg_confidence: 0.76,
  test_cases: [
    {
      id: 'TC-001',
      question: 'Show total revenue by country for January 2026',
      status: 'pass',
      expected_logical_query: { select: [{ type: 'dimension', name: 'country' }, { type: 'metric', name: 'revenue', aggregation: 'SUM' }], filters: [{ field: 'order_date', operator: 'between', value: ['2026-01-01', '2026-01-31'] }], group_by: [{ field: 'country' }] },
      got_logical_query: { select: [{ type: 'dimension', name: 'country' }, { type: 'metric', name: 'revenue', aggregation: 'SUM' }], filters: [{ field: 'order_date', operator: 'between', value: ['2026-01-01', '2026-01-31'] }], group_by: [{ field: 'country' }] },
      confidence: 0.95,
    },
    {
      id: 'TC-002',
      question: 'List top 5 customers by order count',
      status: 'pass',
      expected_logical_query: { select: [{ type: 'dimension', name: 'customer_name' }, { type: 'metric', name: 'order_id', aggregation: 'COUNT' }], order_by: [{ field: 'order_id', direction: 'desc' }], limit: 5, group_by: [{ field: 'customer_name' }] },
      got_logical_query: { select: [{ type: 'dimension', name: 'customer_name' }, { type: 'metric', name: 'order_id', aggregation: 'COUNT' }], order_by: [{ field: 'order_id', direction: 'desc' }], limit: 5, group_by: [{ field: 'customer_name' }] },
      confidence: 0.88,
    },
    {
      id: 'TC-003',
      question: 'Find orders where shipping cost exceeds $100',
      status: 'fail',
      expected_logical_query: { select: [{ type: 'dimension', name: 'order_id' }, { type: 'dimension', name: 'shipping_cost' }], filters: [{ field: 'shipping_cost', operator: 'gt', value: 100 }] },
      got_logical_query: { select: [{ type: 'dimension', name: 'order_id' }], filters: [{ field: 'shipping_cost', operator: 'gte', value: 100 }] },
      confidence: 0.42,
      error_message: 'Missing select column "shipping_cost"; operator mismatch (gte vs gt)',
    },
    {
      id: 'TC-004',
      question: 'Average order value per region for Q4 2025',
      status: 'pass',
      expected_logical_query: { select: [{ type: 'dimension', name: 'region' }, { type: 'metric', name: 'order_value', aggregation: 'AVG' }], filters: [{ field: 'order_date', operator: 'between', value: ['2025-10-01', '2025-12-31'] }], group_by: [{ field: 'region' }] },
      got_logical_query: { select: [{ type: 'dimension', name: 'region' }, { type: 'metric', name: 'order_value', aggregation: 'AVG' }], filters: [{ field: 'order_date', operator: 'between', value: ['2025-10-01', '2025-12-31'] }], group_by: [{ field: 'region' }] },
      confidence: 0.91,
    },
    {
      id: 'TC-005',
      question: 'Count distinct products sold per salesperson',
      status: 'fail',
      expected_logical_query: { select: [{ type: 'dimension', name: 'salesperson' }, { type: 'metric', name: 'product_id', aggregation: 'COUNT_DISTINCT' }], group_by: [{ field: 'salesperson' }] },
      got_logical_query: { select: [{ type: 'dimension', name: 'salesperson' }, { type: 'metric', name: 'product_id', aggregation: 'COUNT' }], group_by: [{ field: 'salesperson' }] },
      confidence: 0.55,
      error_message: 'Used COUNT instead of COUNT_DISTINCT',
    },
  ],
  accuracy_trend: [
    { date: '2026-05-04', pass_rate: 0.72 },
    { date: '2026-05-05', pass_rate: 0.75 },
    { date: '2026-05-06', pass_rate: 0.70 },
    { date: '2026-05-07', pass_rate: 0.78 },
    { date: '2026-05-08', pass_rate: 0.80 },
    { date: '2026-05-09', pass_rate: 0.79 },
    { date: '2026-05-10', pass_rate: 0.81 },
  ],
}

// ─── Sub-components ────────────────────────────────────────────────

function DiffView({ expected, got }: { expected: Record<string, unknown>; got: Record<string, unknown> }) {
  const t = useT()
  const expectedStr = JSON.stringify(expected, null, 2)
  const gotStr = JSON.stringify(got, null, 2)
  return (
    <div className="diff-view">
      <div className="diff-col">
        <div className="diff-col-header">{t('evaluation.diff_expected')}</div>
        <pre className="diff-pre">{expectedStr}</pre>
      </div>
      <div className="diff-col">
        <div className="diff-col-header">{t('evaluation.diff_actual')}</div>
        <pre className="diff-pre">{gotStr}</pre>
      </div>
    </div>
  )
}

function TestCaseRow({ tc }: { tc: EvalTestCase }) {
  const t = useT()
  const [open, setOpen] = useState(false)
  const isFail = tc.status === 'fail'

  return (
    <>
      <tr>
        <td className="eval-tc-id">{tc.id}</td>
        <td className="eval-tc-question">{tc.question}</td>
        <td>
          <span className={`status-badge ${isFail ? 'error' : 'success'}`}>{isFail ? t('evaluation.status_fail_short') : t('evaluation.status_pass_short')}</span>
        </td>
        <td className="eval-tc-confidence">{tc.confidence !== undefined ? `${Math.round(tc.confidence * 100)}%` : t('common.em_dash')}</td>
        <td>
          {isFail && tc.error_message && <span className="eval-error-hint">{tc.error_message}</span>}
          <button className="btn btn-sm btn-ghost" onClick={() => setOpen(!open)}>{open ? t('evaluation.hide_diff') : t('evaluation.show_diff')}</button>
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

// ─── Main Component ────────────────────────────────────────────────

export default function Evaluation() {
  const t = useT()
  const { locale } = useI18n()
  const localeTag = localeLanguageTag(locale)
  const adminApi = useAdminApi()

  const [evalData, setEvalData] = useState<EvalRunResponse | null>(null)
  const [running, setRunning] = useState(false)
  const [showDemo, setShowDemo] = useState(false)
  const [runError, setRunError] = useState<string | null>(null)

  // Eval runs history
  const [tabParam, setTabParam] = useQueryParam('tab')
  const initialTab: 'run' | 'history' | 'regression' =
    tabParam === 'history' || tabParam === 'regression' ? tabParam : 'run'
  const [activeTab, setActiveTab] = useState<'run' | 'history' | 'regression'>(initialTab)
  const [runHistory, setRunHistory] = useState<EvalRunSummary[]>([])
  const [selectedRun, setSelectedRun] = useState<EvalRunDetail | null>(null)
  const [historyLoaded, setHistoryLoaded] = useState(false)

  // Regression
  const [baselineParam, setBaselineParam] = useQueryParam('baseline')
  const [currentParam, setCurrentParam] = useQueryParam('current')
  const [baselineId, setBaselineId] = useState(baselineParam)
  const [currentId, setCurrentId] = useState(currentParam)
  const [regression, setRegression] = useState<RegressionReport | null>(null)
  const [regressionLoading, setRegressionLoading] = useState(false)

  useEffect(() => {
    setTabParam(activeTab === 'run' ? '' : activeTab)
  }, [activeTab, setTabParam])
  useEffect(() => {
    setBaselineParam(baselineId)
  }, [baselineId, setBaselineParam])
  useEffect(() => {
    setCurrentParam(currentId)
  }, [currentId, setCurrentParam])

  const loadRunHistory = async () => {
    if (!adminApi.configured || historyLoaded) return
    const data = await adminApi.get<EvalRunSummary[]>('/api/ai/eval/runs')
    if (data) setRunHistory(data)
    setHistoryLoaded(true)
  }

  useEffect(() => {
    if (activeTab === 'history' || activeTab === 'regression') {
      void loadRunHistory()
    }
  }, [activeTab])

  useEffect(() => {
    if (selectedRun) return // don't reload if already selected
    // Auto-select latest run when on history tab
    if (activeTab === 'history' && runHistory.length > 0 && !selectedRun) {
      // Show list, don't auto-select
    }
  }, [activeTab, runHistory, selectedRun])

  const loadRunDetail = async (runId: string) => {
    if (!adminApi.configured) return
    const data = await adminApi.get<EvalRunDetail>(`/api/ai/eval/runs/${runId}`)
    if (data) setSelectedRun(data)
  }

  const runRegression = async () => {
    if (!baselineId || !currentId || !adminApi.configured) return
    setRegressionLoading(true)
    setRegression(null)
    const data = await adminApi.get<RegressionReport>(`/api/ai/eval/regression?baseline=${baselineId}&current=${currentId}`)
    if (data) setRegression(data)
    setRegressionLoading(false)
  }

  // Derive pie chart data from current eval data
  const pieData = useMemo(() => {
    if (!evalData) return []
    return [
      { name: t('evaluation.pie_passed'), value: evalData.passed, fill: '#22c55e' },
      { name: t('evaluation.pie_failed'), value: evalData.failed, fill: '#ef4444' },
    ]
  }, [evalData, t])

  // Derive trend data
  const trendData = useMemo(() => {
    if (!evalData?.accuracy_trend) return []
    return evalData.accuracy_trend.map((d) => ({
      ...d,
      pass_rate_pct: Math.round(d.pass_rate * 100),
    }))
  }, [evalData])

  const runEvaluation = async () => {
    if (!adminApi.configured) {
      setRunError(t('evaluation.admin_key_missing_run'))
      return
    }
    setRunning(true)
    setRunError(null)
    setShowDemo(false)
    try {
      const res = await adminApi.postData<EvalRunResponse>('/api/ai/eval/run', {})
      if (res) setEvalData(res)
      else setEvalData(null)
    } catch {
      setEvalData(DEMO_DATA)
      setShowDemo(true)
    } finally {
      setRunning(false)
    }
  }

  const activeData = evalData ?? (showDemo ? DEMO_DATA : null)

  return (
    <div className="evaluation-layout">
      {/* Tabs */}
      <div className="page-tabs" role="tablist" aria-label={t('evaluation.tabs_aria')}>
        {([
          { key: 'run' as const, label: t('evaluation.tab_run') },
          { key: 'history' as const, label: t('evaluation.tab_history', { count: runHistory.length }) },
          { key: 'regression' as const, label: t('evaluation.tab_regression') },
        ]).map((tab) => {
          const isActive = activeTab === tab.key
          return (
            <button
              key={tab.key}
              type="button"
              role="tab"
              aria-selected={isActive}
              className={`btn btn-sm${isActive ? '' : ' btn-ghost'}`}
              onClick={() => { setActiveTab(tab.key); setSelectedRun(null); setRegression(null) }}
            >
              {tab.label}
            </button>
          )
        })}
      </div>

      {!adminApi.configured && (
        <ErrorAlert error={t('evaluation.admin_key_missing_ui')}>
          {' '}
          <a
            href="/settings"
            onClick={(e) => {
              e.preventDefault()
              window.history.pushState(null, '', '/settings')
              window.dispatchEvent(new PopStateEvent('popstate'))
            }}
          >
            {t('evaluation.admin_key_open_settings')}
          </a>
        </ErrorAlert>
      )}

      {/* ─── TAB: Run Evaluation ─────────────────────────────── */}
      {activeTab === 'run' && (
      <>
        <div className="card">
        <div className="card-header-row">
          <h2>{t('evaluation.panel_title')}</h2>
          <div style={{ display: 'flex', gap: '0.5rem' }}>
            <button className="btn btn-sm btn-primary" onClick={runEvaluation} disabled={!adminApi.configured || running}>
              {running ? t('evaluation.running') : t('evaluation.run_submit')}
            </button>
          </div>
        </div>

        {showDemo && (
          <div className="demo-banner">
            {t('evaluation.demo_banner')}
          </div>
        )}

        <ErrorAlert error={runError} />
      </div>

      {activeData && (
        <>
          {/* KPI Cards */}
          <div className="kpi-row">
            <KPICard label={t('evaluation.kpi_total_scenarios')} value={activeData.total} color="var(--accent)" />
            <KPICard label={t('evaluation.kpi_pass_rate')} value={`${Math.round(activeData.pass_rate * 100)}%`} color="var(--success)" />
            <KPICard label={t('evaluation.kpi_failed_scenarios')} value={activeData.failed} color="var(--error)" />
            <KPICard label={t('evaluation.kpi_avg_confidence')} value={`${Math.round(activeData.avg_confidence * 100)}%`} color="var(--warning)" />
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
                    <Tooltip contentStyle={chartTooltipStyle} />
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
                      <Tooltip
                        contentStyle={chartTooltipStyle}
                        formatter={(v: number) => `${v}%`}
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
                  <th>{t('evaluation.col_id')}</th>
                  <th>{t('evaluation.col_question')}</th>
                  <th>{t('evaluation.col_status')}</th>
                  <th>{t('evaluation.col_confidence')}</th>
                  <th>{t('evaluation.col_diff')}</th>
                </tr>
              </thead>
              <tbody>
                {activeData.test_cases.map((tc) => (
                  <TestCaseRow key={tc.id} tc={tc} />
                ))}
              </tbody>
            </table>
          </div>
        </>
      )}

      {!activeData && (
        <div className="card empty-state">
          <h2>{t('evaluation.empty_no_results_title')}</h2>
          <p>{t('evaluation.empty_no_results_hint')}</p>
        </div>
      )}
      </>
      )}

      {/* ─── TAB: History ────────────────────────────────────── */}
      {activeTab === 'history' && !selectedRun && (
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
                    <td style={{
                      color: getRateColor(r.pass_rate * 100),
                      fontWeight: 700,
                    }}>
                      {(r.pass_rate * 100).toFixed(0)}%
                    </td>
                    <td>v{r.prompt_template_bundle_version ?? 0}</td>
                    <td>
                      <button className="btn btn-sm" onClick={() => loadRunDetail(r.run_id)}>{t('evaluation.detail_btn')}</button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      )}

      {activeTab === 'history' && selectedRun && (
        <>
          <div style={{ marginBottom: '0.5rem' }}>
            <button className="btn-back" onClick={() => setSelectedRun(null)}>{t('evaluation.back')}</button>
          </div>
          <div className="kpi-row">
            <KPICard label={t('evaluation.detail_kpi_total')} value={selectedRun.summary.total_cases} color="var(--accent)" />
            <KPICard label={t('evaluation.detail_kpi_passed')} value={selectedRun.summary.passed} color="var(--success)" />
            <KPICard label={t('evaluation.detail_kpi_failed')} value={selectedRun.summary.failed} color="var(--error)" />
            <KPICard label={t('evaluation.detail_kpi_rate')} value={`${(selectedRun.summary.pass_rate * 100).toFixed(0)}%`} color="var(--warning)" />
          </div>
          <div className="card">
            <h3>{t('evaluation.detail_cases_title', {
              model: selectedRun.summary.model,
              date: new Date(selectedRun.summary.completed_at).toLocaleString(localeTag),
            })}</h3>
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
                      <span className={`status-badge ${tc.match ? 'success' : 'error'}`}>{tc.match ? t('evaluation.match_yes') : t('evaluation.match_no')}</span>
                    </td>
                    <td style={{ fontSize: '0.8rem', maxWidth: 300, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{tc.reason || t('common.em_dash')}</td>
                    <td>{(tc.confidence * 100).toFixed(0)}%</td>
                    <td>{t('evaluation.latency_ms', { ms: tc.latency_ms })}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </>
      )}

      {/* ─── TAB: Regression ─────────────────────────────────── */}
      {activeTab === 'regression' && (
        <>
          <div className="card">
            <h3>{t('evaluation.regression_title')}</h3>
            <div style={{ display: 'flex', gap: '1rem', alignItems: 'flex-end', flexWrap: 'wrap' }}>
              <div style={{ flex: 1, minWidth: '14rem' }}>
                <label style={{ display: 'block', fontSize: '0.8rem', color: 'var(--text-secondary)', marginBottom: '0.3rem' }}>{t('evaluation.baseline_label')}</label>
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
                <label style={{ display: 'block', fontSize: '0.8rem', color: 'var(--text-secondary)', marginBottom: '0.3rem' }}>{t('evaluation.current_label')}</label>
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
              <button className="btn btn-sm btn-primary" onClick={runRegression} disabled={!adminApi.configured || !baselineId || !currentId || regressionLoading}>
                {regressionLoading ? t('evaluation.comparing') : t('evaluation.compare')}
              </button>
            </div>
          </div>

          {regression && (
            <>
              <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))', gap: '1rem' }}>
                <div className="card" style={{ borderColor: 'var(--error)', marginBottom: 0 }}>
                  <p style={{ color: 'var(--text-secondary)', fontSize: '0.8rem' }}>{t('evaluation.reg_new_failures')}</p>
                  <p style={{ fontSize: '2rem', fontWeight: 700, color: 'var(--error)' }}>{regression.new_failures.length}</p>
                </div>
                <div className="card" style={{ borderColor: 'var(--success)', marginBottom: 0 }}>
                  <p style={{ color: 'var(--text-secondary)', fontSize: '0.8rem' }}>{t('evaluation.reg_fixed')}</p>
                  <p style={{ fontSize: '2rem', fontWeight: 700, color: 'var(--success)' }}>{regression.fixed_failures.length}</p>
                </div>
                <div className="card" style={{ borderColor: 'var(--warning)', marginBottom: 0 }}>
                  <p style={{ color: 'var(--text-secondary)', fontSize: '0.8rem' }}>{t('evaluation.reg_changed')}</p>
                  <p style={{ fontSize: '2rem', fontWeight: 700, color: 'var(--warning)' }}>{regression.changed_cases.length}</p>
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
                          <td><span className="status-badge success">{t('evaluation.match_yes')}</span></td>
                          <td><span className="status-badge error">{t('evaluation.match_no')}</span></td>
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
                          <td><span className="status-badge error">{t('evaluation.match_no')}</span></td>
                          <td><span className="status-badge success">{t('evaluation.match_yes')}</span></td>
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
                          <td><span className={`status-badge ${c.was_match ? 'success' : 'error'}`}>{c.was_match ? t('evaluation.match_yes') : t('evaluation.match_no')}</span></td>
                          <td><span className={`status-badge ${c.is_match ? 'success' : 'error'}`}>{c.is_match ? t('evaluation.match_yes') : t('evaluation.match_no')}</span></td>
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
      )}
    </div>
  )
}
