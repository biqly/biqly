export interface EvalTestCase {
  id: string
  question: string
  status: 'pass' | 'fail'
  expected_logical_query: Record<string, unknown>
  got_logical_query: Record<string, unknown>
  confidence?: number
  error_message?: string
}

export interface EvalRunResponse {
  total: number
  passed: number
  failed: number
  pass_rate: number
  avg_confidence: number
  test_cases: EvalTestCase[]
  accuracy_trend?: { date: string; pass_rate: number }[]
}

export const DEMO_DATA: EvalRunResponse = {
  total: 42,
  passed: 34,
  failed: 8,
  pass_rate: 0.81,
  avg_confidence: 0.76,
  test_cases: [
    {
      id: 'TC-001',
      question: 'Show total revenue by country for January 2026',
      status: 'pass',
      expected_logical_query: {
        select: [
          { type: 'dimension', name: 'country' },
          { type: 'metric', name: 'revenue', aggregation: 'SUM' },
        ],
        filters: [
          { field: 'order_date', operator: 'between', value: ['2026-01-01', '2026-01-31'] },
        ],
        group_by: [{ field: 'country' }],
      },
      got_logical_query: {
        select: [
          { type: 'dimension', name: 'country' },
          { type: 'metric', name: 'revenue', aggregation: 'SUM' },
        ],
        filters: [
          { field: 'order_date', operator: 'between', value: ['2026-01-01', '2026-01-31'] },
        ],
        group_by: [{ field: 'country' }],
      },
      confidence: 0.95,
    },
    {
      id: 'TC-002',
      question: 'List top 5 customers by order count',
      status: 'pass',
      expected_logical_query: {
        select: [
          { type: 'dimension', name: 'customer_name' },
          { type: 'metric', name: 'order_id', aggregation: 'COUNT' },
        ],
        order_by: [{ field: 'order_id', direction: 'desc' }],
        limit: 5,
        group_by: [{ field: 'customer_name' }],
      },
      got_logical_query: {
        select: [
          { type: 'dimension', name: 'customer_name' },
          { type: 'metric', name: 'order_id', aggregation: 'COUNT' },
        ],
        order_by: [{ field: 'order_id', direction: 'desc' }],
        limit: 5,
        group_by: [{ field: 'customer_name' }],
      },
      confidence: 0.88,
    },
    {
      id: 'TC-003',
      question: 'Find orders where shipping cost exceeds $100',
      status: 'fail',
      expected_logical_query: {
        select: [
          { type: 'dimension', name: 'order_id' },
          { type: 'dimension', name: 'shipping_cost' },
        ],
        filters: [{ field: 'shipping_cost', operator: 'gt', value: 100 }],
      },
      got_logical_query: {
        select: [{ type: 'dimension', name: 'order_id' }],
        filters: [{ field: 'shipping_cost', operator: 'gte', value: 100 }],
      },
      confidence: 0.42,
      error_message: 'Missing select column "shipping_cost"; operator mismatch (gte vs gt)',
    },
    {
      id: 'TC-004',
      question: 'Average order value per region for Q4 2025',
      status: 'pass',
      expected_logical_query: {
        select: [
          { type: 'dimension', name: 'region' },
          { type: 'metric', name: 'order_value', aggregation: 'AVG' },
        ],
        filters: [
          { field: 'order_date', operator: 'between', value: ['2025-10-01', '2025-12-31'] },
        ],
        group_by: [{ field: 'region' }],
      },
      got_logical_query: {
        select: [
          { type: 'dimension', name: 'region' },
          { type: 'metric', name: 'order_value', aggregation: 'AVG' },
        ],
        filters: [
          { field: 'order_date', operator: 'between', value: ['2025-10-01', '2025-12-31'] },
        ],
        group_by: [{ field: 'region' }],
      },
      confidence: 0.91,
    },
    {
      id: 'TC-005',
      question: 'Count distinct products sold per salesperson',
      status: 'fail',
      expected_logical_query: {
        select: [
          { type: 'dimension', name: 'salesperson' },
          { type: 'metric', name: 'product_id', aggregation: 'COUNT_DISTINCT' },
        ],
        group_by: [{ field: 'salesperson' }],
      },
      got_logical_query: {
        select: [
          { type: 'dimension', name: 'salesperson' },
          { type: 'metric', name: 'product_id', aggregation: 'COUNT' },
        ],
        group_by: [{ field: 'salesperson' }],
      },
      confidence: 0.55,
      error_message: 'Used COUNT instead of COUNT_DISTINCT',
    },
  ],
  accuracy_trend: [
    { date: '2026-05-04', pass_rate: 0.72 },
    { date: '2026-05-05', pass_rate: 0.75 },
    { date: '2026-05-06', pass_rate: 0.7 },
    { date: '2026-05-07', pass_rate: 0.78 },
    { date: '2026-05-08', pass_rate: 0.8 },
    { date: '2026-05-09', pass_rate: 0.79 },
    { date: '2026-05-10', pass_rate: 0.81 },
  ],
}
