import type { TranslationKey, Locale } from '../../i18n'
import type { AIRuntimeSettings, AIQueryResponse, ConversationMessage, Conversation } from '../../types/ai'
import type { Datasource, Table } from '../../types/metadata'
import type { RequestOptions } from '../../api/apiClient'

export type TableOption = Omit<Table, 'columns'>

export interface SampleColumn {
  name: string
}

export interface SampleData {
  columns: SampleColumn[]
  rows: unknown[][]
}

export const FEEDBACK_CAT_KEYS = [
  'ai_query.feedback_cat_wrong_table',
  'ai_query.feedback_cat_wrong_columns',
  'ai_query.feedback_cat_wrong_agg',
  'ai_query.feedback_cat_missing_date',
  'ai_query.feedback_cat_wrong_logic',
  'ai_query.feedback_cat_sql_error',
  'ai_query.feedback_cat_other',
] as const satisfies readonly TranslationKey[]

export type FeedbackCatKey = (typeof FEEDBACK_CAT_KEYS)[number]

export const AI_QUERY_TIMEOUT_MS = 300_000
export const AI_METADATA_EMBED_TIMEOUT_MS = 600_000

export interface AssistantMessageCardProps {
  message: ConversationMessage
  messageIndex: number
  conversationId: string
  datasourceId: string
  aiRuntime: AIRuntimeSettings | null
  userQuestion: string
  get: <T>(url: string) => Promise<T | null>
  postData: <T>(url: string, body: unknown, options?: RequestOptions) => Promise<T | null>
  updateMessageResponse: (conversationId: string, messageIndex: number, aiResponse: AIQueryResponse) => void
  t: (key: TranslationKey, params?: Record<string, string | number>) => string
  localeNumberTag: (locale: Locale) => string
  localeTag: string
  onSelectClarification: (option: string) => void
  onSkipClarification: () => void
  onFilterByValue: (column: string, value: string) => void
  onCellDrillDown: (column: string, value: string) => void
}

export interface RoutingPanelProps {
  t: (key: TranslationKey, params?: Record<string, string | number>) => string
  aiRuntime: AIRuntimeSettings | null
  aiRuntimeErr: string | null
  datasources: Datasource[]
  datasourceId: string
  setDatasourceId: (id: string) => void
  semanticModels: { id: string; name: string; label?: string | null; status: string }[]
  semanticModelId: string
  setSemanticModelId: (id: string) => void
  tables: TableOption[]
  selectedTables: string[]
  setSelectedTables: (tables: string[]) => void
  tableSearch: string
  setTableSearch: (s: string) => void
  includeBaseTables: boolean
  setIncludeBaseTables: (v: boolean) => void
  includeViews: boolean
  setIncludeViews: (v: boolean) => void
  autoTableRouting: boolean
  setAutoTableRouting: (v: boolean) => void
  embeddingStatus: string | null
  embeddingError: string | null
  embeddingLoading: boolean
  embeddingRunning: boolean
  selectedDatasourceName: string | undefined
  semanticModelName: string
  onRefreshEmbeddings: () => void
}

export interface ChatPanelProps {
  t: (key: TranslationKey, params?: Record<string, string | number>) => string
  localeNumberTag: (locale: Locale) => string
  localeTag: string
  activeConversation: Conversation | null | undefined
  activeConversationId: string | null | undefined
  datasourceId: string
  aiRuntime: AIRuntimeSettings | null
  question: string
  setQuestion: (q: string | ((prev: string) => string)) => void
  loading: boolean
  error: string | null
  jobError: string | null
  queryAction: 'preview' | 'execute' | null
  aiElapsedMs: number
  includePastQueries: boolean
  setIncludePastQueries: (v: boolean) => void
  onSendQuery: (q: string, execute: boolean) => void
  onAbort: () => void
  get: <T>(url: string) => Promise<T | null>
  postData: <T>(url: string, body: unknown, options?: RequestOptions) => Promise<T | null>
  updateMessageResponse: (conversationId: string, messageIndex: number, aiResponse: AIQueryResponse) => void
}
