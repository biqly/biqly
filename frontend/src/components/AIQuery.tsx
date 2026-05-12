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
import { useQueryParam } from '../hooks/useQueryParam'
import { formatResultCell } from '../utils/resultCellFormat'
import ResultTable from './ResultTable'
import { Select } from './ui/Select'
import { ModelBadgeRow } from './ui/ModelBadgeRow'
import type { AIQueryResponse, TableRoutingCandidate, LogicalQueryCandidate, AIRuntimeSettings, EmbedMetadataResponse } from '../types/ai'
import type { Datasource } from '../types/metadata'

const COLORS = ['#3b82f6', '#22c55e', '#f59e0b', '#ef4444', '#8b5cf6', '#ec4899', '#14b8a6', '#f97316']

/** NL→SQL can be slow with local models (routing, LLM, retries, EXPLAIN). */
const AI_QUERY_TIMEOUT_MS = 300_000
const AI_METADATA_EMBED_TIMEOUT_MS = 600_000

// ─── Sub-components ─────────────────────────────────────────────────

function ConfidenceBar({ value, breakdown }: { value: number; breakdown?: { table_routing: number; llm: number; validation: number } }) {
  const pct = Math.round(value * 100)
  const color = value > 0.8 ? 'var(--success)' : value > 0.5 ? 'var(--warning)' : 'var(--error)'
  return (
    <div className="confidence-section">
      <div className="confidence-header">
        <span>Güven</span>
        <span style={{ color, fontWeight: 600 }}>{pct}%</span>
      </div>
      <div className="confidence-bar-bg">
        <div className="confidence-bar-fill" style={{ width: `${pct}%`, backgroundColor: color }} />
      </div>
      {breakdown && (
        <div className="confidence-breakdown">
          <BreakdownRow label="Tablo yönlendirme" value={breakdown.table_routing} />
          <BreakdownRow label="LLM çıktısı" value={breakdown.llm} />
          <BreakdownRow label="Doğrulama" value={breakdown.validation} />
        </div>
      )}
      {value < 0.5 && <p className="confidence-hint">Daha spesifik olun veya tabloları manuel seçin.</p>}
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

function routingMethodLabel(method: string | undefined) {
  const m = (method ?? 'keyword').toLowerCase()
  if (m === 'keyword') return 'anahtar kelime'
  if (m === 'vector') return 'vektör'
  if (m === 'hybrid') return 'hibrit'
  if (m === 'manual') return 'manuel'
  if (m === 'semantic') return 'semantic model'
  return method ?? 'keyword'
}

function contextSourceLabel(source: string | undefined) {
  const s = (source ?? 'auto').toLowerCase()
  if (s === 'semantic_model') return 'kalıcı semantic context'
  if (s === 'manual') return 'manuel tablo kapsamı'
  if (s === 'auto') return 'otomatik context'
  return source ?? 'otomatik context'
}

function compactList(items: string[] | undefined, limit = 8) {
  if (!items || items.length === 0) return null
  const visible = items.slice(0, limit)
  const rest = items.length - visible.length
  return `${visible.join(', ')}${rest > 0 ? ` +${rest}` : ''}`
}

function TableRoutingViz({ routing }: { routing: NonNullable<AIQueryResponse['table_routing']> }) {
  const methodLabel = routingMethodLabel(routing.ranking_method)
  const sourceLabel = contextSourceLabel(routing.context_source)
  const candidateScore = (c: TableRoutingCandidate) => {
    const v = c.total_score ?? c.score ?? c.relevance_score
    return typeof v === 'number' && Number.isFinite(v) ? v : 0
  }
  const maxScore = Math.max(...(routing.candidates ?? []).map(candidateScore), 0)
  const selectedDims = compactList(routing.selected_dimensions)
  const selectedMetrics = compactList(routing.selected_metrics)
  const selectedTables = compactList(routing.selected_tables)
  const selectedModels = compactList(routing.selected_models)
  return (
    <div className="table-routing-viz">
      <div className="routing-header">
        <span>Tablo Yönlendirme (<strong>{methodLabel}</strong>)</span>
        <span className="routing-confidence">{Math.round(routing.confidence * 100)}%</span>
      </div>
      <div className="routing-context-grid">
        <div><span>Kaynak</span><strong>{sourceLabel}</strong></div>
        {selectedModels && <div><span>Model</span><strong>{selectedModels}</strong></div>}
        {selectedTables && <div><span>Tablolar</span><strong>{selectedTables}</strong></div>}
        {selectedDims && <div><span>Boyutlar</span><strong>{selectedDims}</strong></div>}
        {selectedMetrics && <div><span>Metrikler</span><strong>{selectedMetrics}</strong></div>}
        {routing.context_updated_at && <div><span>Context zamanı</span><strong>{new Date(routing.context_updated_at).toLocaleString()}</strong></div>}
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
            <span className="routing-score-detail">
              k:{(c.keyword_score ?? 0).toFixed(2)}
              {c.embedding_score !== undefined && ` · e:${c.embedding_score.toFixed(2)}`}
            </span>
          </div>
        )
      })}
      {routing.debug && (
        <div className="routing-debug">
          {routing.debug.relation_expansion && routing.debug.relation_expansion.length > 0 && (
            <div><span>İlişki genişletmesi</span><code>{routing.debug.relation_expansion.join(' | ')}</code></div>
          )}
          {routing.debug.bridge_tables && routing.debug.bridge_tables.length > 0 && (
            <div><span>Köprü tabloları</span><code>{routing.debug.bridge_tables.join(', ')}</code></div>
          )}
          {routing.debug.eliminated_candidates && routing.debug.eliminated_candidates.length > 0 && (
            <div><span>Elenen adaylar</span><code>{routing.debug.eliminated_candidates.join(', ')}</code></div>
          )}
        </div>
      )}
      {routing.reasoning && <p className="routing-reasoning">{routing.reasoning}</p>}
    </div>
  )
}

