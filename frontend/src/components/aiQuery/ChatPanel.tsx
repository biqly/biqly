import { useEffect, useMemo, useRef, useState } from 'react'

import { listSavedQueries, type SavedQueryOption } from '../../api/aiSkills'
import { jobIsActive, type TrackedAIJob } from '../../hooks/useAIJobs'
import { useJobNow } from '../../hooks/useJobNow'
import { useSemanticCatalog } from '../../hooks/useSemanticCatalog'
import type { TranslationKey } from '../../i18n'
import { buttonClass } from '../../lib/buttonClasses'
import { cn } from '../../lib/cn'
import { legacyFeedbackClass } from '../../lib/feedbackClasses'
import type { PendingAgentClarification } from '../../types/agent'
import type { AIRuntimeSettings, Conversation, ConversationMessage, RunStep } from '../../types/ai'
import { CLARIFICATION_SKIP_CHOICE } from '../../types/ai'
import type { SemanticModelDetail } from '../../types/semantic'
import { formatTimeOnly } from '../../utils/formatters'
import { JobPhaseSteps } from '../ai/JobPhaseSteps'
import { ErrorAlert } from '../ui/ErrorAlert'
import { AgentConfigurationPopover } from './AgentConfigurationPopover'
import { AgentTraceCard } from './AgentTraceCard'
import {
  chatBubbleClass,
  chatComposerActionBtnClass,
  chatComposerActionsClass,
  chatComposerBarClass,
  chatComposerClass,
  chatComposerHintClass,
  chatComposerOptionsClass,
  chatComposerSendClass,
  chatComposerSendIconClass,
  chatEmptyStateChipClass,
  chatEmptyStateClass,
  chatEmptyStateDescClass,
  chatEmptyStateSuggestionsClass,
  chatEmptyStateTitleClass,
  chatFeedClass,
  chatInputAreaClass,
  chatMsgAssistantClass,
  chatMsgAuthorClass,
  chatMsgAvatarClass,
  chatMsgClass,
  chatMsgMainClass,
  chatMsgMetaClass,
  chatMsgUserClass,
  chatTypingClass,
  chatTypingDot1Class,
  chatTypingDot2Class,
  chatTypingDot3Class,
  chatTypingDotsClass,
  chatTypingElapsedClass,
  chatTypingHintLabelClass,
  chatTypingLabelClass,
  userBubbleClass,
  userBubbleContentClass,
  userBubbleTimeClass,
} from './aiQueryClasses'
import { AssistantMessageCard } from './AssistantMessageCard'
import { PromptTextarea } from './PromptTextarea'
import { formatAiWaitElapsed } from './routingViz'
import { buildSuggestedQuestions, type SuggestedQuestion } from './suggestedQuestions'
import type { ChatPanelProps } from './types'
import { AI_QUERY_TIMEOUT_MS } from './types'

function scrollChatFeed({
  feed,
  messages,
  currentCount,
  prevConversationId,
  currentConversationId,
  prevMessageCount,
}: {
  feed: HTMLDivElement
  messages: {
    role?: string
    ai_response?: { needs_clarification?: boolean | null } | null
  }[]
  currentCount: number
  prevConversationId: string | undefined
  currentConversationId: string | undefined
  prevMessageCount: number
}) {
  const isSameConv = prevConversationId === currentConversationId
  const behavior: ScrollBehavior = isSameConv && currentCount > prevMessageCount ? 'smooth' : 'auto'
  // Welcome/empty state: scrolling to the bottom would clip the intro text at
  // the top of the feed; keep it anchored to the top instead.
  if (currentCount === 0) {
    feed.scrollTo({ top: 0, behavior })
    return
  }
  const lastMessage = messages[currentCount - 1]
  const lastNeedsClarification =
    lastMessage?.role === 'assistant' && Boolean(lastMessage.ai_response?.needs_clarification)
  const target = lastNeedsClarification
    ? feed.querySelector<HTMLElement>(`[data-message-index="${currentCount - 1}"]`)
    : null
  if (target) {
    const feedRect = feed.getBoundingClientRect()
    const targetRect = target.getBoundingClientRect()
    feed.scrollTo({
      top: feed.scrollTop + (targetRect.top - feedRect.top) - 8,
      behavior,
    })
  } else {
    feed.scrollTo({ top: feed.scrollHeight, behavior })
  }
}

interface TypingIndicatorProps {
  queryAction: string | null
  aiElapsedMs: number
  activeJob: TrackedAIJob | null
  queueNotice: string | null
  onAbort: () => void
  t: (key: TranslationKey, params?: Record<string, string | number>) => string
}

