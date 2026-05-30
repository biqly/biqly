import type { SemanticDimension, SemanticJoin, SemanticMetric } from '../../types/semantic'

type EntityWithActiveState = {
  is_active?: boolean
}

export function activeEntities<T extends EntityWithActiveState>(entities: T[]) {
  return entities.filter((entity) => entity.is_active !== false)
}

export function inactiveEntities<T extends EntityWithActiveState>(entities: T[]) {
  return entities.filter((entity) => entity.is_active === false)
}

export function reactivateJoinPayload(join: SemanticJoin) {
  return {
    name: join.name,
    from_schema: join.from_schema ?? '',
    from_table: join.from_table,
    from_column: join.from_column,
    to_schema: join.to_schema ?? '',
    to_table: join.to_table,
    to_column: join.to_column,
    join_type: join.join_type,
    relationship: join.relationship,
    is_active: true,
  }
}

export function renameDimensionPayload(dimension: SemanticDimension, label: string) {
  return {
    name: dimension.name,
    label,
    column_ref: dimension.column_ref,
    type: dimension.type,
    synonyms: dimension.synonyms ?? [],
    description: dimension.description ?? '',
    is_active: dimension.is_active,
  }
}

export function reactivateDimensionPayload(dimension: SemanticDimension) {
  return {
    ...renameDimensionPayload(dimension, dimension.label ?? ''),
    is_active: true,
  }
}

export function renameMetricPayload(metric: SemanticMetric, label: string) {
  return {
    name: metric.name,
    label,
    expression: metric.expression,
    aggregation: metric.aggregation,
    format: metric.format ?? '',
    synonyms: metric.synonyms ?? [],
    description: metric.description ?? '',
    is_active: metric.is_active,
  }
}

export function reactivateMetricPayload(metric: SemanticMetric) {
  return {
    ...renameMetricPayload(metric, metric.label ?? ''),
    is_active: true,
  }
}
