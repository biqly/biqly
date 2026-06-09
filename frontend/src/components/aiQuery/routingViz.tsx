/* eslint-disable react-refresh/only-export-components */
import type { ReactNode } from 'react'

import type { TranslationKey } from '../../i18n'
import { useLocale, useT } from '../../i18n'
import type {
  AIQueryResponse,
  Clarification,
  EmbedMetadataResponse,
  GenerationTrace,
  LogicalQuery,
  LogicalQueryCandidate,
  PromptStats,
  TableRoutingCandidate,
  TokenUsage,
} from '../../types/ai'
import { localeNumberTag } from '../../utils/formatters'
import { GenerationTracePanel } from './generationTrace'

type TFn = ReturnType<typeof useT>

export function formatAiWaitElapsed(ms: number, t: TFn): string {
  if (ms < 1000) {
    return t('ai_query.wait_ms', { ms })
  }
  const sec = ms / 1000
  if (sec < 60) {
    return t('ai_query.wait_sec', { sec: Number(sec.toFixed(1)) })
  }
  const m = Math.floor(sec / 60)
  const s = Math.floor(sec % 60)
  return t('ai_query.wait_min_sec', { m, s })
}

export function warningBodyKey(result: AIQueryResponse): TranslationKey {
  const hasQueryShapeWarning = result.warnings?.some((warning) =>
    /validation|semantic|unknown (dimension|field|metric)|ambiguous|dry-run|compilation|compile/i.test(
      warning,
    ),
  )
  if (result.sql && !hasQueryShapeWarning) {
    return 'ai_query.warnings_body_success'
  }
  return 'ai_query.warnings_body'
}

export function routingMethodLabel(method: string | undefined, t: TFn): string {
  const m = (method ?? 'keyword').toLowerCase()
  if (m === 'keyword') {
    return t('ai_query.routing_method_keyword')
  }
  if (m === 'vector') {
    return t('ai_query.routing_method_vector')
  }
  if (m === 'hybrid') {
    return t('ai_query.routing_method_hybrid')
  }
  if (m === 'manual') {
    return t('ai_query.routing_method_manual')
  }
  if (m === 'semantic') {
    return t('ai_query.routing_method_semantic')
  }
  return method ?? t('ai_query.routing_method_keyword')
}

export function contextSourceLabel(source: string | undefined, t: TFn): string {
  const s = (source ?? 'auto').toLowerCase()
  if (s === 'semantic_model') {
    return t('ai_query.context_source_semantic_model')
  }
  if (s === 'manual') {
    return t('ai_query.context_source_manual')
  }
  if (s === 'auto') {
    return t('ai_query.context_source_auto')
  }
  return source ?? t('ai_query.context_source_auto')
}

export function compactItems(items: string[] | undefined, limit = 8) {
  if (!items || items.length === 0) {
    return null
  }
  const visible = items.slice(0, limit)
  const rest = items.length - visible.length
  return { visible, rest }
}

export function compactList(items: string[] | undefined, limit = 8) {
  const compacted = compactItems(items, limit)
  if (!compacted) {
    return null
  }
  return `${compacted.visible.join(', ')}${compacted.rest > 0 ? ` +${compacted.rest}` : ''}`
}

export function embeddingSummary(response: EmbedMetadataResponse, t: TFn): string {
  const tableKeys = new Set<string>()
  const columnKeys = new Set<string>()
  for (const item of response.results ?? []) {
    if (item.skipped) {
      continue
    }
    const kind = item.kind ?? 'table'
    if (kind === 'column') {
      columnKeys.add(`${item.schema}.${item.table}.${item.column ?? ''}`)
    } else {
      tableKeys.add(`${item.schema}.${item.table}`)
    }
  }
  const tables = tableKeys.size
  const columns = columnKeys.size
  const vectors = response.embedded
  const unique = tables + columns
  const locales = unique > 0 ? Math.max(1, Math.round(vectors / unique)) : 1
  return t('ai_query.embedding_summary', {
    tables,
    columns,
    vectors,
    locales,
    model: response.model,
  })
}

