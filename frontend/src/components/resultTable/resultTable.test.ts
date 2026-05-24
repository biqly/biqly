import { describe, expect, it } from 'vitest'
import { formatResultCell } from '../../utils/resultCellFormat'
import { buildAnomalyCellSet, isAnomalyCell } from './anomalies'
import {
  buildContextMenuFromCellRect,
  buildContextMenuFromPointer,
  isContextMenuKey,
} from './contextMenu'
import {
  ariaSortValue,
  compareCellValues,
  cycleSortState,
  indexRows,
  sortArrow,
  sortIndexedRows,
} from './sort'

describe('sorting', () => {
  const rows = [
    ['gamma', 30],
    ['alpha', 10],
    [null, 20],
    ['beta', null],
  ]

  it('cycles sort state asc → desc → cleared on same column', () => {
    expect(cycleSortState(null, null, 1)).toEqual({ sortColIdx: 1, sortDir: 'asc' })
    expect(cycleSortState(1, 'asc', 1)).toEqual({ sortColIdx: 1, sortDir: 'desc' })
    expect(cycleSortState(1, 'desc', 1)).toEqual({ sortColIdx: null, sortDir: null })
  })

  it('resets to asc when a different column is clicked', () => {
    expect(cycleSortState(1, 'desc', 0)).toEqual({ sortColIdx: 0, sortDir: 'asc' })
  })

  it('sorts strings ascending and descending', () => {
    const indexed = indexRows(rows)
    const asc = sortIndexedRows(indexed, 0, 'asc').map((r) => r.row[0])
    expect(asc).toEqual(['alpha', 'beta', 'gamma', null])

    const desc = sortIndexedRows(indexed, 0, 'desc').map((r) => r.row[0])
    expect(desc).toEqual([null, 'gamma', 'beta', 'alpha'])
  })

  it('sorts numeric columns and places nulls last in ascending order', () => {
    const indexed = indexRows(rows)
    const asc = sortIndexedRows(indexed, 1, 'asc').map((r) => r.row[1])
    expect(asc).toEqual([10, 20, 30, null])
  })

  it('compareCellValues uses numeric comparison when both parse as numbers', () => {
    expect(compareCellValues('10', '2', 1)).toBe(8)
    expect(compareCellValues('10', '2', -1)).toBe(-8)
  })

  it('exposes aria-sort and arrow helpers for active column', () => {
    expect(ariaSortValue(1, 'asc', 1)).toBe('ascending')
    expect(ariaSortValue(1, 'desc', 1)).toBe('descending')
    expect(ariaSortValue(1, 'asc', 0)).toBe('none')
    expect(sortArrow(1, 'asc', 1)).toBe('↑')
    expect(sortArrow(1, 'desc', 1)).toBe('↓')
    expect(sortArrow(null, null, 0)).toBe('')
  })
})

describe('context menu', () => {
  it('recognizes ContextMenu and Shift+F10 keys', () => {
    expect(isContextMenuKey('ContextMenu', false)).toBe(true)
    expect(isContextMenuKey('F10', true)).toBe(true)
    expect(isContextMenuKey('F10', false)).toBe(false)
    expect(isContextMenuKey('Enter', false)).toBe(false)
  })

  it('builds menu state from pointer or cell rect', () => {
    expect(buildContextMenuFromPointer(100, 200, 'region', 'EMEA')).toEqual({
      x: 100,
      y: 200,
      colName: 'region',
      value: 'EMEA',
    })
    expect(buildContextMenuFromCellRect({ left: 12, bottom: 48 }, 'status', null)).toEqual({
      x: 12,
      y: 48,
      colName: 'status',
      value: '',
    })
  })
})

describe('anomaly highlighting', () => {
  it('marks cells by row index and column name', () => {
    const set = buildAnomalyCellSet([
      { row_index: 2, column: 'total' },
      { row_index: 0, column: 'region' },
    ])
    expect(isAnomalyCell(set, 2, 'total')).toBe(true)
    expect(isAnomalyCell(set, 2, 'region')).toBe(false)
    expect(isAnomalyCell(set, 0, 'region')).toBe(true)
  })
})

describe('cell formatting (ResultTable display)', () => {
  it('renders empty string for null and undefined cells', () => {
    expect(formatResultCell(null, 'amount')).toBe('')
    expect(formatResultCell(undefined, 'amount')).toBe('')
  })

  it('passes through non-numeric text unchanged', () => {
    expect(formatResultCell('shipped', 'status')).toBe('shipped')
  })

  it('formats metric columns with grouping for table display', () => {
    expect(formatResultCell(9876543, 'total_revenue')).toBe('9,876,543')
    expect(formatResultCell(42.789, 'total_revenue', { question: '2 decimal places' })).toBe('42.79')
  })
})
