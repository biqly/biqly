import { useEffect, useMemo, useRef, useState, type KeyboardEvent } from 'react'
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  LineChart,
  Line,
  PieChart,
  Pie,
  Cell,
} from 'recharts'
import { useApi } from '../hooks/useApi'
import { useConversation } from '../hooks/useConversation'
import { formatResultCell } from '../utils/resultCellFormat'
import ResultTable from './ResultTable'
import type { AIQueryResponse, TableRoutingCandidate, LogicalQueryCandidate, AIRuntimeSettings } from '../types/ai'
import type { Datasource } from '../types/metadata'

const COLORS = ['#3b82f6', '#22c55e', '#f59e0b', '#ef4444', '#8b5cf6', '#ec4899', '#14b8a6', '#f97316']

/** NL→SQL can be slow with local models (routing, LLM, retries, EXPLAIN). */
const AI_QUERY_TIMEOUT_MS = 300_000

// ─── Sub-components ─────────────────────────────────────────────────

function ConfidenceBar({ value, breakdown }: { value: number; breakdown?: { table_routing: number; llm: number; validation: number } }) {
  const pct = Math.round(value * 100)
  const color = value > 0.8 ? 'var(--success)' : value > 0.5 ? 'var(--warning)' : 'var(--error)'
  return (
    <div className="confidence-section">
      <div className="confidence-header">
        <span>Confidence</span>
        <span style={{ color, fontWeight: 600 }}>{pct}%</span>
      </div>
      <div className="confidence-bar-bg">
        <div className="confidence-bar-fill" style={{ width: `${pct}%`, backgroundColor: color }} />
      </div>
      {breakdown && (
        <div className="confidence-breakdown">
          <BreakdownRow label="Table routing" value={breakdown.table_routing} />
          <BreakdownRow label="LLM output" value={breakdown.llm} />
          <BreakdownRow label="Validation" value={breakdown.validation} />
        </div>
      )}
      {value < 0.5 && <p className="confidence-hint">Try being more specific, or select tables manually.</p>}
    </div>
  )
}

function BreakdownRow({ label, value }: { label: string; value: number }) {
  const pct = Math.round(value * 100)
  const color = value > 0.8 ? 'var(--success)' : value > 0.5 ? 'var(--warning)' : 'var(--error)'
  return (
    <div className="breakdown-row">
      <span>{label}</span>
      <div className="breakdown-bar-bg"><div className="breakdown-bar-fill" style={{ width: `${pct}%`, backgroundColor: color }} /></div>
      <span style={{ minWidth: 36, textAlign: 'right', fontSize: 12, color }}>{pct}%</span>
    </div>
  )
}

function TableRoutingViz({ routing }: { routing: NonNullable<AIQueryResponse['table_routing']> }) {
  const methodLabel = routing.ranking_method ?? 'keyword'
  const candidateScore = (c: TableRoutingCandidate) => {
    const v = c.score ?? c.relevance_score
    return typeof v === 'number' && Number.isFinite(v) ? v : 0
  }
  const maxScore = Math.max(...(routing.candidates ?? []).map(candidateScore), 0)
  return (
    <div className="table-routing-viz">
      <div className="routing-header">
        <span>Table Routing (<strong>{methodLabel}</strong>)</span>
        <span className="routing-confidence">{Math.round(routing.confidence * 100)}%</span>
      </div>
      {(routing.candidates ?? []).map((c: TableRoutingCandidate) => {
        const score = candidateScore(c)
        const pct = maxScore > 0 ? Math.round((score / maxScore) * 100) : 0
        return (
          <div key={c.table} className="routing-candidate">
            <span className="routing-table-name">{c.table}</span>
            <div className="routing-bar-bg"><div className="routing-bar-fill" style={{ width: `${pct}%` }} /></div>
            <span className="routing-score">{score.toFixed(2)}</span>
            {c.selected && <span className="routing-selected">✓</span>}
          </div>
        )
      })}
      {routing.reasoning && <p className="routing-reasoning">{routing.reasoning}</p>}
    </div>
  )
}

function ClarificationCard({ question, options, onSelect, onSkip }: { question: string; options: string[]; onSelect: (o: string) => void; onSkip: () => void }) {
  return (
    <div className="clarification-card">
      <div className="clarification-title"><span>🤔</span><span>AI needs clarification</span></div>
      <p className="clarification-question">{question}</p>
      <div className="clarification-options">
        {options.map((opt) => (
          <button key={opt} className="btn btn-clarification" onClick={() => onSelect(opt)}>{opt}</button>
        ))}
      </div>
      <button className="btn btn-skip" onClick={onSkip}>Skip — show whatever you have</button>
    </div>
  )
}