export function ConfidenceBar({
  value,
  breakdown,
}: {
  value: number
  breakdown?: { table_routing: number; llm: number; validation: number }
}) {
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
          <BreakdownRow
            label={t('ai_query.breakdown_table_routing')}
            value={breakdown.table_routing}
          />
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
      <div className="breakdown-bar-bg">
        <div className="breakdown-bar-fill" style={{ width: `${pct}%`, backgroundColor: color }} />
      </div>
      <span style={{ minWidth: 36, textAlign: 'right', fontSize: 12, color }}>{pct}%</span>
    </div>
  )
}

function RoutingTableList({ items }: { items: string[] | undefined }) {
  const compacted = compactItems(items)
  if (!compacted) {
    return null
  }
  return (
    <strong className="routing-table-list">
      {compacted.visible.map((item) => (
        <span key={item}>{item}</span>
      ))}
      {compacted.rest > 0 && <span>+{compacted.rest}</span>}
    </strong>
  )
}

function RoutingDebugList({ items }: { items: string[] | undefined }) {
  const compacted = compactItems(items, 12)
  if (!compacted) {
    return null
  }
  return (
    <code className="routing-debug-list">
      {compacted.visible.map((item) => (
        <span key={item}>{item}</span>
      ))}
      {compacted.rest > 0 && <span>+{compacted.rest}</span>}
    </code>
  )
}

function routingCandidateScore(c: TableRoutingCandidate): number {
  const v = c.total_score ?? c.score ?? c.relevance_score
  return typeof v === 'number' && Number.isFinite(v) ? v : 0
}

function RoutingCandidatesList({
  candidates,
  maxScore,
  t,
}: {
  candidates: TableRoutingCandidate[]
  maxScore: number
  t: ReturnType<typeof useT>
}) {
  return (
    <>
      {candidates.map((c) => {
        const score = routingCandidateScore(c)
        const pct = maxScore > 0 ? Math.round((score / maxScore) * 100) : 0
        return (
          <div key={c.table} className="routing-candidate">
            <span className="routing-table-name">{c.table}</span>
            <div className="routing-bar-bg">
              <div className="routing-bar-fill" style={{ width: `${pct}%` }} />
            </div>
            <span className="routing-score">{score.toFixed(2)}</span>
            <span className={`routing-selected ${c.selected ? '' : 'routing-selected--empty'}`}>
              {c.selected ? '✓' : ''}
            </span>
            <span className="routing-score-detail">
              {t('ai_query.routing_score_k')}
              {(c.keyword_score ?? 0).toFixed(2)}
              {c.embedding_score !== undefined
                ? ` · ${t('ai_query.routing_score_e')}${c.embedding_score.toFixed(2)}`
                : ''}
            </span>
          </div>
        )
      })}
    </>
  )
}

function RoutingDebugPanel({
  debug,
  t,
}: {
  debug: NonNullable<NonNullable<AIQueryResponse['table_routing']>['debug']>
  t: ReturnType<typeof useT>
}) {
  return (
    <div className="routing-debug">
      {debug.relation_expansion && debug.relation_expansion.length > 0 && (
        <div>
          <span>{t('ai_query.routing_debug_relation')}</span>
          <code>{debug.relation_expansion.join(' | ')}</code>
        </div>
      )}
      {debug.bridge_tables && debug.bridge_tables.length > 0 && (
        <div>
          <span>{t('ai_query.routing_debug_bridge')}</span>
          <RoutingDebugList items={debug.bridge_tables} />
        </div>
      )}
      {debug.schema_partitions && debug.schema_partitions.length > 0 && (
        <div>
          <span>{t('ai_query.routing_debug_schema_parts')}</span>
          <RoutingDebugList items={debug.schema_partitions} />
        </div>
      )}
      {debug.eliminated_candidates && debug.eliminated_candidates.length > 0 && (
        <div>
          <span>{t('ai_query.routing_debug_eliminated')}</span>
          <RoutingDebugList items={debug.eliminated_candidates} />
        </div>
      )}
    </div>
  )
}

