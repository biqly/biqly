import type { KeyboardEvent } from 'react'
import { useEffect, useRef } from 'react'

import { ErrorAlert } from '../ui/ErrorAlert'
import { AssistantMessageCard } from './AssistantMessageCard'
import { formatAiWaitElapsed } from './routingViz'
import type { ChatPanelProps } from './types'
import { AI_QUERY_TIMEOUT_MS } from './types'

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
  includePastQueries,
  setIncludePastQueries,
  onSendQuery,
  onAbort,
  get,
  postData,
  updateMessageResponse,
}: ChatPanelProps) {
  const chatFeedRef = useRef<HTMLDivElement>(null)
  const prevConvIdRef = useRef<string | undefined>(undefined)
  const prevMsgCountRef = useRef<number>(0)

  useEffect(() => {
    const feed = chatFeedRef.current
    const currentId = activeConversation?.id
    const messages = activeConversation?.messages ?? []
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
  }, [activeConversation, activeConversationId, queryAction])

  const loadingLabel = loading && queryAction !== null ? t('ai_query.loading_thinking') : ''

  const previewButtonLabel =
    loading && queryAction === 'preview' ? loadingLabel : t('ai_query.preview_btn')
  const executeButtonLabel =
    loading && queryAction === 'execute' ? loadingLabel : t('ai_query.execute_btn')

  const formatMessageTime = (timestamp: string) =>
    new Date(timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })

  const typingIndicator =
    queryAction !== null ? (
      <div className="chat-msg chat-msg--assistant chat-msg--typing" role="status">
        <span className="chat-msg__avatar" aria-hidden="true">
          ✦
        </span>
        <div className="chat-msg__main">
          <div className="chat-typing">
            <span className="chat-typing__dots" aria-hidden="true">
              <i />
              <i />
              <i />
            </span>
            <span className="chat-typing__label">{t('ai_query.loading_thinking')}</span>
            <span className="chat-typing__elapsed">{formatAiWaitElapsed(aiElapsedMs, t)}</span>
          </div>
          <p className="chat-typing__hint">
            {t('ai_query.wait_hint', { minutes: Math.round(AI_QUERY_TIMEOUT_MS / 60_000) })}
          </p>
        </div>
      </div>
    ) : null

  return (
    <>
      <div ref={chatFeedRef} className="chat-feed">
        {activeConversation && activeConversation.messages.length > 0 ? (
          (() => {
            const conv = activeConversation
            return conv.messages.map((message, index) => {
              if (message.role === 'user') {
                return (
                  <div key={index} className="chat-msg chat-msg--user">
                    <div className="chat-bubble user-bubble">
                      <div className="bubble-content">{message.content}</div>
                      <span className="bubble-time">{formatMessageTime(message.timestamp)}</span>
                    </div>
                  </div>
                )
              } else {
                const userQuestion = index > 0 ? (conv.messages[index - 1]?.content ?? '') : ''
                return (
                  <div
                    key={index}
                    className="chat-msg chat-msg--assistant"
                    data-message-index={index}
                  >
                    <span className="chat-msg__avatar" aria-hidden="true">
                      ✦
                    </span>
                    <div className="chat-msg__main">
                      <div className="chat-msg__meta">
                        <span className="chat-msg__author">{t('ai_query.assistant_label')}</span>
                        {message.timestamp && (
                          <span className="chat-msg__time">
                            {formatMessageTime(message.timestamp)}
                          </span>
                        )}
                      </div>
                      <AssistantMessageCard
                        message={message}
                        messageIndex={index}
                        conversationId={conv.id}
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
                          setQuestion((prev: string) =>
                            prev ? `${prev} ${filterText}` : filterText,
                          )
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
          })()
        ) : (
          <div className="chat-empty-state">
            <h3>✨ {t('ai_query.workspace_title')}</h3>
            <p>{t('ai_query.subtitle')}</p>
            <div
              className="chat-empty-state__suggestions"
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
                  className="chat-empty-state__chip"
                  onClick={() => setQuestion(s)}
                >
                  {s}
                </button>
              ))}
            </div>
          </div>
        )}
        {typingIndicator}
      </div>

      <footer className="chat-input-area">
        <div className="chat-composer">
          <textarea
            id="ai-question"
            className="chat-composer__input"
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
          />
          <div className="chat-composer__bar">
            <div className="chat-composer__options">
              {activeConversation && activeConversation.messages.length > 0 && (
                <div className="past-queries-toggle">
                  <input
                    type="checkbox"
                    id="include-past"
                    checked={includePastQueries}
                    onChange={(e) => setIncludePastQueries(e.target.checked)}
                  />
                  <label htmlFor="include-past">{t('ai_query.include_past_checkbox')}</label>
                </div>
              )}
              <span className="chat-composer__hint">{t('ai_query.enter_hint')}</span>
            </div>
            <div className="chat-composer__actions">
              {loading && queryAction !== null && (
                <button className="btn btn-ghost" onClick={onAbort}>
                  {t('ai_query.cancel')}
                </button>
              )}
              <button
                className="btn"
                onClick={() => onSendQuery(question, false)}
                disabled={loading || !question || !datasourceId}
              >
                {previewButtonLabel}
              </button>
              <button
                className="btn btn-primary chat-composer__send"
                onClick={() => onSendQuery(question, true)}
                disabled={loading || !question || !datasourceId}
              >
                {executeButtonLabel}
                <span className="chat-composer__send-icon" aria-hidden="true">
                  ➤
                </span>
              </button>
            </div>
          </div>
        </div>

        <ErrorAlert error={error ?? jobError} className="error--top-gap" />
      </footer>
    </>
  )
}
