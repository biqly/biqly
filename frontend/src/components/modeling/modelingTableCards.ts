import type { SemanticModelDetail, TableRow } from '../../types/semantic'
import { columnRefMatchesTable, tableKey } from './utils'

export function buildModelTableKeys(model: SemanticModelDetail | null): Set<string> {
  const keys = new Set<string>()
  if (!model) {
    return keys
  }
  keys.add(tableKey(model.base_schema, model.base_table))
  for (const join of model.joins ?? []) {
    if (join.is_active !== false) {
      keys.add(tableKey(join.from_schema ?? model.base_schema, join.from_table))
      keys.add(tableKey(join.to_schema ?? model.base_schema, join.to_table))
    }
  }
  return keys
}

export function buildModelingTableCards(
  model: SemanticModelDetail | null,
  includedTables: TableRow[],
  manualShown: Set<string>,
  manualHidden: Set<string>,
): TableRow[] {
  const keys = buildModelTableKeys(model)
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
