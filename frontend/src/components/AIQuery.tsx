import { useEffect, useMemo, useRef, useState, type KeyboardEvent, type ReactNode } from 'react'
import { useApi } from '../hooks/useApi'
import { useConversation } from '../hooks/useConversation'
import { useQueryParam } from '../hooks/useQueryParam'
import { formatResultCell } from '../utils/resultCellFormat'
import { buildPivotTable } from '../utils/pivotTable'
import { ResultTable } from './ResultTable'
import { ChartContainer } from './ui/ChartContainer'
import { ChartTypeSelector } from './ui/ChartTypeSelector'
import { ErrorAlert } from './ui/ErrorAlert'
import { LoadingOverlay } from './ui/LoadingOverlay'
import { Select } from './ui/Select'
import { ModelBadgeRow } from './ui/ModelBadgeRow'
import { Modal } from './ui/Modal'
import type {
  AIQueryRequest,
  AIQueryResponse,
  Clarification,
  LogicalQuery,
  TableRoutingCandidate,
  LogicalQueryCandidate,
  AIRuntimeSettings,
  EmbedMetadataResponse,
  PriorTurn,
  SelectField,
  PromptStats,
  TokenUsage,
} from '../types/ai'
import type { Datasource } from '../types/metadata'
import { useLocale, useT, type Locale } from '../i18n'
import type { TranslationKey } from '../i18n'

/** NL→SQL can be slow with local models (routing, LLM, retries, EXPLAIN). */
const AI_QUERY_TIMEOUT_MS = 300_000
const AI_METADATA_EMBED_TIMEOUT_MS = 600_000

function localeNumberTag(locale: Locale): string {
  return locale === 'en' ? 'en-US' : 'tr-TR'
}

type TFunction = ReturnType<typeof useT>

function formatAiWaitElapsed(ms: number, t: TFunction): string {
  if (ms < 1000) return t('ai_query.wait_ms', { ms })
  const sec = ms / 1000
  if (sec < 60) return t('ai_query.wait_sec', { sec: Number(sec.toFixed(1)) })
  const m = Math.floor(sec / 60)
  const s = Math.floor(sec % 60)
  return t('ai_query.wait_min_sec', { m, s })
}

// ─── Sub-components ─────────────────────────────────────────────────

