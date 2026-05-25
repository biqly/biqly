import type { CTE } from '../../types/ai'
import type { QueryBuilderFormState } from './types'
import { parseCTEBody } from './utils'

export type QueryRunPayload = ReturnType<typeof buildQueryPayload>

export function buildQueryPayload(state: QueryBuilderFormState) {
  const {
    datasourceId,
    modelId,
    mode,
    selectItems,
    filters,
    groupBy,
    having,
    orderBy,
    orderDir,
    limit,
    offset,
    windowFunctions,
    ctes,
  } = state

  return {
    datasource_id: datasourceId,
    model_id: modelId,
    filters: filters
      .filter((f) => f.field && f.value)
      .map((f) => {
        let parsedValue: unknown = f.value
        if (typeof f.value === 'string' && f.value.startsWith('[') && f.value.endsWith(']')) {
          try {
            parsedValue = JSON.parse(f.value)
          } catch (e) {
            // fallback to raw value if invalid JSON
          }
        }
        return {
          field: f.field,
          operator: f.operator,
          value: parsedValue,
          case_sensitive: f.caseSensitive,
        }
      }),
    group_by: groupBy.filter(Boolean).map((g) => ({ field: g })),
    having:
      mode === 'advanced'
        ? having
            .filter((h) => h.field && h.value)
            .map((h) => ({
              field: h.field,
              operator: h.operator,
              value: h.value,
            }))
        : undefined,
    order_by: orderBy ? [{ field: orderBy, direction: orderDir }] : [],
    limit: parseInt(String(limit)) || 100,
    offset: (() => {
      const n = Math.max(0, parseInt(String(offset)) || 0)
      return n > 0 || mode === 'advanced' ? n : undefined
    })(),
    ...(mode === 'advanced'
      ? {
          select: [
            ...selectItems.filter((s) => s.name).map(({ type, name }) => ({ type, name })),
            ...windowFunctions
              .filter((w) => w.field)
              .map((w) => ({
                type: 'window' as const,
                name: w.field,
                window: {
                  aggregation: (w.func || 'row_number').toLowerCase(),
                  expression: w.field,
                  partition_by: w.partition_by
                    ? w.partition_by.split(',').map((s) => s.trim()).filter(Boolean)
                    : undefined,
                  order_by: w.order_by ? [{ field: w.order_by, direction: 'asc' as const }] : undefined,
                },
              })),
          ],
        }
      : {
          select: selectItems.filter((s) => s.name).map(({ type, name }) => ({ type, name })),
        }),
    ctes:
      mode === 'advanced'
        ? ctes
            .filter((c) => c.name)
            .map(
              (c): CTE => ({
                name: c.name,
                ...parseCTEBody(c.query),
              }),
            )
        : undefined,
  }
}
