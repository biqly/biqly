import type { FilterRow, HavingRow } from './types'
import { newRowId } from './types'

export function removeRowAt<T>(rows: T[], index: number): T[] {
  return rows.filter((_, i) => i !== index)
}

export function appendRow<T>(rows: T[], row: T): T[] {
  return [...rows, row]
}

export function updateRowAt<T>(rows: T[], index: number, row: T): T[] {
  return rows.map((existing, i) => (i === index ? row : existing))
}

export function defaultFilterRow(id = newRowId()): FilterRow {
  return { id, field: '', operator: 'eq', value: '' }
}

export function defaultHavingRow(): HavingRow {
  return { field: '', operator: 'gt', value: '' }
}

export function patchFilterRow(
  existing: FilterRow | undefined,
  field: keyof FilterRow,
  value: string,
  id = newRowId(),
): FilterRow {
  return {
    id: existing?.id ?? id,
    field: existing?.field ?? '',
    operator: existing?.operator ?? 'eq',
    value: existing?.value ?? '',
    [field]: value,
  }
}

export function patchHavingRow(
  existing: HavingRow | undefined,
  field: keyof HavingRow,
  value: string,
): HavingRow {
  return {
    field: existing?.field ?? '',
    operator: existing?.operator ?? 'gt',
    value: existing?.value ?? '',
    [field]: value,
  }
}

export function addFilterRow(filters: FilterRow[]): FilterRow[] {
  return appendRow(filters, defaultFilterRow())
}

export function removeFilterRow(filters: FilterRow[], index: number): FilterRow[] {
  return removeRowAt(filters, index)
}

export function addGroupByRow(groupBy: string[]): string[] {
  return appendRow(groupBy, '')
}

export function removeGroupByRow(groupBy: string[], index: number): string[] {
  return removeRowAt(groupBy, index)
}

export function updateGroupByRow(groupBy: string[], index: number, value: string): string[] {
  return updateRowAt(groupBy, index, value)
}

export function addHavingRow(having: HavingRow[]): HavingRow[] {
  return appendRow(having, defaultHavingRow())
}

export function removeHavingRow(having: HavingRow[], index: number): HavingRow[] {
  return removeRowAt(having, index)
}
