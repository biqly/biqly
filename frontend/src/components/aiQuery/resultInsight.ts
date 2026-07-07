import type { TFunction, TranslationKey } from '../../i18n'
import type { QueryResultPayload } from '../../types/ai'

type TimeGrain = 'hour' | 'day' | 'month' | 'year'

const GRAIN_KEY: Record<TimeGrain, TranslationKey> = {
  hour: 'ai_query.time_grain_hour',
  day: 'ai_query.time_grain_day',
  month: 'ai_query.time_grain_month',
  year: 'ai_query.time_grain_year',
}

/**
 * Conservatively detects a time grain from a column name using English or
 * Turkish substrings. Returns a neutral, language-agnostic token (never a
 * hardcoded display word) so callers can localize it via `t()`. Returns
 * `null` when the column name does not clearly indicate a grain — never
 * guess.
 */
function timeGrainForColumn(name: string): TimeGrain | null {
  const lower = name.toLocaleLowerCase('tr')
  if (lower.includes('hour') || lower.includes('saat')) {
    return 'hour'
  }
  if (lower.includes('day') || lower.includes('gun')) {
    return 'day'
  }
  if (lower.includes('month') || lower.includes('ay')) {
    return 'month'
  }
  if (lower.includes('year') || lower.includes('yil')) {
    return 'year'
  }
  return null
}

/**
 * Deterministic, localized natural-language caption summarizing an AI query
 * result. Pure: no side effects, no formatting beyond `toLocaleString`.
 *
 * Values are quoted exactly as the chart/table render them (raw), formatted
 * only for readability — never scaled or unit-suffixed.
 */
export function buildResultInsight(
  result: QueryResultPayload | undefined,
  t: TFunction,
  localeTag: string,
): string | null {
  if (!result || result.columns.length === 0 || result.rows.length === 0) {
    return null
  }

  const { columns, rows } = result
  const metricIdx = columns.findIndex((c) => c.semantic_type === 'metric')
  const dimIdx = columns.findIndex((c) => c.semantic_type === 'dimension')
  const metricCol = metricIdx >= 0 ? columns[metricIdx] : undefined
  if (!metricCol) {
    return null
  }

  const metricName = metricCol.name
  const fmt = (value: number) => value.toLocaleString(localeTag, { maximumFractionDigits: 2 })

  // Finite metric values paired with their row.
  const finite: { value: number; row: unknown[] }[] = []
  for (const row of rows) {
    const value = Number(row[metricIdx])
    if (Number.isFinite(value)) {
      finite.push({ value, row })
    }
  }
  const first = finite[0]

  // Case A — ranked: a metric and a dimension with at least two data points.
  const dimCol = dimIdx >= 0 ? columns[dimIdx] : undefined
  if (dimCol && first && finite.length >= 2) {
    let maxEntry = first
    let minValue = first.value
    let maxValue = first.value
    for (const entry of finite) {
      if (entry.value > maxEntry.value) {
        maxEntry = entry
      }
      minValue = Math.min(minValue, entry.value)
      maxValue = Math.max(maxValue, entry.value)
    }
    const ranked = t('ai_query.insight_ranked_explained', {
      top: String(maxEntry.row[dimIdx]),
      metric: metricName,
      topVal: fmt(maxEntry.value),
      minVal: fmt(minValue),
      maxVal: fmt(maxValue),
      n: rows.length,
      dim: dimCol.name,
    })

    const grain = timeGrainForColumn(dimCol.name)
    if (grain) {
      const bucket = t('ai_query.insight_time_bucket_explained', { grain: t(GRAIN_KEY[grain]) })
      return `${bucket} ${ranked}`
    }
    return ranked
  }

  // Case B — single KPI: exactly one metric, no dimension, one finite row.
  const metricCount = columns.filter((c) => c.semantic_type === 'metric').length
  if (metricCount === 1 && dimIdx < 0 && rows.length === 1 && first) {
    return t('ai_query.insight_single_explained', {
      metric: metricName,
      val: fmt(first.value),
    })
  }

  return null
}