function RoutingContextSummary({
  routing,
  localeTag,
  t,
}: {
  routing: NonNullable<AIQueryResponse['table_routing']>
  localeTag: string
  t: ReturnType<typeof useT>
}) {
  const sourceLabel = contextSourceLabel(routing.context_source, t)
  const selectedDims = compactList(routing.selected_dimensions)
  const selectedMetrics = compactList(routing.selected_metrics)
  const selectedTables = compactItems(routing.selected_tables)
  const selectedModels = compactList(routing.selected_models)

  return (
    <div className="routing-context-grid">
      <div>
        <span>{t('ai_query.routing_source')}</span>
        <strong>{sourceLabel}</strong>
      </div>
      {selectedModels && (
        <div>
          <span>{t('ai_query.routing_model')}</span>
          <strong>{selectedModels}</strong>
        </div>
      )}
      {selectedTables && (
        <div>
          <span>{t('ai_query.routing_tables')}</span>
          <RoutingTableList items={routing.selected_tables} />
        </div>
      )}
      {selectedDims && (
        <div>
          <span>{t('ai_query.routing_dimensions')}</span>
          <strong>{selectedDims}</strong>
        </div>
      )}
      {selectedMetrics && (
        <div>
          <span>{t('ai_query.routing_metrics')}</span>
          <strong>{selectedMetrics}</strong>
        </div>
      )}
      {(routing.join_paths?.length ?? 0) > 0 && (
        <div>
          <span>{t('ai_query.routing_join_paths')}</span>
          <RoutingDebugList items={routing.join_paths} />
        </div>
      )}
      {routing.context_updated_at && (
        <div>
          <span>{t('ai_query.routing_context_time')}</span>
          <strong>{new Date(routing.context_updated_at).toLocaleString(localeTag)}</strong>
        </div>
      )}
    </div>
  )
}

export function TableRoutingViz({
  routing,
}: {
  routing: NonNullable<AIQueryResponse['table_routing']>
}) {
  const t = useT()
  const [locale] = useLocale()
  const localeTag = localeNumberTag(locale)
  const methodLabel = routingMethodLabel(routing.ranking_method, t)
  const candidates = routing.candidates ?? []
  const maxScore = Math.max(...candidates.map(routingCandidateScore), 0)

  return (
    <div className="table-routing-viz">
      <div className="routing-header">
        <span>{t('ai_query.routing_header', { method: methodLabel })}</span>
        <span className="routing-confidence">{Math.round(routing.confidence * 100)}%</span>
      </div>
      <RoutingContextSummary routing={routing} localeTag={localeTag} t={t} />
      <RoutingCandidatesList candidates={candidates} maxScore={maxScore} t={t} />
      {routing.debug && <RoutingDebugPanel debug={routing.debug} t={t} />}
      {routing.reasoning && <p className="routing-reasoning">{routing.reasoning}</p>}
    </div>
  )
}

