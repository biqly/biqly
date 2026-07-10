import { useMemo, useState } from 'react'

import { cn } from '../../lib/cn'
import { infoNoticeClass } from '../../lib/feedbackClasses'
import type { QueryResultPayload } from '../../types/ai'
import { normalizeAIQueryResponse } from '../../utils/normalizeAIQueryResponse'
import { buildPivotTable } from '../../utils/pivotTable'
import { ErrorAlert } from '../ui/ErrorAlert'
import {
  assistantCardClass,
  assistantCardDetailsToggleClass,
  assistantCardTopClass,
} from './aiQueryClasses'
import { AnswerArtifactActions } from './AnswerArtifactActions'
import {
  AssistantMessageAnswer,
  AssistantMessageCaveat,
  AssistantMessageClarificationSections,
  AssistantMessageHeader,
  AssistantMessageQueryDetails,
  AssistantMessageResults,
  AssistantMessageRunQuery,
  AssistantMessageSummary,
} from './assistantMessageCardSections'
import { FeedbackSection } from './FeedbackSection'
import { buildFallbackFollowUps, filterFollowUpSuggestions } from './followUpSuggestions'
import { FollowUpSuggestions } from './FollowUpSuggestionsSection'
import { buildResultInsight } from './resultInsight'
import { RunTracePanel } from './RunTrace'
import { SampleDataModal } from './SampleDataModal'
import type { AssistantMessageCardProps, FeedbackCatKey } from './types'
import { AI_QUERY_TIMEOUT_MS } from './types'

function AssistantCardTop({
  result,
  showDetails,
  onToggle,
  t,
}: {
  result: NonNullable<ReturnType<typeof normalizeAIQueryResponse>>
  showDetails: boolean
  onToggle: () => void
  t: AssistantMessageCardProps['t']
}) {
  const warningCount = result.warnings?.length ?? 0
  return (
    <div className={assistantCardTopClass}>
      <AssistantMessageSummary result={result} t={t} />
      <button
        type="button"
        className={assistantCardDetailsToggleClass}
        aria-expanded={showDetails}
        onClick={onToggle}
      >
        <span aria-hidden="true">☰</span>
        {showDetails ? t('ai_query.details_hide') : t('ai_query.details_show')}
        {!showDetails && warningCount > 0 ? ` (${warningCount})` : ''}
      </button>
    </div>
  )
}

function AssistantRunPrompt({
  result,
  loading,
  onRunQuery,
  t,
}: {
  result: NonNullable<ReturnType<typeof normalizeAIQueryResponse>>
  loading: boolean
  onRunQuery: () => void
  t: AssistantMessageCardProps['t']
}) {
  if (result.result || !result.sql) {
    return null
  }
  return <AssistantMessageRunQuery loading={loading} onRunQuery={onRunQuery} t={t} />
}

/** Failure/cancellation notes persisted from background jobs carry plain
 * text without a structured ai_response. */
function AssistantPlainNote({ content }: { content: string }) {
  if (!content) {
    return null
  }
  return (
    <div className={cn(infoNoticeClass, 'mb-0')} role="status">
      <span className="text-accent" aria-hidden="true">
        ⓘ{' '}
      </span>
      {content}
    </div>
  )
}

function AssistantMessageFeedbackRow({
  userQuestion,
  datasourceId,
  submitFeedback,
  t,
}: {
  userQuestion: string
  datasourceId: string
  submitFeedback: (
    body: Record<string, unknown>,
  ) => Promise<{ status: string; learned?: boolean } | null>
  t: AssistantMessageCardProps['t']
}) {
  return (
    <div className="feedback-row-wrapper">
      <FeedbackSection
        onSubmitPositive={async () => {
          const res = await submitFeedback({
            question: userQuestion,
            datasource_id: datasourceId,
            rating: 'positive',
          })
          return res?.learned === true
        }}
        onSubmitNegative={(categories: FeedbackCatKey[], text: string) => {
          void submitFeedback({
            question: userQuestion,
            datasource_id: datasourceId,
            rating: 'negative',
            categories: categories.map((k) => t(k)),
            text,
          })
        }}
      />
    </div>
  )
}

