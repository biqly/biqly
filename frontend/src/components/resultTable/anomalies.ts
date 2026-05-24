import type { ResultAnomalyLike } from './types'

export function buildAnomalyCellSet(anomalies: ResultAnomalyLike[] | undefined): Set<string> {
  const set = new Set<string>()
  for (const a of anomalies ?? []) {
    set.add(`${a.row_index}:${a.column}`)
  }
  return set
}

export function isAnomalyCell(set: Set<string>, originalIndex: number, colName: string): boolean {
  return set.has(`${originalIndex}:${colName}`)
}
