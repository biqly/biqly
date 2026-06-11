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
  const jt = ` ${j.join_type}`
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

import type { SelectItem } from './types'

export function getFieldLabel(
  name: string,
  label?: string | null,
  mode: 'technical' | 'human' = 'human',
): string {
  if (mode === 'technical') {
    return name
  }
  return label?.trim() ? label : name
}

export function dimFieldOptions(dims: SemanticDimension[], mode: 'technical' | 'human' = 'human') {
  return dims.map((d) => ({
    value: d.name,
    label: getFieldLabel(d.name, d.label, mode),
    hint: d.type,
  }))
}

export function metricFieldOptions(
  metrics: SemanticMetric[],
  mode: 'technical' | 'human' = 'human',
) {
  return metrics
    .filter((m) => m.name !== 'count' && m.aggregation !== 'count')
    .map((m) => ({
      value: m.name,
      label: getFieldLabel(m.name, m.label, mode),
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
  mode: 'technical' | 'human' = 'human',
) {
  const out: { value: string; label: string; hint: string }[] = []
  for (const d of dims) {
    out.push({
      value: d.name,
      label: getFieldLabel(d.name, d.label, mode),
      hint: t('query_builder.order_hint_dimension', { detail: d.type }),
    })
  }
  for (const m of metrics) {
    if (m.name !== 'count' && m.aggregation !== 'count') {
      out.push({
        value: m.name,
        label: getFieldLabel(m.name, m.label, mode),
        hint: t('query_builder.order_hint_metric', { detail: m.aggregation }),
      })
    }
  }
  return out
}

export function filterFieldOptions(
  dims: SemanticDimension[],
  metrics: SemanticMetric[],
  t: Translate,
  mode: 'technical' | 'human' = 'human',
) {
  return orderByFieldOptions(dims, metrics, t, mode)
}

export function dimOptionsForGroupRow(
  dimensions: SemanticDimension[],
  groupBy: string[],
  rowIndex: number,
  selectItems: SelectItem[],
  mode: 'technical' | 'human' = 'human',
): { value: string; label: string; hint: string; disabled?: boolean }[] {
  const chosenElsewhere = new Set(
    groupBy.filter((g, j) => j !== rowIndex && g !== '').map((g) => g),
  )

  const availableDims = dimensions.filter(
    (d) => !chosenElsewhere.has(d.name) || d.name === groupBy[rowIndex],
  )

  const selectedDimNames = selectItems
    .filter((item) => item.type === 'dimension' && item.name)
    .map((item) => item.name)

  const selectedDimSet = new Set(selectedDimNames)

  const selectedList = availableDims.filter((d) => selectedDimSet.has(d.name))
  const unselectedList = availableDims.filter((d) => !selectedDimSet.has(d.name))

  selectedList.sort((a, b) => selectedDimNames.indexOf(a.name) - selectedDimNames.indexOf(b.name))

  const selectedOptions = selectedList.map((d) => ({
    value: d.name,
    label: getFieldLabel(d.name, d.label, mode),
    hint: d.type,
    disabled: false,
  }))

  const unselectedOptions = unselectedList.map((d) => ({
    value: d.name,
    label: getFieldLabel(d.name, d.label, mode),
    hint: d.type,
    disabled: true,
  }))

  return [...selectedOptions, ...unselectedOptions]
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
