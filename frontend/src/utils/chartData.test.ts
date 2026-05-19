import { expect, test } from 'vitest'
import { rowsToChartData } from './chartData'

test('converts rows to chart objects', () => {
  const rows = [['Jan', 100], ['Feb', 200]]
  const result = rowsToChartData(rows)
  expect(result).toEqual([
    { name: 'Jan', value: 100 },
    { name: 'Feb', value: 200 }
  ])
})

test('handles undefined rows', () => {
  const result = rowsToChartData(undefined)
  expect(result).toEqual([])
})

test('handles non-numeric values gracefully', () => {
  const rows = [['Jan', 'invalid'], ['Feb', null]]
  const result = rowsToChartData(rows as any)
  expect(result).toEqual([
    { name: 'Jan', value: 0 },
    { name: 'Feb', value: 0 }
  ])
})

test('handles single column rows', () => {
  const rows = [['Jan'], ['Feb']]
  const result = rowsToChartData(rows)
  expect(result).toEqual([
    { name: 'Jan' },
    { name: 'Feb' }
  ])
})
