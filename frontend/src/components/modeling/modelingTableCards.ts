import type {
  SemanticDimension,
  SemanticMetric,
  SemanticModelDetail,
  TableRow,
} from '../../types/semantic'
import { columnRefMatchesTable, tableKey } from './utils'

function addFallbackTableKey(keys: Set<string>, expr: string, baseSchema: string): void {
  const parts = expr.split('.')
  if (parts.length === 3) {
    const s = parts[0]
    const t = parts[1]
    if (s !== undefined && t !== undefined) {
      keys.add(tableKey(s, t))
    }
  } else if (parts.length === 2) {
    const t = parts[0]
    if (t !== undefined) {
      keys.add(tableKey(baseSchema, t))
    }
  }
}

function addMatchingIncludedTable(
  keys: Set<string>,
  columnRef: string,
  baseSchema: string,
  includedTables: TableRow[],
): boolean {
  for (const tbl of includedTables) {
    if (columnRefMatchesTable(columnRef, tbl.schema_name, tbl.table_name, baseSchema)) {
      keys.add(tableKey(tbl.schema_name, tbl.table_name))
      return true
    }
  }
  return false
}

function addDimensionTableKeys(
  keys: Set<string>,
  dimensions: SemanticDimension[],
  baseSchema: string,
  includedTables?: TableRow[],
): void {
  for (const dim of dimensions) {
    if (dim.is_active === false || !dim.column_ref) {
      continue
    }
    let matched = false
    if (includedTables) {
      matched = addMatchingIncludedTable(keys, dim.column_ref, baseSchema, includedTables)
    }
    if (!matched) {
      addFallbackTableKey(keys, dim.column_ref, baseSchema)
    }
  }
}

function addMatchingMetricTables(
  keys: Set<string>,
  expression: string,
  baseSchema: string,
  includedTables: TableRow[],
): boolean {
  let matched = false
  const exprLower = expression.toLowerCase()
  for (const tbl of includedTables) {
    const tKey = tableKey(tbl.schema_name, tbl.table_name)
    const tokens = [
      `${tbl.schema_name}.${tbl.table_name}.`,
      `"${tbl.schema_name}"."${tbl.table_name}".`,
    ]
    if (tbl.schema_name === baseSchema) {
      tokens.push(`${tbl.table_name}.`, `"${tbl.table_name}".`)
    }
    if (tokens.some((tok) => exprLower.includes(tok.toLowerCase()))) {
      keys.add(tKey)
      matched = true
    }
  }
  return matched
}

function addMetricTableKeys(
  keys: Set<string>,
  metrics: SemanticMetric[],
  baseSchema: string,
  includedTables?: TableRow[],
): void {
  for (const metric of metrics) {
    if (metric.is_active === false || !metric.expression) {
      continue
    }
    let matched = false
    if (includedTables) {
      matched = addMatchingMetricTables(keys, metric.expression, baseSchema, includedTables)
    }
    if (!matched) {
      addFallbackTableKey(keys, metric.expression, baseSchema)
    }
  }
}

export function buildModelTableKeys(
  model: SemanticModelDetail | null,
  includedTables?: TableRow[],
): Set<string> {
  const keys = new Set<string>()
  if (!model) {
    return keys
  }
  const baseSchema = model.base_schema
  const baseTable = model.base_table
  keys.add(tableKey(baseSchema, baseTable))

  // 1. Joins
  for (const join of model.joins ?? []) {
    if (join.is_active !== false) {
      keys.add(tableKey(join.from_schema ?? baseSchema, join.from_table))
      keys.add(tableKey(join.to_schema ?? baseSchema, join.to_table))
    }
  }

  // 2. Dimensions
  if (model.dimensions) {
    addDimensionTableKeys(keys, model.dimensions, baseSchema, includedTables)
  }

  // 3. Metrics
  if (model.metrics) {
    addMetricTableKeys(keys, model.metrics, baseSchema, includedTables)
  }

  return keys
}

export function buildModelingTableCards(
  model: SemanticModelDetail | null,
  includedTables: TableRow[],
  manualShown: Set<string>,
  manualHidden: Set<string>,
): TableRow[] {
  const keys = buildModelTableKeys(model, includedTables)
  const baseKey = model ? tableKey(model.base_schema, model.base_table) : ''
  const preferred = includedTables.filter((t) => keys.has(tableKey(t.schema_name, t.table_name)))
  const autofill = includedTables
    .filter((t) => !keys.has(tableKey(t.schema_name, t.table_name)))
    .slice(0, Math.max(0, 9 - preferred.length))
  const auto = [...preferred, ...autofill]
  const autoKeys = new Set(auto.map((t) => tableKey(t.schema_name, t.table_name)))
  const filteredAuto = auto.filter((t) => {
    const k = tableKey(t.schema_name, t.table_name)
    return k === baseKey || !manualHidden.has(k)
  })
  const extras = includedTables.filter((t) => {
    const k = tableKey(t.schema_name, t.table_name)
    return manualShown.has(k) && !autoKeys.has(k)
  })
  return [...filteredAuto, ...extras]
}

export function countTableImpact(
  model: SemanticModelDetail,
  schema: string,
  table: string,
  expressionRefsTable: (expr: string | undefined | null, s: string, t: string) => boolean,
): { joins: number; dims: number; metrics: number } {
  const base = model.base_schema
  const joins = (model.joins ?? []).filter((j) => {
    if (j.is_active === false) {
      return false
    }
    const fs = j.from_schema ?? base
    const ts = j.to_schema ?? base
    return (fs === schema && j.from_table === table) || (ts === schema && j.to_table === table)
  }).length
  const dims = (model.dimensions ?? []).filter(
    (d) =>
      d.is_active !== false &&
      columnRefMatchesTable(d.column_ref, schema, table, model.base_schema),
  ).length
  const metrics = (model.metrics ?? []).filter(
    (m) => m.is_active !== false && expressionRefsTable(m.expression, schema, table),
  ).length
  return { joins, dims, metrics }
}
