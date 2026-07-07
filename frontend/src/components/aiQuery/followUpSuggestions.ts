import type { TFunction } from '../../i18n'
import type { AIQueryResponse, SuggestedFollowUp } from '../../types/ai'

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

/**
 * Builds client-side fallback follow-up suggestions (trend/comparison/chart)
 * from the shape of the query result, for use when the backend didn't
 * return any suggested_followups. Returns [] for empty results.
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

  const hasMetric = result.columns.some((column) => column.semantic_type === 'metric')
  const hasDimension = result.columns.some((column) => column.semantic_type === 'dimension')
  const hasTime = result.columns.some((column) =>
    /date|time|hour|day|month|year|_ts$/i.test(column.name),
  )

  const candidates: SuggestedFollowUp[] = []
  if (hasTime && hasMetric) {
    candidates.push({
      id: 'fallback-trend',
      kind: 'trend',
      label: args.t('ai_query.followups_trend_label'),
      question: args.t('ai_query.followups_trend_question'),
    })
  }
  if (hasDimension && hasMetric && result.rows.length > 1) {
    candidates.push({
      id: 'fallback-compare',
      kind: 'comparison',
      label: args.t('ai_query.followups_compare_label'),
      question: args.t('ai_query.followups_compare_question'),
    })
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