function ClarificationCard({ question, options, onSelect, onSkip }: { question: string; options: string[]; onSelect: (o: string) => void; onSkip: () => void }) {
  return (
    <div className="clarification-card">
      <div className="clarification-title"><span>🤔</span><span>AI'nın netleştirmeye ihtiyacı var</span></div>
      <p className="clarification-question">{question}</p>
      <div className="clarification-options">
        {options.map((opt) => (
          <button key={opt} className="btn btn-clarification" onClick={() => onSelect(opt)}>{opt}</button>
        ))}
      </div>
      <button className="btn btn-skip" onClick={onSkip}>Atla — elimizdekini göster</button>
    </div>
  )
}

function CandidateComparisonPanel({ candidates, onUse }: { candidates: LogicalQueryCandidate[]; onUse: (i: number) => void }) {
  const bestIdx = candidates.reduce((best, c, i) => (c.confidence > (candidates[best]?.confidence ?? 0) ? i : best), 0)
  return (
    <div className="candidate-panel">
      <div className="candidate-header"><span>🔄 {candidates.length} aday üretildi</span></div>
      <div className="candidate-cards">
        {candidates.map((c, i) => {
          const isBest = i === bestIdx
          const pct = Math.round(c.confidence * 100)
          return (
            <div key={i} className={`candidate-card ${isBest ? 'candidate-best' : ''}`}>
              <div className="candidate-card-header">
                 <span>Aday #{i + 1}</span>
                 <span className={`candidate-score ${isBest ? 'score-best' : ''}`}>Puan: {pct}%</span>
              </div>
              {c.reasoning && <p className="candidate-reasoning">{c.reasoning}</p>}
              <details>
                <summary>Mantıksal Sorgu (JSON)</summary>
                <pre className="sql-preview candidate-json">{JSON.stringify(c.logical_query, null, 2)}</pre>
              </details>
              <button className="btn btn-candidate-use" onClick={() => onUse(i)}>{isBest ? 'Kullan (önerilen)' : 'Bunu kullan'}</button>
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
  if (tokenUsage) parts.push(`🪙 ${tokenUsage.total} token`)
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

function embeddingSummary(response: EmbedMetadataResponse) {
  const counts = (response.results ?? []).reduce(
    (acc, item) => {
      if (item.skipped) return acc
      const kind = item.kind ?? 'table'
      if (kind === 'column') acc.columns += 1
      else acc.tables += 1
      return acc
    },
    { tables: 0, columns: 0 },
  )
  const details = counts.tables || counts.columns
    ? ` (${counts.tables} tablo, ${counts.columns} kolon)`
    : ''
  return `Gömülen ${response.embedded} metadata ögesi${details} (${response.model}).`
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
          <h3>Örnek Veri — {tableName}</h3>
          <button className="modal-close" onClick={onClose}>×</button>
        </div>
        <div className="modal-body">
          {loading && <p>Yükleniyor…</p>}
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
  const { postData: postEmbedData, loading: embeddingLoading, error: embeddingError } = useApi()
  const { activeConversation, addMessage, createConversation, setActiveConversationId } = useConversation()

  // Datasource / table state
  const [datasources, setDatasources] = useState<Datasource[]>([])
  const [tables, setTables] = useState<TableOption[]>([])
  const [dsParam, setDsParam] = useQueryParam('ds')
  const [datasourceId, setDatasourceId] = useState(dsParam)
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
  const [embeddingStatus, setEmbeddingStatus] = useState<string | null>(null)
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

  /** Which primary action is in flight — avoids both buttons showing the same loading text */
  const [queryAction, setQueryAction] = useState<'preview' | 'execute' | null>(null)

  // Cell drill-down
  const [drillDownTarget, setDrillDownTarget] = useState<string | null>(null)

  const messagesEndRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    get<Datasource[]>('/api/datasources').then((data) => {
      if (!data) return
      setDatasources(data)
      setDatasourceId((prev) => {
        if (prev && data.some((d) => d.id === prev)) return prev
        return data[0]?.id ?? ''
      })
    })
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    setDsParam(datasourceId)
  }, [datasourceId, setDsParam])

  useEffect(() => {
    get<AIRuntimeSettings>('/api/ai/settings').then((data) => {
      if (data) {
        setAiRuntime(data)
        setAiRuntimeErr(null)
      } else {
        setAiRuntime(null)
        setAiRuntimeErr('Sunucu AI ayarları yüklenemedi')
      }
    })
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    setSelectedTables([]); setTableSearch(''); setIncludeBaseTables(true); setIncludeViews(true); setTables([])
    setEmbeddingStatus(null)
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

  const selectedDatasourceName = useMemo(
    () => datasources.find((ds) => ds.id === datasourceId)?.name,
    [datasources, datasourceId],
  )

  const refreshMetadataEmbeddings = async () => {
    if (!datasourceId || embeddingLoading) return
    setEmbeddingStatus(null)
    const res = await postEmbedData<EmbedMetadataResponse>(
      '/api/ai/metadata/embed',
      { datasource_id: datasourceId },
      { timeout: AI_METADATA_EMBED_TIMEOUT_MS },
    )
    if (!res) return
    setEmbeddingStatus(embeddingSummary(res))
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
    setQueryAction(execute ? 'execute' : 'preview')
    try {
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
        addMessage({ role: 'assistant', content: res.clarification_question ?? 'Lütfen netleştirin.', ai_response: res })
        return
      }
      addMessage({ role: 'user', content: q })
      const summary = res.sql ? `SQL: ${res.sql.slice(0, 120)}…` : 'Sorgu çalıştırıldı'
      addMessage({ role: 'assistant', content: summary, ai_response: res })
    } finally {
      setQueryAction(null)
    }
  }

  const handleClarificationSelect = (opt: string) => sendQuery(`${question} (${opt})`, true)
  const handleClarificationSkip = () => sendQuery(question, true)

  const handleFilterByValue = (column: string, value: string) => setQuestion((prev) => `${prev} ${column} = "${value}" ile filtrele`)

  const handleSampleData = (tableName: string) => { setSampleModalTable(tableName); setSampleModalOpen(true) }

  const FEEDBACK_CATS = ['Yanlış tablo', 'Yanlış kolonlar', 'Yanlış toplama', 'Tarih filtresi eksik', 'Yanlış mantık', 'SQL hatası', 'Diğer']

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
    sendQuery(`${column} = "${value}" olan satırların detayını göster`, true)
  }

  const chartData = result?.result?.rows?.map((row) => {
    const obj: Record<string, any> = { name: String(row[0]) }
    if (row[1] !== undefined) obj.value = Number(row[1]) || 0
    return obj
  }) ?? []

  useEffect(() => { if (result?.visualization_hint?.chart_type) setChartType(result.visualization_hint.chart_type) }, [result?.visualization_hint?.chart_type])

  const loadingLabel = loading
    ? result?.retry_count ? `AI kendini düzeltti (deneme ${result.retry_count + 1}/3)…`
    : result?.candidates_count ? `Adaylar üretiliyor…`
    : 'Düşünülüyor…'
    : ''

  const previewButtonLabel = loading && queryAction === 'preview' ? loadingLabel : 'SQL Önizle'
  const executeButtonLabel = loading && queryAction === 'execute' ? loadingLabel : 'Önizle & Çalıştır'

  return (
    <div className="ai-query-layout">
      {/* ─── Conversation Sidebar ─────────────────────────────── */}
      <aside className="conversation-sidebar">
        <div className="sidebar-header">
          <h3>Konuşmalar</h3>
          <button className="btn btn-sm" onClick={() => { createConversation(); setResult(null); setQuestion('') }}>+ Yeni</button>
        </div>
        {activeConversation && (
          <>
            <button className="conversation-item active" onClick={() => setActiveConversationId(activeConversation.id)}>
              <span className="conv-title">{activeConversation.title ?? 'Şu anki'}</span>
              <span className="conv-time">{activeConversation.messages.length} mesaj</span>
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
            <h2>Doğal Dil Sorgusu</h2>
            {activeConversation && activeConversation.messages.length > 0 && (
              <div className="context-badge">Bağlam: {activeConversation.messages.length} mesaj</div>
            )}
          </div>
          <p className="card-subtitle">Doğal dilde bir soru sorun. AI bir LogicalQuery oluşturur, backend bunu SQL'e derler.</p>
          <ModelBadgeRow
            primaryLabel="Sorgu"
            primaryModel={
              aiRuntime?.query_model_override ? aiRuntime?.query_model : aiRuntime?.llm_model
            }
            primaryNote={
              aiRuntime?.query_model_override
                ? undefined
                : aiRuntime
                  ? 'BI_AI_MODEL mirası'
                  : undefined
            }
            embeddingModel={aiRuntime?.embeddings_enabled ? aiRuntime?.embedding_model : undefined}
            translationModel={aiRuntime?.translation_enabled ? aiRuntime?.translation_model : undefined}
            style={{ marginBottom: '0.5rem' }}
          />

          <div className="query-controls">
            <div className="form-group">
              <label htmlFor="ai-datasource">Veri Kaynağı</label>
              <Select
                id="ai-datasource"
                value={datasourceId}
                onChange={setDatasourceId}
                placeholder="— seçin —"
                header="Veri kaynakları"
                options={datasources.map((d) => ({ value: d.id, label: d.name, hint: d.type }))}
              />
            </div>
            <div className="form-group routing-toggle">
              <label>Tablo Yönlendirme</label>
              <div className="toggle-group">
                <button className={`toggle-btn ${autoTableRouting ? 'active' : ''}`} onClick={() => setAutoTableRouting(true)}>Otomatik</button>
                <button className={`toggle-btn ${!autoTableRouting ? 'active' : ''}`} onClick={() => setAutoTableRouting(false)}>Manuel</button>
              </div>
            </div>
            {aiRuntime?.embeddings_enabled === true && (
              <div className="form-group ai-settings-group" style={{ alignItems: 'flex-start' }}>
                <label style={{ visibility: 'hidden' }}>&nbsp;</label>
                <div style={{ display: 'inline-flex', flexDirection: 'column', gap: '0.25rem' }}>
                  <button
                    type="button"
                    className="btn btn-sm"
                    onClick={refreshMetadataEmbeddings}
                    disabled={!datasourceId || embeddingLoading}
                    title={
                      datasourceId
                        ? `Seçilen veri kaynağı (${selectedDatasourceName ?? ''}) için embedding'leri yenile`
                        : 'Önce bir veri kaynağı seçin'
                    }
                  >
                    {embeddingLoading ? 'Embedding\'ler yenileniyor…' : 'Embedding\'leri yenile'}
                  </button>
                  {embeddingStatus && <span className="ai-embedding-status" style={{ fontSize: '0.75rem' }}>{embeddingStatus}</span>}
                  {embeddingError && <span className="ai-embedding-error" style={{ fontSize: '0.75rem' }}>{embeddingError}</span>}
                  {aiRuntimeErr && <span className="error" style={{ fontSize: '0.75rem' }}>{aiRuntimeErr}</span>}
                </div>
              </div>
            )}
          </div>

          {!autoTableRouting && (
            <div className="form-group">
              <span className="ai-scope-label">Tablolar / Anlamsal Kapsam</span>
              <div className="ai-scope-type-filters" role="group">
                <label className="ai-scope-type-option">
                  <input type="checkbox" checked={includeBaseTables} onChange={(e) => setIncludeBaseTables(e.target.checked)} disabled={!datasourceId || tables.length === 0} />
                  <span>Temel tablolar</span>
                </label>
                <label className="ai-scope-type-option">
                  <input type="checkbox" checked={includeViews} onChange={(e) => setIncludeViews(e.target.checked)} disabled={!datasourceId || tables.length === 0} />
                  <span>Görünümler</span>
                </label>
              </div>
              <input value={tableSearch} onChange={(e) => setTableSearch(e.target.value)} placeholder="Tablo ara…" disabled={!datasourceId || tables.length === 0} autoComplete="off" />
              <select aria-label="Seçilen tablolar" multiple value={selectedTables} onChange={(e) => setSelectedTables(Array.from(e.target.selectedOptions, (o) => o.value))}
                disabled={!datasourceId || tables.length === 0 || (!includeBaseTables && !includeViews)}
                className="ai-scope-multiselect" size={Math.min(8, Math.max(3, filteredTables.length || 3))}>
                {filteredTables.map((table) => { const label = tableLabel(table); return <option key={label} value={label}>{label}</option> })}
              </select>
            </div>
          )}

          <div className="form-group">
            <label htmlFor="ai-question">Sorunuz</label>
            <textarea id="ai-question" value={question} onChange={(e) => setQuestion(e.target.value)}
              onKeyDown={(e: KeyboardEvent<HTMLTextAreaElement>) => { if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) { e.preventDefault(); sendQuery(question, true) } }}
              placeholder="Ülkeye göre toplam geliri göster, Ocak 2026…" rows={3} autoComplete="off" />
            {activeConversation && activeConversation.messages.length > 0 && (
              <div className="past-queries-toggle">
                <input type="checkbox" id="include-past" checked={includePastQueries} onChange={(e) => setIncludePastQueries(e.target.checked)} />
                <label htmlFor="include-past">Geçmiş sorgularımı few-shot örneği olarak dahil et</label>
              </div>
            )}
          </div>

          <div className="button-row">
            <button className="btn" onClick={() => sendQuery(question, false)} disabled={loading || !question || !datasourceId}>{previewButtonLabel}</button>
            <button className="btn btn-primary" onClick={() => sendQuery(question, true)} disabled={loading || !question || !datasourceId}>{executeButtonLabel}</button>
            {loading && <button className="btn btn-ghost" onClick={abort}>İptal</button>}
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
                Sorgu modeli: <code translate="no">{result.model_used}</code>
                {aiRuntime?.query_model_override && aiRuntime.query_model && result.model_used !== aiRuntime.query_model && (
                  <span> (yapılandırılan: <code translate="no">{aiRuntime.query_model}</code>)</span>
                )}
                {!aiRuntime?.query_model_override && aiRuntime?.llm_model && result.model_used !== aiRuntime.llm_model && (
                  <span> (yapılandırılan: <code translate="no">{aiRuntime.llm_model}</code>)</span>
                )}
              </div>
            )}

            {result.retry_count !== undefined && result.retry_count > 0 && (
              <div className="retry-badge">🔄 AI kendini düzeltti (deneme {result.retry_count}/3)</div>
            )}

            {result.needs_clarification && result.clarification_options && (
              <ClarificationCard question={result.clarification_question ?? 'Lütfen netleştirin.'} options={result.clarification_options} onSelect={handleClarificationSelect} onSkip={handleClarificationSkip} />
            )}

            {result.candidates && result.candidates.length > 1 && !result.needs_clarification && (
              <CandidateComparisonPanel candidates={result.candidates} onUse={(i: number) => {
                const c = result.candidates?.[i]
                if (c) setResult({ ...result, logical_query: c.logical_query, confidence: c.confidence })
              }} />
            )}

            {result.table_routing && (
              <Collapsible title="📋 Tablo Yönlendirme" defaultOpen>
                <TableRoutingViz routing={result.table_routing} />
                {(result.table_routing.selected_tables?.length ?? 0) > 0 && (
                  <button className="btn btn-sm btn-sample" onClick={() => { const t = result.table_routing?.selected_tables?.[0]; if (t) handleSampleData(t) }}>Örnek veriyi önizle</button>
                )}
              </Collapsible>
            )}

            {result.validation_result && (
              <Collapsible title={result.validation_result.plan_ok ? '✅ SQL Planı' : '⚠️ SQL Planı — Sorun bulundu'} defaultOpen={!result.validation_result.plan_ok}>
                {result.validation_result.explain_output && <pre className="sql-preview explain-output">{result.validation_result.explain_output}</pre>}
                <p className={`plan-status ${result.validation_result.plan_ok ? 'plan-ok' : 'plan-warn'}`}>
                  {result.validation_result.plan_ok ? 'Plan iyi görünüyor — sorgu güvenle çalıştırılacak.' : 'Planda uyarılar var. Çalıştırmadan önce inceleyin.'}
                </p>
              </Collapsible>
            )}

            {(result.logical_query?.select?.filter((s: any) => s.type === 'window') ?? []).length > 0 && (
              <div style={{ marginBottom: '0.5rem' }}>
                {(result.logical_query?.select ?? []).filter((s: any) => s.type === 'window').map((s: any, i: number) => (
                  <span key={i} className="wf-badge">Pencere fonksiyonu: {s.window?.aggregation || s.name}</span>
                ))}
              </div>
            )}

            {result.logical_query && (
              <Collapsible title="🧠 Oluşturulan LogicalQuery" defaultOpen>
                <pre className="sql-preview">{JSON.stringify(result.logical_query, null, 2)}</pre>
              </Collapsible>
            )}

            {result.sql && (
              <Collapsible title="📝 Derlenmiş SQL" defaultOpen>
                <pre className="sql-preview">{result.sql}</pre>
              </Collapsible>
            )}

            {result.prompt && (
              <Collapsible title="🔍 İstem metnini göster">
                <pre className="sql-preview prompt-preview">{result.prompt}</pre>
                {result.token_usage && (
                  <p className="token-info">
                    Token: {result.token_usage.prompt} istem · {result.token_usage.completion} tamamlama · {result.token_usage.total} toplam
                  </p>
                )}
                {result.token_usage && result.token_usage.prompt > 30000 && (
                  <p className="prompt-warning">
                    ⚠️ İstem büyük ({(result.token_usage.prompt / 1000).toFixed(1)}K token) — yanıt kalitesini etkileyebilir.
                  </p>
                )}
              </Collapsible>
            )}

            {result.warnings && result.warnings.length > 0 && (
              <section className="warning-panel" aria-live="polite">
                <div>
                  <strong>Uyarılar</strong>
                  <p>
                    AI, anlamsal modelle uyuşmayan bir sorgu şekli üretti. En iyi eşleşen tabloyu elle seçmeyi veya alanları net belirterek soruyu yeniden yazmayı deneyin.
                  </p>
                </div>
                <ul>
                  {result.warnings.map((w, i) => <li key={i}>{w}</li>)}
                </ul>
              </section>
            )}

            {result.retry_count !== undefined && result.retry_count >= 3 && !result.sql && (
              <div className="error-recovery">
                <p>AI, {result.retry_count} denemeden sonra geçerli bir sorgu üretemedi.</p>
                <div className="recovery-options">
                  <button onClick={() => document.getElementById('ai-question')?.focus()}>Soruyu yeniden yaz</button>
                  <a className="btn" href="/query-builder">Manuel sorgu oluşturucu</a>
                </div>
              </div>
            )}

            {/* Results */}
            {result.result?.columns && result.result.rows && (
              <div className="results-section">
                <div className="results-header">
                  <h3>Sonuçlar ({result.result.stats?.row_count ?? 0} satır)</h3>
                  {result.visualization_hint && (
                    <span className="viz-hint" title={result.visualization_hint.reason}>
                      💡 {result.visualization_hint.chart_type}
                    </span>
                  )}
                  <div className="chart-toggle">
                    {(['bar', 'line', 'pie', 'table'] as const).map((t) => (
                      <button key={t} className={chartType === t ? 'active' : ''} onClick={() => setChartType(t)}>
                        {t === 'table' ? 'Tablo' : t === 'bar' ? 'Çubuk' : t === 'line' ? 'Çizgi' : 'Pasta'}
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
              <span style={{ fontSize: '0.8rem', color: 'var(--text-secondary)', marginRight: '0.5rem' }}>Bu yanıt yararlı oldu mu?</span>
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
                <p style={{ fontSize: '0.8rem', marginBottom: '0.4rem', color: 'var(--text-secondary)' }}>Ne yanlış gitti?</p>
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
                  placeholder="Ek açıklama (isteğe bağlı)…"
                  rows={2}
                  style={{ width: '100%', fontSize: '0.8rem', resize: 'vertical' }}
                />
                <div style={{ display: 'flex', gap: '0.5rem', marginTop: '0.4rem' }}>
                  <button className="btn btn-sm btn-primary" onClick={submitNegativeFeedback}>Geri bildirimi gönder</button>
                  <button className="btn btn-sm btn-ghost" onClick={() => { setShowFeedbackForm(false); setFeedbackCategories([]); setFeedbackText('') }}>Vazgeç</button>
                </div>
              </div>
            )}
          </div>
        )}

        {/* Follow-up input (when conversation active) */}
        {activeConversation && activeConversation.messages.length > 0 && (
          <div className="follow-up-bar">
            <input
              placeholder="Takip sorusu yazın…"
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
    const summary = res.sql ? `SQL: ${res.sql.slice(0, 120)}…` : 'Sorgu çalıştırıldı'
    addMessage({ role: 'assistant', content: summary, ai_response: res })
  }
}