function TypingIndicator({
  queryAction,
  aiElapsedMs,
  activeJob,
  queueNotice,
  onAbort,
  t,
}: TypingIndicatorProps) {
  const jobRunning = activeJob != null && jobIsActive(activeJob)
  const now = useJobNow(queryAction !== null && jobRunning)
  if (queryAction === null) {
    return null
  }
  return (
    <div className={cn(chatMsgClass, chatMsgAssistantClass)} role="status">
      <span className={chatMsgAvatarClass} aria-hidden="true">
        ✦
      </span>
      <div className={chatMsgMainClass}>
        <div className={chatTypingClass}>
          <span className={chatTypingDotsClass} aria-hidden="true">
            <i className={chatTypingDot1Class} />
            <i className={chatTypingDot2Class} />
            <i className={chatTypingDot3Class} />
          </span>
          <span className={chatTypingLabelClass}>{t('ai_query.agent_working')}</span>
          <span className={chatTypingElapsedClass}>{formatAiWaitElapsed(aiElapsedMs, t)}</span>
          <button
            type="button"
            className="border-border text-foreground-muted hover:text-error hover:border-error/50 ml-1 cursor-pointer rounded-md border bg-transparent px-2 py-0.5 text-[0.72rem] font-semibold transition-colors"
            onClick={onAbort}
          >
            {t('ai_query.stop_run')}
          </button>
        </div>
        {jobRunning && (
          <div className="border-border bg-canvas-subtle mt-2 max-w-xs rounded-lg border px-3 py-2.5">
            <JobPhaseSteps job={activeJob} now={now} />
            {queueNotice && (
              <p className="text-foreground-muted mt-1.5 mb-0 text-[0.72rem]">{queueNotice}</p>
            )}
          </div>
        )}
        <p className={chatTypingHintLabelClass}>
          {t('ai_query.wait_hint', { minutes: Math.round(AI_QUERY_TIMEOUT_MS / 60_000) })}
        </p>
      </div>
    </div>
  )
}

interface ChatEmptyStateProps {
  t: (key: TranslationKey, params?: Record<string, string | number>) => string
  setQuestion: (q: string | ((prev: string) => string)) => void
  suggested: SuggestedQuestion[]
}

const suggestionCategoryKeys: Record<string, TranslationKey> = {
  aggregation: 'ai_query.suggest_cat_aggregation',
  segmentation: 'ai_query.suggest_cat_segmentation',
  trend: 'ai_query.suggest_cat_trend',
  comparison: 'ai_query.suggest_cat_comparison',
}

const placeholderKeys = [
  'ai_query.placeholder',
  'ai_query.placeholder_compare',
  'ai_query.placeholder_explain',
  'ai_query.placeholder_forecast',
] as const satisfies readonly TranslationKey[]

function useRotatingPlaceholder(t: ChatPanelProps['t'], paused: boolean): string {
  const [index, setIndex] = useState(0)
  useEffect(() => {
    const reduceMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches
    if (paused || reduceMotion) {
      return
    }
    const timer = window.setInterval(
      () => setIndex((current) => (current + 1) % placeholderKeys.length),
      4_000,
    )
    return () => window.clearInterval(timer)
  }, [paused])
  return t(placeholderKeys[index] ?? 'ai_query.placeholder')
}

