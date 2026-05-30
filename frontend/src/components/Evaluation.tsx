import { useEffect, useMemo, useState } from 'react'
import '../styles/evaluation.css'
import { useAdminApi } from '../hooks/useApi'
import { useQueryParam } from '../hooks/useQueryParam'
import { localeLanguageTag, useI18n, useT } from '../i18n'
import { ErrorAlert } from './ui/ErrorAlert'
import type { EvalRunSummary, EvalRunDetail, RegressionReport } from '../types/ai'
import { EvalRunTab } from './evaluation/EvalRunTab'
import { EvalHistoryTab } from './evaluation/EvalHistoryTab'
import { EvalRegressionTab } from './evaluation/EvalRegressionTab'

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
        <EvalRunTab
          running={running}
          runEvaluation={runEvaluation}
          showDemo={showDemo}
          runError={runError}
          activeData={activeData}
          pieData={pieData}
          trendData={trendData}
          t={t}
        />
      )}

      {/* ─── TAB: History ────────────────────────────────────── */}
      {activeTab === 'history' && (
        <EvalHistoryTab
          runHistory={runHistory}
          selectedRun={selectedRun}
          setSelectedRun={setSelectedRun}
          loadRunDetail={loadRunDetail}
          localeTag={localeTag}
          t={t}
        />
      )}

      {/* ─── TAB: Regression ─────────────────────────────────── */}
      {activeTab === 'regression' && (
        <EvalRegressionTab
          runHistory={runHistory}
          baselineId={baselineId}
          currentId={currentId}
          setBaselineId={setBaselineId}
          setCurrentId={setCurrentId}
          regression={regression}
          regressionLoading={regressionLoading}
          runRegression={runRegression}
          configured={adminApi.configured}
          localeTag={localeTag}
          t={t}
        />
      )}
    </div>
  )
}
