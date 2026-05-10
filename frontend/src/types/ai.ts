// ─── AI Query Types ────────────────────────────────────────────────

export interface AIQueryRequest {
  datasource_id: string
  question: string
  tables?: string[]
  include_base_tables?: boolean
  include_views?: boolean
  model?: string
  conversation_id?: string
  example_ids?: string[]
  prior_turns?: PriorTurn[]
}

export interface PriorTurn {
  question: string
  logical_query?: LogicalQuery
  note?: string
}

export interface TableRoutingCandidate {
  table: string
  relevance_score: number
  selected: boolean
}

export interface TableRoutingResult {
  selected_tables: string[]
  confidence: number
  ranking_method: 'keyword' | 'vector' | 'hybrid'
  candidates: TableRoutingCandidate[]
  reasoning?: string
}

export interface LogicalQueryCandidate {
  logical_query: LogicalQuery
  confidence: number
  reasoning?: string
}

export interface ConfidenceBreakdown {
  table_routing: number
  llm: number
  validation: number
}

export interface TokenUsage {
  prompt: number
  completion: number
  total: number
}

export interface ValidationExplainResult {
  explain_output?: string
  plan_ok: boolean
}

export interface VisualizationHint {
  chart_type: 'bar' | 'line' | 'pie' | 'table'
  reason: string
}

export interface AIQueryResponse {
  confidence?: number
  confidence_breakdown?: ConfidenceBreakdown
  table_routing?: TableRoutingResult
  logical_query?: LogicalQuery
  sql?: string
  warnings?: string[]
  result?: QueryResultPayload
  // Disambiguation
  needs_clarification?: boolean
  clarification_question?: string
  clarification_options?: string[]
  // Multi-candidate
  candidates?: LogicalQueryCandidate[]
  candidates_count?: number
  // Retry / validation
  retry_count?: number
  validation_result?: ValidationExplainResult
  // Prompt transparency
  prompt?: string
  token_usage?: TokenUsage
  // Model info
  model_used?: string
  // Visualization
  visualization_hint?: VisualizationHint
  // Performance
  latency_ms?: number
  cost_usd?: number
}

export interface ConversationMessage {
  role: 'user' | 'assistant'
  content: string
  timestamp: string
  ai_response?: AIQueryResponse
}

export interface Conversation {
  id: string
  messages: ConversationMessage[]
  created_at: string
  updated_at: string
  title?: string
}

// ─── LogicalQuery Types ────────────────────────────────────────────

export interface LogicalQuery {
  data_source: string
  tables?: string[]
  select?: SelectField[]
  filters?: FilterClause[]
  group_by?: GroupByField[]
  order_by?: OrderByField[]
  having?: FilterClause[]
  limit?: number
  ctes?: CTE[]
  window_functions?: WindowFunction[]
}

export interface SelectField {
  type: 'dimension' | 'metric'
  name: string
  alias?: string
  aggregation?: string
}

export interface FilterClause {
  field: string
  operator: string
  value: unknown
}

export interface GroupByField {
  field: string
}

export interface OrderByField {
  field: string
  direction: 'asc' | 'desc'
}

export interface CTE {
  name: string
  query: LogicalQuery
}

export interface WindowFunction {
  function: 'ROW_NUMBER' | 'RANK' | 'DENSE_RANK' | 'LAG' | 'LEAD' | 'SUM' | 'AVG' | 'COUNT'
  field: string
  partition_by?: string[]
  order_by?: OrderByField[]
  alias?: string
}

// ─── Query Result Types ────────────────────────────────────────────

export interface QueryResultPayload {
  columns: QueryColumn[]
  rows: unknown[][]
  stats?: {
    row_count: number
    duration_ms: number
  }
}

export interface QueryColumn {
  name: string
  type?: string
}

export interface CompiledQuery {
  sql: string
  dialect: string
  parameters?: Record<string, unknown>
}
