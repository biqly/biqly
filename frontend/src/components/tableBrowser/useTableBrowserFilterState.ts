import { type DragEvent, useCallback, useMemo, useState } from 'react'

import type { TFunction } from '../../i18n'
import {
  defaultFilterField,
  filterPopoverStateForAdd,
  filterPopoverStateForEdit,
  saveTableBrowserFilter,
  type TableBrowserFilter,
} from './tableBrowserFilterHandlers'

function reorderColumnNames(order: string[], source: string, target: string): string[] {
  const next = order.filter((n) => n !== source)
  const idx = next.indexOf(target)
  if (idx === -1) {
    next.push(source)
  } else {
    next.splice(idx, 0, source)
  }
  return next
}

export function useTableBrowserFilterState({
  activeDimensions,
  t,
  onFiltersChange,
  dimensionNamesKey,
  modelId,
  selectedTableKey,
}: {
  activeDimensions: { name: string; label?: string | null }[]
  t: TFunction
  onFiltersChange: (filters: TableBrowserFilter[]) => void
  dimensionNamesKey: string
  modelId: string
  selectedTableKey: string
}) {
  const scopeKey = `${modelId}:${selectedTableKey}:${dimensionNamesKey}`
  const defaultColumnOrder = useMemo(
    () => activeDimensions.map((d) => d.name).sort((a, b) => a.localeCompare(b)),
    [activeDimensions],
  )
  const [filtersState, setFiltersState] = useState<{
    key: string
    filters: TableBrowserFilter[]
  }>({ key: '', filters: [] })
  const filters = useMemo(() => {
    const scopedFilters = filtersState.key === scopeKey ? filtersState.filters : []
    return scopedFilters.filter((f) => defaultColumnOrder.includes(f.field))
  }, [filtersState, scopeKey, defaultColumnOrder])
  const [popoverOpen, setPopoverOpen] = useState(false)
  const [popoverAnchorEl, setPopoverAnchorEl] = useState<HTMLElement | null>(null)
  const [popoverField, setPopoverField] = useState('')
  const [popoverOperator, setPopoverOperator] = useState('contains')
  const [popoverChips, setPopoverChips] = useState<string[]>([])
  const [chipInputText, setChipInputText] = useState('')
  const [popoverCaseSensitive, setPopoverCaseSensitive] = useState(false)
  const [editingFilterId, setEditingFilterId] = useState<string | null>(null)
  const [dragColumn, setDragColumn] = useState<string | null>(null)
  const [dropTargetColumn, setDropTargetColumn] = useState<string | null>(null)
  const [columnOrderState, setColumnOrderState] = useState<{
    key: string
    order: string[]
  }>({ key: '', order: [] })
  const columnOrder =
    columnOrderState.key === scopeKey ? columnOrderState.order : defaultColumnOrder

  const setColumnOrder = useCallback(
    (next: string[] | ((prev: string[]) => string[])) => {
      setColumnOrderState((prev) => {
        const current = prev.key === scopeKey ? prev.order : defaultColumnOrder
        const resolved = typeof next === 'function' ? next(current) : next
        return { key: scopeKey, order: resolved }
      })
    },
    [defaultColumnOrder, scopeKey],
  )

  const updateFilters = useCallback(
    (next: TableBrowserFilter[] | ((prev: TableBrowserFilter[]) => TableBrowserFilter[])) => {
      setFiltersState((prev) => {
        const current = prev.key === scopeKey ? prev.filters : []
        const resolved = typeof next === 'function' ? next(current) : next
        onFiltersChange(resolved)
        return { key: scopeKey, filters: resolved }
      })
    },
    [onFiltersChange, scopeKey],
  )

  const filtersKey = useMemo(() => JSON.stringify(filters), [filters])

  const filterPayload = useMemo(
    () =>
      filters.map((f) => ({
        id: f.id,
        field: f.field,
        operator: f.operator,
        value: f.value,
        caseSensitive: f.caseSensitive,
      })),
    [filters],
  )

  const handleAddChip = (text: string) => {
    const clean = text.trim()
    if (clean && !popoverChips.includes(clean)) {
      setPopoverChips((prev) => [...prev, clean])
    }
    setChipInputText('')
  }

  const handleRemoveChip = (index: number) => {
    setPopoverChips((prev) => prev.filter((_, i) => i !== index))
  }

  const resetFilterPopover = () => {
    setPopoverOpen(false)
    setPopoverAnchorEl(null)
    setEditingFilterId(null)
    setPopoverChips([])
    setChipInputText('')
    setPopoverCaseSensitive(false)
  }

  const handleCloseFilterPopover = () => {
    resetFilterPopover()
  }

  const handleSaveFilter = () => {
    const { filters: next, saved } = saveTableBrowserFilter({
      popoverField,
      popoverOperator,
      popoverChips,
      chipInputText,
      popoverCaseSensitive,
      editingFilterId,
      filters,
    })
    if (!saved) {
      return
    }
    updateFilters(next)
    resetFilterPopover()
  }

  const handleColumnDragStart = (colName: string) => (e: DragEvent) => {
    setDragColumn(colName)
    e.dataTransfer.effectAllowed = 'move'
    e.dataTransfer.setData('text/plain', colName)
  }

  const handleColumnDragOver = (colName: string) => (e: DragEvent) => {
    e.preventDefault()
    e.dataTransfer.dropEffect = 'move'
    if (dragColumn && dragColumn !== colName) {
      setDropTargetColumn(colName)
    }
  }

  const handleColumnDrop = (colName: string) => (e: DragEvent) => {
    e.preventDefault()
    const source = e.dataTransfer.getData('text/plain') || dragColumn
    if (source && source !== colName) {
      const fallback = activeDimensions.map((d) => d.name)
      setColumnOrder((prev) => reorderColumnNames(prev.length ? prev : fallback, source, colName))
    }
    setDragColumn(null)
    setDropTargetColumn(null)
  }

  const handleColumnDragEnd = () => {
    setDragColumn(null)
    setDropTargetColumn(null)
  }

  const applyFilterPopoverState = (state: ReturnType<typeof filterPopoverStateForAdd>) => {
    setEditingFilterId(state.editingFilterId)
    setPopoverField(state.popoverField)
    setPopoverOperator(state.popoverOperator)
    setPopoverChips(state.popoverChips)
    setChipInputText(state.chipInputText)
    setPopoverCaseSensitive(state.popoverCaseSensitive)
    setPopoverOpen(true)
  }

  const handleOpenAddFilter = (defaultField = '', anchorEl?: HTMLElement | null) => {
    setPopoverAnchorEl(anchorEl ?? null)
    applyFilterPopoverState(
      filterPopoverStateForAdd(defaultField || defaultFilterField(activeDimensions)),
    )
  }

  const handleOpenEditFilter = (filter: TableBrowserFilter, anchorEl?: HTMLElement | null) => {
    setPopoverAnchorEl(anchorEl ?? null)
    applyFilterPopoverState(filterPopoverStateForEdit(filter))
  }

  const handleRemoveFilter = (id: string) => {
    updateFilters(filters.filter((f) => f.id !== id))
  }

  const getDimensionLabel = (name: string) => {
    const dim = activeDimensions.find((d) => d.name === name)
    return dim ? (dim.label ?? dim.name) : name
  }

  const operatorLabels = useMemo(
    () => ({
      eq: t('table_browser.op_eq'),
      neq: t('table_browser.op_neq'),
      contains: t('table_browser.op_contains'),
      starts_with: t('table_browser.op_starts_with'),
      ends_with: t('table_browser.op_ends_with'),
      gt: t('table_browser.op_gt'),
      lt: t('table_browser.op_lt'),
      gte: t('table_browser.op_gte'),
      lte: t('table_browser.op_lte'),
    }),
    [t],
  )

  const operatorOptions = useMemo(
    () => [
      { value: 'contains', label: t('table_browser.op_contains') },
      { value: 'starts_with', label: t('table_browser.op_starts_with') },
      { value: 'ends_with', label: t('table_browser.op_ends_with') },
      { value: 'eq', label: t('table_browser.op_eq') },
      { value: 'neq', label: t('table_browser.op_neq') },
      { value: 'gt', label: t('table_browser.op_gt') },
      { value: 'lt', label: t('table_browser.op_lt') },
      { value: 'gte', label: t('table_browser.op_gte') },
      { value: 'lte', label: t('table_browser.op_lte') },
    ],
    [t],
  )

  const filterFieldOpts = useMemo(
    () =>
      activeDimensions.map((d) => ({
        value: d.name,
        label: d.label ?? d.name,
      })),
    [activeDimensions],
  )

  return {
    filters,
    setFilters: updateFilters,
    filterPayload,
    filtersKey,
    columnOrder,
    setColumnOrder,
    popoverOpen,
    setPopoverOpen,
    popoverAnchorEl,
    popoverField,
    setPopoverField,
    popoverOperator,
    setPopoverOperator,
    popoverChips,
    chipInputText,
    setChipInputText,
    popoverCaseSensitive,
    setPopoverCaseSensitive,
    editingFilterId,
    dragColumn,
    dropTargetColumn,
    handleAddChip,
    handleRemoveChip,
    handleSaveFilter,
    handleColumnDragStart,
    handleColumnDragOver,
    handleColumnDrop,
    handleColumnDragEnd,
    handleOpenAddFilter,
    handleOpenEditFilter,
    handleCloseFilterPopover,
    handleRemoveFilter,
    getDimensionLabel,
    operatorLabels,
    operatorOptions,
    filterFieldOpts,
  }
}
