/* eslint-disable react-refresh/only-export-components */
import { type ReactNode, useId } from 'react'

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
import {
  breakdownBarBgClass,
  breakdownBarFillClass,
  breakdownRowClass,
  btnCandidateUseClass,
  btnClarificationClass,
  btnSkipClass,
  candidateCardClass,
  candidateCardHeaderClass,
  candidateCardsClass,
  candidateHeaderClass,
  candidateJsonClass,
  candidatePanelClass,
  candidateReasoningClass,
  candidateScoreClass,
  clarificationAmbiguityTermsClass,
  clarificationAmbiguityTermsLiClass,
  clarificationAmbiguityTermsStrongClass,
  clarificationAmbiguityTypeClass,
  clarificationCapNoticeClass,
  clarificationCardAmbiguityClass,
  clarificationCardClass,
  clarificationOptionsClass,
  clarificationQuestionClass,
  clarificationRoundIndicatorClass,
  clarificationTitleClass,
  collapsibleContentClass,
  collapsibleSectionClass,
  collapsibleSectionSummaryClass,
  confidenceBarBgClass,
  confidenceBarFillClass,
  confidenceBreakdownClass,
  confidenceHeaderClass,
  confidenceHintClass,
  confidenceSectionClass,
  costBadgeClass,
  routingBarBgClass,
  routingBarFillClass,
  routingCandidateClass,
  routingConfidenceClass,
  routingContextGridClass,
  routingContextGridItemClass,
  routingContextLabelClass,
  routingContextValueClass,
  routingDebugClass,
  routingDebugCodeClass,
  routingHeaderClass,
  routingReasoningClass,
  routingScoreClass,
  routingScoreDetailClass,
  routingSelectedClass,
  routingSelectedEmptyClass,
  routingTableListClass,
  routingTableNameClass,
  tableRoutingVizClass,
} from './aiQueryClasses'
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
    <div className={confidenceSectionClass}>
      <div className={confidenceHeaderClass}>
        <span>{t('ai_query.confidence')}</span>
        <span style={{ color, fontWeight: 600 }}>{pct}%</span>
      </div>
      <div className={confidenceBarBgClass}>
        <div
          className={confidenceBarFillClass(
            value > 0.8 ? 'success' : value > 0.5 ? 'warning' : 'error',
          )}
          style={{ width: `${pct}%` }}
        />
      </div>
      {breakdown && (
        <div className={confidenceBreakdownClass}>
          <BreakdownRow
            label={t('ai_query.breakdown_table_routing')}
            value={breakdown.table_routing}
          />
          <BreakdownRow label={t('ai_query.breakdown_llm')} value={breakdown.llm} />
          <BreakdownRow label={t('ai_query.breakdown_validation')} value={breakdown.validation} />
        </div>
      )}
      {value < 0.5 && <p className={confidenceHintClass}>{t('ai_query.confidence_low_hint')}</p>}
    </div>
  )
}