export function ClarificationCard({
  question,
  options,
  clarification,
  generationTrace,
  interactiveTier = false,
  capReached = false,
  onSelect,
  onSkip,
}: {
  question: string
  options: string[]
  clarification?: Clarification
  generationTrace?: GenerationTrace
  interactiveTier?: boolean
  capReached?: boolean
  onSelect: (choice: string) => void
  onSkip: () => void
}) {
  const t = useT()
  const structured = (clarification?.options ?? []).filter((o) => o.label.trim())
  const useStructured = structured.length > 0
  const isAmbiguity = clarification?.source === 'ambiguity_analyzer'
  const ambiguities = clarification?.ambiguity_detail?.ambiguities ?? []
  return (
    <div className={`clarification-card${isAmbiguity ? ' clarification-card--ambiguity' : ''}`}>
      <div className="clarification-title">{t('ai_query.clarification_title')}</div>
      {interactiveTier && (
        <p className="clarification-cap-notice clarification-cap-notice--interactive" role="status">
          {t('ai_query.clarification_interactive_tier')}
        </p>
      )}
      {capReached && (
        <p className="clarification-cap-notice" role="status">
          {t('ai_query.clarification_cap_reached')}
        </p>
      )}
      {clarification?.reason && <p className="clarification-reason">{clarification.reason}</p>}
      {isAmbiguity && ambiguities.length > 0 ? (
        <ul className="clarification-ambiguity-terms">
          {ambiguities.map((item) => (
            <li key={item.term}>
              <strong>{item.term}</strong>
              <span className="clarification-ambiguity-type">{item.type}</span>
            </li>
          ))}
        </ul>
      ) : null}
      <p className="clarification-question">{question}</p>
      <div className="clarification-options">
        {useStructured
          ? structured.map((opt) => (
              <button
                key={opt.key}
                type="button"
                className="btn btn-clarification"
                title={opt.hint}
                onClick={() => onSelect(opt.key || opt.label)}
              >
                {opt.label}
                {opt.hint ? <span className="clarification-option-hint">{opt.hint}</span> : null}
              </button>
            ))
          : options.map((opt) => (
              <button
                key={opt}
                type="button"
                className="btn btn-clarification"
                onClick={() => onSelect(opt)}
              >
                {opt}
              </button>
            ))}
      </div>
      <button type="button" className="btn btn-skip" onClick={onSkip}>
        {t('ai_query.clarification_skip')}
      </button>
      {generationTrace ? <GenerationTracePanel trace={generationTrace} defaultOpen /> : null}
    </div>
  )
}

export function CandidateComparisonPanel({
  candidates,
  onUse,
}: {
  candidates: LogicalQueryCandidate[]
  onUse: (i: number) => void
}) {
  const t = useT()
  const bestIdx = candidates.reduce(
    (best, c, i) => (c.confidence > (candidates[best]?.confidence ?? 0) ? i : best),
    0,
  )
  return (
    <div className="candidate-panel">
      <div className="candidate-header">
        <span>{t('ai_query.candidates_header', { count: candidates.length })}</span>
      </div>
      <div className="candidate-cards">
        {candidates.map((c, i) => {
          const isBest = i === bestIdx
          const pct = Math.round(c.confidence * 100)
          return (
            <div key={i} className={`candidate-card ${isBest ? 'candidate-best' : ''}`}>
              <div className="candidate-card-header">
                <span>{t('ai_query.candidate_number', { n: i + 1 })}</span>
                <span className={`candidate-score ${isBest ? 'score-best' : ''}`}>
                  {t('ai_query.candidate_score', { pct })}
                </span>
              </div>
              {c.reasoning && <p className="candidate-reasoning">{c.reasoning}</p>}
              <details>
                <summary>{t('ai_query.logical_query_json')}</summary>
                <pre className="sql-preview candidate-json">
                  {JSON.stringify(c.logical_query, null, 2)}
                </pre>
              </details>
              <button className="btn btn-candidate-use" onClick={() => onUse(i)}>
                {isBest ? t('ai_query.use_recommended') : t('ai_query.use_this')}
              </button>
            </div>
          )
        })}
      </div>
    </div>
  )
}

export function CostBadge({
  latencyMs,
  tokenUsage,
  costUsd,
}: {
  latencyMs?: number
  tokenUsage?: { prompt: number; completion: number; total: number }
  costUsd?: number
}) {
  const t = useT()
  const [locale] = useLocale()
  const localeTag = localeNumberTag(locale)
  if (!latencyMs && !tokenUsage && costUsd === undefined) {
    return null
  }
  const parts: string[] = []
  if (latencyMs !== undefined && latencyMs > 0) {
    parts.push(t('ai_query.cost_sec', { s: (latencyMs / 1000).toFixed(1) }))
  }
  if (tokenUsage) {
    parts.push(t('ai_query.cost_tokens', { n: tokenUsage.total }))
  }
  if (costUsd !== undefined) {
    parts.push(`$${costUsd.toFixed(4)}`)
  }
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
          ({tokenUsage.prompt.toLocaleString(localeTag)} +{' '}
          {tokenUsage.completion.toLocaleString(localeTag)})
        </span>
      )}
    </div>
  )
}

