import { useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'

import { useT } from '../../i18n'
import type { QueryResultPayload } from '../../types/ai'
import { normalizeAIQueryResponse } from '../../utils/normalizeAIQueryResponse'
import { buildPivotTable } from '../../utils/pivotTable'
import { formatResultCell } from '../../utils/resultCellFormat'
import { ErrorAlert } from '../ui/ErrorAlert'
import { LoadingOverlay } from '../ui/LoadingOverlay'
import { Modal } from '../ui/Modal'
import {
  AssistantMessageClarificationSections,
  AssistantMessageHeader,
  AssistantMessageQueryDetails,
  AssistantMessageResults,
  AssistantMessageRunQuery,
} from './assistantMessageCardSections'
import { FeedbackSection } from './FeedbackSection'
import type { AssistantMessageCardProps, FeedbackCatKey, SampleData } from './types'
import { AI_QUERY_TIMEOUT_MS } from './types'

function SampleDataModal({
  open,
  onClose,
  tableName,
  datasourceId,
  get,
}: {
  open: boolean
  onClose: () => void
  tableName: string
  datasourceId: string
  get: <T>(url: string) => Promise<T | null>
}) {
  const t = useT()
  const [sample, setSample] = useState<SampleData | null>(null)
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    if (!open) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setSample(null)
      return
    }
    setLoading(true)
    const [schema, ...rest] = tableName.split('.')
    const tName = rest.length > 0 ? rest.join('.') : schema
    const url = `/api/datasources/${datasourceId}/tables/${schema ?? 'public'}/${tName}/sample`
    void get<SampleData>(url).then((data) => {
      setSample(data)
      setLoading(false)
    })
  }, [datasourceId, get, open, tableName])

  return (
    <Modal
      open={open}
      title={t('ai_query.sample_modal_title', { table: tableName })}
      onClose={onClose}
      labelledBy="sample-data-title"
    >
      <LoadingOverlay loading={loading} />
      {sample && (
        <div className="results-table-scroll">
          <table className="results-table">
            <thead>
              <tr>
                {sample.columns.map((c) => (
                  <th key={c.name}>{c.name}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {sample.rows.map((row, i) => (
                <tr key={i}>
                  {row.map((cell, j) => (
                    <td key={j}>{formatResultCell(cell, sample.columns[j]?.name ?? '', {})}</td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </Modal>
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
}: AssistantMessageCardProps) {
  const navigate = useNavigate()
  const result = normalizeAIQueryResponse(message.ai_response)

  const [chartType, setChartType] = useState<'bar' | 'line' | 'pie' | 'table'>('table')
  const [tableView, setTableView] = useState<'flat' | 'pivot'>('flat')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [sampleModalOpen, setSampleModalOpen] = useState(false)
  const [sampleModalTable, setSampleModalTable] = useState('')

  const pivotTable = useMemo(() => {
    const hint = result?.result?.pivot_hint
    const cols = result?.result?.columns
    const rows = result?.result?.rows
    if (!hint || !cols || !rows) {
      return null
    }
    return buildPivotTable(cols, rows, hint)
  }, [result?.result?.pivot_hint, result?.result?.columns, result?.result?.rows])

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setTableView(pivotTable ? 'pivot' : 'flat')
  }, [pivotTable, result?.logical_query?.model_id])

  useEffect(() => {
    const mapped = mapChartSuggestion(
      result?.visualization_hint?.chart_type ?? result?.result?.chart_suggestions?.[0],
    )
    if (mapped && mapped !== 'table') {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setChartType(mapped)
    } else if (mapped === 'table') {
      setChartType('table')
    }
  }, [result?.visualization_hint?.chart_type, result?.result?.chart_suggestions])

  if (!result) {
    return null
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
      const res = await postData<QueryResultPayload>('/api/query/run', result.logical_query, {
        timeout: AI_QUERY_TIMEOUT_MS,
      })
      if (!res) {
        setError(t('ai_query.err_execute_query'))
        return
      }
      const mapped = mapChartSuggestion(res.chart_suggestions?.[0])
      if (mapped) {
        setChartType(mapped)
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
      await postData('/api/ai/feedback', body)
    } catch {
      /* ignore */
    }
  }

  const handleSaveToLibrary = () => {
    if (!result.logical_query) {
      return
    }
    const params = new URLSearchParams({
      prefill: '1',
      question: userQuestion,
      logical_query: JSON.stringify(result.logical_query),
      datasource_id: datasourceId,
      model_id: String(result.logical_query.model_id || ''),
    })
    void navigate(`/saved?${params.toString()}`)
  }

  const resultWithPayload = result.result
    ? (result as typeof result & { result: NonNullable<typeof result.result> })
    : null

  return (
    <div className="assistant-card">
      <AssistantMessageHeader result={result} aiRuntime={aiRuntime} t={t} />
      <AssistantMessageClarificationSections
        result={result}
        userQuestion={userQuestion}
        onSelectClarification={onSelectClarification}
        onSkipClarification={onSkipClarification}
        onUseCandidate={handleUseCandidate}
        onSampleData={(tableName) => {
          setSampleModalTable(tableName)
          setSampleModalOpen(true)
        }}
        t={t}
      />
      <AssistantMessageQueryDetails result={result} t={t} localeTag={localeTag} />
      {!result.result && result.sql && (
        <AssistantMessageRunQuery
          loading={loading}
          onRunQuery={() => {
            void runQuery()
          }}
          t={t}
        />
      )}
      {error && <ErrorAlert error={error} />}
      {resultWithPayload && (
        <AssistantMessageResults
          result={resultWithPayload}
          chartType={chartType}
          setChartType={setChartType}
          tableView={tableView}
          setTableView={setTableView}
          pivotTable={pivotTable}
          userQuestion={userQuestion}
          onFilterByValue={onFilterByValue}
          onCellDrillDown={onCellDrillDown}
          t={t}
        />
      )}
      <div className="feedback-row-wrapper">
        <FeedbackSection
          onSubmitPositive={() => {
            void submitFeedback({
              question: userQuestion,
              datasource_id: datasourceId,
              rating: 'positive',
            })
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
        {result.logical_query && (
          <button
            type="button"
            className="btn btn-sm btn-ghost"
            style={{
              marginLeft: 'auto',
              display: 'inline-flex',
              alignItems: 'center',
              gap: '0.25rem',
            }}
            onClick={handleSaveToLibrary}
            title={t('saved_questions.new')}
          >
            💾 {t('saved_questions.new')}
          </button>
        )}
      </div>
      <SampleDataModal
        open={sampleModalOpen}
        onClose={() => setSampleModalOpen(false)}
        tableName={sampleModalTable}
        datasourceId={datasourceId}
        get={get}
      />
    </div>
  )
}
