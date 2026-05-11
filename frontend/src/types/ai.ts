// ─── AI runtime (server env) ───────────────────────────────────────

export interface AIRuntimeSettings {
  provider: string
  llm_model: string
  base_url: string
  base_url_effective: string
  api_key_configured: boolean
  /** When true, embedding env block is active. Absent/false: no embedding UI. */
  embeddings_enabled?: boolean
  embedding_model?: string
  embedding_base_url?: string
  embedding_base_url_effective?: string
  embedding_api_key_configured?: boolean
  /** True if BI_AI_EMBEDDING_API_KEY is set (vs only BI_AI_API_KEY). */
  embedding_api_key_dedicated?: boolean
  /** When true, AI-generated metadata descriptions are translated/normalized after Describe. */
  translation_enabled?: boolean
  translation_model?: string
  translation_base_url?: string
  translation_base_url_effective?: string
  translation_api_key_configured?: boolean
  /** True if BI_AI_TRANSLATION_API_KEY is set (vs only BI_AI_API_KEY). */
  translation_api_key_dedicated?: boolean
  translation_target_language?: string
  translation_target_code?: string
}

export interface EmbedMetadataResult {
  schema: string
  table: string
  column?: string
  kind?: 'table' | 'column'
  skipped?: boolean
  reason?: string
}

export interface EmbedMetadataResponse {
  datasource_id: string
  model: string
  embedded: number
  skipped: number
  results?: EmbedMetadataResult[]
}

// ─── AI Query Types ────────────────────────────────────────────────

export interface AIQueryRequest {
  datasource_id: string
  question: string
  tables?: string[]
  include_base_tables?: boolean
  include_views?: boolean
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
  /** Normalized routing score from the API (0–1 scale in practice). */
  score?: number
  total_score?: number
  keyword_score?: number
  embedding_score?: number
  /** @deprecated API sends `score`; kept for older responses */
  relevance_score?: number
  selected?: boolean
  rejected_reason?: string
  description?: string
}

export interface TableRoutingDebug {
  relation_expansion?: string[]
  bridge_tables?: string[]
  eliminated_candidates?: string[]
}

export interface TableRoutingResult {
  selected_models?: string[]
  selected_tables?: string[]
  selected_dimensions?: string[]
  selected_metrics?: string[]
  join_paths?: string[]
  confidence: number
  ranking_method?: 'keyword' | 'vector' | 'hybrid' | 'manual' | 'semantic'
  context_source?: 'auto' | 'manual' | 'semantic_model' | string
  context_key?: string
  context_updated_at?: string
  candidates?: TableRoutingCandidate[]
  debug?: TableRoutingDebug
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
  datasource_id: string
  model_id: string
  select?: SelectField[]
  filters?: FilterClause[]
  group_by?: GroupByField[]
  order_by?: OrderByField[]
  having?: FilterClause[]
  limit?: number
  offset?: number
  ctes?: CTE[]
}

export interface SelectField {
  type: 'dimension' | 'metric' | 'window'
  name: string
  alias?: string
  window?: WindowSpec
}

export interface WindowSpec {
  aggregation: string
  expression?: string
  metric?: string
  partition_by?: string[]
  order_by?: OrderByField[]
  frame?: string
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

// ─── Model Success Rates (Dashboard) ───────────────────────────────

export interface ModelStats {
  model_id: string
  model_name?: string
  total_queries: number
  success_count: number
  failure_count: number
  success_rate: number
  avg_confidence: number
  avg_latency_ms: number
  positive_count: number
  negative_count: number
}

// ─── Eval Runs (History & Detail) ──────────────────────────────────

export interface EvalRunSummary {
  run_id: string
  provider: string
  model: string
  context_version: number
  total_cases: number
  passed: number
  failed: number
  pass_rate: number
  avg_confidence: number
  avg_latency_ms: number
  total_tokens: number
  completed_at: string
}

export interface EvalRunTestCase {
  case_id: string
  question: string
  match: boolean
  reason: string
  confidence: number
  latency_ms: number
}

export interface EvalRunDetail {
  summary: EvalRunSummary
  test_cases: EvalRunTestCase[]
}

// ─── Regression Report ─────────────────────────────────────────────

export interface RegressionChange {
  case_id: string
  question: string
  was_match: boolean
  is_match: boolean
  was_reason: string
  is_reason: string
}

export interface RegressionReport {
  baseline_run_id: string
  current_run_id: string
  new_failures: RegressionChange[]
  fixed_failures: RegressionChange[]
  changed_cases: RegressionChange[]
}