export function LogicalQueryMetaBadges({ lq }: { lq: LogicalQuery }) {
  const t = useT()
  const badges: string[] = []
  if (lq.default_schema?.trim()) {
    badges.push(t('ai_query.lq_default_schema', { schema: lq.default_schema }))
  }
  const schemaMap = lq.table_schemas ?? {}
  const mapped = Object.entries(schemaMap).filter(([, s]) => s.trim())
  if (mapped.length > 0) {
    badges.push(
      t('ai_query.lq_schema_map', { map: mapped.map(([tb, s]) => `${tb}→${s}`).join(', ') }),
    )
  }
  if ((lq.ctes?.length ?? 0) > 0) {
    badges.push(t('ai_query.lq_cte', { n: lq.ctes!.length }))
  }
  const caseCount = (lq.select ?? []).filter((s) => s.type === 'case').length
  if (caseCount > 0) {
    badges.push(t('ai_query.lq_case', { n: caseCount }))
  }
  if (badges.length === 0) {
    return null
  }
  return (
    <div className="lq-meta-badges">
      {badges.map((b) => (
        <span key={b} className="wf-badge">
          {b}
        </span>
      ))}
    </div>
  )
}

export function PromptStatsPanel({
  stats,
  tokenUsage,
}: {
  stats?: PromptStats
  tokenUsage?: TokenUsage
}) {
  const t = useT()
  const [locale] = useLocale()
  const localeTag = localeNumberTag(locale)
  if (!stats && !tokenUsage) {
    return null
  }
  return (
    <div className="prompt-stats-panel">
      {stats && (
        <>
          {stats.context_tier_label && (
            <span
              className="wf-badge"
              title={t('ai_query.prompt_context_tier_title', { tier: stats.context_tier ?? '' })}
            >
              {t('ai_query.prompt_context_label')} {stats.context_tier_label}
            </span>
          )}
          {stats.est_prompt_tokens !== undefined && (
            <span className="wf-badge" title={t('ai_query.prompt_est_tokens_title')}>
              {t('ai_query.prompt_est_tokens_badge', {
                n: stats.est_prompt_tokens.toLocaleString(localeTag),
              })}
            </span>
          )}
          {stats.context_window_tokens !== undefined && (
            <span className="wf-badge" title={t('ai_query.prompt_window_title')}>
              {t('ai_query.prompt_window_badge', {
                n: stats.context_window_tokens.toLocaleString(localeTag),
              })}
            </span>
          )}
          {stats.prompt_runes !== undefined && (
            <span className="wf-badge" title={t('ai_query.prompt_runes_title')}>
              {t('ai_query.prompt_runes_badge', {
                n: stats.prompt_runes.toLocaleString(localeTag),
              })}
            </span>
          )}
        </>
      )}
      {tokenUsage && stats?.est_prompt_tokens !== undefined && tokenUsage.prompt > 0 && (
        <span className="wf-badge" title={t('ai_query.prompt_token_compare_title')}>
          {t('ai_query.prompt_token_compare_badge', {
            n: tokenUsage.prompt.toLocaleString(localeTag),
          })}
        </span>
      )}
    </div>
  )
}

export function Collapsible({
  title,
  children,
  defaultOpen = false,
}: {
  title: string
  children: ReactNode
  defaultOpen?: boolean
}) {
  return (
    <details open={defaultOpen} className="collapsible-section">
      <summary>{title}</summary>
      <div className="collapsible-content">{children}</div>
    </details>
  )
}
