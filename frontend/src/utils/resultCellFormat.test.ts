import { describe, expect, it } from 'vitest'
import { formatResultCell } from './resultCellFormat'

describe('formatResultCell', () => {
  it('formats regular numeric values with grouping and rounded digits by default', () => {
    expect(formatResultCell(12345.67, 'total_amount')).toBe('12,346')
  })

  it('keeps identifier-like and calendar values ungrouped', () => {
    expect(formatResultCell(11091, 'customer_id')).toBe('11091')
    expect(formatResultCell(2025, 'order_year')).toBe('2025')
  })

  it('shows fractional digits only when the question asks for them', () => {
    expect(formatResultCell(12.3456, 'average_price')).toBe('12')
    expect(formatResultCell(12.3456, 'average_price', { question: 'show 2 decimal places' })).toBe(
      '12.35',
    )
  })
})
