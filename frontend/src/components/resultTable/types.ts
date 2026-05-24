export type SortDirection = 'asc' | 'desc' | null

export interface IndexedRow {
  row: unknown[]
  originalIndex: number
}

export interface ContextMenuState {
  x: number
  y: number
  colName: string
  value: string
}

export interface ResultAnomalyLike {
  row_index: number
  column: string
}