function CandidateComparisonPanel({ candidates, onUse }: { candidates: LogicalQueryCandidate[]; onUse: (i: number) => void }) {
  const bestIdx = candidates.reduce((best, c, i) => (c.confidence > (candidates[best]?.confidence ?? 0) ? i : best), 0)
  return (
    <div className="candidate-panel">
      <div className="candidate-header"><span>🔄 {candidates.length} candidates generated</span></div>
      <div className="candidate-cards">
        {candidates.map((c, i) => {
          const isBest = i === bestIdx
          const pct = Math.round(c.confidence * 100)
          return (
            <div key={i} className={`candidate-card ${isBest ? 'candidate-best' : ''}`}>
              <div className="candidate-card-header">
                <span>Candidate #{i + 1}</span>
                <span className={`candidate-score ${isBest ? 'score-best' : ''}`}>Score: {pct}%</span>
              </div>
              {c.reasoning && <p className="candidate-reasoning">{c.reasoning}</p>}
              <details>
                <summary>LogicalQuery JSON</summary>
                <pre className="sql-preview candidate-json">{JSON.stringify(c.logical_query, null, 2)}</pre>
              </details>
              <button className="btn btn-candidate-use" onClick={() => onUse(i)}>{isBest ? 'Use (recommended)' : 'Use this'}</button>
            </div>
          )
        })}
      </div>
    </div>
  )
}

function CostBadge({ latencyMs, tokenUsage, costUsd }: { latencyMs?: number; tokenUsage?: { prompt: number; completion: number; total: number }; costUsd?: number }) {
  if (!latencyMs && !tokenUsage && !costUsd) return null
  const parts: string[] = []
  if (latencyMs) parts.push(`⏱ ${(latencyMs / 1000).toFixed(1)}s`)
  if (tokenUsage) parts.push(`🪙 ${tokenUsage.total} tokens`)
  if (costUsd !== undefined) parts.push(`$${costUsd.toFixed(4)}`)
  return <div className="cost-badge">{parts.join(' · ')}</div>
}

function Collapsible({ title, children, defaultOpen = false }: { title: string; children: React.ReactNode; defaultOpen?: boolean }) {
  return (
    <details open={defaultOpen} className="collapsible-section">
      <summary>{title}</summary>
      <div className="collapsible-content">{children}</div>
    </details>
  )
}

function AIRuntimeSettingsPanel({ settings, err }: { settings: AIRuntimeSettings | null; err: string | null }) {
  if (err) {
    return (
      <div className="ai-runtime-settings" role="status">
        <div className="ai-runtime-settings-title">Server AI (from env)</div>
        <p className="error" style={{ margin: '0.35rem 0 0', fontSize: '0.85rem' }}>{err}</p>
      </div>
    )
  }
  if (!settings) {
    return (
      <div className="ai-runtime-settings" role="status">
        <div className="ai-runtime-settings-title">Server AI (from env)</div>
        <p className="ai-settings-hint" style={{ margin: '0.35rem 0 0' }}>Loading server AI settings…</p>
      </div>
    )
  }
  const baseConfigured = settings.base_url?.trim() !== ''
  return (
    <div className="ai-runtime-settings" id="ai-runtime-config" role="region" aria-label="Server AI configuration from environment">
      <div className="ai-runtime-settings-title">Server AI (from env)</div>
      <dl className="ai-settings-dl">
        <dt>Provider</dt>
        <dd>{settings.provider?.trim() ? settings.provider : '—'}</dd>
        <dt>Model</dt>
        <dd><code>{settings.llm_model?.trim() ? settings.llm_model : '—'}</code> <span className="ai-settings-meta">(BI_AI_MODEL)</span></dd>
        <dt>Base URL</dt>
        <dd>
          {baseConfigured ? (
            <code>{settings.base_url}</code>
          ) : (
            <span><span className="ai-settings-meta">(default)</span> <code>{settings.base_url_effective}</code></span>
          )}{' '}
          <span className="ai-settings-meta">(BI_AI_BASE_URL)</span>
        </dd>
        <dt>API key</dt>
        <dd>{settings.api_key_configured ? 'Configured' : 'Not set'} <span className="ai-settings-meta">(BI_AI_API_KEY)</span></dd>
        {settings.embeddings_enabled === true ? (
          <>
            <dt>Embedding model</dt>
            <dd><code>{settings.embedding_model ?? '—'}</code> <span className="ai-settings-meta">(BI_AI_EMBEDDING_MODEL)</span></dd>
            <dt>Embedding base URL</dt>
            <dd>
              {settings.embedding_base_url?.trim() ? (
                <code>{settings.embedding_base_url}</code>
              ) : (
                <span><span className="ai-settings-meta">(resolved)</span> <code>{settings.embedding_base_url_effective ?? '—'}</code></span>
              )}{' '}
              <span className="ai-settings-meta">(BI_AI_EMBEDDING_BASE_URL)</span>
            </dd>
            <dt>Embedding API key</dt>
            <dd>
              {settings.embedding_api_key_configured ? 'Configured' : 'Not set'}{' '}
              <span className="ai-settings-meta">
                (BI_AI_EMBEDDING_API_KEY
                {settings.embedding_api_key_dedicated ? '' : ' — falls back to BI_AI_API_KEY'})
              </span>
            </dd>
          </>
        ) : null}
      </dl>
      <p className="ai-settings-hint">
        The UI does not override these values. Set env vars on the API process and restart to change model or endpoint.
      </p>
    </div>
  )
}