function AssistantMessageDetailsSection({
  showDetails,
  result,
  aiRuntime,
  onSampleData,
  t,
  localeTag,
}: {
  showDetails: boolean
  result: NonNullable<ReturnType<typeof normalizeAIQueryResponse>>
  aiRuntime: AssistantMessageCardProps['aiRuntime']
  onSampleData: (tableName: string) => void
  t: AssistantMessageCardProps['t']
  localeTag: string
}) {
  if (!showDetails) {
    return null
  }
  return (
    <>
      <AssistantMessageHeader result={result} aiRuntime={aiRuntime} t={t} />
      <AssistantMessageQueryDetails
        result={result}
        onSampleData={onSampleData}
        t={t}
        localeTag={localeTag}
      />
    </>
  )
}

// The step summary the live trace showed while running survives here as a
// collapsed timeline instead of disappearing with the answer.
function AssistantThinkingSteps({
  result,
  t,
}: {
  result: NonNullable<ReturnType<typeof normalizeAIQueryResponse>>
  t: AssistantMessageCardProps['t']
}) {
  if ((result.run_steps?.length ?? 0) === 0 && !result.run_id) {
    return null
  }
  return (
    <RunTracePanel
      steps={result.run_steps ?? []}
      runId={result.run_id}
      title={t('ai_query.thinking_steps_title')}
    />
  )
}

function mapChartSuggestion(raw: string | undefined): 'bar' | 'line' | 'pie' | 'table' | null {
  if (!raw) {
    return null
  }
  const mapped = raw === 'number' ? 'table' : raw
  if (mapped === 'bar' || mapped === 'line' || mapped === 'pie' || mapped === 'table') {
    return mapped
  }
  return null
}

function useResetStateOnDepsChange<T>(initialValue: T, deps: unknown[]) {
  const [state, setState] = useState<T>(initialValue)
  const [prevDeps, setPrevDeps] = useState(deps)

  const changed = deps.some((d, i) => d !== prevDeps[i])
  if (changed) {
    setPrevDeps(deps)
    setState(initialValue)
  }

  return [state, setState] as const
}