function BreakdownRow({ label, value }: { label: string; value: number }) {
  const pct = Math.round(value * 100)
  const color = value > 0.8 ? 'var(--success)' : value > 0.5 ? 'var(--warning)' : 'var(--error)'
  return (
    <div className={breakdownRowClass}>
      <span>{label}</span>
      <div className={breakdownBarBgClass}>
        <div
          className={breakdownBarFillClass(
            value > 0.8 ? 'success' : value > 0.5 ? 'warning' : 'error',
          )}
          style={{ width: `${pct}%` }}
        />
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
    <strong className={routingTableListClass}>
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
    <code className={routingTableListClass}>
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
          <div key={c.table} className={routingCandidateClass}>
            <span className={routingTableNameClass}>{c.table}</span>
            <div className={routingBarBgClass}>
              <div className={routingBarFillClass} style={{ width: `${pct}%` }} />
            </div>
            <span className={routingScoreClass}>{score.toFixed(2)}</span>
            <span className={c.selected ? routingSelectedClass : routingSelectedEmptyClass}>
              {c.selected ? '✓' : ''}
            </span>
            <span className={routingScoreDetailClass}>
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
    <div className={routingDebugClass}>
      {debug.relation_expansion && debug.relation_expansion.length > 0 && (
        <div>
          <span className="text-[0.72rem] text-foreground-faint">
            {t('ai_query.routing_debug_relation')}
          </span>
          <code className={routingDebugCodeClass}>{debug.relation_expansion.join(' | ')}</code>
        </div>
      )}
      {debug.bridge_tables && debug.bridge_tables.length > 0 && (
        <div>
          <span className="text-[0.72rem] text-foreground-faint">
            {t('ai_query.routing_debug_bridge')}
          </span>
          <RoutingDebugList items={debug.bridge_tables} />
        </div>
      )}
      {debug.schema_partitions && debug.schema_partitions.length > 0 && (
        <div>
          <span className="text-[0.72rem] text-foreground-faint">
            {t('ai_query.routing_debug_schema_parts')}
          </span>
          <RoutingDebugList items={debug.schema_partitions} />
        </div>
      )}
      {debug.eliminated_candidates && debug.eliminated_candidates.length > 0 && (
        <div>
          <span className="text-[0.72rem] text-foreground-faint">
            {t('ai_query.routing_debug_eliminated')}
          </span>
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
    <div className={routingContextGridClass}>
      <div className={routingContextGridItemClass}>
        <span className={routingContextLabelClass}>{t('ai_query.routing_source')}</span>
        <strong className={routingContextValueClass}>{sourceLabel}</strong>
      </div>
      {selectedModels && (
        <div className={routingContextGridItemClass}>
          <span className={routingContextLabelClass}>{t('ai_query.routing_model')}</span>
          <strong className={routingContextValueClass}>{selectedModels}</strong>
        </div>
      )}
      {selectedTables && (
        <div className={routingContextGridItemClass}>
          <span className={routingContextLabelClass}>{t('ai_query.routing_tables')}</span>
          <RoutingTableList items={routing.selected_tables} />
        </div>
      )}
      {selectedDims && (
        <div className={routingContextGridItemClass}>
          <span className={routingContextLabelClass}>{t('ai_query.routing_dimensions')}</span>
          <strong className={routingContextValueClass}>{selectedDims}</strong>
        </div>
      )}
      {selectedMetrics && (
        <div className={routingContextGridItemClass}>
          <span className={routingContextLabelClass}>{t('ai_query.routing_metrics')}</span>
          <strong className={routingContextValueClass}>{selectedMetrics}</strong>
        </div>
      )}
      {(routing.join_paths?.length ?? 0) > 0 && (
        <div className={routingContextGridItemClass}>
          <span className={routingContextLabelClass}>{t('ai_query.routing_join_paths')}</span>
          <RoutingDebugList items={routing.join_paths} />
        </div>
      )}
      {routing.context_updated_at && (
        <div className={routingContextGridItemClass}>
          <span className={routingContextLabelClass}>{t('ai_query.routing_context_time')}</span>
          <strong className={routingContextValueClass}>
            {new Date(routing.context_updated_at).toLocaleString(localeTag)}
          </strong>
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
    <div className={tableRoutingVizClass}>
      <div className={routingHeaderClass}>
        <span>{t('ai_query.routing_header', { method: methodLabel })}</span>
        <span className={routingConfidenceClass}>{Math.round(routing.confidence * 100)}%</span>
      </div>
      <RoutingContextSummary routing={routing} localeTag={localeTag} t={t} />
      <RoutingCandidatesList candidates={candidates} maxScore={maxScore} t={t} />
      {routing.debug && <RoutingDebugPanel debug={routing.debug} t={t} />}
      {routing.reasoning && <p className={routingReasoningClass}>{routing.reasoning}</p>}
    </div>
  )
}

// ClarificationRoundChip renders the "Round x/y" indicator next to the title.
function ClarificationRoundChip({ round, maxRounds }: { round: number; maxRounds: number }) {
  const t = useT()
  if (round <= 0 || maxRounds <= 0) {
    return null
  }
  return (
    <span className={clarificationRoundIndicatorClass}>
      {t('ai_query.clarification_round_indicator', { current: round, max: maxRounds })}
    </span>
  )
}

// AmbiguityTermsList summarizes the detected ambiguous terms (term + detector type).
function AmbiguityTermsList({ clarification }: { clarification?: Clarification }) {
  const t = useT()
  if (clarification?.source !== 'ambiguity_analyzer') {
    return null
  }
  const ambiguities = clarification.ambiguity_detail?.ambiguities ?? []
  if (ambiguities.length === 0) {
    return null
  }
  return (
    <ul
      className={clarificationAmbiguityTermsClass}
      aria-label={t('ai_query.clarification_terms_label')}
    >
      {ambiguities.map((item) => (
        <li key={item.term} className={clarificationAmbiguityTermsLiClass}>
          <strong className={clarificationAmbiguityTermsStrongClass}>{item.term}</strong>
          <span className={clarificationAmbiguityTypeClass}>{item.type}</span>
        </li>
      ))}
    </ul>
  )
}

export function ClarificationCard({
  question,
  options,
  clarification,
  generationTrace,
  interactiveTier = false,
  capReached = false,
  round = 0,
  maxRounds = 0,
  onSelect,
  onSkip,
}: {
  question: string
  options: string[]
  clarification?: Clarification
  generationTrace?: GenerationTrace
  interactiveTier?: boolean
  capReached?: boolean
  /** 1-based clarification round for the "Round x/y" indicator; 0 hides it. */
  round?: number
  maxRounds?: number
  onSelect: (choice: string) => void
  onSkip: () => void
}) {
  const t = useT()
  const titleId = useId()
  const structured = (clarification?.options ?? []).filter((o) => o.label.trim())
  const useStructured = structured.length > 0
  const isAmbiguity = clarification?.source === 'ambiguity_analyzer'
  return (
    <div
      className={isAmbiguity ? clarificationCardAmbiguityClass : clarificationCardClass}
      role="group"
      aria-labelledby={titleId}
    >
      <div className={clarificationTitleClass} id={titleId}>
        {t('ai_query.clarification_title')}
        <ClarificationRoundChip round={round} maxRounds={maxRounds} />
      </div>
      {interactiveTier && (
        <p
          className={`${clarificationCapNoticeClass} clarification-cap-notice--interactive`}
          role="status"
        >
          {t('ai_query.clarification_interactive_tier')}
        </p>
      )}
      {capReached && (
        <p className={clarificationCapNoticeClass} role="status">
          {t('ai_query.clarification_cap_reached')}
        </p>
      )}
      {clarification?.reason && <p className="clarification-reason">{clarification.reason}</p>}
      <AmbiguityTermsList clarification={clarification} />
      <p className={clarificationQuestionClass}>{question}</p>
      <div className={clarificationOptionsClass}>
        {useStructured
          ? structured.map((opt) => (
              <button
                key={opt.key}
                type="button"
                className={btnClarificationClass}
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
                className={btnClarificationClass}
                onClick={() => onSelect(opt)}
              >
                {opt}
              </button>
            ))}
      </div>
      <button type="button" className={btnSkipClass} onClick={onSkip}>
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
    <div className={candidatePanelClass}>
      <div className={candidateHeaderClass}>
        <span>{t('ai_query.candidates_header', { count: candidates.length })}</span>
      </div>
      <div className={candidateCardsClass}>
        {candidates.map((c, i) => {
          const isBest = i === bestIdx
          const pct = Math.round(c.confidence * 100)
          return (
            <div key={i} className={candidateCardClass(isBest)}>
              <div className={candidateCardHeaderClass}>
                <span>{t('ai_query.candidate_number', { n: i + 1 })}</span>
                <span className={candidateScoreClass(isBest)}>
                  {t('ai_query.candidate_score', { pct })}
                </span>
              </div>
              {c.reasoning && <p className={candidateReasoningClass}>{c.reasoning}</p>}
              <details>
                <summary>{t('ai_query.logical_query_json')}</summary>
                <pre className={`sql-preview ${candidateJsonClass}`}>
                  {JSON.stringify(c.logical_query, null, 2)}
                </pre>
              </details>
              <button className={btnCandidateUseClass} onClick={() => onUse(i)}>
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
  if (latencyMs !== undefined && latencyMs >= 50) {
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
    <div className={costBadgeClass} title={tokenTitle}>
      {parts.join(' · ')}
      {tokenUsage && (
        <span className="font-normal text-foreground-faint">
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
    <details open={defaultOpen} className={`${collapsibleSectionClass} group`}>
      <summary className={collapsibleSectionSummaryClass}>
        {title}
        <span className="text-[0.65rem] text-foreground-faint transition-transform duration-180 group-open:rotate-180">
          ▼
        </span>
      </summary>
      <div className={collapsibleContentClass}>{children}</div>
    </details>
  )
}
