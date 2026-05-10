import { useMemo, useState } from 'react'
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
import { useApi } from '../hooks/useApi'
import useStreamingApi from '../hooks/useStreamingApi'

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

const COLORS = ['#3b82f6', '#22c55e', '#f59e0b', '#ef4444']

// ─── Sub-components ────────────────────────────────────────────────

function KPICard({ label, value, color }: { label: string; value: string | number; color: string }) {
  return (
    <div className="kpi-card" style={{ borderColor: color }}>
      <div className="kpi-label">{label}</div>
      <div className="kpi-value" style={{ color }}>{value}</div>
    </div>
  )
}

function DiffView({ expected, got }: { expected: Record<string, unknown>; got: Record<string, unknown> }) {
  const expectedStr = JSON.stringify(expected, null, 2)
  const gotStr = JSON.stringify(got, null, 2)
  return (
    <div className="diff-view">
      <div className="diff-col">
        <div className="diff-col-header">Expected</div>
        <pre className="diff-pre">{expectedStr}</pre>
      </div>
      <div className="diff-col">
        <div className="diff-col-header">Got</div>
        <pre className="diff-pre">{gotStr}</pre>
      </div>
    </div>
  )
}

function TestCaseRow({ tc }: { tc: EvalTestCase }) {
  const [open, setOpen] = useState(false)
  const isFail = tc.status === 'fail'

  return (
    <>
      <tr>
        <td className="eval-tc-id">{tc.id}</td>
        <td className="eval-tc-question">{tc.question}</td>
        <td>
          <span className={`status-badge ${isFail ? 'error' : 'success'}`}>{tc.status}</span>
        </td>
        <td className="eval-tc-confidence">{tc.confidence !== undefined ? `${Math.round(tc.confidence * 100)}%` : '—'}</td>
        <td>
          {isFail && tc.error_message && <span className="eval-error-hint">{tc.error_message}</span>}
          <button className="btn btn-sm btn-ghost" onClick={() => setOpen(!open)}>{open ? 'Hide diff' : 'Show diff'}</button>
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
  const { postData, loading: apiLoading, error: apiError } = useApi()
  const streaming = useStreamingApi({ typingSpeed: 4 })

  const [evalData, setEvalData] = useState<EvalRunResponse | null>(null)
  const [running, setRunning] = useState(false)
  const [showDemo, setShowDemo] = useState(false)

  // Derive pie chart data from current eval data
  const pieData = useMemo(() => {
    if (!evalData) return []
    return [
      { name: 'Passed', value: evalData.passed, fill: '#22c55e' },
      { name: 'Failed', value: evalData.failed, fill: '#ef4444' },
    ]
  }, [evalData])

  // Derive trend data
  const trendData = useMemo(() => {
    if (!evalData?.accuracy_trend) return []
    return evalData.accuracy_trend.map((d) => ({
      ...d,
      pass_rate_pct: Math.round(d.pass_rate * 100),
    }))
  }, [evalData])

  const runEvaluation = async () => {
    setRunning(true)
    const res = await postData<EvalRunResponse>('/api/ai/eval/run', {})
    setRunning(false)

    if (res) {
      setEvalData(res)
      setShowDemo(false)
    } else {
      // API not ready — show demo data
      setEvalData(DEMO_DATA)
      setShowDemo(true)
    }
  }

  const runStreamEvaluation = () => {
    // Alternative: use streaming endpoint if available
    streaming.start('/api/ai/eval/run/stream', {})
  }

  const activeData = evalData ?? (showDemo ? DEMO_DATA : null)

  return (
    <div className="evaluation-layout">
      <div className="card">
        <div className="card-header-row">
          <h2>Evaluation Dashboard</h2>
          <div style={{ display: 'flex', gap: '0.5rem' }}>
            <button className="btn" onClick={runStreamEvaluation} disabled={running || streaming.loading}>
              {streaming.loading ? 'Streaming…' : 'Run (Stream)'}
            </button>
            <button className="btn btn-primary" onClick={runEvaluation} disabled={running || apiLoading}>
              {running || apiLoading ? 'Running…' : 'Run Evaluation'}
            </button>
          </div>
        </div>

        {showDemo && (
          <div className="demo-banner">
            API endpoint not available — showing demo data. Start the backend to run live evaluation.
          </div>
        )}

        {apiError && !showDemo && <div className="error">{apiError}</div>}
        {streaming.error && <div className="error">{streaming.error}</div>}

        {/* Streaming output */}
        {streaming.data && (
          <div className="stream-output">
            <pre>{streaming.data}</pre>
          </div>
        )}
      </div>

      {activeData && (
        <>
          {/* KPI Cards */}
          <div className="kpi-row">
            <KPICard label="Total Cases" value={activeData.total} color="var(--accent)" />
            <KPICard label="Pass Rate" value={`${Math.round(activeData.pass_rate * 100)}%`} color="var(--success)" />
            <KPICard label="Failed Cases" value={activeData.failed} color="var(--error)" />
            <KPICard label="Avg Confidence" value={`${Math.round(activeData.avg_confidence * 100)}%`} color="var(--warning)" />
          </div>

          {/* Charts */}
          <div className="eval-charts-row">
            <div className="card">
              <h3>Pass Rate</h3>
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
                    <Tooltip contentStyle={{ background: '#1e293b', border: '1px solid #475569' }} />
                  </PieChart>
                </ResponsiveContainer>
              </div>
            </div>

            <div className="card">
              <h3>Accuracy Trend</h3>
              {trendData.length > 0 ? (
                <div className="chart-container" style={{ height: 240 }}>
                  <ResponsiveContainer width="100%" height="100%">
                    <BarChart data={trendData}>
                      <CartesianGrid strokeDasharray="3 3" stroke="#475569" />
                      <XAxis dataKey="date" stroke="#94a3b8" tick={{ fontSize: 11 }} />
                      <YAxis stroke="#94a3b8" domain={[0, 100]} tick={{ fontSize: 11 }} />
                      <Tooltip
                        contentStyle={{ background: '#1e293b', border: '1px solid #475569' }}
                        formatter={(v: number) => `${v}%`}
                      />
                      <Bar dataKey="pass_rate_pct" fill="#3b82f6" radius={[4, 4, 0, 0]} />
                    </BarChart>
                  </ResponsiveContainer>
                </div>
              ) : (
                <p className="eval-empty-chart">No trend data available.</p>
              )}
            </div>
          </div>

          {/* Test Cases Table */}
          <div className="card">
            <h3>Test Cases</h3>
            <table className="results-table eval-results-table">
              <thead>
                <tr>
                  <th>ID</th>
                  <th>Question</th>
                  <th>Status</th>
                  <th>Confidence</th>
                  <th>Diff</th>
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
          <h2>No evaluation results yet</h2>
          <p>Click &quot;Run Evaluation&quot; to start a test run against the AI text-to-SQL pipeline.</p>
        </div>
      )}
    </div>
  )
}
