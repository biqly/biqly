import type { TranslationKey } from '../../i18n'
import type { SemanticDimension, SemanticMetric, SemanticModelDetail } from '../../types/semantic'

export interface SuggestedQuestion {
  /** Stable category key: 'aggregation' | 'segmentation' | 'trend' | 'comparison'. */
  category: string
  text: string
}

type Translate = (key: TranslationKey, vars?: Record<string, string | number>) => string

const CATEGORICAL_TYPES = new Set(['text', 'boolean', 'geo'])
const DATE_TYPES = new Set(['date', 'timestamp', 'datetime'])

const MAX_SUGGESTIONS = 6

function isActive(entity: { is_active?: boolean }): boolean {
  return entity.is_active !== false
}

/** Human-friendly display string: prefer label, fall back to name; underscores → spaces. */
function humanize(entity: { name: string; label?: string | null }): string {
  const base = entity.label?.trim() ? entity.label : entity.name
  return base.replace(/_/g, ' ').trim()
}

/**
 * Build data-derived, categorized starter questions from a semantic model.
 * Returns an empty array when the model has no metrics so callers can fall
 * back to the static suggestions.
 */
export function buildSuggestedQuestions(
  model: SemanticModelDetail | null | undefined,
  t: Translate,
): SuggestedQuestion[] {
  const metrics: SemanticMetric[] = (model?.metrics ?? []).filter(isActive)
  const firstMetricEntity = metrics[0]
  if (!firstMetricEntity) {
    return []
  }

  const dimensions: SemanticDimension[] = (model?.dimensions ?? []).filter(isActive)
  const catDims = dimensions.filter((d) => CATEGORICAL_TYPES.has(d.type))
  const dateDims = dimensions.filter((d) => DATE_TYPES.has(d.type))

  const firstMetric = humanize(firstMetricEntity)
  const suggestions: SuggestedQuestion[] = []

  // Aggregation: total of the first metric.
  suggestions.push({
    category: 'aggregation',
    text: t('ai_query.suggest_aggregation', { metric: firstMetric }),
  })

  // Segmentation: first metric by first categorical dimension.
  const firstCatDim = catDims[0]
  if (firstCatDim) {
    suggestions.push({
      category: 'segmentation',
      text: t('ai_query.suggest_segmentation', {
        metric: firstMetric,
        dim: humanize(firstCatDim),
      }),
    })
  }

  // Trend: first metric over time (requires a date dimension).
  if (dateDims.length > 0) {
    suggestions.push({
      category: 'trend',
      text: t('ai_query.suggest_trend', { metric: firstMetric }),
    })
  }

  // Comparison: first metric across a second (or first) categorical dimension.
  const comparisonDim = catDims[1] ?? firstCatDim
  if (comparisonDim) {
    suggestions.push({
      category: 'comparison',
      text: t('ai_query.suggest_comparison', {
        metric: firstMetric,
        dim: humanize(comparisonDim),
      }),
    })
  }

  // Second aggregation to help fill up to the cap when multiple metrics exist.
  const secondMetricEntity = metrics[1]
  if (secondMetricEntity) {
    suggestions.push({
      category: 'aggregation',
      text: t('ai_query.suggest_aggregation', { metric: humanize(secondMetricEntity) }),
    })
  }

  const seen = new Set<string>()
  const deduped: SuggestedQuestion[] = []
  for (const s of suggestions) {
    if (seen.has(s.text)) {
      continue
    }
    seen.add(s.text)
    deduped.push(s)
    if (deduped.length >= MAX_SUGGESTIONS) {
      break
    }
  }

  return deduped
}