function ConfidenceBar({ value, breakdown }: { value: number; breakdown?: { table_routing: number; llm: number; validation: number } }) {
  const t = useT()
  const pct = Math.round(value * 100)
  const color = value > 0.8 ? 'var(--success)' : value > 0.5 ? 'var(--warning)' : 'var(--error)'
  return (
    <div className="confidence-section">
      <div className="confidence-header">
        <span>{t('ai_query.confidence')}</span>
        <span style={{ color, fontWeight: 600 }}>{pct}%</span>
      </div>
      <div className="confidence-bar-bg">
        <div className="confidence-bar-fill" style={{ width: `${pct}%`, backgroundColor: color }} />
      </div>
      {breakdown && (
        <div className="confidence-breakdown">
          <BreakdownRow label={t('ai_query.breakdown_table_routing')} value={breakdown.table_routing} />
          <BreakdownRow label={t('ai_query.breakdown_llm')} value={breakdown.llm} />
          <BreakdownRow label={t('ai_query.breakdown_validation')} value={breakdown.validation} />
        </div>
      )}
      {value < 0.5 && <p className="confidence-hint">{t('ai_query.confidence_low_hint')}</p>}
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

function routingMethodLabel(method: string | undefined, t: TFunction): string {
  const m = (method ?? 'keyword').toLowerCase()
  if (m === 'keyword') return t('ai_query.routing_method_keyword')
  if (m === 'vector') return t('ai_query.routing_method_vector')
  if (m === 'hybrid') return t('ai_query.routing_method_hybrid')
  if (m === 'manual') return t('ai_query.routing_method_manual')
  if (m === 'semantic') return t('ai_query.routing_method_semantic')
  return method ?? t('ai_query.routing_method_keyword')
}

function contextSourceLabel(source: string | undefined, t: TFunction): string {
  const s = (source ?? 'auto').toLowerCase()
  if (s === 'semantic_model') return t('ai_query.context_source_semantic_model')
  if (s === 'manual') return t('ai_query.context_source_manual')
  if (s === 'auto') return t('ai_query.context_source_auto')
  return source ?? t('ai_query.context_source_auto')
}

function compactItems(items: string[] | undefined, limit = 8) {
  if (!items || items.length === 0) return null
  const visible = items.slice(0, limit)
  const rest = items.length - visible.length
  return { visible, rest }
}

function compactList(items: string[] | undefined, limit = 8) {
  const compacted = compactItems(items, limit)
  if (!compacted) return null
  return `${compacted.visible.join(', ')}${compacted.rest > 0 ? ` +${compacted.rest}` : ''}`
}

function RoutingTableList({ items }: { items: string[] | undefined }) {
  const compacted = compactItems(items)
  if (!compacted) return null
  return (
    <strong className="routing-table-list">
      {compacted.visible.map((item) => <span key={item}>{item}</span>)}
      {compacted.rest > 0 && <span>+{compacted.rest}</span>}
    </strong>
  )
}

function RoutingDebugList({ items }: { items: string[] | undefined }) {
  const compacted = compactItems(items, 12)
  if (!compacted) return null
  return (
    <code className="routing-debug-list">
      {compacted.visible.map((item) => <span key={item}>{item}</span>)}
      {compacted.rest > 0 && <span>+{compacted.rest}</span>}
    </code>
  )
}

function TableRoutingViz({ routing }: { routing: NonNullable<AIQueryResponse['table_routing']> }) {
  const t = useT()
  const [locale] = useLocale()
  const localeTag = localeNumberTag(locale)
  const methodLabel = routingMethodLabel(routing.ranking_method, t)
  const sourceLabel = contextSourceLabel(routing.context_source, t)
  const candidateScore = (c: TableRoutingCandidate) => {
    const v = c.total_score ?? c.score ?? c.relevance_score
    return typeof v === 'number' && Number.isFinite(v) ? v : 0
  }
  const maxScore = Math.max(...(routing.candidates ?? []).map(candidateScore), 0)
  const selectedDims = compactList(routing.selected_dimensions)
  const selectedMetrics = compactList(routing.selected_metrics)
  const selectedTables = compactItems(routing.selected_tables)
  const selectedModels = compactList(routing.selected_models)
  return (
    <div className="table-routing-viz">
      <div className="routing-header">
        <span>{t('ai_query.routing_header', { method: methodLabel })}</span>
        <span className="routing-confidence">{Math.round(routing.confidence * 100)}%</span>
      </div>
      <div className="routing-context-grid">
        <div><span>{t('ai_query.routing_source')}</span><strong>{sourceLabel}</strong></div>
        {selectedModels && <div><span>{t('ai_query.routing_model')}</span><strong>{selectedModels}</strong></div>}
        {selectedTables && <div><span>{t('ai_query.routing_tables')}</span><RoutingTableList items={routing.selected_tables} /></div>}
        {selectedDims && <div><span>{t('ai_query.routing_dimensions')}</span><strong>{selectedDims}</strong></div>}
        {selectedMetrics && <div><span>{t('ai_query.routing_metrics')}</span><strong>{selectedMetrics}</strong></div>}
        {(routing.join_paths?.length ?? 0) > 0 && (
          <div><span>{t('ai_query.routing_join_paths')}</span><RoutingDebugList items={routing.join_paths} /></div>
        )}
        {routing.context_updated_at && (
          <div><span>{t('ai_query.routing_context_time')}</span><strong>{new Date(routing.context_updated_at).toLocaleString(localeTag)}</strong></div>
        )}
      </div>
      {(routing.candidates ?? []).map((c: TableRoutingCandidate) => {
        const score = candidateScore(c)
        const pct = maxScore > 0 ? Math.round((score / maxScore) * 100) : 0
        return (
          <div key={c.table} className="routing-candidate">
            <span className="routing-table-name">{c.table}</span>
            <div className="routing-bar-bg"><div className="routing-bar-fill" style={{ width: `${pct}%` }} /></div>
            <span className="routing-score">{score.toFixed(2)}</span>
            <span className={`routing-selected ${c.selected ? '' : 'routing-selected--empty'}`}>{c.selected ? '✓' : ''}</span>
            <span className="routing-score-detail">
              {t('ai_query.routing_score_k')}{(c.keyword_score ?? 0).toFixed(2)}
              {c.embedding_score !== undefined ? ` · ${t('ai_query.routing_score_e')}${c.embedding_score.toFixed(2)}` : ''}
            </span>
          </div>
        )
      })}
      {routing.debug && (
        <div className="routing-debug">
          {routing.debug.relation_expansion && routing.debug.relation_expansion.length > 0 && (
            <div><span>{t('ai_query.routing_debug_relation')}</span><code>{routing.debug.relation_expansion.join(' | ')}</code></div>
          )}
          {routing.debug.bridge_tables && routing.debug.bridge_tables.length > 0 && (
            <div><span>{t('ai_query.routing_debug_bridge')}</span><RoutingDebugList items={routing.debug.bridge_tables} /></div>
          )}
          {routing.debug.schema_partitions && routing.debug.schema_partitions.length > 0 && (
            <div><span>{t('ai_query.routing_debug_schema_parts')}</span><RoutingDebugList items={routing.debug.schema_partitions} /></div>
          )}
          {routing.debug.eliminated_candidates && routing.debug.eliminated_candidates.length > 0 && (
            <div><span>{t('ai_query.routing_debug_eliminated')}</span><RoutingDebugList items={routing.debug.eliminated_candidates} /></div>
          )}
        </div>
      )}
      {routing.reasoning && <p className="routing-reasoning">{routing.reasoning}</p>}
    </div>
  )
}

function ClarificationCard({
  question,
  options,
  clarification,
  onSelect,
  onSkip,
}: {
  question: string
  options: string[]
  clarification?: Clarification
  onSelect: (o: string) => void
  onSkip: () => void
}) {
  const t = useT()
  const structured = clarification?.options?.filter((o) => o.label?.trim()) ?? []
  const useStructured = structured.length > 0
  return (
    <div className="clarification-card">
      <div className="clarification-title">{t('ai_query.clarification_title')}</div>
      {clarification?.reason && <p className="clarification-reason">{clarification.reason}</p>}
      <p className="clarification-question">{question}</p>
      <div className="clarification-options">
        {useStructured
          ? structured.map((opt) => (
              <button
                key={opt.key}
                type="button"
                className="btn btn-clarification"
                title={opt.hint}
                onClick={() => onSelect(opt.label)}
              >
                {opt.label}
                {opt.hint ? <span className="clarification-option-hint">{opt.hint}</span> : null}
              </button>
            ))
          : options.map((opt) => (
              <button key={opt} type="button" className="btn btn-clarification" onClick={() => onSelect(opt)}>{opt}</button>
            ))}
      </div>
      <button type="button" className="btn btn-skip" onClick={onSkip}>{t('ai_query.clarification_skip')}</button>
    </div>
  )
}

function CandidateComparisonPanel({ candidates, onUse }: { candidates: LogicalQueryCandidate[]; onUse: (i: number) => void }) {
  const t = useT()
  const bestIdx = candidates.reduce((best, c, i) => (c.confidence > (candidates[best]?.confidence ?? 0) ? i : best), 0)
  return (
    <div className="candidate-panel">
      <div className="candidate-header"><span>{t('ai_query.candidates_header', { count: candidates.length })}</span></div>
      <div className="candidate-cards">
        {candidates.map((c, i) => {
          const isBest = i === bestIdx
          const pct = Math.round(c.confidence * 100)
          return (
            <div key={i} className={`candidate-card ${isBest ? 'candidate-best' : ''}`}>
              <div className="candidate-card-header">
                 <span>{t('ai_query.candidate_number', { n: i + 1 })}</span>
                 <span className={`candidate-score ${isBest ? 'score-best' : ''}`}>{t('ai_query.candidate_score', { pct })}</span>
              </div>
              {c.reasoning && <p className="candidate-reasoning">{c.reasoning}</p>}
              <details>
                <summary>{t('ai_query.logical_query_json')}</summary>
                <pre className="sql-preview candidate-json">{JSON.stringify(c.logical_query, null, 2)}</pre>
              </details>
              <button className="btn btn-candidate-use" onClick={() => onUse(i)}>{isBest ? t('ai_query.use_recommended') : t('ai_query.use_this')}</button>
            </div>
          )
        })}
      </div>
    </div>
  )
}

function CostBadge({ latencyMs, tokenUsage, costUsd }: { latencyMs?: number; tokenUsage?: { prompt: number; completion: number; total: number }; costUsd?: number }) {
  const t = useT()
  const [locale] = useLocale()
  const localeTag = localeNumberTag(locale)
  if (!latencyMs && !tokenUsage && costUsd === undefined) return null
  const parts: string[] = []
  if (latencyMs !== undefined && latencyMs > 0) parts.push(t('ai_query.cost_sec', { s: (latencyMs / 1000).toFixed(1) }))
  if (tokenUsage) {
    parts.push(t('ai_query.cost_tokens', { n: tokenUsage.total }))
  }
  if (costUsd !== undefined) parts.push(`$${costUsd.toFixed(4)}`)
  const tokenTitle = tokenUsage
    ? t('ai_query.cost_token_title', {
        prompt: tokenUsage.prompt.toLocaleString(localeTag),
        completion: tokenUsage.completion.toLocaleString(localeTag),
        total: tokenUsage.total.toLocaleString(localeTag),
      })
    : undefined
  return (
    <div className="cost-badge" title={tokenTitle}>
      {parts.join(' · ')}
      {tokenUsage && (
        <span className="cost-badge-detail">
          {' '}
          ({tokenUsage.prompt.toLocaleString(localeTag)} + {tokenUsage.completion.toLocaleString(localeTag)})
        </span>
      )}
    </div>
  )
}

function LogicalQueryMetaBadges({ lq }: { lq: LogicalQuery }) {
  const t = useT()
  const badges: string[] = []
  if (lq.default_schema?.trim()) badges.push(t('ai_query.lq_default_schema', { schema: lq.default_schema }))
  const schemaMap = lq.table_schemas ?? {}
  const mapped = Object.entries(schemaMap).filter(([, s]) => s?.trim())
  if (mapped.length > 0) {
    badges.push(t('ai_query.lq_schema_map', { map: mapped.map(([tb, s]) => `${tb}→${s}`).join(', ') }))
  }
  if ((lq.ctes?.length ?? 0) > 0) badges.push(t('ai_query.lq_cte', { n: lq.ctes!.length }))
  const caseCount = (lq.select ?? []).filter((s) => s.type === 'case').length
  if (caseCount > 0) badges.push(t('ai_query.lq_case', { n: caseCount }))
  if (badges.length === 0) return null
  return (
    <div className="lq-meta-badges">
      {badges.map((b) => (
        <span key={b} className="wf-badge">{b}</span>
      ))}
    </div>
  )
}

function PromptStatsPanel({ stats, tokenUsage }: { stats?: PromptStats; tokenUsage?: TokenUsage }) {
  const t = useT()
  const [locale] = useLocale()
  const localeTag = localeNumberTag(locale)
  if (!stats && !tokenUsage) return null
  return (
    <div className="prompt-stats-panel">
      {stats && (
        <>
          {stats.context_tier_label && (
            <span className="wf-badge" title={t('ai_query.prompt_context_tier_title', { tier: stats.context_tier ?? '' })}>
              {t('ai_query.prompt_context_label')} {stats.context_tier_label}
            </span>
          )}
          {stats.est_prompt_tokens !== undefined && (
            <span className="wf-badge" title={t('ai_query.prompt_est_tokens_title')}>
              {t('ai_query.prompt_est_tokens_badge', { n: stats.est_prompt_tokens.toLocaleString(localeTag) })}
            </span>
          )}
          {stats.context_window_tokens !== undefined && (
            <span className="wf-badge" title={t('ai_query.prompt_window_title')}>
              {t('ai_query.prompt_window_badge', { n: stats.context_window_tokens.toLocaleString(localeTag) })}
            </span>
          )}
          {stats.prompt_runes !== undefined && (
            <span className="wf-badge" title={t('ai_query.prompt_runes_title')}>
              {t('ai_query.prompt_runes_badge', { n: stats.prompt_runes.toLocaleString(localeTag) })}
            </span>
          )}
        </>
      )}
      {tokenUsage && stats?.est_prompt_tokens !== undefined && tokenUsage.prompt > 0 && (
        <span
          className="wf-badge"
          title={t('ai_query.prompt_token_compare_title')}
        >
          {t('ai_query.prompt_token_compare_badge', { n: tokenUsage.prompt.toLocaleString(localeTag) })}
        </span>
      )}
    </div>
  )
}

function Collapsible({ title, children, defaultOpen = false }: { title: string; children: ReactNode; defaultOpen?: boolean }) {
  return (
    <details open={defaultOpen} className="collapsible-section">
      <summary>{title}</summary>
      <div className="collapsible-content">{children}</div>
    </details>
  )
}

function embeddingSummary(response: EmbedMetadataResponse, t: TFunction): string {
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
    ? t('ai_query.embedding_details', { tables: counts.tables, columns: counts.columns })
    : ''
  return t('ai_query.embedding_summary', {
    embedded: response.embedded,
    details,
    model: response.model,
  })
}

interface SampleColumn {
  name: string
}

interface SampleData {
  columns: SampleColumn[]
  rows: unknown[][]
}

function SampleDataModal({ open, onClose, tableName, datasourceId, get }: { open: boolean; onClose: () => void; tableName: string; datasourceId: string; get: <T>(url: string) => Promise<T | null> }) {
  const t = useT()
  const [sample, setSample] = useState<SampleData | null>(null)
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    if (!open) { setSample(null); return }
    setLoading(true)
    const [schema, ...rest] = tableName.split('.')
    const tName = rest.length > 0 ? rest.join('.') : schema
    const url = `/api/datasources/${datasourceId}/tables/${schema ?? 'public'}/${tName}/sample`
    get<SampleData>(url).then((data) => { setSample(data); setLoading(false) })
  }, [datasourceId, get, open, tableName])

  return (
    <Modal open={open} title={t('ai_query.sample_modal_title', { table: tableName })} onClose={onClose} labelledBy="sample-data-title">
      <LoadingOverlay loading={loading} />
      {sample?.columns && sample?.rows && (
        <table className="results-table">
          <thead><tr>{sample.columns.map((c) => <th key={c.name}>{c.name}</th>)}</tr></thead>
          <tbody>
            {sample.rows.map((row, i) => (
              <tr key={i}>{row.map((cell, j) => <td key={j}>{formatResultCell(cell, sample.columns[j]?.name ?? '', {})}</td>)}</tr>
            ))}
          </tbody>
        </table>
      )}
    </Modal>
  )
}

