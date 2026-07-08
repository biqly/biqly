import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useLocation } from 'react-router-dom'

import { fetchOwnAIJobs, jobIsActive, useAIJobs } from '../hooks/useAIJobs'
import { useApi } from '../hooks/useApi'
import { useConversation } from '../hooks/useConversation'
import { useDatasources } from '../hooks/useDatasources'
import { useQueryParam } from '../hooks/useQueryParam'
import { useSemanticModels } from '../hooks/useSemanticModels'
import { useLocale, useT } from '../i18n'
import { buttonClass } from '../lib/buttonClasses'
import { cn } from '../lib/cn'
import type { AgentChatRequest } from '../types/agent'
import type {
  AIJob,
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
import { buildResultSummary } from '../utils/priorTurnSummary'
import { queuePositionLine } from './ai/jobProgressUtils'
import { loadAgentModeEnabled, saveAgentModeEnabled } from './aiQuery/agentModeStorage'
import { runAgentModeTurn } from './aiQuery/agentModeTurn'
import {
  aiQueryLayoutClass,
  aiQueryMainClass,
  conversationSidebarClass,
  conversationsListClass,
  sidebarHeaderTitleClass,
} from './aiQuery/aiQueryClasses'
import { ChatPanel } from './aiQuery/ChatPanel'
import { RoutingPanel } from './aiQuery/RoutingPanel'
import { embeddingSummary } from './aiQuery/routingViz'
import { SidebarConversationItem } from './aiQuery/SidebarConversationItem'
import type { TableOption } from './aiQuery/types'
import { AI_METADATA_EMBED_TIMEOUT_MS, AI_QUERY_TIMEOUT_MS } from './aiQuery/types'
import { useAuth } from './auth/AuthProvider'

const AUTO_FIND_STORAGE_KEY = 'biqly.aiQuery.autoFindSkills'

// Auto-find defaults ON; only an explicit "false" turns it off. Guarded so a
// storage-less environment (SSR/tests) simply falls back to the default.
function loadAutoFindEnabled(): boolean {
  try {
    return window.localStorage.getItem(AUTO_FIND_STORAGE_KEY) !== 'false'
  } catch {
    return true
  }
}

function saveAutoFindEnabled(enabled: boolean): void {
  try {
    window.localStorage.setItem(AUTO_FIND_STORAGE_KEY, enabled ? 'true' : 'false')
  } catch {
    // Non-fatal: the toggle still works for the session without persistence.
  }
}

function buildAIQueryRequest(args: {
  datasourceId: string
  semanticModelId: string
  clarificationRound: number
  contextEnabled: boolean
  autoFindEnabled: boolean
  savedQueryIds: string[]
  recentPriorTurns: PriorTurn[] | undefined
  question: string
  conversationId: string
  clarificationChoice?: string
}): AIQueryRequest {
  const isComposite = args.semanticModelId.startsWith('composite:')
  return {
    datasource_id: args.datasourceId,
    model_id: isComposite ? undefined : args.semanticModelId || undefined,
    composite_id: isComposite ? args.semanticModelId.slice('composite:'.length) : undefined,
    question: args.question,
    clarification_choice: args.clarificationChoice,
    clarification_round: args.clarificationRound > 0 ? args.clarificationRound : undefined,
    // Table routing is always automatic now; @-mentions pin specific fields.
    tables: undefined,
    include_base_tables: true,
    include_views: true,
    conversation_id: args.conversationId,
    prior_turns: args.contextEnabled ? args.recentPriorTurns : undefined,
    // Only send auto_find_skills when off — omitting it preserves the
    // default-on behavior server-side.
    auto_find_skills: args.autoFindEnabled ? undefined : false,
    saved_query_ids: args.savedQueryIds.length > 0 ? args.savedQueryIds : undefined,
  }
}

export default function AIQuery() {
  const t = useT()
  const [locale] = useLocale()
  const localeTag = localeNumberTag(locale)
  const { runJob, cancelJob, jobs, queueStatus, sessionId } = useAIJobs()
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
    updateConversationContext,
    updateMessageResponse,
  } = useConversation(accessToken)

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
  const [question, setQuestion] = useState(
    () => (location.state as { question?: string } | null)?.question ?? '',
  )
  const [aiRuntime, setAiRuntime] = useState<AIRuntimeSettings | null>(null)
  const [aiRuntimeErr, setAiRuntimeErr] = useState<string | null>(null)
  const [embeddingStatus, setEmbeddingStatus] = useState<string | null>(null)
  const [embeddingRunning, setEmbeddingRunning] = useState(false)
  const [queryAction, setQueryAction] = useState<'preview' | 'execute' | null>(null)
  // Auto-find (embedding-RAG grounding) defaults on and persists across sessions
  // as a composer preference. Selected saved-query grounding is per-question and
  // clears after each send.
  const [autoFindEnabled, setAutoFindEnabled] = useState(loadAutoFindEnabled)
  // Agent Mode routes sendQuery through the T9 SSE agent stream
  // (POST /api/agent/chat) instead of the job/polling path. See T10 brief.
  const [agentModeEnabled, setAgentModeEnabled] = useState(loadAgentModeEnabled)
  const [selectedSavedQueryIds, setSelectedSavedQueryIds] = useState<string[]>([])
  const [jobError, setJobError] = useState<string | null>(null)
  const [aiElapsedMs, setAiElapsedMs] = useState(0)
  const [isConversationsExpanded, setIsConversationsExpanded] = useState(false)

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
      setTables([])
      setEmbeddingStatus(null)
      setSemanticModelId('')
      setComposites([])
    },
    [setDsParam],
  )

  useEffect(() => {
    const controller = new AbortController()
    void get<AIRuntimeSettings>('/api/ai/settings').then((data) => {
      if (controller.signal.aborted) {
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
      controller.abort()
    }
  }, [get, t])

  useEffect(() => {
    if (!datasourceId) {
      return
    }
    const controller = new AbortController()
    void get<TableOption[]>(`/api/datasources/${datasourceId}/tables`).then((data) => {
      if (controller.signal.aborted) {
        return
      }
      setTables(data ?? [])
    })
    void get<CompositeModelSummary[]>(
      `/api/semantic/composites?datasource_id=${datasourceId}`,
    ).then((data) => {
      if (controller.signal.aborted) {
        return
      }
      setComposites((data ?? []).filter((c) => c.status === 'published'))
    })
    return () => {
      controller.abort()
    }
  }, [datasourceId, get])

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
      const resultSummary =
        next?.role === 'assistant' ? buildResultSummary(next.ai_response ?? null) : undefined
      turns.push({
        question: m.content,
        logical_query: lq ?? undefined,
        note: next?.role === 'assistant' && next.ai_response?.sql ? 'executed' : undefined,
        result_summary: resultSummary,
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
  ): AIQueryRequest =>
    buildAIQueryRequest({
      datasourceId,
      semanticModelId,
      clarificationRound,
      contextEnabled: activeConversation?.context_enabled !== false,
      autoFindEnabled,
      savedQueryIds: selectedSavedQueryIds,
      recentPriorTurns,
      question: q,
      conversationId,
      clarificationChoice,
    })

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
        const question =
          job.request_json && typeof job.request_json === 'object'
            ? (job.request_json as Record<string, unknown>).question
            : undefined
        appendAssistantForJob(convId, job.id, {
          content: t('ai_query.job_cancelled_message', {
            question: typeof question === 'string' ? question : '',
          }),
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
  // caller's recent jobs and attach any outcome the conversation is missing.
  // The token is passed explicitly (child effects can run before AuthProvider's
  // own effects); on failure the ref resets so the next token change retries.
  const sweptFinishedJobsRef = useRef(false)
  useEffect(() => {
    if (!accessToken || sweptFinishedJobsRef.current) {
      return
    }
    sweptFinishedJobsRef.current = true
    void fetchOwnAIJobs({ token: accessToken, sessionId, activeOnly: false }).then((recent) => {
      if (!recent) {
        sweptFinishedJobsRef.current = false
        return
      }
      // The API returns newest-first; apply oldest-first so multiple missed
      // answers land in chronological order.
      for (const job of [...recent].reverse()) {
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
    const conversationScope = {
      datasource_id: datasourceId,
      model_id: semanticModelId.startsWith('composite:') ? null : semanticModelId || null,
    }
    const convId = activeConversation?.id ?? createConversation(conversationScope).id
    // The selected grounding is now captured in the request; clear the chips so
    // the next question starts fresh.
    setSelectedSavedQueryIds([])
    setQueryAction(execute ? 'execute' : 'preview')
    setJobError(null)

    if (agentModeEnabled) {
      addMessage({ role: 'user', content: q }, convId, conversationScope)
      setQuestion('')
      try {
        const agentRequest: AgentChatRequest = {
          message: q,
          conversation_id: convId,
          datasource_id: datasourceId,
          // Mirrors buildAIQueryRequest's context gating: the per-conversation
          // "link context" toggle controls whether prior turns are sent.
          prior_turns: activeConversation?.context_enabled !== false ? recentPriorTurns : undefined,
        }
        const outcome = await runAgentModeTurn(agentRequest, {
          token: accessToken ?? undefined,
          clarificationFallback: t('ai_query.agent_clarification_fallback'),
          genericErrorMessage: t('ai_query.err_agent_stream'),
        })
        if (outcome.kind === 'result') {
          addMessage(
            {
              role: 'assistant',
              content: assistantContentFor(outcome.response),
              ai_response: outcome.response,
            },
            convId,
          )
        } else if (outcome.kind === 'clarification') {
          // T11 owns the real clarification card; T10 only needs to avoid a
          // silent stall when one arrives mid-development.
          addMessage({ role: 'assistant', content: outcome.message }, convId)
        } else if (outcome.kind === 'error') {
          setJobError(outcome.message)
        }
      } finally {
        setQueryAction(null)
      }
      return
    }

    const body = requestBody(q, convId, clarificationChoice)
    try {
      const kind = execute ? 'run' : 'preview'
      const outcome = await runJob(kind, body, {
        onEnqueued: (job) => {
          // Persist the question immediately, linked to the job, so both
          // survive a page refresh; the job sweep attaches the answer later.
          addMessage({ role: 'user', content: q, job_id: job.id }, convId, conversationScope)
          setQuestion('')
        },
        onError: (message) => setJobError(message),
      })
      if (outcome === 'fallback') {
        addMessage({ role: 'user', content: q }, convId, conversationScope)
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
    <div className={aiQueryLayoutClass}>
      <aside
        className={cn(
          conversationSidebarClass,
          conversations.length === 0 && 'max-[900px]:hidden',
          'max-[900px]:p-4',
        )}
      >
        <div
          className={cn(
            'border-border mb-4 flex flex-col gap-3 border-b pb-3',
            'max-[900px]:mb-0 max-[900px]:flex-row max-[900px]:items-center max-[900px]:justify-between max-[900px]:gap-2 max-[900px]:border-b-0 max-[900px]:pb-0',
            isConversationsExpanded && 'max-[900px]:mb-4 max-[900px]:border-b max-[900px]:pb-3',
          )}
        >
          <div className="flex min-w-0 items-center gap-2">
            <h3 className={sidebarHeaderTitleClass}>{t('ai_query.conv_title')}</h3>
            {conversations.length > 0 && (
              <span className="bg-canvas-subtle border-border text-foreground-muted shrink-0 rounded-full border px-2 py-0.5 text-[0.7rem] font-semibold">
                {conversations.length}
              </span>
            )}
          </div>
          <div className="flex w-full shrink-0 flex-col items-center gap-2 max-[900px]:w-auto max-[900px]:flex-row">
            <button
              className={cn(
                buttonClass('primary', { size: 'sm' }),
                'w-full justify-center max-[900px]:w-auto',
              )}
              style={{
                display: 'flex',
                alignItems: 'center',
              }}
              onClick={() => {
                createConversation({
                  datasource_id: datasourceId,
                  model_id: semanticModelId.startsWith('composite:')
                    ? null
                    : semanticModelId || null,
                })
                setQuestion('')
                setIsConversationsExpanded(true)
              }}
            >
              {t('ai_query.conv_new')}
            </button>
            <button
              type="button"
              className="border-border bg-card-raised text-foreground hidden h-8 w-8 cursor-pointer items-center justify-center rounded-lg border transition-colors hover:bg-(--control-hover-bg) max-[900px]:inline-flex"
              onClick={() => setIsConversationsExpanded(!isConversationsExpanded)}
              aria-label={
                isConversationsExpanded ? t('common.collapse_panel') : t('common.expand_panel')
              }
            >
              <svg
                className={cn(
                  'h-4 w-4 transition-transform duration-200',
                  isConversationsExpanded && 'rotate-180',
                )}
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                strokeWidth={2}
              >
                <path strokeLinecap="round" strokeLinejoin="round" d="M19 9l-7 7-7-7" />
              </svg>
            </button>
          </div>
        </div>
        <div
          className={cn(conversationsListClass, !isConversationsExpanded && 'max-[900px]:hidden')}
        >
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

      <div className={aiQueryMainClass}>
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
          semanticModelId={semanticModelId}
          tables={tables}
          aiRuntime={aiRuntime}
          question={question}
          setQuestion={setQuestion}
          loading={loading}
          error={error}
          jobError={jobError}
          queryAction={effectiveQueryAction}
          aiElapsedMs={displayElapsedMs}
          activeJob={activeConversationJob}
          queueNotice={
            activeConversationJob ? queuePositionLine(activeConversationJob, queueStatus, t) : null
          }
          contextEnabled={activeConversation?.context_enabled !== false}
          onContextEnabledChange={updateConversationContext}
          autoFindEnabled={autoFindEnabled}
          onAutoFindEnabledChange={(enabled) => {
            setAutoFindEnabled(enabled)
            saveAutoFindEnabled(enabled)
          }}
          agentModeEnabled={agentModeEnabled}
          onAgentModeEnabledChange={(enabled) => {
            setAgentModeEnabled(enabled)
            saveAgentModeEnabled(enabled)
          }}
          selectedSavedQueryIds={selectedSavedQueryIds}
          onSelectedSavedQueryIdsChange={setSelectedSavedQueryIds}
          onSendQuery={(q, execute, clarificationChoice) => {
            void sendQuery(q, execute, clarificationChoice)
          }}
          onAbort={() => {
            abort()
            if (activeConversationJob) {
              void cancelJob(activeConversationJob.id)
            }
          }}
          get={get}
          postData={postData}
          updateMessageResponse={updateMessageResponse}
        />
      </div>
    </div>
  )
}
