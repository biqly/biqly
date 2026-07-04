import type { TFunction } from '../../i18n'
import type { QueryResultPayload } from '../../types/ai'

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
    return t('ai_query.insight_ranked', {
      top: String(maxEntry.row[dimIdx]),
      metric: metricName,
      topVal: fmt(maxEntry.value),
      minVal: fmt(minValue),
      maxVal: fmt(maxValue),
      n: rows.length,
      dim: dimCol.name,
    })
  }

  // Case B — single KPI: exactly one metric, no dimension, one finite row.
  const metricCount = columns.filter((c) => c.semantic_type === 'metric').length
  if (metricCount === 1 && dimIdx < 0 && rows.length === 1 && first) {
    return t('ai_query.insight_single', {
      metric: metricName,
      val: fmt(first.value),
    })
  }

  return null
}