function ChatEmptyState({ t, setQuestion, suggested }: ChatEmptyStateProps) {
  const staticSuggestions = [
    t('ai_query.suggestion_1'),
    t('ai_query.suggestion_2'),
    t('ai_query.suggestion_3'),
    t('ai_query.suggestion_4'),
  ]
  const selectPrompt = (prompt: string) => {
    setQuestion(prompt)
    window.requestAnimationFrame(() => document.getElementById('ai-question')?.focus())
  }
  const capabilityCards = [
    ['✦', 'ai_query.empty_capability_explore_title', 'ai_query.empty_capability_explore_desc'],
    ['⌁', 'ai_query.empty_capability_explain_title', 'ai_query.empty_capability_explain_desc'],
    ['▥', 'ai_query.empty_capability_visualize_title', 'ai_query.empty_capability_visualize_desc'],
  ] as const satisfies readonly (readonly [string, TranslationKey, TranslationKey])[]
  return (
    <div className={chatEmptyStateClass}>
      <div
        className="bg-accent/15 text-accent flex h-11 w-11 items-center justify-center rounded-xl text-xl"
        aria-hidden="true"
      >
        ✦
      </div>
      <h3 className={chatEmptyStateTitleClass}>{t('ai_query.empty_analyst_title')}</h3>
      <p className={chatEmptyStateDescClass}>{t('ai_query.empty_analyst_desc')}</p>
      <div className="grid w-full max-w-2xl grid-cols-3 gap-2 max-[700px]:grid-cols-1">
        {capabilityCards.map(([icon, title, description]) => (
          <div
            key={title}
            className="border-border bg-card/60 flex items-start gap-2.5 rounded-xl border p-3 text-left"
          >
            <span className="text-accent mt-0.5" aria-hidden="true">
              {icon}
            </span>
            <span>
              <strong className="text-foreground block text-xs font-semibold">{t(title)}</strong>
              <span className="text-foreground-muted mt-0.5 block text-[0.72rem] leading-relaxed">
                {t(description)}
              </span>
            </span>
          </div>
        ))}
      </div>
      <p className="text-foreground-faint mt-2 mb-0 text-[0.7rem] font-semibold tracking-wide uppercase">
        {t('ai_query.empty_try_prompt')}
      </p>
      <ul className={chatEmptyStateSuggestionsClass} aria-label={t('ai_query.suggestions_aria')}>
        {suggested.length > 0
          ? suggested.map((q) => {
              const catKey = suggestionCategoryKeys[q.category]
              const catLabel = catKey ? t(catKey) : ''
              return (
                <li key={`${q.category}:${q.text}`}>
                  <button
                    type="button"
                    className={chatEmptyStateChipClass}
                    onClick={() => selectPrompt(q.text)}
                  >
                    {catLabel && (
                      <span className="text-foreground-faint mr-1.5 text-[0.72rem] font-medium tracking-wide uppercase">
                        {catLabel}
                      </span>
                    )}
                    {q.text}
                  </button>
                </li>
              )
            })
          : staticSuggestions.map((s) => (
              <li key={s}>
                <button
                  type="button"
                  className={chatEmptyStateChipClass}
                  onClick={() => selectPrompt(s)}
                >
                  {s}
                </button>
              </li>
            ))}
      </ul>
    </div>
  )
}

interface BusinessGlossaryTerm {
  term: string
  definition?: string
}

function trimmedOrUndefined(value: string | undefined): string | undefined {
  const trimmed = value?.trim()
  if (!trimmed) {
    return undefined
  }
  return trimmed
}

interface ChatMessageFeedProps {
  activeConversation: Conversation | null | undefined
  messages: ConversationMessage[]
  suggested: SuggestedQuestion[]
  t: ChatPanelProps['t']
  setQuestion: ChatPanelProps['setQuestion']
  localeNumberTag: ChatPanelProps['localeNumberTag']
  localeTag: string
  datasourceId: string
  aiRuntime: AIRuntimeSettings | null
  get: ChatPanelProps['get']
  postData: ChatPanelProps['postData']
  updateMessageResponse: ChatPanelProps['updateMessageResponse']
  onSendQuery: ChatPanelProps['onSendQuery']
  onSelectFollowUp: (question: string) => void
  priorQuestions: string[]
  agentModeEnabled: boolean
  agentTraceSteps: RunStep[]
  agentClarification: PendingAgentClarification | null
  onAgentClarificationChoice: (choiceId: string) => void
  onAgentClarificationSkip: () => void
  queryAction: ChatPanelProps['queryAction']
  aiElapsedMs: number
  activeJob: TrackedAIJob | null
  queueNotice: string | null
  onAbort: ChatPanelProps['onAbort']
}

