import type { SemanticDimension, SemanticMetric, TableRow } from '../../types/semantic'

export interface SuggestedJoin {
  from_schema: string
  from_table: string
  from_column: string
  to_schema: string
  to_table: string
  to_column: string
  name: string
}

export interface JoinForm {
  fromTable: string
  fromColumn: string
  toTable: string
  toColumn: string
  joinType: 'LEFT' | 'INNER' | 'RIGHT'
  relationship: 'many_to_one' | 'one_to_many' | 'one_to_one' | 'many_to_many'
}

export type Tab = 'joins' | 'dimensions' | 'metrics' | 'tables'

export type RenameTarget =
  | { kind: 'model'; current: string; title: string; subtitle: string }
  | { kind: 'table'; current: string; table: TableRow; title: string; subtitle: string }
  | {
      kind: 'dimension'
      current: string
      dimension: SemanticDimension
      title: string
      subtitle: string
    }
  | { kind: 'metric'; current: string; metric: SemanticMetric; title: string; subtitle: string }

export interface Pt {
  x: number
  y: number
}

export interface Viewport {
  scale: number
  tx: number
  ty: number
}

export interface JoinPath {
  x1: number
  y1: number
  x2: number
  y2: number
  d: string
}

export interface CardLayout {
  columnsShown: import('../../types/semantic').ColumnRow[]
  columnIndex: Map<string, number>
  height: number
  hiddenCount: number
}

export interface JoinPayload {
  name: string
  from_schema: string
  from_table: string
  from_column: string
  to_schema: string
  to_table: string
  to_column: string
  join_type: JoinForm['joinType']
  relationship: JoinForm['relationship']
}

export interface SuggestedJoinPayload extends JoinPayload {
  join_type: 'LEFT'
  relationship: 'many_to_one'
}

export function suggestedJoinToPayload(suggestion: SuggestedJoin): SuggestedJoinPayload {
  return {
    name: suggestion.name,
    from_schema: suggestion.from_schema,
    from_table: suggestion.from_table,
    from_column: suggestion.from_column,
    to_schema: suggestion.to_schema,
    to_table: suggestion.to_table,
    to_column: suggestion.to_column,
    join_type: 'LEFT',
    relationship: 'many_to_one',
  }
}

export function publishModelRequest(modelId: string) {
  return {
    url: `/api/semantic/models/${modelId}/publish`,
    body: { published_by: 'modeling-ui' },
  }
}
