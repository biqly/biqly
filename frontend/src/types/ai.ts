// ─── AI runtime (server env) ───────────────────────────────────────

export interface AIRuntimeSettings {
  provider: string
  llm_model: string
  base_url: string
  base_url_effective: string
  api_key_configured: boolean
  /** True when BI_AI_QUERY_* split is active. When false the query_* fields
   * mirror the base llm_model/base_url and the UI hides the duplicate row. */
  query_model_override?: boolean
  query_provider?: string
  query_model?: string
  query_base_url?: string
  query_base_url_effective?: string
  query_api_key_configured?: boolean
  query_api_key_dedicated?: boolean
  query_http_timeout_seconds?: number
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
  model_id?: string
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
  /** Schemas included after multi-schema partitioning (audit §2.4). */
  schema_partitions?: string[]
}

export interface PromptStats {
  prompt_runes?: number
  est_prompt_tokens?: number
  est_completion_reserve?: number
  context_window_tokens?: number
  max_prompt_runes?: number
  context_tier?: number
  context_tier_label?: string
  model?: string
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
  clarification?: Clarification
  // Multi-candidate
  candidates?: LogicalQueryCandidate[]
  candidates_count?: number
  // Retry / validation
  retry_count?: number
  validation_result?: ValidationExplainResult
  // Prompt transparency
  prompt?: string
  prompt_stats?: PromptStats
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
  version?: string
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
  from_subquery?: SubqueryBody
  from_cte?: string
  from_alias?: string
  default_schema?: string
  table_schemas?: Record<string, string>
}

export interface SubqueryBody {
  select?: SelectField[]
  filters?: FilterClause[]
  group_by?: GroupByField[]
  having?: FilterClause[]
  order_by?: OrderByField[]
  limit?: number
  offset?: number
}

export interface SelectField {
  type: 'dimension' | 'metric' | 'window' | 'case'
  name: string
  alias?: string
  window?: WindowSpec
  case?: CaseExpr
}

export interface CaseExpr {
  branches: CaseBranch[]
  else?: CaseThen
}

export interface CaseBranch {
  when: FilterClause[]
  then: CaseThen
}

export interface CaseThen {
  type: 'dimension' | 'literal'
  dimension?: string
  literal?: unknown
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
  subquery?: SubqueryFilter
}

export interface SubqueryFilter {
  body: SubqueryBody
  result_field: string
}

export interface GroupByField {
  field: string
  time_grain?: 'day' | 'week' | 'month' | 'quarter' | 'year'
}

export interface OrderByField {
  field: string
  direction: 'asc' | 'desc'
}

export interface CTE {
  name: string
  select?: SelectField[]
  filters?: FilterClause[]
  group_by?: GroupByField[]
  having?: FilterClause[]
  order_by?: OrderByField[]
  limit?: number
  offset?: number
}

// ─── Query Result Types ────────────────────────────────────────────

export interface PivotHint {
  row_field: string
  column_field: string
  value_fields: string[]
  reason?: string
}

export interface ResultAnomaly {
  row_index: number
  column: string
  value: unknown
  score: number
}

export interface QueryResultPayload {
  columns: QueryColumn[]
  rows: unknown[][]
  stats?: {
    row_count: number
    duration_ms: number
  }
  chart_suggestions?: ChartSuggestion[]
  pivot_hint?: PivotHint
  anomalies?: ResultAnomaly[]
}

export type ChartSuggestion = 'bar' | 'line' | 'table' | 'number' | 'pie'

export type ClarificationStatus = 'needs_clarification'

export interface ClarificationOption {
  key: string
  label: string
  hint?: string
}

export interface ClarificationContext {
  type: 'semantic_model' | 'table'
  name: string
  score?: number
  reason?: string
}

export interface Clarification {
  status: ClarificationStatus
  question: string
  reason?: string
  options?: ClarificationOption[]
  candidates?: ClarificationContext[]
  source?: 'router' | 'validator' | 'ai'
}

export type QueryColumnSemanticType = 'dimension' | 'metric'

export type QueryColumnFormat =
  | 'number'
  | 'currency'
  | 'percent'
  | 'date'
  | 'datetime'
  | 'text'

export interface QueryColumn {
  name: string
  type?: string
  semantic_type?: QueryColumnSemanticType
  format?: QueryColumnFormat
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
