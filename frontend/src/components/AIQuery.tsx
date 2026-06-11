import '../styles/aiQuery.css'

import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useLocation } from 'react-router-dom'

import { fetchJSON } from '../api/apiClient'
import { jobIsActive, useAIJobs } from '../hooks/useAIJobs'
import { useApi } from '../hooks/useApi'
import { useConversation } from '../hooks/useConversation'
import { useDatasources } from '../hooks/useDatasources'
import { useQueryParam } from '../hooks/useQueryParam'
import { useSemanticModels } from '../hooks/useSemanticModels'
import { useLocale, useT } from '../i18n'
import type {
  AIJob,
  AIJobListResponse,
  AIQueryRequest,
  AIQueryResponse,
  AIRuntimeSettings,
  EmbedMetadataResponse,
  PriorTurn,
} from '../types/ai'
import type { CompositeModelSummary } from '../types/composite'
import { pickValidIdOrFirst } from '../utils/effectiveSelection'
import { localeNumberTag } from '../utils/formatters'
import { normalizeAIQueryResponse } from '../utils/normalizeAIQueryResponse'
import { ChatPanel } from './aiQuery/ChatPanel'
import { RoutingPanel } from './aiQuery/RoutingPanel'
import { embeddingSummary } from './aiQuery/routingViz'
import { SidebarConversationItem } from './aiQuery/SidebarConversationItem'
import type { TableOption } from './aiQuery/types'
import { AI_METADATA_EMBED_TIMEOUT_MS, AI_QUERY_TIMEOUT_MS } from './aiQuery/types'
import { useAuth } from './auth/AuthProvider'