// ChatMessageFeed renders the message list (or the empty-state suggestions),
// the ephemeral Agent Mode trace/clarification slot, and the typing
// indicator — everything inside ChatPanel's scrollable feed div except the
// div itself (ChatPanel keeps the ref for its own autoscroll effect).
// Split out from ChatPanel as a plain prop-forwarding extraction (no
// behavior change) to keep ChatPanel's own cyclomatic complexity down.
function ChatMessageFeed({
  activeConversation,
  messages,
  suggested,
  t,
  setQuestion,
  localeNumberTag,
  localeTag,
  datasourceId,
  aiRuntime,
  get,
  postData,
  updateMessageResponse,
  onSendQuery,
  onSelectFollowUp,
  priorQuestions,
  agentModeEnabled,
  agentTraceSteps,
  agentClarification,
  onAgentClarificationChoice,
  onAgentClarificationSkip,
  queryAction,
  aiElapsedMs,
  activeJob,
  queueNotice,
  onAbort,
}: ChatMessageFeedProps) {
  const formatMessageTime = (timestamp: string) => formatTimeOnly(timestamp, localeTag)

  return (
    <>
      {activeConversation && messages.length > 0 ? (
        messages.map((message, index) => {
          if (message.role === 'user') {
            return (
              <div key={index} className={cn(chatMsgClass, chatMsgUserClass)}>
                <div className={cn(chatBubbleClass, userBubbleClass)}>
                  <div className={userBubbleContentClass}>{message.content}</div>
                  <span className={userBubbleTimeClass}>
                    {formatMessageTime(message.timestamp)}
                  </span>
                </div>
              </div>
            )
          }
          const userQuestion = index > 0 ? (messages[index - 1]?.content ?? '') : ''
          return (
            <div
              key={index}
              className={cn(chatMsgClass, chatMsgAssistantClass)}
              data-message-index={index}
            >
              <span className={chatMsgAvatarClass} aria-hidden="true">
                ✦
              </span>
              <div className={chatMsgMainClass}>
                <div className={chatMsgMetaClass}>
                  <span className={chatMsgAuthorClass}>{t('ai_query.assistant_label')}</span>
                  {message.timestamp && (
                    <span className="chat-msg__time">{formatMessageTime(message.timestamp)}</span>
                  )}
                </div>
                <AssistantMessageCard
                  message={message}
                  messageIndex={index}
                  conversationId={activeConversation.id}
                  datasourceId={datasourceId}
                  aiRuntime={aiRuntime}
                  userQuestion={userQuestion}
                  get={get}
                  postData={postData}
                  updateMessageResponse={updateMessageResponse}
                  t={t}
                  localeNumberTag={localeNumberTag}
                  localeTag={localeTag}
                  onSelectClarification={(choice, originalQuestion) =>
                    onSendQuery(originalQuestion, true, choice)
                  }
                  onSkipClarification={(originalQuestion) =>
                    onSendQuery(originalQuestion, true, CLARIFICATION_SKIP_CHOICE)
                  }
                  onFilterByValue={(column, value) => {
                    const filterText = t('ai_query.filter_by_value', { column, value })
                    setQuestion((prev: string) => (prev ? `${prev} ${filterText}` : filterText))
                  }}
                  onCellDrillDown={(col, val) =>
                    onSendQuery(t('ai_query.drill_down_prompt', { column: col, value: val }), true)
                  }
                  onSelectFollowUp={onSelectFollowUp}
                  priorQuestions={priorQuestions}
                  isLatest={index === messages.length - 1}
                />
              </div>
            </div>
          )
        })
      ) : (
        <ChatEmptyState t={t} setQuestion={setQuestion} suggested={suggested} />
      )}
      {agentModeEnabled && (agentTraceSteps.length > 0 || agentClarification) ? (
        <AgentTraceCard
          steps={agentTraceSteps}
          clarification={agentClarification}
          onSelectClarificationChoice={onAgentClarificationChoice}
          onSkipClarification={onAgentClarificationSkip}
          t={t}
        />
      ) : null}
      <TypingIndicator
        queryAction={queryAction}
        aiElapsedMs={aiElapsedMs}
        activeJob={activeJob}
        queueNotice={queueNotice}
        onAbort={onAbort}
        t={t}
      />
    </>
  )
}

