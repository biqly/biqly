import type { SemanticExprNode, SemanticMetric, SemanticModelDetail } from '../../types/semantic'

export function parseMetricExpressionParts(
  metric: SemanticMetric | undefined,
  model: SemanticModelDetail,
): { schema: string; table: string; column: string } {
  const parts = metric && metric.aggregation !== 'custom' ? metric.expression.split('.') : []
  let schema = model.base_schema
  let table = model.base_table
  let column = ''
  if (parts.length === 3) {
    schema = parts[0] ?? model.base_schema
    table = parts[1] ?? model.base_table
    column = parts[2] ?? ''
  } else if (parts.length === 2) {
    table = parts[0] ?? model.base_table
    column = parts[1] ?? ''
  } else if (parts.length === 1) {
    column = parts[0] ?? ''
  }
  return { schema, table, column }
}

export function buildMetricColumnRef(
  model: SemanticModelDetail,
  schema: string,
  table: string,
  column: string,
): string {
  return schema === model.base_schema ? `${table}.${column}` : `${schema}.${table}.${column}`
}

export function buildMetricSubmitPayload(
  mode: 'simple' | 'custom',
  model: SemanticModelDetail,
  fields: {
    name: string
    label: string
    format: string
    selectedSchema: string
    selectedTable: string
    selectedColumn: string
    selectedAggregation: string
    expression: string
    astNode?: SemanticExprNode
  },
): { expression: string; aggregation: string; expr?: SemanticExprNode } | null {
  if (mode === 'simple') {
    if (!fields.selectedColumn) {
      return null
    }
    return {
      expression: buildMetricColumnRef(
        model,
        fields.selectedSchema,
        fields.selectedTable,
        fields.selectedColumn,
      ),
      aggregation: fields.selectedAggregation,
    }
  }
  if (!fields.expression.trim()) {
    return null
  }
  return {
    expression: fields.expression.trim(),
    aggregation: 'custom',
    expr: fields.astNode,
  }
}
