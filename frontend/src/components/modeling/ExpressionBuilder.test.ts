import { describe, expect, it } from 'vitest'

import { computeExpressionSuggestions, type SuggestionContext } from './ExpressionBuilder'

const ctx: SuggestionContext = {
  allowAggregates: false,
  columns: [
    { table: 'orders', column: 'total_amount', dataType: 'numeric' },
    { table: 'customers', column: 'first_name', dataType: 'text' },
  ],
  dimensions: [{ name: 'region', label: 'Region' }],
  metrics: [{ name: 'revenue', label: 'Revenue' }],
}

describe('computeExpressionSuggestions', () => {
  it('suggests functions by prefix, excluding aggregates when not allowed', () => {
    const res = computeExpressionSuggestions('co', 2, ctx)
    expect(res).not.toBeNull()
    expect(res?.start).toBe(0)
    const names = res?.items.map((i) => i.label)
    expect(names).toContain('COALESCE')
    expect(names).toContain('CONCAT')
    expect(names).not.toContain('COUNT')
  })

  it('includes aggregate functions when allowed', () => {
    const res = computeExpressionSuggestions('su', 2, { ...ctx, allowAggregates: true })
    expect(res?.items.map((i) => i.label)).toContain('SUM')
    expect(res?.items.map((i) => i.label)).toContain('SUBSTRING')
  })

  it('places the caret inside parentheses for function inserts', () => {
    const res = computeExpressionSuggestions('upp', 3, ctx)
    const upper = res?.items.find((i) => i.label === 'UPPER')
    expect(upper?.insertText).toBe('UPPER()')
    expect(upper?.caretOffset).toBe('UPPER('.length)
  })

  it('suggests columns and dimensions after [ but metrics only with aggregates', () => {
    const res = computeExpressionSuggestions('[', 1, ctx)
    const kinds = res?.items.map((i) => `${i.kind}:${i.label}`)
    expect(kinds).toContain('column:orders.total_amount')
    expect(kinds).toContain('dimension:region')
    expect(kinds?.some((k) => k.startsWith('metric:'))).toBe(false)

    const withAgg = computeExpressionSuggestions('[', 1, { ...ctx, allowAggregates: true })
    expect(withAgg?.items.map((i) => `${i.kind}:${i.label}`)).toContain('metric:revenue')
  })

  it('filters bracket suggestions by substring and wraps insert in brackets', () => {
    const res = computeExpressionSuggestions('sum([tot', 8, ctx)
    expect(res?.start).toBe(4)
    expect(res?.items).toHaveLength(1)
    expect(res?.items[0]?.insertText).toBe('[orders.total_amount]')
  })

  it('returns null when there is no token at the caret', () => {
    expect(computeExpressionSuggestions('1 + ', 4, ctx)).toBeNull()
    expect(computeExpressionSuggestions('', 0, ctx)).toBeNull()
  })

  it('only matches the token immediately before the caret', () => {
    const res = computeExpressionSuggestions('upper(x) lo', 11, ctx)
    expect(res?.start).toBe(9)
    expect(res?.items.map((i) => i.label)).toContain('LOWER')
  })
})