export function ChatPanel({
  t,
  localeNumberTag,
  localeTag,
  activeConversation,
  activeConversationId,
  datasourceId,
  semanticModelId,
  tables,
  aiRuntime,
  question,
  setQuestion,
  loading,
  error,
  jobError,
  queryAction,
  aiElapsedMs,
  activeJob,
  queueNotice,
  contextEnabled,
  onContextEnabledChange,
  autoFindEnabled,
  onAutoFindEnabledChange,
  agentModeEnabled,
  onAgentModeEnabledChange,
  agentTraceSteps,
  agentClarification,
  onAgentClarificationChoice,
  onAgentClarificationSkip,
  selectedSavedQueryIds,
  onSelectedSavedQueryIdsChange,
  onSendQuery,
  onAbort,
  get,
  postData,
  updateMessageResponse,
}: ChatPanelProps) {
  const chatFeedRef = useRef<HTMLDivElement>(null)
  const prevConvIdRef = useRef<string | undefined>(undefined)
  const prevMsgCountRef = useRef<number>(0)

  const messages = useMemo(() => activeConversation?.messages ?? [], [activeConversation])

  const priorQuestions = useMemo(
    () => messages.filter((m) => m.role === 'user').map((m) => m.content),
    [messages],
  )

  const handleSelectFollowUp = (nextQuestion: string) => {
    setQuestion(nextQuestion)
    window.requestAnimationFrame(() => {
      document.getElementById('ai-question')?.focus()
    })
  }

  const {
    items: catalogItems,
    canRetranslate,
    retranslate,
    retranslating,
  } = useSemanticCatalog(semanticModelId, tables)

  const [model, setModel] = useState<SemanticModelDetail | null>(null)
  const [savedQueries, setSavedQueries] = useState<SavedQueryOption[]>([])
  const [glossaryResult, setGlossaryResult] = useState<{
    key: string
    rows: BusinessGlossaryTerm[]
  }>({ key: '', rows: [] })

  useEffect(() => {
    if (!datasourceId) {
      return
    }
    let cancelled = false
    const params = new URLSearchParams({ datasource_id: datasourceId })
    if (semanticModelId && !semanticModelId.startsWith('composite:')) {
      params.set('model_id', semanticModelId)
    }
    const key = params.toString()
    void get<BusinessGlossaryTerm[]>(`/api/ai/glossary?${params.toString()}`)
      .then((rows) => {
        if (!cancelled) {
          setGlossaryResult({ key, rows: rows ?? [] })
        }
      })
      .catch(() => {
        if (!cancelled) {
          setGlossaryResult({ key, rows: [] })
        }
      })
    return () => {
      cancelled = true
    }
  }, [datasourceId, semanticModelId, get])

  const glossaryKey = useMemo(() => {
    if (!datasourceId) {
      return ''
    }
    const params = new URLSearchParams({ datasource_id: datasourceId })
    if (semanticModelId && !semanticModelId.startsWith('composite:')) {
      params.set('model_id', semanticModelId)
    }
    return params.toString()
  }, [datasourceId, semanticModelId])
  const composerItems = useMemo(() => {
    const glossaryTerms = glossaryResult.key === glossaryKey ? glossaryResult.rows : []
    return [
      ...catalogItems,
      ...glossaryTerms.map((entry) => ({
        type: 'term' as const,
        name: entry.term.trim().replaceAll(/\s+/g, '_'),
        label: entry.term,
        description: trimmedOrUndefined(entry.definition),
        group: t('ai_query.term_group'),
      })),
    ]
  }, [catalogItems, glossaryKey, glossaryResult, t])

  // Load the datasource's saved queries for the "/" grounding picker.
  // listSavedQueries returns [] for an empty datasource, so the reset flows
  // through the async setter rather than a synchronous setState in the effect.
  useEffect(() => {
    let cancelled = false
    void listSavedQueries(datasourceId)
      .then((rows) => {
        if (!cancelled) {
          setSavedQueries(rows)
        }
      })
      .catch(() => {
        if (!cancelled) {
          setSavedQueries([])
        }
      })
    return () => {
      cancelled = true
    }
  }, [datasourceId])

  useEffect(() => {
    if (!semanticModelId) {
      return
    }
    let cancelled = false
    void get<SemanticModelDetail>(`/api/semantic/models/${semanticModelId}`).then((detail) => {
      if (!cancelled) {
        setModel(detail)
      }
    })
    return () => {
      cancelled = true
    }
  }, [semanticModelId, get])

  // Gate on id match so a stale detail (from a previous model or after the
  // selection is cleared) never produces suggestions for the wrong model.
  const suggested = useMemo(
    () => (model?.id === semanticModelId ? buildSuggestedQuestions(model, t) : []),
    [model, semanticModelId, t],
  )

  useEffect(() => {
    const feed = chatFeedRef.current
    const currentId = activeConversation?.id
    const currentCount = messages.length

    if (feed) {
      scrollChatFeed({
        feed,
        messages,
        currentCount,
        prevConversationId: prevConvIdRef.current,
        currentConversationId: currentId,
        prevMessageCount: prevMsgCountRef.current,
      })
    }

    prevConvIdRef.current = currentId
    prevMsgCountRef.current = currentCount
  }, [activeConversation, activeConversationId, queryAction, messages])

  const loadingLabel = loading && queryAction !== null ? t('ai_query.loading_thinking') : ''

  const executeButtonLabel =
    loading && queryAction === 'execute' ? loadingLabel : t('ai_query.run_analysis')
  const previewButtonLabel =
    loading && queryAction === 'preview' ? loadingLabel : t('ai_query.preview_plan_query')
  const composerPlaceholder = useRotatingPlaceholder(t, question.length > 0 || queryAction !== null)

  return (
    <>
      <div ref={chatFeedRef} className={chatFeedClass}>
        <ChatMessageFeed
          activeConversation={activeConversation}
          messages={messages}
          suggested={suggested}
          t={t}
          setQuestion={setQuestion}
          localeNumberTag={localeNumberTag}
          localeTag={localeTag}
          datasourceId={datasourceId}
          aiRuntime={aiRuntime}
          get={get}
          postData={postData}
          updateMessageResponse={updateMessageResponse}
          onSendQuery={onSendQuery}
          onSelectFollowUp={handleSelectFollowUp}
          priorQuestions={priorQuestions}
          agentModeEnabled={agentModeEnabled}
          agentTraceSteps={agentTraceSteps}
          agentClarification={agentClarification}
          onAgentClarificationChoice={onAgentClarificationChoice}
          onAgentClarificationSkip={onAgentClarificationSkip}
          queryAction={queryAction}
          aiElapsedMs={aiElapsedMs}
          activeJob={activeJob}
          queueNotice={queueNotice}
          onAbort={onAbort}
        />
      </div>

      <footer className={chatInputAreaClass}>
        <div className={chatComposerClass}>
          <PromptTextarea
            value={question}
            onChange={(v) => setQuestion(v)}
            onSubmit={() => onSendQuery(question, true)}
            onAbort={onAbort}
            disabled={queryAction !== null}
            inFlight={queryAction !== null}
            placeholder={composerPlaceholder}
            items={composerItems}
            savedQueries={savedQueries}
            selectedSavedQueryIds={selectedSavedQueryIds}
            onSelectedSavedQueryIdsChange={onSelectedSavedQueryIdsChange}
            t={t}
          />
          <div className={chatComposerBarClass}>
            <div className={chatComposerOptionsClass}>
              <AgentConfigurationPopover
                t={t}
                contextAvailable={Boolean(activeConversation && messages.length > 0)}
                contextEnabled={contextEnabled}
                onContextEnabledChange={(enabled) => {
                  if (activeConversation) {
                    onContextEnabledChange(activeConversation.id, enabled)
                  }
                }}
                autoFindEnabled={autoFindEnabled}
                onAutoFindEnabledChange={onAutoFindEnabledChange}
                agentModeEnabled={agentModeEnabled}
                onAgentModeEnabledChange={onAgentModeEnabledChange}
              />
              <span className={chatComposerHintClass}>{t('ai_query.saved_query_hint')}</span>
              {canRetranslate && (
                <button
                  type="button"
                  className="text-foreground-muted hover:text-foreground inline-flex items-center gap-1 text-[0.72rem] underline decoration-dotted underline-offset-2 disabled:cursor-not-allowed disabled:opacity-60"
                  onClick={() => void retranslate()}
                  disabled={retranslating}
                  title={t('ai_query.retranslate_title')}
                >
                  {retranslating ? t('ai_query.retranslating') : `↻ ${t('ai_query.retranslate')}`}
                </button>
              )}
            </div>
            <div className={chatComposerActionsClass}>
              {/* Agent Mode's SSE stream never sets useApi()'s `loading` (it
                  doesn't go through postData/get), so the Cancel button also
                  needs to show whenever an Agent Mode turn is in flight
                  (queryAction is set unconditionally at the top of
                  sendQuery for both paths). */}
              {(loading || agentModeEnabled) && queryAction !== null && (
                <button
                  className={cn(buttonClass('ghost'), chatComposerActionBtnClass)}
                  onClick={onAbort}
                >
                  {t('ai_query.cancel')}
                </button>
              )}
              <button
                className={cn(buttonClass('secondary'), chatComposerActionBtnClass)}
                onClick={() => onSendQuery(question, false)}
                disabled={loading || !question.trim() || !datasourceId}
              >
                {previewButtonLabel}
              </button>
              <button
                className={cn(
                  buttonClass('primary'),
                  chatComposerActionBtnClass,
                  chatComposerSendClass,
                )}
                onClick={() => onSendQuery(question, true)}
                disabled={loading || !question.trim() || !datasourceId}
              >
                {executeButtonLabel}
                <span className={chatComposerSendIconClass} aria-hidden="true">
                  ➤
                </span>
              </button>
            </div>
          </div>
        </div>

        <ErrorAlert error={error ?? jobError} className={legacyFeedbackClass('error--top-gap')} />
      </footer>
    </>
  )
}