export function AssistantMessageCard({
  message,
  messageIndex,
  conversationId,
  datasourceId,
  aiRuntime,
  userQuestion,
  get,
  postData,
  updateMessageResponse,
  t,
  localeTag,
  onSelectClarification,
  onSkipClarification,
  onFilterByValue,
  onCellDrillDown,
  onSelectFollowUp,
  priorQuestions,
  isLatest,
}: AssistantMessageCardProps) {
  const result = useMemo(() => normalizeAIQueryResponse(message.ai_response), [message.ai_response])

  const followUps = useMemo(() => {
    if (!result) {
      return []
    }
    const backend = filterFollowUpSuggestions(result.suggested_followups ?? [], priorQuestions)
    if (backend.length > 0) {
      return backend
    }
    return buildFallbackFollowUps({
      response: result,
      priorQuestions,
      t,
    })
  }, [priorQuestions, result, t])

  const [chartTypeOverride, setChartTypeOverride] = useResetStateOnDepsChange<
    'bar' | 'line' | 'pie' | 'table' | null
  >(null, [message.ai_response, conversationId])
  const [tableViewOverride, setTableViewOverride] = useResetStateOnDepsChange<
    'flat' | 'pivot' | null
  >(null, [message.ai_response, conversationId])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [sampleModalOpen, setSampleModalOpen] = useState(false)
  const [sampleModalTable, setSampleModalTable] = useState('')
  // Details start open only when there is no answer to show (hard failure),
  // so the diagnostics are immediately visible.
  const [showDetails, setShowDetails] = useState(
    () => !result?.result && !result?.sql && !result?.needs_clarification,
  )

  const pivotTable = useMemo(() => {
    const hint = result?.result?.pivot_hint
    const cols = result?.result?.columns
    const rows = result?.result?.rows
    if (!hint || !cols || !rows) {
      return null
    }
    return buildPivotTable(cols, rows, hint)
  }, [result?.result?.pivot_hint, result?.result?.columns, result?.result?.rows])

  const suggestedChartType = useMemo(
    () =>
      mapChartSuggestion(
        result?.visualization_hint?.chart_type ?? result?.result?.chart_suggestions?.[0],
      ),
    [result?.visualization_hint?.chart_type, result?.result?.chart_suggestions],
  )
  const chartType = chartTypeOverride ?? suggestedChartType ?? 'table'
  const tableView = tableViewOverride ?? (pivotTable ? 'pivot' : 'flat')

  if (!result) {
    return <AssistantPlainNote content={message.content} />
  }

  const handleUseCandidate = (i: number) => {
    const c = result.candidates?.[i]
    if (!c) {
      return
    }
    updateMessageResponse(conversationId, messageIndex, {
      ...result,
      logical_query: c.logical_query,
      confidence: c.confidence,
    })
  }

  const runQuery = async () => {
    if (!result.logical_query) {
      return
    }
    setLoading(true)
    setError(null)
    try {
      // Ensure datasource_id is set on the logical query before dispatching;
      // the AI response may omit it depending on the model.
      const queryBody = {
        ...result.logical_query,
        datasource_id: result.logical_query.datasource_id || datasourceId,
      }
      const res = await postData<QueryResultPayload>('/api/query/run', queryBody, {
        timeout: AI_QUERY_TIMEOUT_MS,
      })
      if (!res) {
        setError(t('ai_query.err_execute_query'))
        return
      }
      const mapped = mapChartSuggestion(res.chart_suggestions?.[0])
      if (mapped) {
        setChartTypeOverride(mapped)
      }
      updateMessageResponse(conversationId, messageIndex, { ...result, result: res })
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : t('ai_query.err_execution_failed'))
    } finally {
      setLoading(false)
    }
  }

  const submitFeedback = async (body: Record<string, unknown>) => {
    try {
      return await postData<{ status: string; learned?: boolean }>('/api/ai/feedback', body)
    } catch {
      return null
    }
  }

  const resultWithPayload = result.result
    ? (result as typeof result & { result: NonNullable<typeof result.result> })
    : null

  // Server-synthesized prose answer (falls back to the deterministic insight
  // caption) rendered left-aligned in the ✦ message column, above the card.
  const insightCaption = buildResultInsight(result.result, t, localeTag)

  return (
    <>
      <AssistantMessageAnswer answer={result.answer} caption={insightCaption} />
      <AssistantMessageCaveat caveat={result.caveat} />
      <div className={assistantCardClass}>
        <AssistantCardTop
          result={result}
          showDetails={showDetails}
          onToggle={() => setShowDetails((v) => !v)}
          t={t}
        />
        <AssistantMessageDetailsSection
          showDetails={showDetails}
          result={result}
          aiRuntime={aiRuntime}
          onSampleData={(tableName) => {
            setSampleModalTable(tableName)
            setSampleModalOpen(true)
          }}
          t={t}
          localeTag={localeTag}
        />
        <AssistantThinkingSteps result={result} t={t} />
        <AssistantMessageClarificationSections
          result={result}
          userQuestion={result.resolved_question ?? userQuestion}
          onSelectClarification={onSelectClarification}
          onSkipClarification={onSkipClarification}
          onUseCandidate={handleUseCandidate}
          t={t}
        />
        <AssistantRunPrompt
          result={result}
          loading={loading}
          onRunQuery={() => {
            void runQuery()
          }}
          t={t}
        />
        <ErrorAlert error={error} />
        {resultWithPayload && (
          <AssistantMessageResults
            result={resultWithPayload}
            chartType={chartType}
            setChartType={(value) => setChartTypeOverride(value)}
            tableView={tableView}
            setTableView={(value) => setTableViewOverride(value)}
            pivotTable={pivotTable}
            userQuestion={userQuestion}
            onFilterByValue={onFilterByValue}
            onCellDrillDown={onCellDrillDown}
            defaultOpen={isLatest}
            t={t}
            localeTag={localeTag}
          />
        )}
        <FollowUpSuggestions suggestions={followUps} onSelect={onSelectFollowUp} t={t} />
        {resultWithPayload && (
          <AnswerArtifactActions
            result={result}
            datasourceId={datasourceId}
            userQuestion={userQuestion}
            t={t}
          />
        )}
        <AssistantMessageFeedbackRow
          userQuestion={userQuestion}
          datasourceId={datasourceId}
          submitFeedback={submitFeedback}
          t={t}
        />
        <SampleDataModal
          open={sampleModalOpen}
          onClose={() => setSampleModalOpen(false)}
          tableName={sampleModalTable}
          datasourceId={datasourceId}
          get={get}
        />
      </div>
    </>
  )
}
