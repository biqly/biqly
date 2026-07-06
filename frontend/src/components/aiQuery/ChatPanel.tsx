import { useEffect, useMemo, useRef, useState } from 'react'

import { listSavedQueries, type SavedQueryOption } from '../../api/aiSkills'
import { jobIsActive, type TrackedAIJob } from '../../hooks/useAIJobs'
import { useJobNow } from '../../hooks/useJobNow'
import { useSemanticCatalog } from '../../hooks/useSemanticCatalog'
import type { TranslationKey } from '../../i18n'
import { buttonClass } from '../../lib/buttonClasses'
import { cn } from '../../lib/cn'
import { legacyFeedbackClass } from '../../lib/feedbackClasses'
import type { SemanticModelDetail } from '../../types/semantic'
import { formatTimeOnly } from '../../utils/formatters'
import { JobPhaseSteps } from '../ai/JobPhaseSteps'
import { ErrorAlert } from '../ui/ErrorAlert'
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
  pastQueriesToggleClass,
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
  t: (key: TranslationKey, params?: Record<string, string | number>) => string
}

function TypingIndicator({
  queryAction,
  aiElapsedMs,
  activeJob,
  queueNotice,
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
          <span className={chatTypingLabelClass}>{t('ai_query.loading_thinking')}</span>
          <span className={chatTypingElapsedClass}>{formatAiWaitElapsed(aiElapsedMs, t)}</span>
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

function ChatEmptyState({ t, setQuestion, suggested }: ChatEmptyStateProps) {
  const staticSuggestions = [
    t('ai_query.suggestion_1'),
    t('ai_query.suggestion_2'),
    t('ai_query.suggestion_3'),
    t('ai_query.suggestion_4'),
  ]
  return (
    <div className={chatEmptyStateClass}>
      <h3 className={chatEmptyStateTitleClass}>✨ {t('ai_query.workspace_title')}</h3>
      <p className={chatEmptyStateDescClass}>{t('ai_query.subtitle')}</p>
      <div
        className={chatEmptyStateSuggestionsClass}
        role="list"
        aria-label={t('ai_query.suggestions_aria')}
      >
        {suggested.length > 0
          ? suggested.map((q) => {
              const catKey = suggestionCategoryKeys[q.category]
              const catLabel = catKey ? t(catKey) : ''
              return (
                <button
                  key={`${q.category}:${q.text}`}
                  type="button"
                  role="listitem"
                  className={chatEmptyStateChipClass}
                  onClick={() => setQuestion(q.text)}
                >
                  {catLabel && (
                    <span className="text-foreground-faint mr-1.5 text-[0.72rem] font-medium tracking-wide uppercase">
                      {catLabel}
                    </span>
                  )}
                  {q.text}
                </button>
              )
            })
          : staticSuggestions.map((s) => (
              <button
                key={s}
                type="button"
                role="listitem"
                className={chatEmptyStateChipClass}
                onClick={() => setQuestion(s)}
              >
                {s}
              </button>
            ))}
      </div>
    </div>
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

  const {
    items: catalogItems,
    canRetranslate,
    retranslate,
    retranslating,
  } = useSemanticCatalog(semanticModelId, tables)

  const [model, setModel] = useState<SemanticModelDetail | null>(null)
  const [savedQueries, setSavedQueries] = useState<SavedQueryOption[]>([])

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

  const previewButtonLabel =
    loading && queryAction === 'preview' ? loadingLabel : t('ai_query.preview_btn')
  const executeButtonLabel =
    loading && queryAction === 'execute' ? loadingLabel : t('ai_query.execute_btn')

  const formatMessageTime = (timestamp: string) => formatTimeOnly(timestamp, localeTag)

  return (
    <>
      <div ref={chatFeedRef} className={chatFeedClass}>
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
            } else {
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
                        <span className="chat-msg__time">
                          {formatMessageTime(message.timestamp)}
                        </span>
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
                        onSendQuery(originalQuestion, true)
                      }
                      onFilterByValue={(column, value) => {
                        const filterText = t('ai_query.filter_by_value', { column, value })
                        setQuestion((prev: string) => (prev ? `${prev} ${filterText}` : filterText))
                      }}
                      onCellDrillDown={(col, val) =>
                        onSendQuery(
                          t('ai_query.drill_down_prompt', { column: col, value: val }),
                          true,
                        )
                      }
                    />
                  </div>
                </div>
              )
            }
          })
        ) : (
          <ChatEmptyState t={t} setQuestion={setQuestion} suggested={suggested} />
        )}
        <TypingIndicator
          queryAction={queryAction}
          aiElapsedMs={aiElapsedMs}
          activeJob={activeJob}
          queueNotice={queueNotice}
          t={t}
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
            loading={loading}
            placeholder={t('ai_query.placeholder')}
            items={catalogItems}
            savedQueries={savedQueries}
            selectedSavedQueryIds={selectedSavedQueryIds}
            onSelectedSavedQueryIdsChange={onSelectedSavedQueryIdsChange}
            t={t}
          />
          <div className={chatComposerBarClass}>
            <div className={chatComposerOptionsClass}>
              {activeConversation && messages.length > 0 && (
                <div className={pastQueriesToggleClass}>
                  <input
                    type="checkbox"
                    id={`conversation-context-${activeConversation.id}`}
                    checked={contextEnabled}
                    onChange={(e) =>
                      onContextEnabledChange(activeConversation.id, e.target.checked)
                    }
                  />
                  <label htmlFor={`conversation-context-${activeConversation.id}`}>
                    {t('ai_query.context_toggle')}
                  </label>
                </div>
              )}
              <div className={pastQueriesToggleClass}>
                <input
                  type="checkbox"
                  id="ai-auto-find-skills"
                  checked={autoFindEnabled}
                  onChange={(e) => onAutoFindEnabledChange(e.target.checked)}
                />
                <label htmlFor="ai-auto-find-skills" title={t('ai_query.auto_find_toggle_title')}>
                  {t('ai_query.auto_find_toggle')}
                </label>
              </div>
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
              {loading && queryAction !== null && (
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
                disabled={loading || !question || !datasourceId}
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
                disabled={loading || !question || !datasourceId}
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