export default function AIQuery() {
  const t = useT()
  const [locale] = useLocale()
  const localeTag = localeNumberTag(locale)
  const { runJob, jobs, sessionId } = useAIJobs()
  const { accessToken } = useAuth()
  const { get, postData, loading, error, abort } = useApi()
  const { postData: postEmbedData, loading: embeddingLoading, error: embeddingError } = useApi()
  const {
    conversations,
    activeConversation,
    activeConversationId,
    setActiveConversationId,
    createConversation,
    addMessage,
    appendAssistantForJob,
    deleteConversation,
    renameConversation,
    updateMessageResponse,
  } = useConversation()

  const location = useLocation()
  const { datasources } = useDatasources()
  const [tables, setTables] = useState<TableOption[]>([])
  const [dsParam, setDsParam] = useQueryParam('ds')
  const [selectedDatasourceId, setSelectedDatasourceId] = useState(dsParam)
  const datasourceId = useMemo(
    () => pickValidIdOrFirst(selectedDatasourceId, datasources),
    [selectedDatasourceId, datasources],
  )
  const { models: semanticModels } = useSemanticModels(datasourceId)
  const [semanticModelId, setSemanticModelId] = useState<string>('')
  const [composites, setComposites] = useState<CompositeModelSummary[]>([])
  const [selectedTables, setSelectedTables] = useState<string[]>([])
  const [tableSearch, setTableSearch] = useState('')
  const [includeBaseTables, setIncludeBaseTables] = useState(true)
  const [includeViews, setIncludeViews] = useState(true)
  const [autoTableRouting, setAutoTableRouting] = useState(true)
  const [question, setQuestion] = useState(
    () => (location.state as { question?: string } | null)?.question ?? '',
  )
  const [aiRuntime, setAiRuntime] = useState<AIRuntimeSettings | null>(null)
  const [aiRuntimeErr, setAiRuntimeErr] = useState<string | null>(null)
  const [embeddingStatus, setEmbeddingStatus] = useState<string | null>(null)
  const [embeddingRunning, setEmbeddingRunning] = useState(false)
  const [includePastQueries, setIncludePastQueries] = useState(false)
  const [queryAction, setQueryAction] = useState<'preview' | 'execute' | null>(null)
  const [jobError, setJobError] = useState<string | null>(null)
  const [aiElapsedMs, setAiElapsedMs] = useState(0)

  // Derive the clarification round from the conversation itself so it survives
  // page refreshes and conversation switches.
  const clarificationRound = useMemo(() => {
    const lastAssistant = [...(activeConversation?.messages ?? [])]
      .reverse()
      .find((m) => m.role === 'assistant')
    const res = lastAssistant?.ai_response
    return res?.needs_clarification ? (res.clarification_round ?? 0) : 0
  }, [activeConversation])

  // A query job for the active conversation that is still queued/running —
  // present after a page refresh even though local submit state was lost.
  const activeConversationJob = useMemo(() => {
    if (!activeConversationId) {
      return null
    }
    return (
      jobs.find((j) => {
        if (j.kind !== 'run' && j.kind !== 'preview' && j.kind !== 'query') {
          return false
        }
        if (!jobIsActive(j)) {
          return false
        }
        const req = j.request_json
        return (
          !!req &&
          typeof req === 'object' &&
          (req as { conversation_id?: unknown }).conversation_id === activeConversationId
        )
      }) ?? null
    )
  }, [jobs, activeConversationId])

  const effectiveQueryAction =
    queryAction ??
    (activeConversationJob
      ? activeConversationJob.kind === 'preview'
        ? 'preview'
        : 'execute'
      : null)
  const aiBusy = effectiveQueryAction !== null
  const displayElapsedMs = aiBusy ? aiElapsedMs : 0

  const busyJobStartedAt = activeConversationJob?.created_at ?? null

  useEffect(() => {
    if (!aiBusy) {
      return
    }
    const startedAtMs = busyJobStartedAt ? new Date(busyJobStartedAt).getTime() : Date.now()
    const id = window.setInterval(() => {
      setAiElapsedMs(Math.max(0, Math.round(Date.now() - startedAtMs)))
    }, 200)
    return () => window.clearInterval(id)
  }, [aiBusy, busyJobStartedAt])

  const setDatasourceId = useCallback(
    (id: string) => {
      setSelectedDatasourceId(id)
      setDsParam(id)
      setSelectedTables([])
      setTableSearch('')
      setIncludeBaseTables(true)
      setIncludeViews(true)
      setTables([])
      setEmbeddingStatus(null)
      setSemanticModelId('')
      setComposites([])
    },
    [setDsParam],
  )

  useEffect(() => {
    let cancelled = false
    void get<AIRuntimeSettings>('/api/ai/settings').then((data) => {
      if (cancelled) {
        return
      }
      if (data) {
        setAiRuntime(data)
        setAiRuntimeErr(null)
      } else {
        setAiRuntime(null)
        setAiRuntimeErr(t('ai_query.err_settings_load'))
      }
    })
    return () => {
      cancelled = true
    }
  }, [get, t])

  useEffect(() => {
    if (!datasourceId) {
      return
    }
    let cancelled = false
    void get<TableOption[]>(`/api/datasources/${datasourceId}/tables`).then((data) => {
      if (!cancelled) {
        setTables(data ?? [])
      }
    })
    void get<CompositeModelSummary[]>(
      `/api/semantic/composites?datasource_id=${datasourceId}`,
    ).then((data) => {
      if (!cancelled) {
        setComposites((data ?? []).filter((c) => c.status === 'published'))
      }
    })
    return () => {
      cancelled = true
    }
  }, [datasourceId, get])

  const tableLabel = (table: TableOption) =>
    table.label ?? `${table.schema_name}.${table.table_name}`

  const tablesInTypeScope = useMemo(
    () =>
      tables.filter((table) => {
        const typ = (table.table_type ?? '').toUpperCase()
        if (typ === 'VIEW') {
          return includeViews
        }
        if (typ === 'BASE TABLE') {
          return includeBaseTables
        }
        return includeBaseTables
      }),
    [tables, includeBaseTables, includeViews],
  )

  const allowedLabels = useMemo(
    () => new Set(tablesInTypeScope.map((t) => tableLabel(t))),
    [tablesInTypeScope],
  )
  const effectiveSelectedTables = useMemo(
    () => selectedTables.filter((label) => allowedLabels.has(label)),
    [selectedTables, allowedLabels],
  )

  const recentPriorTurns = useMemo(() => {
    if (!activeConversation) {
      return undefined
    }
    const maxRecentTurns = 5
    const turns: PriorTurn[] = []
    const msgs = activeConversation.messages
    for (let i = 0; i < msgs.length; i++) {
      const m = msgs[i]
      if (m?.role !== 'user') {
        continue
      }
      const next = msgs[i + 1]
      const lq = next?.role === 'assistant' ? next.ai_response?.logical_query : undefined
      turns.push({
        question: m.content,
        logical_query: lq ?? undefined,
        note: next?.role === 'assistant' && next.ai_response?.sql ? 'executed' : undefined,
      })
    }
    return turns.slice(-maxRecentTurns)
  }, [activeConversation])

  const selectedDatasourceName = useMemo(
    () => datasources.find((ds) => ds.id === datasourceId)?.name,
    [datasources, datasourceId],
  )

  const semanticModelName = useMemo(
    () =>
      semanticModels.find((m) => m.id === semanticModelId)?.label ??
      semanticModels.find((m) => m.id === semanticModelId)?.name ??
      '',
    [semanticModels, semanticModelId],
  )

  const activeEmbeddingJob = useMemo(() => {
    if (!datasourceId) {
      return null
    }
    const targetModelId = semanticModelId.trim()
    return (
      jobs.find((j) => {
        if (j.kind !== 'embed_metadata') {
          return false
        }
        if (!jobIsActive(j)) {
          return false
        }
        if (!j.request_json || typeof j.request_json !== 'object') {
          return false
        }
        const req = j.request_json as Record<string, unknown>
        if (req.datasource_id !== datasourceId) {
          return false
        }
        const reqModel = typeof req.model_id === 'string' ? req.model_id.trim() : ''
        // Treat empty model_id and missing as the "all tables" embed refresh.
        return (reqModel || '') === (targetModelId || '')
      }) ?? null
    )
  }, [jobs, datasourceId, semanticModelId])

  const embeddingActive = Boolean(activeEmbeddingJob) || embeddingRunning || embeddingLoading

  const refreshMetadataEmbeddings = async () => {
    if (!datasourceId || embeddingActive) {
      return
    }
    setEmbeddingStatus(null)
    setEmbeddingRunning(true)

    const request = {
      datasource_id: datasourceId,
      model_id: semanticModelId.startsWith('composite:') ? undefined : semanticModelId || undefined,
    }

    try {
      const outcome = await runJob<typeof request, EmbedMetadataResponse>(
        'embed_metadata',
        request,
        {
          onComplete: (res) => {
            setEmbeddingRunning(false)
            setEmbeddingStatus(embeddingSummary(res, t))
          },
          onError: (err) => {
            setEmbeddingRunning(false)
            setEmbeddingStatus(err || 'Failed to refresh embeddings')
          },
        },
      )

      if (outcome === 'fallback') {
        const res = await postEmbedData<EmbedMetadataResponse>('/api/ai/metadata/embed', request, {
          timeout: AI_METADATA_EMBED_TIMEOUT_MS,
        })
        setEmbeddingRunning(false)
        if (res) {
          setEmbeddingStatus(embeddingSummary(res, t))
        }
      }
    } catch (err) {
      setEmbeddingRunning(false)
      setEmbeddingStatus(err instanceof Error ? err.message : 'Error refreshing embeddings')
    }
  }

  const requestBody = (
    q: string,
    conversationId: string,
    clarificationChoice?: string,
  ): AIQueryRequest => {
    const isComposite = semanticModelId.startsWith('composite:')
    return {
      datasource_id: datasourceId,
      model_id: isComposite ? undefined : semanticModelId || undefined,
      composite_id: isComposite ? semanticModelId.slice('composite:'.length) : undefined,
      question: q,
      clarification_choice: clarificationChoice,
      clarification_round: clarificationRound > 0 ? clarificationRound : undefined,
      tables: autoTableRouting ? undefined : effectiveSelectedTables,
      include_base_tables: includeBaseTables,
      include_views: includeViews,
      conversation_id: conversationId,
      prior_turns: includePastQueries ? recentPriorTurns : undefined,
    }
  }

  const assistantContentFor = useCallback(
    (flat: AIQueryResponse): string => {
      if (flat.needs_clarification) {
        return flat.clarification_question ?? t('ai_query.clarify_default')
      }
      return flat.sql
        ? t('ai_query.assistant_sql_preview', { snippet: flat.sql.slice(0, 120) })
        : t('ai_query.assistant_executed')
    },
    [t],
  )

  // Write a finished job's outcome into its conversation. Idempotent (keyed by
  // job id), so it is safe to run for live polls, resumed jobs after a page
  // refresh, and jobs that finished while this page was closed.
  const applyJobOutcome = useCallback(
    (job: AIJob) => {
      if (job.kind !== 'run' && job.kind !== 'preview' && job.kind !== 'query') {
        return
      }
      const req = job.request_json
      const convId =
        req && typeof req === 'object'
          ? (req as { conversation_id?: unknown }).conversation_id
          : undefined
      if (typeof convId !== 'string' || !convId) {
        return
      }
      if (job.status === 'succeeded') {
        const flat = normalizeAIQueryResponse(job.result_json)
        if (!flat) {
          return
        }
        appendAssistantForJob(convId, job.id, {
          content: assistantContentFor(flat),
          ai_response: flat,
        })
      } else if (job.status === 'failed') {
        appendAssistantForJob(convId, job.id, {
          content: t('ai_query.job_failed_message', { error: job.error_message ?? '' }),
        })
      } else if (job.status === 'cancelled') {
        appendAssistantForJob(convId, job.id, {
          content: t('ai_query.job_cancelled_message'),
        })
      }
    },
    [appendAssistantForJob, assistantContentFor, t],
  )

  // Live sweep: attach results as tracked jobs reach a terminal state.
  useEffect(() => {
    for (const job of jobs) {
      if (job.status === 'succeeded' || job.status === 'failed' || job.status === 'cancelled') {
        applyJobOutcome(job)
      }
    }
  }, [jobs, applyJobOutcome])

  // One-time sweep for jobs that finished while the page was closed: fetch the
  // session's recent jobs and attach any outcome the conversation is missing.
  const sweptFinishedJobsRef = useRef(false)
  useEffect(() => {
    if (!accessToken || sweptFinishedJobsRef.current) {
      return
    }
    sweptFinishedJobsRef.current = true
    void fetchJSON<AIJobListResponse>(
      `/api/ai/jobs?client_session_id=${encodeURIComponent(sessionId)}`,
    ).then(({ data, error: listError }) => {
      if (listError || !data?.jobs) {
        return
      }
      // The API returns newest-first; apply oldest-first so multiple missed
      // answers land in chronological order.
      for (const job of [...data.jobs].reverse()) {
        if (job.status === 'succeeded' || job.status === 'failed' || job.status === 'cancelled') {
          applyJobOutcome(job)
        }
      }
    })
  }, [accessToken, applyJobOutcome, sessionId])

  const sendQuery = async (q: string, execute: boolean, clarificationChoice?: string) => {
    if (!q.trim()) {
      return
    }
    const convId = activeConversation?.id ?? createConversation().id
    const body = requestBody(q, convId, clarificationChoice)
    setQueryAction(execute ? 'execute' : 'preview')
    setJobError(null)
    try {
      const kind = execute ? 'run' : 'preview'
      const outcome = await runJob(kind, body, {
        onEnqueued: (job) => {
          // Persist the question immediately, linked to the job, so both
          // survive a page refresh; the job sweep attaches the answer later.
          addMessage({ role: 'user', content: q, job_id: job.id }, convId)
          setQuestion('')
        },
        onError: (message) => setJobError(message),
      })
      if (outcome === 'fallback') {
        addMessage({ role: 'user', content: q }, convId)
        const endpoint = execute ? '/api/ai/query/run' : '/api/ai/query/preview'
        const res = await postData<AIQueryResponse>(endpoint, body, {
          timeout: AI_QUERY_TIMEOUT_MS,
        })
        const flat = normalizeAIQueryResponse(res)
        if (!flat) {
          return
        }
        addMessage(
          { role: 'assistant', content: assistantContentFor(flat), ai_response: flat },
          convId,
        )
        setQuestion('')
      }
      // Job outcomes (success, failure, cancellation) reach the conversation
      // through the job sweep above.
    } finally {
      setQueryAction(null)
    }
  }

  return (
    <div className="ai-query-layout">
      <aside className="conversation-sidebar">
        <div
          className="sidebar-header"
          style={{
            display: 'flex',
            flexDirection: 'column',
            alignItems: 'stretch',
            gap: '0.75rem',
          }}
        >
          <h3 style={{ textAlign: 'center', width: '100%', margin: 0 }}>
            {t('ai_query.conv_title')}
          </h3>
          <button
            className="btn btn-primary btn-sm"
            style={{
              width: '100%',
              display: 'flex',
              justifyContent: 'center',
              alignItems: 'center',
            }}
            onClick={() => {
              createConversation()
              setQuestion('')
            }}
          >
            {t('ai_query.conv_new')}
          </button>
        </div>
        <div className="conversations-list">
          {conversations.map((c) => (
            <SidebarConversationItem
              key={c.id}
              conv={c}
              isActive={c.id === activeConversationId}
              onSelect={() => setActiveConversationId(c.id)}
              onRename={renameConversation}
              onDelete={deleteConversation}
              t={t}
            />
          ))}
        </div>
      </aside>

      <div className="ai-query-main">
        <RoutingPanel
          t={t}
          aiRuntime={aiRuntime}
          aiRuntimeErr={aiRuntimeErr}
          datasources={datasources}
          datasourceId={datasourceId}
          setDatasourceId={setDatasourceId}
          semanticModels={semanticModels}
          semanticModelId={semanticModelId}
          setSemanticModelId={setSemanticModelId}
          composites={composites}
          tables={tables}
          selectedTables={effectiveSelectedTables}
          setSelectedTables={setSelectedTables}
          tableSearch={tableSearch}
          setTableSearch={setTableSearch}
          includeBaseTables={includeBaseTables}
          setIncludeBaseTables={setIncludeBaseTables}
          includeViews={includeViews}
          setIncludeViews={setIncludeViews}
          autoTableRouting={autoTableRouting}
          setAutoTableRouting={setAutoTableRouting}
          embeddingStatus={embeddingStatus}
          embeddingError={embeddingError}
          embeddingLoading={embeddingLoading}
          embeddingRunning={embeddingActive}
          selectedDatasourceName={selectedDatasourceName}
          semanticModelName={semanticModelName}
          onRefreshEmbeddings={() => {
            void refreshMetadataEmbeddings()
          }}
        />

        <ChatPanel
          t={t}
          localeNumberTag={localeNumberTag}
          localeTag={localeTag}
          activeConversation={activeConversation}
          activeConversationId={activeConversationId}
          datasourceId={datasourceId}
          aiRuntime={aiRuntime}
          question={question}
          setQuestion={setQuestion}
          loading={loading}
          error={error}
          jobError={jobError}
          queryAction={effectiveQueryAction}
          aiElapsedMs={displayElapsedMs}
          includePastQueries={includePastQueries}
          setIncludePastQueries={setIncludePastQueries}
          onSendQuery={(q, execute, clarificationChoice) => {
            void sendQuery(q, execute, clarificationChoice)
          }}
          onAbort={abort}
          get={get}
          postData={postData}
          updateMessageResponse={updateMessageResponse}
        />
      </div>
    </div>
  )
}
