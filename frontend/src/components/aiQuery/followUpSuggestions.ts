import type { TFunction, TranslationKey } from '../../i18n'
import type { AIQueryResponse, SuggestedFollowUp } from '../../types/ai'

// DETERMINISTIC_FOLLOWUP_I18N maps the backend's deterministic follow-up
// suggestion ids (internal/http/handlers/ai_followups.go
// BuildDeterministicFollowUps) to the i18n keys for their label and question.
// The backend emits these chips with hardcoded English text; localizeFollowUps
// swaps in the active locale's copy so a Turkish (or any non-English) UI does
// not show English chips. Ids not in this map — e.g. AI-rewritten,
// context-specific suggestions — are left exactly as the server sent them.
const DETERMINISTIC_FOLLOWUP_I18N: Record<
  string,
  { label: TranslationKey; question: TranslationKey }
> = {
  'trend-over-time': {
    label: 'ai_query.followups_show_trend_label',
    question: 'ai_query.followups_show_trend_question',
  },
  'visualize-as-chart': {
    label: 'ai_query.followups_chart_label',
    question: 'ai_query.followups_chart_question',
  },
  'compare-top-values': {
    label: 'ai_query.followups_compare_label',
    question: 'ai_query.followups_compare_question',
  },
  'drilldown-detail': {
    label: 'ai_query.followups_more_detail_label',
    question: 'ai_query.followups_more_detail_question',
  },
  'breakdown-by-dimension': {
    label: 'ai_query.followups_breakdown_category_label',
    question: 'ai_query.followups_breakdown_category_question',
  },
}

/**
 * Localizes the backend's deterministic follow-up chips by id, leaving any
 * suggestion whose id is not a known deterministic one (e.g. AI-rewritten)
 * untouched. Both label (shown) and question (sent as the next query) are
 * translated so a non-English user drives the follow-up in their own language.
 */
export function localizeFollowUps(
  suggestions: SuggestedFollowUp[],
  t: TFunction,
): SuggestedFollowUp[] {
  return suggestions.map((suggestion) => {
    const keys = DETERMINISTIC_FOLLOWUP_I18N[suggestion.id]
    if (!keys) {
      return suggestion
    }
    return { ...suggestion, label: t(keys.label), question: t(keys.question) }
  })
}

/** Locale-aware, punctuation-insensitive normalization used for follow-up dedup matching. */
export function normalizeFollowUpText(value: string): string {
  return value
    .toLocaleLowerCase('tr')
    .replace(/[^\p{L}\p{N}\s]/gu, ' ')
    .replace(/\s+/g, ' ')
    .trim()
}

/**
 * Drops follow-up suggestions that are blank, duplicate each other, or
 * substantially match a question already asked in this conversation.
 * Caps the result at 3 suggestions.
 */
export function filterFollowUpSuggestions(
  suggestions: SuggestedFollowUp[],
  priorQuestions: string[],
): SuggestedFollowUp[] {
  const prior = priorQuestions.map(normalizeFollowUpText).filter(Boolean)
  const seen = new Set<string>()
  const kept: SuggestedFollowUp[] = []

  for (const suggestion of suggestions) {
    const label = suggestion.label.trim()
    const question = suggestion.question.trim()
    const normalized = normalizeFollowUpText(question)
    if (!label || !question || !normalized || seen.has(normalized)) {
      continue
    }
    if (
      prior.some(
        (item) => item === normalized || item.includes(normalized) || normalized.includes(item),
      )
    ) {
      continue
    }
    seen.add(normalized)
    kept.push({ ...suggestion, label, question })
    if (kept.length >= 3) {
      break
    }
  }

  return kept
}

const TIME_COLUMN_RE = /date|time|hour|day|week|month|quarter|year|_ts$|_at$/i

/**
 * Builds client-side fallback follow-up suggestions from the shape of the
 * query result, for use when the backend didn't return any
 * suggested_followups. Contextual by result shape: a single KPI invites a
 * period comparison and a breakdown; a time series invites the previous
 * period and its biggest change; a categorical result invites a top-N of the
 * actual dimension column. Returns [] for empty results.
 */
export function buildFallbackFollowUps(args: {
  response: AIQueryResponse
  priorQuestions: string[]
  t: TFunction
}): SuggestedFollowUp[] {
  const result = args.response.result
  if (!result || result.rows.length === 0) {
    return []
  }

  const metricColumn = result.columns.find((column) => column.semantic_type === 'metric')
  // Top-N only makes sense for non-time breakdowns ("top 5 regions", not
  // "top 5 order_date").
  const dimensionColumn = result.columns.find(
    (column) => column.semantic_type === 'dimension' && !TIME_COLUMN_RE.test(column.name),
  )
  const hasMetric = metricColumn !== undefined
  const hasTime = result.columns.some((column) => TIME_COLUMN_RE.test(column.name))
  const isSingleValue = result.rows.length === 1 && result.columns.length === 1

  const candidates: SuggestedFollowUp[] = []
  if (isSingleValue) {
    candidates.push(
      {
        id: 'fallback-prev-period',
        kind: 'comparison',
        label: args.t('ai_query.followups_prev_period_label'),
        question: args.t('ai_query.followups_prev_period_question'),
      },
      {
        id: 'fallback-breakdown',
        kind: 'breakdown',
        label: args.t('ai_query.followups_breakdown_label'),
        question: args.t('ai_query.followups_breakdown_question'),
      },
      {
        id: 'fallback-trend',
        kind: 'trend',
        label: args.t('ai_query.followups_trend_label'),
        question: args.t('ai_query.followups_trend_question'),
      },
    )
    return filterFollowUpSuggestions(candidates, args.priorQuestions)
  }

  if (hasTime && hasMetric) {
    candidates.push(
      {
        id: 'fallback-prev-period',
        kind: 'comparison',
        label: args.t('ai_query.followups_prev_period_label'),
        question: args.t('ai_query.followups_prev_period_question'),
      },
      {
        id: 'fallback-explain-change',
        kind: 'explain',
        label: args.t('ai_query.followups_explain_change_label'),
        question: args.t('ai_query.followups_explain_change_question'),
      },
    )
  }
  if (dimensionColumn && hasMetric && result.rows.length > 1) {
    candidates.push(
      {
        id: 'fallback-topn',
        kind: 'breakdown',
        label: args.t('ai_query.followups_topn_label', { column: dimensionColumn.name }),
        question: args.t('ai_query.followups_topn_question', { column: dimensionColumn.name }),
      },
      {
        id: 'fallback-compare',
        kind: 'comparison',
        label: args.t('ai_query.followups_compare_label'),
        question: args.t('ai_query.followups_compare_question'),
      },
    )
  }
  if (hasMetric) {
    candidates.push({
      id: 'fallback-chart',
      kind: 'chart',
      label: args.t('ai_query.followups_chart_label'),
      question: args.t('ai_query.followups_chart_question'),
    })
  }

  return filterFollowUpSuggestions(candidates, args.priorQuestions)
}
