import { useEffect, useMemo, useState } from 'react'
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
import useStreamingApi from '../hooks/useStreamingApi'
import { useAdminApi } from '../hooks/useApi'
import { useQueryParam } from '../hooks/useQueryParam'
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
        <div className="diff-col-header">Beklenen</div>
        <pre className="diff-pre">{expectedStr}</pre>
      </div>
      <div className="diff-col">
        <div className="diff-col-header">Üretilen</div>
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
          <span className={`status-badge ${isFail ? 'error' : 'success'}`}>{isFail ? 'kaldı' : 'geçti'}</span>
        </td>
        <td className="eval-tc-confidence">{tc.confidence !== undefined ? `${Math.round(tc.confidence * 100)}%` : '—'}</td>
        <td>
          {isFail && tc.error_message && <span className="eval-error-hint">{tc.error_message}</span>}
          <button className="btn btn-sm btn-ghost" onClick={() => setOpen(!open)}>{open ? 'Farkı gizle' : 'Farkı göster'}</button>
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
  const streaming = useStreamingApi({ typingSpeed: 4 })
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

  useEffect(() => {
    adminApi.get<EvalRunSummary[]>('/api/eval/runs').then((data) => {
      if (data) setRunHistory(data)
    })
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    if (selectedRun) return // don't reload if already selected
    // Auto-select latest run when on history tab
    if (activeTab === 'history' && runHistory.length > 0 && !selectedRun) {
      // Show list, don't auto-select
    }
  }, [activeTab, runHistory, selectedRun])

  const loadRunDetail = async (runId: string) => {
    const data = await adminApi.get<EvalRunDetail>(`/api/eval/runs/${runId}`)
    if (data) setSelectedRun(data)
  }

  const runRegression = async () => {
    if (!baselineId || !currentId) return
    setRegressionLoading(true)
    setRegression(null)
    const data = await adminApi.get<RegressionReport>(`/api/eval/regression?baseline=${baselineId}&current=${currentId}`)
    if (data) setRegression(data)
    setRegressionLoading(false)
  }

  // Derive pie chart data from current eval data
  const pieData = useMemo(() => {
    if (!evalData) return []
    return [
      { name: 'Geçen', value: evalData.passed, fill: '#22c55e' },
      { name: 'Kalan', value: evalData.failed, fill: '#ef4444' },
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
    setRunError(null)
    setShowDemo(false)
    try {
      const res = await fetch('/api/ai/eval/run', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: '{}',
      })
      const text = await res.text()
      const payload = (text ? JSON.parse(text) : null) as EvalRunResponse | { error?: string } | null
      if (!res.ok) {
        setEvalData(null)
        const msg =
          payload && typeof payload === 'object' && 'error' in payload && typeof payload.error === 'string'
            ? payload.error
            : `HTTP ${res.status}`
        setRunError(msg)
        return
      }
      setEvalData(payload as EvalRunResponse)
    } catch {
      setEvalData(DEMO_DATA)
      setShowDemo(true)
    } finally {
      setRunning(false)
    }
  }

  const runStreamEvaluation = () => {
    // Alternative: use streaming endpoint if available
    streaming.start('/api/ai/eval/run/stream', {})
  }

  const activeData = evalData ?? (showDemo ? DEMO_DATA : null)

  return (
    <div className="evaluation-layout">
      {/* Tabs */}
      <div className="page-tabs" role="tablist" aria-label="Değerlendirme sekmeleri">
        {([
          { key: 'run' as const, label: '🧪 Değerlendirme Çalıştır' },
          { key: 'history' as const, label: `📋 Geçmiş (${runHistory.length})` },
          { key: 'regression' as const, label: '📉 Regresyon' },
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

      {/* ─── TAB: Run Evaluation ─────────────────────────────── */}
      {activeTab === 'run' && (
      <>
      <div className="card">
        <div className="card-header-row">
          <h2>Değerlendirme Paneli</h2>
          <div style={{ display: 'flex', gap: '0.5rem' }}>
            <button className="btn btn-sm" onClick={runStreamEvaluation} disabled={running || streaming.loading}>
              {streaming.loading ? 'Akış…' : 'Çalıştır (akış)'}
            </button>
            <button className="btn btn-sm btn-primary" onClick={runEvaluation} disabled={running}>
              {running ? 'Çalışıyor…' : 'Değerlendirmeyi çalıştır'}
            </button>
          </div>
        </div>

        {showDemo && (
          <div className="demo-banner">
            Sunucuya ulaşılamadı — örnek veriler gösteriliyor. API hazır olduğunda «Değerlendirmeyi çalıştır»a tekrar basın.
          </div>
        )}

        {runError && <div className="error">{runError}</div>}
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
            <KPICard label="Toplam senaryo" value={activeData.total} color="var(--accent)" />
            <KPICard label="Geçme oranı" value={`${Math.round(activeData.pass_rate * 100)}%`} color="var(--success)" />
            <KPICard label="Kalan senaryo" value={activeData.failed} color="var(--error)" />
            <KPICard label="Ort. güven" value={`${Math.round(activeData.avg_confidence * 100)}%`} color="var(--warning)" />
          </div>

          {/* Charts */}
          <div className="eval-charts-row">
            <div className="card">
              <h3>Geçme oranı</h3>
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
              <h3>Doğruluk eğilimi</h3>
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
                <p className="eval-empty-chart">Eğilim verisi yok.</p>
              )}
            </div>
          </div>

          {/* Test Cases Table */}
          <div className="card">
            <h3>Test senaryoları</h3>
            <table className="results-table eval-results-table">
              <thead>
                <tr>
                  <th>Kimlik</th>
                  <th>Soru</th>
                  <th>Durum</th>
                  <th>Güven</th>
                  <th>Fark</th>
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
          <h2>Henüz değerlendirme sonucu yok</h2>
          <p>AI metinden-SQL hattına karşı bir test çalıştırmak için «Değerlendirmeyi çalıştır»a tıklayın.</p>
        </div>
      )}
      </>
      )}

      {/* ─── TAB: History ────────────────────────────────────── */}
      {activeTab === 'history' && !selectedRun && (
        <div className="card">
          <h3>Geçmiş Değerlendirmeler</h3>
          {runHistory.length === 0 ? (
            <p className="eval-empty-chart">Henüz geçmiş değerlendirme yok.</p>
          ) : (
            <table className="results-table">
              <thead>
                <tr>
                  <th>Tarih</th>
                  <th>Model</th>
                  <th>Toplam</th>
                  <th>Geçen</th>
                  <th>Kalan</th>
                  <th>Başarı %</th>
                  <th>Detay</th>
                </tr>
              </thead>
              <tbody>
                {runHistory.map((r) => (
                  <tr key={r.run_id}>
                    <td>{new Date(r.completed_at).toLocaleString('tr-TR')}</td>
                    <td>{r.model}</td>
                    <td>{r.total_cases}</td>
                    <td style={{ color: 'var(--success)' }}>{r.passed}</td>
                    <td style={{ color: 'var(--error)' }}>{r.failed}</td>
                    <td style={{
                      color: r.pass_rate >= 80 ? 'var(--success)' : r.pass_rate >= 50 ? 'var(--warning)' : 'var(--error)',
                      fontWeight: 700,
                    }}>
                      {(r.pass_rate * 100).toFixed(0)}%
                    </td>
                    <td>
                      <button className="btn btn-sm" onClick={() => loadRunDetail(r.run_id)}>Detay</button>
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
            <button className="btn btn-sm btn-ghost" onClick={() => setSelectedRun(null)}>← Geri</button>
          </div>
          <div className="kpi-row">
            <div className="kpi-card" style={{ borderColor: 'var(--accent)' }}>
              <div className="kpi-label">Toplam</div>
              <div className="kpi-value">{selectedRun.summary.total_cases}</div>
            </div>
            <div className="kpi-card" style={{ borderColor: 'var(--success)' }}>
              <div className="kpi-label">Geçen</div>
              <div className="kpi-value" style={{ color: 'var(--success)' }}>{selectedRun.summary.passed}</div>
            </div>
            <div className="kpi-card" style={{ borderColor: 'var(--error)' }}>
              <div className="kpi-label">Kalan</div>
              <div className="kpi-value" style={{ color: 'var(--error)' }}>{selectedRun.summary.failed}</div>
            </div>
            <div className="kpi-card" style={{ borderColor: 'var(--warning)' }}>
              <div className="kpi-label">Başarı %</div>
              <div className="kpi-value" style={{ color: 'var(--warning)' }}>{(selectedRun.summary.pass_rate * 100).toFixed(0)}%</div>
            </div>
          </div>
          <div className="card">
            <h3>Test Senaryoları — {selectedRun.summary.model} ({new Date(selectedRun.summary.completed_at).toLocaleString('tr-TR')})</h3>
            <table className="results-table eval-results-table">
              <thead>
                <tr>
                  <th>Senaryo</th>
                  <th>Soru</th>
                  <th>Durum</th>
                  <th>Neden</th>
                  <th>Güven</th>
                  <th>Gecikme</th>
                </tr>
              </thead>
              <tbody>
                {selectedRun.test_cases.map((tc) => (
                  <tr key={tc.case_id}>
                    <td>{tc.case_id}</td>
                    <td>{tc.question}</td>
                    <td>
                      <span className={`status-badge ${tc.match ? 'success' : 'error'}`}>{tc.match ? 'eşleşti' : 'eşleşmedi'}</span>
                    </td>
                    <td style={{ fontSize: '0.8rem', maxWidth: 300, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{tc.reason || '—'}</td>
                    <td>{(tc.confidence * 100).toFixed(0)}%</td>
                    <td>{tc.latency_ms}ms</td>
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
            <h3>Regresyon Karşılaştırması</h3>
            <div style={{ display: 'flex', gap: '1rem', alignItems: 'flex-end', flexWrap: 'wrap' }}>
              <div style={{ flex: 1, minWidth: '14rem' }}>
                <label style={{ display: 'block', fontSize: '0.8rem', color: 'var(--text-secondary)', marginBottom: '0.3rem' }}>Temel (Baseline)</label>
                <Select
                  value={baselineId}
                  onChange={setBaselineId}
                  placeholder="Seçiniz"
                  header="Çalıştırma geçmişi"
                  options={runHistory.map((r) => ({
                    value: r.run_id,
                    label: `${r.model} — ${(r.pass_rate * 100).toFixed(0)}%`,
                    hint: new Date(r.completed_at).toLocaleString('tr-TR'),
                  }))}
                />
              </div>
              <div style={{ flex: 1, minWidth: '14rem' }}>
                <label style={{ display: 'block', fontSize: '0.8rem', color: 'var(--text-secondary)', marginBottom: '0.3rem' }}>Karşılaştırılacak (Current)</label>
                <Select
                  value={currentId}
                  onChange={setCurrentId}
                  placeholder="Seçiniz"
                  header="Çalıştırma geçmişi"
                  options={runHistory.map((r) => ({
                    value: r.run_id,
                    label: `${r.model} — ${(r.pass_rate * 100).toFixed(0)}%`,
                    hint: new Date(r.completed_at).toLocaleString('tr-TR'),
                  }))}
                />
              </div>
              <button className="btn btn-sm btn-primary" onClick={runRegression} disabled={!baselineId || !currentId || regressionLoading}>
                {regressionLoading ? 'Karşılaştırılıyor…' : 'Karşılaştır'}
              </button>
            </div>
          </div>

          {regression && (
            <>
              <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))', gap: '1rem' }}>
                <div className="card" style={{ borderColor: 'var(--error)', marginBottom: 0 }}>
                  <p style={{ color: 'var(--text-secondary)', fontSize: '0.8rem' }}>Yeni Başarısızlıklar</p>
                  <p style={{ fontSize: '2rem', fontWeight: 700, color: 'var(--error)' }}>{regression.new_failures.length}</p>
                </div>
                <div className="card" style={{ borderColor: 'var(--success)', marginBottom: 0 }}>
                  <p style={{ color: 'var(--text-secondary)', fontSize: '0.8rem' }}>Düzeltilen</p>
                  <p style={{ fontSize: '2rem', fontWeight: 700, color: 'var(--success)' }}>{regression.fixed_failures.length}</p>
                </div>
                <div className="card" style={{ borderColor: 'var(--warning)', marginBottom: 0 }}>
                  <p style={{ color: 'var(--text-secondary)', fontSize: '0.8rem' }}>Değişen</p>
                  <p style={{ fontSize: '2rem', fontWeight: 700, color: 'var(--warning)' }}>{regression.changed_cases.length}</p>
                </div>
              </div>

              {regression.new_failures.length > 0 && (
                <div className="card">
                  <h3 style={{ color: 'var(--error)' }}>🔴 Yeni Başarısızlıklar</h3>
                  <table className="results-table">
                    <thead><tr><th>Senaryo</th><th>Soru</th><th>Önce</th><th>Sonra</th><th>Neden</th></tr></thead>
                    <tbody>
                      {regression.new_failures.map((c) => (
                        <tr key={c.case_id}>
                          <td>{c.case_id}</td>
                          <td>{c.question}</td>
                          <td><span className="status-badge success">eşleşti</span></td>
                          <td><span className="status-badge error">eşleşmedi</span></td>
                          <td style={{ fontSize: '0.8rem' }}>{c.is_reason}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}

              {regression.fixed_failures.length > 0 && (
                <div className="card">
                  <h3 style={{ color: 'var(--success)' }}>🟢 Düzeltilen Başarısızlıklar</h3>
                  <table className="results-table">
                    <thead><tr><th>Senaryo</th><th>Soru</th><th>Önce</th><th>Sonra</th></tr></thead>
                    <tbody>
                      {regression.fixed_failures.map((c) => (
                        <tr key={c.case_id}>
                          <td>{c.case_id}</td>
                          <td>{c.question}</td>
                          <td><span className="status-badge error">eşleşmedi</span></td>
                          <td><span className="status-badge success">eşleşti</span></td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}

              {regression.changed_cases.length > 0 && (
                <div className="card">
                  <h3 style={{ color: 'var(--warning)' }}>🟡 Değişen Senaryolar</h3>
                  <table className="results-table">
                    <thead><tr><th>Senaryo</th><th>Soru</th><th>Önce</th><th>Sonra</th><th>Önce Neden</th><th>Sonra Neden</th></tr></thead>
                    <tbody>
                      {regression.changed_cases.map((c) => (
                        <tr key={c.case_id}>
                          <td>{c.case_id}</td>
                          <td>{c.question}</td>
                          <td><span className={`status-badge ${c.was_match ? 'success' : 'error'}`}>{c.was_match ? 'eşleşti' : 'eşleşmedi'}</span></td>
                          <td><span className={`status-badge ${c.is_match ? 'success' : 'error'}`}>{c.is_match ? 'eşleşti' : 'eşleşmedi'}</span></td>
                          <td style={{ fontSize: '0.8rem' }}>{c.was_reason || '—'}</td>
                          <td style={{ fontSize: '0.8rem' }}>{c.is_reason || '—'}</td>
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
