import type { SemanticModelDetail } from '../../types/semantic'
import { columnRefMatchesTable } from './utils'

export function expressionRefsTable(
  expr: string | undefined | null,
  schema: string,
  table: string,
  baseSchema: string,
): boolean {
  if (!expr) {
    return false
  }
  const e = expr.toLowerCase()
  const tokens = [`${schema}.${table}.`, `"${schema}"."${table}".`]
  if (schema === baseSchema) {
    tokens.push(`${table}.`, `"${table}".`)
  }
  return tokens.some((tok) => e.includes(tok.toLowerCase()))
}

export function columnRefMatchesSchema(
  ref: string | undefined | null,
  schema: string,
  baseSchema: string,
): boolean {
  if (!ref) {
    return false
  }
  const r = ref.trim()
  if (!r) {
    return false
  }
  if (r.startsWith(`${schema}.`)) {
    return true
  }
  return schema === baseSchema && r.split('.').length === 2
}

export function expressionRefsSchema(expr: string | undefined | null, schema: string): boolean {
  if (!expr) {
    return false
  }
  const e = expr.toLowerCase()
  const tokens = [`${schema}.`, `"${schema}".`]
  return tokens.some((tok) => e.includes(tok.toLowerCase()))
}

export function tableImpact(model: SemanticModelDetail, schema: string, table: string) {
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
    (m) =>
      m.is_active !== false && expressionRefsTable(m.expression, schema, table, model.base_schema),
  ).length
  return { joins, dims, metrics }
}

export function schemaImpact(model: SemanticModelDetail, schema: string) {
  const base = model.base_schema
  const joins = (model.joins ?? []).filter((j) => {
    if (j.is_active === false) {
      return false
    }
    const fs = j.from_schema ?? base
    const ts = j.to_schema ?? base
    return fs === schema || ts === schema
  }).length
  const dims = (model.dimensions ?? []).filter(
    (d) => d.is_active !== false && columnRefMatchesSchema(d.column_ref, schema, model.base_schema),
  ).length
  const metrics = (model.metrics ?? []).filter(
    (m) => m.is_active !== false && expressionRefsSchema(m.expression, schema),
  ).length
  return { joins, dims, metrics }
}
