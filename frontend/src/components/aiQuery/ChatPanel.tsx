import type { KeyboardEvent } from 'react'
import { useEffect, useMemo, useRef } from 'react'

import type { TranslationKey } from '../../i18n'
import { legacyButtonClass } from '../../lib/buttonClasses'
import { cn } from '../../lib/cn'
import { legacyFeedbackClass } from '../../lib/feedbackClasses'
import { ErrorAlert } from '../ui/ErrorAlert'
import {
  chatBubbleClass,
  chatComposerActionsClass,
  chatComposerBarClass,
  chatComposerClass,
  chatComposerHintClass,
  chatComposerInputClass,
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
import { formatAiWaitElapsed } from './routingViz'
import type { ChatPanelProps } from './types'
import { AI_QUERY_TIMEOUT_MS } from './types'
interface TypingIndicatorProps {
  queryAction: string | null
  aiElapsedMs: number
  t: (key: TranslationKey, params?: Record<string, string | number>) => string
}

function TypingIndicator({ queryAction, aiElapsedMs, t }: TypingIndicatorProps) {
  if (queryAction === null) {
    return null
  }
  return (
    <div className={`${chatMsgClass} ${chatMsgAssistantClass}`} role="status">
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
}

function ChatEmptyState({ t, setQuestion }: ChatEmptyStateProps) {
  return (
    <div className={chatEmptyStateClass}>
      <h3 className={chatEmptyStateTitleClass}>✨ {t('ai_query.workspace_title')}</h3>
      <p className={chatEmptyStateDescClass}>{t('ai_query.subtitle')}</p>
      <div
        className={chatEmptyStateSuggestionsClass}
        role="list"
        aria-label={t('ai_query.suggestions_aria')}
      >
        {[
          t('ai_query.suggestion_1'),
          t('ai_query.suggestion_2'),
          t('ai_query.suggestion_3'),
          t('ai_query.suggestion_4'),
        ].map((s) => (
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
  aiRuntime,
  question,
  setQuestion,
  loading,
  error,
  jobError,
  queryAction,
  aiElapsedMs,
  contextEnabled,
  onContextEnabledChange,
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

  useEffect(() => {
    const feed = chatFeedRef.current
    const currentId = activeConversation?.id
    const currentCount = messages.length

    if (feed) {
      const isSameConv = prevConvIdRef.current === currentId
      const behavior: ScrollBehavior =
        isSameConv && currentCount > prevMsgCountRef.current ? 'smooth' : 'auto'

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

    prevConvIdRef.current = currentId
    prevMsgCountRef.current = currentCount
  }, [activeConversation, activeConversationId, queryAction, messages])

  const loadingLabel = loading && queryAction !== null ? t('ai_query.loading_thinking') : ''

  const previewButtonLabel =
    loading && queryAction === 'preview' ? loadingLabel : t('ai_query.preview_btn')
  const executeButtonLabel =
    loading && queryAction === 'execute' ? loadingLabel : t('ai_query.execute_btn')

  const formatMessageTime = (timestamp: string) =>
    new Date(timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })

  return (
    <>
      <div ref={chatFeedRef} className={chatFeedClass}>
        {activeConversation && messages.length > 0 ? (
          messages.map((message, index) => {
            if (message.role === 'user') {
              return (
                <div key={index} className={`${chatMsgClass} ${chatMsgUserClass}`}>
                  <div className={`${chatBubbleClass} ${userBubbleClass}`}>
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
                  className={`${chatMsgClass} ${chatMsgAssistantClass}`}
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
          <ChatEmptyState t={t} setQuestion={setQuestion} />
        )}
        <TypingIndicator queryAction={queryAction} aiElapsedMs={aiElapsedMs} t={t} />
      </div>

      <footer className={chatInputAreaClass}>
        <div className={chatComposerClass}>
          <textarea
            id="ai-question"
            className={chatComposerInputClass}
            value={question}
            onChange={(e) => setQuestion(e.target.value)}
            onKeyDown={(e: KeyboardEvent<HTMLTextAreaElement>) => {
              if (e.key === 'Enter' && !e.shiftKey) {
                e.preventDefault()
                if (!loading && question && datasourceId) {
                  onSendQuery(question, true)
                }
              }
            }}
            placeholder={t('ai_query.placeholder')}
            rows={2}
            autoComplete="off"
            disabled={queryAction !== null}
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
              <span className={chatComposerHintClass}>{t('ai_query.enter_hint')}</span>
            </div>
            <div className={chatComposerActionsClass}>
              {loading && queryAction !== null && (
                <button className={legacyButtonClass('btn btn-ghost')} onClick={onAbort}>
                  {t('ai_query.cancel')}
                </button>
              )}
              <button
                className={legacyButtonClass('btn')}
                onClick={() => onSendQuery(question, false)}
                disabled={loading || !question || !datasourceId}
              >
                {previewButtonLabel}
              </button>
              <button
                className={cn(legacyButtonClass('btn btn-primary'), chatComposerSendClass)}
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
