import type { TranslationKey } from '../../i18n'
import type { CTE } from '../../types/ai'
import type { SemanticDimension, SemanticJoin, SemanticMetric } from '../../types/semantic'

type Translate = (key: TranslationKey, params?: Record<string, string | number>) => string

export function joinEdgeLabel(j: SemanticJoin, baseSchema?: string): string {
  const fromS = j.from_schema?.trim() ?? baseSchema ?? ''
  const toS = j.to_schema?.trim() ?? baseSchema ?? ''
  const from = fromS
    ? `${fromS}.${j.from_table}.${j.from_column}`
    : `${j.from_table}.${j.from_column}`
  const to = toS ? `${toS}.${j.to_table}.${j.to_column}` : `${j.to_table}.${j.to_column}`
  const jt = j.join_type ? ` ${j.join_type}` : ''
  return `${from} → ${to}${jt}`
}

export function isCrossSchemaJoin(j: SemanticJoin, baseSchema?: string): boolean {
  const fromS = j.from_schema?.trim() ?? baseSchema ?? ''
  const toS = j.to_schema?.trim() ?? baseSchema ?? ''
  if (!fromS || !toS) {
    return false
  }
  return fromS !== toS
}

export function dimFieldOptions(dims: SemanticDimension[]) {
  return dims.map((d) => ({
    value: d.name,
    label: d.label?.trim() ? `${d.name} (${d.label})` : d.name,
    hint: d.type,
  }))
}

export function metricFieldOptions(metrics: SemanticMetric[]) {
  return metrics.map((m) => ({
    value: m.name,
    label: m.label?.trim() ? `${m.name} (${m.label})` : m.name,
    hint: m.aggregation,
  }))
}

export function metricDisplayName(metric: SemanticMetric) {
  return metric.label?.trim() ? metric.label : metric.name
}

export function aggregationDisplayName(aggregation: string) {
  return aggregation.replace(/_/g, ' ').toUpperCase()
}

export function orderByFieldOptions(
  dims: SemanticDimension[],
  metrics: SemanticMetric[],
  t: Translate,
) {
  const out: { value: string; label: string; hint: string }[] = []
  for (const d of dims) {
    out.push({
      value: d.name,
      label: d.label?.trim() ? `${d.name} (${d.label})` : d.name,
      hint: t('query_builder.order_hint_dimension', { detail: d.type }),
    })
  }
  for (const m of metrics) {
    out.push({
      value: m.name,
      label: m.label?.trim() ? `${m.name} (${m.label})` : m.name,
      hint: t('query_builder.order_hint_metric', { detail: m.aggregation }),
    })
  }
  return out
}

export function filterFieldOptions(
  dims: SemanticDimension[],
  metrics: SemanticMetric[],
  t: Translate,
) {
  return orderByFieldOptions(dims, metrics, t)
}

export function dimOptionsForGroupRow(
  dimensions: SemanticDimension[],
  groupBy: string[],
  rowIndex: number,
): { value: string; label: string; hint: string }[] {
  const chosenElsewhere = new Set(
    groupBy.filter((g, j) => j !== rowIndex && g !== '').map((g) => g),
  )
  return dimensions
    .filter((d) => !chosenElsewhere.has(d.name) || d.name === groupBy[rowIndex])
    .map((d) => ({
      value: d.name,
      label: d.label?.trim() ? `${d.name} (${d.label})` : d.name,
      hint: d.type,
    }))
}

export function parseCTEBody(raw: string): Omit<CTE, 'name'> {
  const trimmed = raw.trim()
  if (!trimmed) {
    return {}
  }
  try {
    const parsed = JSON.parse(trimmed) as Partial<CTE>
    const { name: _name, ...body } = parsed
    return body
  } catch {
    return {}
  }
}