function SampleDataModal({ open, onClose, tableName, datasourceId, get }: { open: boolean; onClose: () => void; tableName: string; datasourceId: string; get: <T>(url: string) => Promise<T | null> }) {
  const [sample, setSample] = useState<any>(null)
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    if (!open) { setSample(null); return }
    setLoading(true)
    const [schema, ...rest] = tableName.split('.')
    const tName = rest.length > 0 ? rest.join('.') : schema
    const url = `/api/datasources/${datasourceId}/tables/${schema ?? 'public'}/${tName}/sample`
    get<any>(url).then((data) => { setSample(data); setLoading(false) })
  }, [open]) // eslint-disable-line react-hooks/exhaustive-deps

  if (!open) return null
  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal-content" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h3>Sample Data — {tableName}</h3>
          <button className="modal-close" onClick={onClose}>×</button>
        </div>
        <div className="modal-body">
          {loading && <p>Loading…</p>}
          {sample?.columns && sample?.rows && (
            <table className="results-table">
              <thead><tr>{sample.columns.map((c: any) => <th key={c.name}>{c.name}</th>)}</tr></thead>
              <tbody>
                {sample.rows.map((row: any[], i: number) => (
                  <tr key={i}>{row.map((cell, j) => <td key={j}>{formatResultCell(cell, sample.columns[j]?.name ?? '', {})}</td>)}</tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      </div>
    </div>
  )
}

// ─── Main Component ─────────────────────────────────────────────────

interface TableOption {
  schema_name: string
  table_name: string
  table_type?: string
  description?: string | null
  label?: string
}

export default function AIQuery() {
  const { get, postData, loading, error, abort } = useApi()
  const { activeConversation, addMessage, createConversation, setActiveConversationId } = useConversation()

  // Datasource / table state
  const [datasources, setDatasources] = useState<Datasource[]>([])
  const [tables, setTables] = useState<TableOption[]>([])
  const [datasourceId, setDatasourceId] = useState('')
  const [selectedTables, setSelectedTables] = useState<string[]>([])
  const [tableSearch, setTableSearch] = useState('')
  const [includeBaseTables, setIncludeBaseTables] = useState(true)
  const [includeViews, setIncludeViews] = useState(true)
  const [autoTableRouting, setAutoTableRouting] = useState(true)

  // Query state
  const [question, setQuestion] = useState('')
  const [result, setResult] = useState<AIQueryResponse | null>(null)
  const [aiRuntime, setAiRuntime] = useState<AIRuntimeSettings | null>(null)
  const [aiRuntimeErr, setAiRuntimeErr] = useState<string | null>(null)
  const [chartType, setChartType] = useState<'bar' | 'line' | 'pie' | 'table'>('table')

  // Sample data modal
  const [sampleModalOpen, setSampleModalOpen] = useState(false)
  const [sampleModalTable, setSampleModalTable] = useState('')

  // Feedback state
  const [userFeedback, setUserFeedback] = useState<'positive' | 'negative' | null>(null)
  const [showFeedbackForm, setShowFeedbackForm] = useState(false)
  const [feedbackCategories, setFeedbackCategories] = useState<string[]>([])
  const [feedbackText, setFeedbackText] = useState('')

  // Include past queries toggle
  const [includePastQueries, setIncludePastQueries] = useState(false)

  // Cell drill-down
  const [drillDownTarget, setDrillDownTarget] = useState<string | null>(null)

  const messagesEndRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    get<Datasource[]>('/api/datasources').then((data) => {
      if (data) { setDatasources(data); if (data[0]) setDatasourceId(data[0].id) }
    })
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    get<AIRuntimeSettings>('/api/ai/settings').then((data) => {
      if (data) {
        setAiRuntime(data)
        setAiRuntimeErr(null)
      } else {
        setAiRuntime(null)
        setAiRuntimeErr('Could not load server AI settings')
      }
    })
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    setSelectedTables([]); setTableSearch(''); setIncludeBaseTables(true); setIncludeViews(true); setTables([])
    if (!datasourceId) return
    get<TableOption[]>(`/api/datasources/${datasourceId}/tables`).then((data) => setTables(data || []))
  }, [datasourceId]) // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => { messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' }) }, [activeConversation?.messages.length])

  const tableLabel = (table: TableOption) => table.label || `${table.schema_name}.${table.table_name}`

  const tablesInTypeScope = useMemo(
    () => tables.filter((table) => {
      const typ = (table.table_type || '').toUpperCase()
      if (typ === 'VIEW') return includeViews
      if (typ === 'BASE TABLE') return includeBaseTables
      return includeBaseTables
    }),
    [tables, includeBaseTables, includeViews],
  )

  const filteredTables = useMemo(() => {
    const search = tableSearch.trim().toLowerCase()
    return tablesInTypeScope.filter((table) => {
      if (!search) return true
      return tableLabel(table).toLowerCase().includes(search) || (table.description || '').toLowerCase().includes(search)
    })
  }, [tablesInTypeScope, tableSearch])

  const allowedLabels = useMemo(() => new Set(tablesInTypeScope.map((t) => tableLabel(t))), [tablesInTypeScope])
  useEffect(() => { setSelectedTables((prev) => prev.filter((s) => allowedLabels.has(s))) }, [allowedLabels])

  const recentPriorTurns = () => {
    if (!activeConversation) return undefined
    const MAX = 5
    const turns: { question: string; logical_query?: unknown; note?: string }[] = []
    const msgs = activeConversation.messages
    for (let i = 0; i < msgs.length; i++) {
      const m = msgs[i]
      if (!m || m.role !== 'user') continue
      const next = msgs[i + 1]
      const lq = next?.role === 'assistant' ? next.ai_response?.logical_query : undefined
      turns.push({
        question: m.content,
        logical_query: lq ?? undefined,
        note: next?.role === 'assistant' && next.ai_response?.sql ? 'executed' : undefined,
      })
    }
    return turns.slice(-MAX)
  }

  const requestBody = () => ({
    datasource_id: datasourceId,
    question,
    tables: autoTableRouting ? undefined : selectedTables,
    include_base_tables: includeBaseTables,
    include_views: includeViews,
    conversation_id: activeConversation?.id,
    prior_turns: includePastQueries ? recentPriorTurns() : undefined,
  })

  const sendQuery = async (q: string, execute: boolean) => {
    setQuestion(q)
    const body = {
      datasource_id: datasourceId,
      question: q,
      tables: autoTableRouting ? undefined : selectedTables,
      include_base_tables: includeBaseTables,
      include_views: includeViews,
      conversation_id: activeConversation?.id,
      prior_turns: includePastQueries ? recentPriorTurns() : undefined,
    }
    const endpoint = execute ? '/api/ai/query/run' : '/api/ai/query/preview'
    const res = await postData<AIQueryResponse>(endpoint, body, { timeout: AI_QUERY_TIMEOUT_MS })
    if (!res) return
    setResult(res); setChartType('table')
    setUserFeedback(null); setShowFeedbackForm(false); setFeedbackCategories([]); setFeedbackText('')

    if (res.needs_clarification) {
      addMessage({ role: 'user', content: q })
      addMessage({ role: 'assistant', content: res.clarification_question ?? 'Please clarify.', ai_response: res })
      return
    }
    addMessage({ role: 'user', content: q })
    const summary = res.sql ? `SQL: ${res.sql.slice(0, 120)}…` : 'Query executed'
    addMessage({ role: 'assistant', content: summary, ai_response: res })
  }

  const handleClarificationSelect = (opt: string) => sendQuery(`${question} (${opt})`, true)
  const handleClarificationSkip = () => sendQuery(question, true)

  const handleFilterByValue = (column: string, value: string) => setQuestion((prev) => `${prev} where ${column} = "${value}"`)

  const handleSampleData = (tableName: string) => { setSampleModalTable(tableName); setSampleModalOpen(true) }

  const FEEDBACK_CATS = ['Wrong table', 'Wrong columns', 'Incorrect aggregation', 'Missed date filter', 'Wrong logic', 'SQL error', 'Other']

  const submitFeedback = async (rating: 'positive' | 'negative') => {
    setUserFeedback(rating)
    if (rating === 'positive') {
      try { await postData('/api/ai/feedback', { question, datasource_id: datasourceId, rating: 'positive' }) } catch { /* noop */ }
    } else {
      setShowFeedbackForm(true)
    }
  }

  const submitNegativeFeedback = async () => {
    try {
      await postData('/api/ai/feedback', {
        question,
        datasource_id: datasourceId,
        rating: 'negative',
        categories: feedbackCategories,
        text: feedbackText,
      })
    } catch { /* noop */ }
    setShowFeedbackForm(false)
    setFeedbackCategories([])
    setFeedbackText('')
  }

  const handleCellDrillDown = (column: string, value: string) => {
    sendQuery(`Show details where ${column} = "${value}"`, true)
  }

  const chartData = result?.result?.rows?.map((row) => {
    const obj: Record<string, any> = { name: String(row[0]) }
    if (row[1] !== undefined) obj.value = Number(row[1]) || 0
    return obj
  }) ?? []

  useEffect(() => { if (result?.visualization_hint?.chart_type) setChartType(result.visualization_hint.chart_type) }, [result?.visualization_hint?.chart_type])

  const loadingLabel = loading
    ? result?.retry_count ? `AI self-corrected (attempt ${result.retry_count + 1}/3)…`
    : result?.candidates_count ? `Generating candidates…`
    : 'Thinking…'
    : ''

  return (
    <div className="ai-query-layout">
      {/* ─── Conversation Sidebar ─────────────────────────────── */}
      <aside className="conversation-sidebar">
        <div className="sidebar-header">
          <h3>Conversations</h3>
          <button className="btn btn-sm" onClick={() => { createConversation(); setResult(null); setQuestion('') }}>+ New</button>
        </div>
        {activeConversation && (
          <>
            <button className="conversation-item active" onClick={() => setActiveConversationId(activeConversation.id)}>
              <span className="conv-title">{activeConversation.title ?? 'Current'}</span>
              <span className="conv-time">{activeConversation.messages.length} msgs</span>
            </button>
            <div className="conv-messages-list">
              {activeConversation.messages.slice(-10).map((m, i) => (
                <div key={i} className={`conv-msg conv-${m.role}`}>
                  <span>{m.role === 'user' ? '→' : '←'} {m.content.slice(0, 45)}{m.content.length > 45 ? '…' : ''}</span>
                </div>
              ))}
              <div ref={messagesEndRef} />
            </div>
          </>
        )}
      </aside>

      {/* ─── Main Content ─────────────────────────────────────── */}
      <div className="ai-query-main">
        {/* Input Card */}
        <div className="card">
          <div className="card-header-row">
            <h2>Natural-language Query</h2>
            {activeConversation && activeConversation.messages.length > 0 && (
              <div className="context-badge">Context: {activeConversation.messages.length} msgs</div>
            )}
          </div>
          <p className="card-subtitle">Ask a question in natural language. The AI generates a LogicalQuery, the backend compiles it to SQL.</p>

          <div className="query-controls">
            <div className="form-group">
              <label htmlFor="ai-datasource">Datasource</label>
              <select id="ai-datasource" value={datasourceId} onChange={(e) => setDatasourceId(e.target.value)}>
                <option value="">— select —</option>
                {datasources.map((d) => <option key={d.id} value={d.id}>{d.name}</option>)}
              </select>
            </div>
            <div className="form-group">
              <AIRuntimeSettingsPanel settings={aiRuntime} err={aiRuntimeErr} />
            </div>
            <div className="form-group routing-toggle">
              <label>Table Routing</label>
              <div className="toggle-group">
                <button className={`toggle-btn ${autoTableRouting ? 'active' : ''}`} onClick={() => setAutoTableRouting(true)}>Auto</button>
                <button className={`toggle-btn ${!autoTableRouting ? 'active' : ''}`} onClick={() => setAutoTableRouting(false)}>Manual</button>
              </div>
            </div>
          </div>

          {!autoTableRouting && (
            <div className="form-group">
              <span className="ai-scope-label">Tables / Semantic Scope</span>
              <div className="ai-scope-type-filters" role="group">
                <label className="ai-scope-type-option">
                  <input type="checkbox" checked={includeBaseTables} onChange={(e) => setIncludeBaseTables(e.target.checked)} disabled={!datasourceId || tables.length === 0} />
                  <span>Base tables</span>
                </label>
                <label className="ai-scope-type-option">
                  <input type="checkbox" checked={includeViews} onChange={(e) => setIncludeViews(e.target.checked)} disabled={!datasourceId || tables.length === 0} />
                  <span>Views</span>
                </label>
              </div>
              <input value={tableSearch} onChange={(e) => setTableSearch(e.target.value)} placeholder="Search tables…" disabled={!datasourceId || tables.length === 0} autoComplete="off" />
              <select aria-label="Selected tables" multiple value={selectedTables} onChange={(e) => setSelectedTables(Array.from(e.target.selectedOptions, (o) => o.value))}
                disabled={!datasourceId || tables.length === 0 || (!includeBaseTables && !includeViews)}
                className="ai-scope-multiselect" size={Math.min(8, Math.max(3, filteredTables.length || 3))}>
                {filteredTables.map((table) => { const label = tableLabel(table); return <option key={label} value={label}>{label}</option> })}
              </select>
            </div>
          )}

          <div className="form-group">
            <label htmlFor="ai-question">Your Question</label>
            <textarea id="ai-question" value={question} onChange={(e) => setQuestion(e.target.value)}
              onKeyDown={(e: KeyboardEvent<HTMLTextAreaElement>) => { if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) { e.preventDefault(); sendQuery(question, true) } }}
              placeholder="Show total revenue by country for January 2026…" rows={3} autoComplete="off" />
            {activeConversation && activeConversation.messages.length > 0 && (
              <div className="past-queries-toggle">
                <input type="checkbox" id="include-past" checked={includePastQueries} onChange={(e) => setIncludePastQueries(e.target.checked)} />
                <label htmlFor="include-past">Include my past queries as few-shot examples</label>
              </div>
            )}
          </div>

          <div className="button-row">
            <button className="btn" onClick={() => sendQuery(question, false)} disabled={loading || !question || !datasourceId}>{loadingLabel || 'Preview SQL'}</button>
            <button className="btn btn-primary" onClick={() => sendQuery(question, true)} disabled={loading || !question || !datasourceId}>{loadingLabel || 'Preview & Execute'}</button>
            {loading && <button className="btn btn-ghost" onClick={abort}>Cancel</button>}
          </div>
          {error && <div className="error" style={{ marginTop: '1rem' }}>{error}</div>}
        </div>

        {/* ─── Result Card ──────────────────────────────────────── */}
        {result && (
          <div className="card result-card">
            {result.confidence !== undefined && <ConfidenceBar value={result.confidence} breakdown={result.confidence_breakdown} />}
            <CostBadge latencyMs={result.latency_ms} tokenUsage={result.token_usage} costUsd={result.cost_usd} />

            {result.model_used && (
              <div className="model-used-badge" style={{ fontSize: '0.8rem', color: 'var(--text-secondary)', marginBottom: '0.5rem' }}>
                Model used: <code>{result.model_used}</code>
                {aiRuntime?.llm_model && result.model_used !== aiRuntime.llm_model && (
                  <span> (configured: <code>{aiRuntime.llm_model}</code>)</span>
                )}
              </div>
            )}

            {result.retry_count !== undefined && result.retry_count > 0 && (
              <div className="retry-badge">🔄 AI self-corrected (attempt {result.retry_count}/3)</div>
            )}

            {result.needs_clarification && result.clarification_options && (
              <ClarificationCard question={result.clarification_question ?? 'Please clarify.'} options={result.clarification_options} onSelect={handleClarificationSelect} onSkip={handleClarificationSkip} />
            )}

            {result.candidates && result.candidates.length > 1 && !result.needs_clarification && (
              <CandidateComparisonPanel candidates={result.candidates} onUse={(i: number) => {
                const c = result.candidates?.[i]
                if (c) setResult({ ...result, logical_query: c.logical_query, confidence: c.confidence })
              }} />
            )}

            {result.table_routing && (
              <Collapsible title="📋 Table Routing" defaultOpen>
                <TableRoutingViz routing={result.table_routing} />
                {result.table_routing.selected_tables?.length > 0 && (
                  <button className="btn btn-sm btn-sample" onClick={() => { const t = result.table_routing?.selected_tables?.[0]; if (t) handleSampleData(t) }}>Preview sample data</button>
                )}
              </Collapsible>
            )}

            {result.validation_result && (
              <Collapsible title={result.validation_result.plan_ok ? '✅ SQL Plan' : '⚠️ SQL Plan — Issues found'} defaultOpen={!result.validation_result.plan_ok}>
                {result.validation_result.explain_output && <pre className="sql-preview explain-output">{result.validation_result.explain_output}</pre>}
                <p className={`plan-status ${result.validation_result.plan_ok ? 'plan-ok' : 'plan-warn'}`}>
                  {result.validation_result.plan_ok ? 'Plan looks good — query will execute safely.' : 'Plan has warnings. Review before executing.'}
                </p>
              </Collapsible>
            )}

            {result.logical_query?.window_functions && result.logical_query.window_functions.length > 0 && (
              <div style={{ marginBottom: '0.5rem' }}>
                {result.logical_query.window_functions.map((wf: any, i: number) => (
                  <span key={i} className="wf-badge">Uses: Window Function ({wf.function})</span>
                ))}
              </div>
            )}

            {result.logical_query && (
              <Collapsible title="🧠 Generated LogicalQuery" defaultOpen>
                <pre className="sql-preview">{JSON.stringify(result.logical_query, null, 2)}</pre>
              </Collapsible>
            )}

            {result.sql && (
              <Collapsible title="📝 Compiled SQL" defaultOpen>
                <pre className="sql-preview">{result.sql}</pre>
              </Collapsible>
            )}

            {result.prompt && (
              <Collapsible title="🔍 Show prompt">
                <pre className="sql-preview prompt-preview">{result.prompt}</pre>
                {result.token_usage && <p className="token-info">Tokens: {result.token_usage.prompt} prompt · {result.token_usage.completion} completion · {result.token_usage.total} total</p>}
                {result.token_usage && result.token_usage.prompt > 30000 && (
                  <p className="prompt-warning">⚠️ Prompt is large ({(result.token_usage.prompt / 1000).toFixed(1)}K tokens) — may affect response quality.</p>
                )}
              </Collapsible>
            )}

            {result.warnings && result.warnings.length > 0 && (
              <div className="error"><strong>Warnings:</strong><ul>{result.warnings.map((w, i) => <li key={i}>{w}</li>)}</ul></div>
            )}

            {result.retry_count !== undefined && result.retry_count >= 3 && !result.sql && (
              <div className="error-recovery">
                <p>AI couldn't generate a valid query after {result.retry_count} attempts.</p>
                <div className="recovery-options">
                  <button onClick={() => document.getElementById('ai-question')?.focus()}>Rephrase question</button>
                  <button type="button" onClick={() => document.getElementById('ai-runtime-config')?.scrollIntoView({ behavior: 'smooth', block: 'nearest' })}>
                    Check server AI settings
                  </button>
                  <button onClick={() => { window.location.hash = '/query-builder' }}>Manual query builder</button>
                </div>
              </div>
            )}

            {/* Results */}
            {result.result?.columns && result.result.rows && (
              <div className="results-section">
                <div className="results-header">
                  <h3>Results ({result.result.stats?.row_count ?? 0} rows)</h3>
                  {result.visualization_hint && <span className="viz-hint" title={result.visualization_hint.reason}>💡 {result.visualization_hint.chart_type}</span>}
                  <div className="chart-toggle">
                    {(['bar', 'line', 'pie', 'table'] as const).map((t) => (
                      <button key={t} className={chartType === t ? 'active' : ''} onClick={() => setChartType(t)}>
                        {t === 'table' ? 'Table' : t.charAt(0).toUpperCase() + t.slice(1)}
                      </button>
                    ))}
                  </div>
                </div>

                {chartType !== 'table' && chartData.length > 0 && (
                  <div className="chart-container" style={{ height: 300 }}>
                    <ResponsiveContainer width="100%" height="100%">
                      {chartType === 'bar' ? (
                        <BarChart data={chartData}><CartesianGrid strokeDasharray="3 3" stroke="#475569" /><XAxis dataKey="name" stroke="#94a3b8" /><YAxis stroke="#94a3b8" /><Tooltip contentStyle={{ background: '#1e293b', border: '1px solid #475569' }} /><Bar dataKey="value" fill="#3b82f6" /></BarChart>
                      ) : chartType === 'line' ? (
                        <LineChart data={chartData}><CartesianGrid strokeDasharray="3 3" stroke="#475569" /><XAxis dataKey="name" stroke="#94a3b8" /><YAxis stroke="#94a3b8" /><Tooltip contentStyle={{ background: '#1e293b', border: '1px solid #475569' }} /><Line type="monotone" dataKey="value" stroke="#3b82f6" strokeWidth={2} /></LineChart>
                      ) : (
                        <PieChart><Pie data={chartData} dataKey="value" nameKey="name" cx="50%" cy="50%" outerRadius={100} label>{chartData.map((_: any, i: number) => <Cell key={i} fill={COLORS[i % COLORS.length]} />)}</Pie><Tooltip contentStyle={{ background: '#1e293b', border: '1px solid #475569' }} /></PieChart>
                      )}
                    </ResponsiveContainer>
                  </div>
                )}

                {chartType === 'table' && (
                  <ResultTable
                    columns={result.result.columns}
                    rows={result.result.rows}
                    rowCount={result.result.stats?.row_count ?? 0}
                    durationMs={result.result.stats?.duration_ms}
                    question={question}
                    onFilterByValue={handleFilterByValue}
                    onCellClick={(colName, value) => handleCellDrillDown(colName, String(value))}
                  />
                )}
              </div>
            )}

            {/* Feedback */}
            <div className="feedback-row">
              <span style={{ fontSize: '0.8rem', color: 'var(--text-secondary)', marginRight: '0.5rem' }}>Was this helpful?</span>
              <button
                className={`feedback-btn ${userFeedback === 'positive' ? 'feedback-active' : ''}`}
                onClick={() => submitFeedback('positive')}
              >👍</button>
              <button
                className={`feedback-btn ${userFeedback === 'negative' ? 'feedback-negative' : ''}`}
                onClick={() => submitFeedback('negative')}
              >👎</button>
            </div>
            {showFeedbackForm && (
              <div className="feedback-form">
                <p style={{ fontSize: '0.8rem', marginBottom: '0.4rem', color: 'var(--text-secondary)' }}>What went wrong?</p>
                <div className="feedback-categories">
                  {FEEDBACK_CATS.map((cat) => (
                    <button
                      key={cat}
                      className={`feedback-cat-btn ${feedbackCategories.includes(cat) ? 'feedback-active' : ''}`}
                      onClick={() => setFeedbackCategories((prev) => prev.includes(cat) ? prev.filter((c) => c !== cat) : [...prev, cat])}
                    >{cat}</button>
                  ))}
                </div>
                <textarea
                  value={feedbackText}
                  onChange={(e) => setFeedbackText(e.target.value)}
                  placeholder="Additional details (optional)…"
                  rows={2}
                  style={{ width: '100%', fontSize: '0.8rem', resize: 'vertical' }}
                />
                <div style={{ display: 'flex', gap: '0.5rem', marginTop: '0.4rem' }}>
                  <button className="btn btn-sm btn-primary" onClick={submitNegativeFeedback}>Submit Feedback</button>
                  <button className="btn btn-sm btn-ghost" onClick={() => { setShowFeedbackForm(false); setFeedbackCategories([]); setFeedbackText('') }}>Cancel</button>
                </div>
              </div>
            )}
          </div>
        )}

        {/* Follow-up input (when conversation active) */}
        {activeConversation && activeConversation.messages.length > 0 && (
          <div className="follow-up-bar">
            <input
              placeholder="Ask a follow-up…"
              onKeyDown={(e: KeyboardEvent<HTMLInputElement>) => {
                if (e.key === 'Enter' && e.currentTarget.value.trim()) {
                  handleFollowUp(e.currentTarget.value.trim())
                  e.currentTarget.value = ''
                }
              }}
            />
          </div>
        )}
      </div>

      <SampleDataModal open={sampleModalOpen} onClose={() => setSampleModalOpen(false)} tableName={sampleModalTable} datasourceId={datasourceId} get={get} />
    </div>
  )

  async function handleFollowUp(followUp: string) {
    if (!activeConversation) createConversation()
    const res = await postData<AIQueryResponse>(
      '/api/ai/query/run',
      { ...requestBody(), question: followUp, conversation_id: activeConversation?.id },
      { timeout: AI_QUERY_TIMEOUT_MS },
    )
    if (!res) return
    setResult(res)
    addMessage({ role: 'user', content: followUp })
    const summary = res.sql ? `SQL: ${res.sql.slice(0, 120)}…` : 'Query executed'
    addMessage({ role: 'assistant', content: summary, ai_response: res })
  }
}