const FEEDBACK_CAT_KEYS = [
  'ai_query.feedback_cat_wrong_table',
  'ai_query.feedback_cat_wrong_columns',
  'ai_query.feedback_cat_wrong_agg',
  'ai_query.feedback_cat_missing_date',
  'ai_query.feedback_cat_wrong_logic',
  'ai_query.feedback_cat_sql_error',
  'ai_query.feedback_cat_other',
] as const satisfies readonly TranslationKey[]

type FeedbackCatKey = (typeof FEEDBACK_CAT_KEYS)[number]

// ─── Main Component ─────────────────────────────────────────────────

interface TableOption {
  schema_name: string
  table_name: string
  table_type?: string
  description?: string | null
  label?: string
}

export default function AIQuery() {
  const t = useT()
  const [locale] = useLocale()
  const localeTag = localeNumberTag(locale)
  const { get, postData, loading, error, abort } = useApi()
  const { postData: postEmbedData, loading: embeddingLoading, error: embeddingError } = useApi()
  const { activeConversation, addMessage, createConversation, setActiveConversationId } = useConversation()

  // Datasource / table state
  const [datasources, setDatasources] = useState<Datasource[]>([])
  const [tables, setTables] = useState<TableOption[]>([])
  const [dsParam, setDsParam] = useQueryParam('ds')
  const [datasourceId, setDatasourceId] = useState(dsParam)
  const [semanticModels, setSemanticModels] = useState<{ id: string; name: string; label?: string | null; status: string }[]>([])
  const [semanticModelId, setSemanticModelId] = useState<string>('')
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
  const [tableView, setTableView] = useState<'flat' | 'pivot'>('flat')

  // Sample data modal
  const [sampleModalOpen, setSampleModalOpen] = useState(false)
  const [sampleModalTable, setSampleModalTable] = useState('')

  // Feedback state
  const [userFeedback, setUserFeedback] = useState<'positive' | 'negative' | null>(null)
  const [showFeedbackForm, setShowFeedbackForm] = useState(false)
  const [feedbackCategories, setFeedbackCategories] = useState<FeedbackCatKey[]>([])
  const [feedbackText, setFeedbackText] = useState('')

  // Include past queries toggle
  const [includePastQueries, setIncludePastQueries] = useState(false)

  /** Which primary action is in flight — avoids both buttons showing the same loading text */
  const [queryAction, setQueryAction] = useState<'preview' | 'execute' | null>(null)
  const [followUpBusy, setFollowUpBusy] = useState(false)
  const [aiElapsedMs, setAiElapsedMs] = useState(0)
  const aiBusy = queryAction !== null || followUpBusy

  useEffect(() => {
    if (!aiBusy) {
      setAiElapsedMs(0)
      return
    }
    const t0 = performance.now()
    setAiElapsedMs(0)
    const id = window.setInterval(() => {
      setAiElapsedMs(Math.round(performance.now() - t0))
    }, 200)
    return () => window.clearInterval(id)
  }, [aiBusy])

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
  }, [])

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
        setAiRuntimeErr(t('ai_query.err_settings_load'))
      }
    })
  }, [get, t])

  useEffect(() => {
    setSelectedTables([]); setTableSearch(''); setIncludeBaseTables(true); setIncludeViews(true); setTables([])
    setEmbeddingStatus(null)
    setSemanticModels([])
    setSemanticModelId('')
    if (!datasourceId) return
    get<TableOption[]>(`/api/datasources/${datasourceId}/tables`).then((data) => setTables(data || []))
    get<{ id: string; name: string; label?: string | null; status: string }[]>(
      `/api/semantic/models?datasource_id=${encodeURIComponent(datasourceId)}`,
    ).then((data) => setSemanticModels(data ?? []))
  }, [datasourceId])

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
    const turns: PriorTurn[] = []
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
    const body: { datasource_id: string; model_id?: string } = { datasource_id: datasourceId }
    if (semanticModelId) body.model_id = semanticModelId
    const res = await postEmbedData<EmbedMetadataResponse>(
      '/api/ai/metadata/embed',
      body,
      { timeout: AI_METADATA_EMBED_TIMEOUT_MS },
    )
    if (!res) return
    setEmbeddingStatus(embeddingSummary(res, t))
  }

  const requestBody = (q = question): AIQueryRequest => ({
    datasource_id: datasourceId,
    model_id: semanticModelId || undefined,
    question: q,
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
      const body = requestBody(q)
      const endpoint = execute ? '/api/ai/query/run' : '/api/ai/query/preview'
      const res = await postData<AIQueryResponse>(endpoint, body, { timeout: AI_QUERY_TIMEOUT_MS })
      if (!res) return
      setResult(res); setChartType('table')
      setUserFeedback(null); setShowFeedbackForm(false); setFeedbackCategories([]); setFeedbackText('')

      if (res.needs_clarification) {
        addMessage({ role: 'user', content: q })
        addMessage({ role: 'assistant', content: res.clarification_question ?? t('ai_query.clarify_default'), ai_response: res })
        return
      }
      addMessage({ role: 'user', content: q })
      const summary = res.sql
        ? t('ai_query.assistant_sql_preview', { snippet: res.sql.slice(0, 120) })
        : t('ai_query.assistant_executed')
      addMessage({ role: 'assistant', content: summary, ai_response: res })
    } finally {
      setQueryAction(null)
    }
  }

  const handleClarificationSelect = (opt: string) => sendQuery(`${question} (${opt})`, true)
  const handleClarificationSkip = () => sendQuery(question, true)

  const handleFilterByValue = (column: string, value: string) =>
    setQuestion((prev) => `${prev} ${t('ai_query.filter_by_value', { column, value })}`)

  const handleSampleData = (tableName: string) => { setSampleModalTable(tableName); setSampleModalOpen(true) }

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
        categories: feedbackCategories.map((k) => t(k)),
        text: feedbackText,
      })
    } catch { /* noop */ }
    setShowFeedbackForm(false)
    setFeedbackCategories([])
    setFeedbackText('')
  }

  const handleCellDrillDown = (column: string, value: string) => {
    sendQuery(t('ai_query.drill_down_prompt', { column, value }), true)
  }

  const chartData = result?.result?.rows?.map((row) => {
    const obj: { name: string; value?: number } = { name: String(row[0]) }
    if (row[1] !== undefined) obj.value = Number(row[1]) || 0
    return obj
  }) ?? []

  const pivotTable = useMemo(() => {
    const hint = result?.result?.pivot_hint
    const cols = result?.result?.columns
    const rows = result?.result?.rows
    if (!hint || !cols || !rows) return null
    return buildPivotTable(cols, rows, hint)
  }, [result?.result?.pivot_hint, result?.result?.columns, result?.result?.rows])

  useEffect(() => {
    if (pivotTable) setTableView('pivot')
    else setTableView('flat')
  }, [pivotTable, result?.logical_query?.model_id])

  useEffect(() => {
    const raw = result?.visualization_hint?.chart_type ?? result?.result?.chart_suggestions?.[0]
    if (!raw) return
    const mapped = raw === 'number' ? 'table' : raw
    if (mapped === 'bar' || mapped === 'line' || mapped === 'pie' || mapped === 'table') {
      setChartType(mapped)
    }
  }, [result?.visualization_hint?.chart_type, result?.result?.chart_suggestions])

  const loadingLabel = loading && (queryAction !== null || followUpBusy)
    ? result?.retry_count
      ? t('ai_query.loading_retry', { n: result.retry_count + 1 })
      : result?.candidates_count
        ? t('ai_query.loading_candidates')
        : t('ai_query.loading_thinking')
    : ''

  const previewButtonLabel = loading && queryAction === 'preview' ? loadingLabel : t('ai_query.preview_btn')
  const executeButtonLabel = loading && (queryAction === 'execute' || followUpBusy) ? loadingLabel : t('ai_query.execute_btn')

  return (
    <div className="ai-query-layout">
      {/* ─── Conversation Sidebar ─────────────────────────────── */}
      <aside className="conversation-sidebar">
        <div className="sidebar-header">
          <h3>{t('ai_query.conv_title')}</h3>
          <button className="btn btn-sm" onClick={() => { createConversation(); setResult(null); setQuestion('') }}>{t('ai_query.conv_new')}</button>
        </div>
        {activeConversation && (
          <>
            <button className="conversation-item active" onClick={() => setActiveConversationId(activeConversation.id)}>
              <span className="conv-title">{activeConversation.title ?? t('ai_query.conv_current')}</span>
              <span className="conv-time">{t('ai_query.conv_messages', { count: activeConversation.messages.length })}</span>
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
            <h2>{t('ai_query.natural_language_title')}</h2>
            {activeConversation && activeConversation.messages.length > 0 && (
              <div className="context-badge">{t('ai_query.context_badge', { count: activeConversation.messages.length })}</div>
            )}
          </div>
          <p className="card-subtitle">{t('ai_query.subtitle')}</p>
          <ModelBadgeRow
            primaryLabel={t('ai_query.model_badge_query')}
            primaryModel={
              aiRuntime?.query_model_override ? aiRuntime?.query_model : aiRuntime?.llm_model
            }
            primaryNote={
              aiRuntime?.query_model_override
                ? undefined
                : aiRuntime
                  ? t('ai_query.model_badge_legacy')
                  : undefined
            }
            embeddingModel={aiRuntime?.embeddings_enabled ? aiRuntime?.embedding_model : undefined}
            translationModel={aiRuntime?.translation_enabled ? aiRuntime?.translation_model : undefined}
            style={{ marginBottom: '0.5rem' }}
          />

          <div className="query-controls">
            <div className="form-group">
              <label htmlFor="ai-datasource">{t('ai_query.datasource_label')}</label>
              <Select
                id="ai-datasource"
                value={datasourceId}
                onChange={setDatasourceId}
                placeholder={t('ai_query.select_placeholder')}
                header={t('ai_query.header_datasources')}
                options={datasources.map((d) => ({ value: d.id, label: d.name, hint: d.type }))}
              />
            </div>
            <div className="form-group">
              <label htmlFor="ai-semantic-model">{t('ai_query.semantic_model_label')}</label>
              <Select
                id="ai-semantic-model"
                value={semanticModelId}
                onChange={setSemanticModelId}
                placeholder={t('ai_query.semantic_model_auto')}
                header={t('ai_query.semantic_model_header')}
                options={[
                  { value: '', label: t('ai_query.semantic_model_auto') },
                  ...semanticModels.map((m) => ({
                    value: m.id,
                    label: m.label || m.name,
                    hint: m.status,
                  })),
                ]}
              />
            </div>
            <div className="form-group routing-toggle">
              <label>{t('ai_query.table_routing_label')}</label>
              <div className="toggle-group">
                <button type="button" className={`toggle-btn ${autoTableRouting ? 'active' : ''}`} onClick={() => setAutoTableRouting(true)}>{t('ai_query.table_routing_auto')}</button>
                <button type="button" className={`toggle-btn ${!autoTableRouting ? 'active' : ''}`} onClick={() => setAutoTableRouting(false)}>{t('ai_query.table_routing_manual')}</button>
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
                        ? semanticModelId
                          ? t('ai_query.embed_title_model', { name: semanticModels.find((m) => m.id === semanticModelId)?.label || semanticModels.find((m) => m.id === semanticModelId)?.name || '' })
                          : t('ai_query.embed_title_ds', { name: selectedDatasourceName ?? '' })
                        : t('ai_query.embed_title_none')
                    }
                  >
                    {embeddingLoading
                      ? t('ai_query.embed_refreshing')
                      : semanticModelId
                        ? t('ai_query.embed_refresh_model')
                        : t('ai_query.embed_refresh')}
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
              <span className="ai-scope-label">{t('ai_query.scope_label')}</span>
              <div className="ai-scope-type-filters" role="group">
                <label className="ai-scope-type-option">
                  <input type="checkbox" checked={includeBaseTables} onChange={(e) => setIncludeBaseTables(e.target.checked)} disabled={!datasourceId || tables.length === 0} />
                  <span>{t('ai_query.scope_base_tables')}</span>
                </label>
                <label className="ai-scope-type-option">
                  <input type="checkbox" checked={includeViews} onChange={(e) => setIncludeViews(e.target.checked)} disabled={!datasourceId || tables.length === 0} />
                  <span>{t('ai_query.scope_views')}</span>
                </label>
              </div>
              <input value={tableSearch} onChange={(e) => setTableSearch(e.target.value)} placeholder={t('ai_query.table_search_placeholder')} disabled={!datasourceId || tables.length === 0} autoComplete="off" />
              <select aria-label={t('ai_query.selected_tables_aria')} multiple value={selectedTables} onChange={(e) => setSelectedTables(Array.from(e.target.selectedOptions, (o) => o.value))}
                disabled={!datasourceId || tables.length === 0 || (!includeBaseTables && !includeViews)}
                className="ai-scope-multiselect" size={Math.min(8, Math.max(3, filteredTables.length || 3))}>
                {filteredTables.map((table) => { const label = tableLabel(table); return <option key={label} value={label}>{label}</option> })}
              </select>
            </div>
          )}

          <div className="form-group">
            <label htmlFor="ai-question">{t('ai_query.question_label')}</label>
            <textarea id="ai-question" value={question} onChange={(e) => setQuestion(e.target.value)}
              onKeyDown={(e: KeyboardEvent<HTMLTextAreaElement>) => { if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) { e.preventDefault(); sendQuery(question, true) } }}
              placeholder={t('ai_query.placeholder')} rows={3} autoComplete="off" />
            {activeConversation && activeConversation.messages.length > 0 && (
              <div className="past-queries-toggle">
                <input type="checkbox" id="include-past" checked={includePastQueries} onChange={(e) => setIncludePastQueries(e.target.checked)} />
                <label htmlFor="include-past">{t('ai_query.include_past_checkbox')}</label>
              </div>
            )}
          </div>

          <div className="button-row">
            <button className="btn" onClick={() => sendQuery(question, false)} disabled={loading || !question || !datasourceId}>{previewButtonLabel}</button>
            <button className="btn btn-primary" onClick={() => sendQuery(question, true)} disabled={loading || !question || !datasourceId}>{executeButtonLabel}</button>
            {loading && (queryAction !== null || followUpBusy) && <button className="btn btn-ghost" onClick={abort}>{t('ai_query.cancel')}</button>}
          </div>
          {aiBusy && (
            <div className="ai-wait-meta" role="status" aria-live="polite">
              <span className="ai-wait-meta-time">{t('ai_query.elapsed_label')} {formatAiWaitElapsed(aiElapsedMs, t)}</span>
              <span className="ai-wait-meta-hint">
                {t('ai_query.wait_hint', { minutes: Math.round(AI_QUERY_TIMEOUT_MS / 60_000) })}
              </span>
            </div>
          )}
          <ErrorAlert error={error} className="error--top-gap" />
        </div>

        {/* ─── Result Card ──────────────────────────────────────── */}
        {result && (
          <div className="card result-card">
            {result.confidence !== undefined && <ConfidenceBar value={result.confidence} breakdown={result.confidence_breakdown} />}
            <CostBadge latencyMs={result.latency_ms} tokenUsage={result.token_usage} costUsd={result.cost_usd} />
            <PromptStatsPanel stats={result.prompt_stats} tokenUsage={result.token_usage} />

            {result.model_used && (
              <div className="model-used-badge" style={{ fontSize: '0.8rem', color: 'var(--text-secondary)', marginBottom: '0.5rem' }}>
                {t('ai_query.model_used')} <code translate="no">{result.model_used}</code>
                {aiRuntime?.query_model_override && aiRuntime.query_model && result.model_used !== aiRuntime.query_model && (
                  <span> ({t('ai_query.configured')} <code translate="no">{aiRuntime.query_model}</code>)</span>
                )}
                {!aiRuntime?.query_model_override && aiRuntime?.llm_model && result.model_used !== aiRuntime.llm_model && (
                  <span> ({t('ai_query.configured')} <code translate="no">{aiRuntime.llm_model}</code>)</span>
                )}
              </div>
            )}

            {result.retry_count !== undefined && result.retry_count > 0 && (
              <div className="retry-badge">{t('ai_query.retry_badge', { n: result.retry_count })}</div>
            )}

            {result.needs_clarification && (result.clarification_options?.length || result.clarification?.options?.length) ? (
              <ClarificationCard
                question={result.clarification?.question ?? result.clarification_question ?? t('ai_query.clarify_default')}
                options={result.clarification_options ?? result.clarification?.options?.map((o) => o.label) ?? []}
                clarification={result.clarification}
                onSelect={handleClarificationSelect}
                onSkip={handleClarificationSkip}
              />
            ) : null}

            {result.candidates && result.candidates.length > 1 && !result.needs_clarification && (
              <CandidateComparisonPanel candidates={result.candidates} onUse={(i: number) => {
                const c = result.candidates?.[i]
                if (c) setResult({ ...result, logical_query: c.logical_query, confidence: c.confidence })
              }} />
            )}

            {result.table_routing && (
              <Collapsible title={t('ai_query.collapsible_routing')} defaultOpen>
                <TableRoutingViz routing={result.table_routing} />
                {(result.table_routing.selected_tables?.length ?? 0) > 0 && (
                  <button
                    type="button"
                    className="btn btn-sm btn-sample"
                    onClick={() => {
                      const firstSel = result.table_routing?.selected_tables?.[0]
                      if (firstSel) handleSampleData(firstSel)
                    }}
                  >
                    {t('ai_query.sample_preview_btn')}
                  </button>
                )}
              </Collapsible>
            )}

            {result.validation_result && (
              <Collapsible
                title={result.validation_result.plan_ok ? t('ai_query.plan_ok_title') : t('ai_query.plan_warn_title')}
                defaultOpen={!result.validation_result.plan_ok}
              >
                {result.validation_result.explain_output && <pre className="sql-preview explain-output">{result.validation_result.explain_output}</pre>}
                <p className={`plan-status ${result.validation_result.plan_ok ? 'plan-ok' : 'plan-warn'}`}>
                  {result.validation_result.plan_ok ? t('ai_query.plan_ok_body') : t('ai_query.plan_warn_body')}
                </p>
              </Collapsible>
            )}

            {(result.logical_query?.select?.filter((s): s is SelectField & { type: 'window' } => s.type === 'window') ?? []).length > 0 && (
              <div style={{ marginBottom: '0.5rem' }}>
                {(result.logical_query?.select ?? []).filter((s): s is SelectField & { type: 'window' } => s.type === 'window').map((s, i) => (
                  <span key={i} className="wf-badge">{t('ai_query.window_fn_badge', { name: s.window?.aggregation || s.name })}</span>
                ))}
              </div>
            )}

            {result.logical_query && (
              <Collapsible title={t('ai_query.collapsible_lq')} defaultOpen>
                <LogicalQueryMetaBadges lq={result.logical_query} />
                <pre className="sql-preview">{JSON.stringify(result.logical_query, null, 2)}</pre>
              </Collapsible>
            )}

            {result.sql && (
              <Collapsible title={t('ai_query.collapsible_sql')} defaultOpen>
                <pre className="sql-preview">{result.sql}</pre>
              </Collapsible>
            )}

            {result.prompt && (
              <Collapsible title={t('ai_query.collapsible_prompt')}>
                <pre className="sql-preview prompt-preview">{result.prompt}</pre>
                {result.token_usage && (
                  <p className="token-info">
                    {t('ai_query.token_line', {
                      prompt: result.token_usage.prompt.toLocaleString(localeTag),
                      completion: result.token_usage.completion.toLocaleString(localeTag),
                      total: result.token_usage.total.toLocaleString(localeTag),
                    })}
                  </p>
                )}
                {result.token_usage && result.token_usage.prompt > 30000 && (
                  <p className="prompt-warning">
                    {t('ai_query.prompt_large_warning', { k: (result.token_usage.prompt / 1000).toFixed(1) })}
                  </p>
                )}
              </Collapsible>
            )}

            {result.warnings && result.warnings.length > 0 && (
              <section className="warning-panel" aria-live="polite">
                <div>
                  <strong>{t('ai_query.warnings_title')}</strong>
                  <p>{t('ai_query.warnings_body')}</p>
                </div>
                <ul>
                  {result.warnings.map((w, i) => <li key={i}>{w}</li>)}
                </ul>
              </section>
            )}

            {result.retry_count !== undefined && result.retry_count >= 3 && !result.sql && (
              <div className="error-recovery">
                <p>{t('ai_query.recovery_failed', { n: result.retry_count })}</p>
                <div className="recovery-options">
                  <button type="button" onClick={() => document.getElementById('ai-question')?.focus()}>{t('ai_query.recovery_rewrite')}</button>
                  <a className="btn" href="/query-builder">{t('ai_query.recovery_query_builder')}</a>
                </div>
              </div>
            )}

            {/* Results */}
            {result.result?.columns && result.result.rows && (
              <div className="results-section">
                <div className="results-header">
                  <h3>{t('ai_query.results_title', { rows: result.result.stats?.row_count ?? 0 })}</h3>
                  {result.visualization_hint && (
                    <span className="viz-hint" title={result.visualization_hint.reason}>
                      💡 {result.visualization_hint.chart_type}
                    </span>
                  )}
                  {result.result.pivot_hint && (
                    <span className="viz-hint" title={result.result.pivot_hint.reason ?? ''}>
                      ↕ {result.result.pivot_hint.row_field} × {result.result.pivot_hint.column_field}
                    </span>
                  )}
                  {(result.result.anomalies?.length ?? 0) > 0 && (
                    <span className="viz-hint" title={t('ai_query.anomalies_title')}>
                      {t('ai_query.anomalies_badge', { count: result.result.anomalies!.length })}
                    </span>
                  )}
                  {pivotTable && (
                    <div className="chart-toggle">
                      <button
                        type="button"
                        className={tableView === 'flat' ? 'active' : ''}
                        onClick={() => setTableView('flat')}
                      >
                        {t('ai_query.pivot_flat')}
                      </button>
                      <button
                        type="button"
                        className={tableView === 'pivot' ? 'active' : ''}
                        onClick={() => setTableView('pivot')}
                      >
                        {t('ai_query.pivot_pivot')}
                      </button>
                    </div>
                  )}
                  <ChartTypeSelector
                    value={chartType}
                    onChange={setChartType}
                    options={['bar', 'line', 'pie', 'table'] as const}
                    ariaLabel={t('ai_query.chart_type_aria')}
                    labels={{
                      bar: t('ai_query.chart_bar'),
                      line: t('ai_query.chart_line'),
                      pie: t('ai_query.chart_pie'),
                      table: t('ai_query.chart_table'),
                    }}
                  />
                </div>

                {chartType !== 'table' && chartData.length > 0 && (
                  <ChartContainer data={chartData} type={chartType} />
                )}

                {chartType === 'table' && (() => {
                  const flat = {
                    columns: result.result.columns,
                    rows: result.result.rows,
                  }
                  const view =
                    tableView === 'pivot' && pivotTable
                      ? pivotTable
                      : flat
                  return (
                    <ResultTable
                      columns={view.columns}
                      rows={view.rows}
                      rowCount={view.rows.length}
                      durationMs={result.result.stats?.duration_ms}
                      question={question}
                      anomalies={tableView === 'flat' ? result.result.anomalies : undefined}
                      onFilterByValue={tableView === 'flat' ? handleFilterByValue : undefined}
                      onCellClick={
                        tableView === 'flat'
                          ? (colName, value) => handleCellDrillDown(colName, String(value))
                          : undefined
                      }
                    />
                  )
                })()}
              </div>
            )}

            {/* Feedback */}
            <div className="feedback-row">
              <span style={{ fontSize: '0.8rem', color: 'var(--text-secondary)', marginRight: '0.5rem' }}>{t('ai_query.feedback_helpful')}</span>
              <button
                type="button"
                className={`feedback-btn ${userFeedback === 'positive' ? 'feedback-active' : ''}`}
                onClick={() => submitFeedback('positive')}
              >👍</button>
              <button
                type="button"
                className={`feedback-btn ${userFeedback === 'negative' ? 'feedback-negative' : ''}`}
                onClick={() => submitFeedback('negative')}
              >👎</button>
            </div>
            {showFeedbackForm && (
              <div className="feedback-form">
                <p style={{ fontSize: '0.8rem', marginBottom: '0.4rem', color: 'var(--text-secondary)' }}>{t('ai_query.feedback_what_wrong')}</p>
                <div className="feedback-categories">
                  {FEEDBACK_CAT_KEYS.map((cat) => (
                    <button
                      type="button"
                      key={cat}
                      className={`feedback-cat-btn ${feedbackCategories.includes(cat) ? 'feedback-active' : ''}`}
                      onClick={() =>
                        setFeedbackCategories((prev) =>
                          prev.includes(cat) ? prev.filter((c) => c !== cat) : [...prev, cat],
                        )
                      }
                    >
                      {t(cat)}
                    </button>
                  ))}
                </div>
                <textarea
                  value={feedbackText}
                  onChange={(e) => setFeedbackText(e.target.value)}
                  placeholder={t('ai_query.feedback_placeholder')}
                  rows={2}
                  style={{ width: '100%', fontSize: '0.8rem', resize: 'vertical' }}
                />
                <div style={{ display: 'flex', gap: '0.5rem', marginTop: '0.4rem' }}>
                  <button type="button" className="btn btn-sm btn-primary" onClick={submitNegativeFeedback}>{t('ai_query.feedback_submit')}</button>
                  <button
                    type="button"
                    className="btn btn-sm btn-ghost"
                    onClick={() => { setShowFeedbackForm(false); setFeedbackCategories([]); setFeedbackText('') }}
                  >
                    {t('ai_query.feedback_cancel')}
                  </button>
                </div>
              </div>
            )}
          </div>
        )}

        {/* Follow-up input (when conversation active) */}
        {activeConversation && activeConversation.messages.length > 0 && (
          <div className="follow-up-bar">
            <input
              placeholder={t('ai_query.follow_up_placeholder')}
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
    setFollowUpBusy(true)
    try {
      const res = await postData<AIQueryResponse>(
        '/api/ai/query/run',
        { ...requestBody(), question: followUp, conversation_id: activeConversation?.id },
        { timeout: AI_QUERY_TIMEOUT_MS },
      )
      if (!res) return
      setResult(res)
      addMessage({ role: 'user', content: followUp })
      const summary = res.sql
        ? t('ai_query.assistant_sql_preview', { snippet: res.sql.slice(0, 120) })
        : t('ai_query.assistant_executed')
      addMessage({ role: 'assistant', content: summary, ai_response: res })
    } finally {
      setFollowUpBusy(false)
    }
  }
}
