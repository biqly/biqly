import type { SemanticDimension } from '../../types/semantic'
import { buildFilterSaveValue, parseFilterChips } from './tableBrowserFilterUtils'

export interface TableBrowserFilter {
  id: string
  field: string
  operator: string
  value: string
  caseSensitive?: boolean
}

export function createTableBrowserFilter(
  field: string,
  operator: string,
  value: string,
  caseSensitive: boolean,
): TableBrowserFilter {
  return {
    id: Math.random().toString(36).substr(2, 9),
    field,
    operator,
    value,
    caseSensitive,
  }
}

export function updateTableBrowserFilter(
  filters: TableBrowserFilter[],
  id: string,
  patch: Omit<TableBrowserFilter, 'id'>,
): TableBrowserFilter[] {
  return filters.map((f) => (f.id === id ? { ...f, ...patch } : f))
}

export function saveTableBrowserFilter(params: {
  popoverField: string
  popoverOperator: string
  popoverChips: string[]
  chipInputText: string
  popoverCaseSensitive: boolean
  editingFilterId: string | null
  filters: TableBrowserFilter[]
}): { filters: TableBrowserFilter[]; saved: boolean } {
  const {
    popoverField,
    popoverOperator,
    popoverChips,
    chipInputText,
    popoverCaseSensitive,
    editingFilterId,
    filters,
  } = params
  if (!popoverField) {
    return { filters, saved: false }
  }
  const finalValue = buildFilterSaveValue(popoverChips, chipInputText)
  if (!finalValue) {
    return { filters, saved: false }
  }
  if (editingFilterId) {
    return {
      filters: updateTableBrowserFilter(filters, editingFilterId, {
        field: popoverField,
        operator: popoverOperator,
        value: finalValue,
        caseSensitive: popoverCaseSensitive,
      }),
      saved: true,
    }
  }
  return {
    filters: [
      ...filters,
      createTableBrowserFilter(popoverField, popoverOperator, finalValue, popoverCaseSensitive),
    ],
    saved: true,
  }
}

export function defaultFilterField(activeDimensions: SemanticDimension[]): string {
  return activeDimensions[0]?.name ?? ''
}

export function filterPopoverStateForAdd(defaultField: string) {
  return {
    editingFilterId: null as string | null,
    popoverField: defaultField,
    popoverOperator: 'contains',
    popoverChips: [] as string[],
    chipInputText: '',
    popoverCaseSensitive: false,
  }
}

export function filterPopoverStateForEdit(filter: TableBrowserFilter) {
  return {
    editingFilterId: filter.id,
    popoverField: filter.field,
    popoverOperator: filter.operator,
    popoverChips: parseFilterChips(filter.value),
    chipInputText: '',
    popoverCaseSensitive: !!filter.caseSensitive,
  }
}
