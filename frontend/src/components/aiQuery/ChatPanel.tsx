import { useEffect, useRef } from 'react'
import type { KeyboardEvent } from 'react'
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
    const currentId = activeConversation?.id
    const currentCount = activeConversation?.messages.length ?? 0

    if (chatFeedRef.current) {
      const isSameConv = prevConvIdRef.current === currentId
      const behavior = isSameConv && currentCount > prevMsgCountRef.current ? 'smooth' : 'auto'
      chatFeedRef.current.scrollTo({
        top: chatFeedRef.current.scrollHeight,
        behavior,
      })
    }

    prevConvIdRef.current = currentId
    prevMsgCountRef.current = currentCount
  }, [activeConversation?.messages.length, activeConversationId])

  const loadingLabel = loading && queryAction !== null
    ? t('ai_query.loading_thinking')
    : ''

  const previewButtonLabel = loading && queryAction === 'preview' ? loadingLabel : t('ai_query.preview_btn')
  const executeButtonLabel = loading && queryAction === 'execute' ? loadingLabel : t('ai_query.execute_btn')

  return (
    <>
      <div ref={chatFeedRef} className="chat-feed">
        {activeConversation && activeConversation.messages.length > 0 ? (() => {
          const conv = activeConversation
          return conv.messages.map((message: any, index: number) => {
            if (message.role === 'user') {
              return (
                <div key={index} className="chat-bubble user-bubble">
                  <div className="bubble-content">{message.content}</div>
                  <span className="bubble-time">
                    {new Date(message.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
                  </span>
                </div>
              )
            } else {
              const userQuestion = index > 0 ? conv.messages[index - 1]?.content ?? '' : ''
              return (
                <div key={index} className="chat-bubble assistant-bubble">
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
                    onSelectClarification={(opt) => onSendQuery(opt, true)}
                    onSkipClarification={() => onSendQuery(question, true)}
                    onFilterByValue={(column, value) => {
                      const filterText = t('ai_query.filter_by_value', { column, value })
                      setQuestion((prev: string) => prev ? `${prev} ${filterText}` : filterText)
                    }}
                    onCellDrillDown={(col, val) => onSendQuery(t('ai_query.drill_down_prompt', { column: col, value: val }), true)}
                  />
                </div>
              )
            }
          })
        })() : (
          <div className="chat-empty-state">
            <h3>✨ {t('ai_query.workspace_title')}</h3>
            <p>{t('ai_query.subtitle')}</p>
            <div className="chat-empty-state__suggestions" role="list" aria-label={t('ai_query.suggestions_aria')}>
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
      </div>

      <footer className="chat-input-area">
        <div className="form-group">
          <textarea
            id="ai-question"
            value={question}
            onChange={(e) => setQuestion(e.target.value)}
            onKeyDown={(e: KeyboardEvent<HTMLTextAreaElement>) => {
              if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
                e.preventDefault()
                onSendQuery(question, true)
              }
            }}
            placeholder={t('ai_query.placeholder')}
            rows={2}
            autoComplete="off"
          />
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
        </div>

        <div className="button-row">
          <button
            className="btn"
            onClick={() => onSendQuery(question, false)}
            disabled={loading || !question || !datasourceId}
          >
            {previewButtonLabel}
          </button>
          <button
            className="btn btn-primary"
            onClick={() => onSendQuery(question, true)}
            disabled={loading || !question || !datasourceId}
          >
            {executeButtonLabel}
          </button>
          {loading && queryAction !== null && (
            <button className="btn btn-ghost" onClick={onAbort}>
              {t('ai_query.cancel')}
            </button>
          )}
        </div>

        {queryAction !== null && (
          <div className="ai-wait-meta" role="status" aria-live="polite">
            <span className="ai-wait-meta-time">
              {t('ai_query.elapsed_label')} {formatAiWaitElapsed(aiElapsedMs, t)}
            </span>
            <span className="ai-wait-meta-hint">
              {t('ai_query.wait_hint', { minutes: Math.round(AI_QUERY_TIMEOUT_MS / 60_000) })}
            </span>
          </div>
        )}

        <ErrorAlert error={error ?? jobError} className="error--top-gap" />
      </footer>
    </>
  )
}
