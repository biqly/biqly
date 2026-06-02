import { useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useT } from '../../i18n'
import { formatResultCell } from '../../utils/resultCellFormat'
import { buildPivotTable } from '../../utils/pivotTable'
import { rowsToChartData } from '../../utils/chartData'
import { normalizeAIQueryResponse } from '../../utils/normalizeAIQueryResponse'
import { ResultTable } from '../ResultTable'
import { ChartContainer } from '../ui/ChartContainer'
import { ChartTypeSelector } from '../ui/ChartTypeSelector'
import { ErrorAlert } from '../ui/ErrorAlert'
import { LoadingOverlay } from '../ui/LoadingOverlay'
import { Modal } from '../ui/Modal'
import { Collapsible, ConfidenceBar, CostBadge, PromptStatsPanel, LogicalQueryMetaBadges, CandidateComparisonPanel, ClarificationCard, TableRoutingViz, warningBodyKey } from './routingViz'
import { FeedbackSection } from './FeedbackSection'
import type { AssistantMessageCardProps, SampleData, FeedbackCatKey } from './types'
import { AI_QUERY_TIMEOUT_MS } from './types'

function SampleDataModal({ open, onClose, tableName, datasourceId, get }: { open: boolean; onClose: () => void; tableName: string; datasourceId: string; get: <T>(url: string) => Promise<T | null> }) {
  const t = useT()
  const [sample, setSample] = useState<SampleData | null>(null)
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    if (!open) { setSample(null); return }
    setLoading(true)
    const [schema, ...rest] = tableName.split('.')
    const tName = rest.length > 0 ? rest.join('.') : schema
    const url = `/api/datasources/${datasourceId}/tables/${schema ?? 'public'}/${tName}/sample`
    get<SampleData>(url).then((data) => { setSample(data); setLoading(false) })
  }, [datasourceId, get, open, tableName])

  return (
    <Modal open={open} title={t('ai_query.sample_modal_title', { table: tableName })} onClose={onClose} labelledBy="sample-data-title">
      <LoadingOverlay loading={loading} />
      {sample?.columns && sample?.rows && (
        <div className="results-table-scroll">
          <table className="results-table">
            <thead><tr>{sample.columns.map((c) => <th key={c.name}>{c.name}</th>)}</tr></thead>
            <tbody>
              {sample.rows.map((row, i) => (
                <tr key={i}>{row.map((cell, j) => <td key={j}>{formatResultCell(cell, sample.columns[j]?.name ?? '', {})}</td>)}</tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </Modal>
  )
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
  localeNumberTag: _lnTag,
  localeTag,
  onSelectClarification,
  onSkipClarification,
  onFilterByValue,
  onCellDrillDown,
}: AssistantMessageCardProps) {
  const navigate = useNavigate()
  const result = normalizeAIQueryResponse(message.ai_response)
  if (!result) return null

  const [chartType, setChartType] = useState<'bar' | 'line' | 'pie' | 'table'>('table')
  const [tableView, setTableView] = useState<'flat' | 'pivot'>('flat')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [sampleModalOpen, setSampleModalOpen] = useState(false)
  const [sampleModalTable, setSampleModalTable] = useState('')

  const pivotTable = useMemo(() => {
    const hint = result.result?.pivot_hint
    const cols = result.result?.columns
    const rows = result.result?.rows
    if (!hint || !cols || !rows) return null
    return buildPivotTable(cols, rows, hint)
  }, [result.result?.pivot_hint, result.result?.columns, result.result?.rows])

  useEffect(() => {
    if (pivotTable) setTableView('pivot')
    else setTableView('flat')
  }, [pivotTable, result.logical_query?.model_id])

  useEffect(() => {
    const raw = result.visualization_hint?.chart_type ?? result.result?.chart_suggestions?.[0]
    if (!raw) return
    const mapped = raw === 'number' ? 'table' : raw
    if (mapped === 'bar' || mapped === 'line' || mapped === 'pie' || mapped === 'table') {
      setChartType(mapped)
    }
  }, [result.visualization_hint?.chart_type, result.result?.chart_suggestions])

  const handleUseCandidate = (i: number) => {
    const c = result.candidates?.[i]
    if (c) {
      updateMessageResponse(conversationId, messageIndex, {
        ...result,
        logical_query: c.logical_query,
        confidence: c.confidence,
      })
    }
  }

  const runQuery = async () => {
    if (!result.logical_query) return
    setLoading(true)
    setError(null)
    try {
      const res = await postData<any>('/api/query/run', result.logical_query, { timeout: AI_QUERY_TIMEOUT_MS })
      if (res) {
        const updated = {
          ...result,
          result: res,
        }
        if (res.chart_suggestions && res.chart_suggestions.length > 0) {
          const raw = res.chart_suggestions[0]
          const mapped = raw === 'number' ? 'table' : raw
          if (mapped === 'bar' || mapped === 'line' || mapped === 'pie' || mapped === 'table') {
            setChartType(mapped)
          }
        }
        updateMessageResponse(conversationId, messageIndex, updated)
      } else {
        setError(t('ai_query.err_execute_query'))
      }
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : t('ai_query.err_execution_failed'))
    } finally {
      setLoading(false)
    }
  }

  const handleSampleData = (tableName: string) => {
    setSampleModalTable(tableName)
    setSampleModalOpen(true)
  }

  const submitPositiveFeedback = async () => {
    try {
      await postData('/api/ai/feedback', { question: userQuestion, datasource_id: datasourceId, rating: 'positive' })
    } catch { /* noop */ }
  }

  const submitNegativeFeedback = async (categories: FeedbackCatKey[], text: string) => {
    try {
      await postData('/api/ai/feedback', {
        question: userQuestion,
        datasource_id: datasourceId,
        rating: 'negative',
        categories: categories.map((k) => t(k)),
        text,
      })
    } catch { /* noop */ }
  }

  const handleSaveToLibrary = () => {
    if (!result.logical_query) return
    const lqStr = JSON.stringify(result.logical_query)
    const dsId = datasourceId
    const modelId = result.logical_query.model_id || ''
    const q = userQuestion

    const params = new URLSearchParams()
    params.set('prefill', '1')
    params.set('question', q)
    params.set('logical_query', lqStr)
    params.set('datasource_id', dsId)
    params.set('model_id', String(modelId))

    const path = `/saved?${params.toString()}`
    navigate(path)
  }

  const chartData = useMemo(() => rowsToChartData(result.result?.rows), [result.result?.rows])

  return (
    <div className="assistant-card">
      {result.confidence !== undefined && <ConfidenceBar value={result.confidence} breakdown={result.confidence_breakdown} />}
      <CostBadge latencyMs={result.latency_ms} tokenUsage={result.token_usage} costUsd={result.cost_usd} />
      <PromptStatsPanel stats={result.prompt_stats} tokenUsage={result.token_usage} />

      {result.model_used && (
        <div className="model-used-badge" style={{ fontSize: '0.8rem', color: 'var(--text-secondary)', marginBottom: '0.5rem' }}>
          {t('ai_query.model_used')} <code translate="no">{result.model_used}</code>
          {aiRuntime?.query_model_override && aiRuntime.query_model && result.model_used !== aiRuntime.query_model && (
            <span> ({t('ai_query.configured')} <code translate="no">{aiRuntime.query_model}</code>)</span>
          )}
          {!aiRuntime?.query_model_override && aiRuntime?.llm_model && result.model_used !== aiRuntime.llm_model && (
            <span> ({t('ai_query.configured')} <code translate="no">{aiRuntime.llm_model}</code>)</span>
          )}
        </div>
      )}

      {result.retry_count !== undefined && result.retry_count > 0 && (
        <div className="retry-badge">{t('ai_query.retry_badge', { n: result.retry_count })}</div>
      )}

      {result.needs_clarification && (result.clarification_options?.length || result.clarification?.options?.length) ? (
        <ClarificationCard
          question={result.clarification?.question ?? result.clarification_question ?? t('ai_query.clarify_default')}
          options={result.clarification_options ?? result.clarification?.options?.map((o) => o.label) ?? []}
          clarification={result.clarification}
          onSelect={(choice) => onSelectClarification(choice, userQuestion)}
          onSkip={() => onSkipClarification(userQuestion)}
        />
      ) : null}

      {result.candidates && result.candidates.length > 1 && !result.needs_clarification && (
        <CandidateComparisonPanel candidates={result.candidates} onUse={handleUseCandidate} />
      )}

      {result.table_routing && (
        <Collapsible title={t('ai_query.collapsible_routing')} defaultOpen>
          <TableRoutingViz routing={result.table_routing} />
          {(result.table_routing.selected_tables?.length ?? 0) > 0 && (
            <button
              type="button"
              className="btn btn-sm btn-sample"
              onClick={() => {
                const firstSel = result.table_routing?.selected_tables?.[0]
                if (firstSel) handleSampleData(firstSel)
              }}
            >
              {t('ai_query.sample_preview_btn')}
            </button>
          )}
        </Collapsible>
      )}

      {result.validation_result && (
        <Collapsible
          title={result.validation_result.plan_ok ? t('ai_query.plan_ok_title') : t('ai_query.plan_warn_title')}
          defaultOpen={!result.validation_result.plan_ok}
        >
          {result.validation_result.explain_output && <pre className="sql-preview explain-output">{result.validation_result.explain_output}</pre>}
          <p className={`plan-status ${result.validation_result.plan_ok ? 'plan-ok' : 'plan-warn'}`}>
            {result.validation_result.plan_ok ? t('ai_query.plan_ok_body') : t('ai_query.plan_warn_body')}
          </p>
        </Collapsible>
      )}

      {(result.logical_query?.select?.filter((s): s is import('../../types/ai').SelectField & { type: 'window' } => s.type === 'window') ?? []).length > 0 && (
        <div style={{ marginBottom: '0.5rem' }}>
          {(result.logical_query?.select ?? []).filter((s): s is import('../../types/ai').SelectField & { type: 'window' } => s.type === 'window').map((s, i) => (
            <span key={i} className="wf-badge">{t('ai_query.window_fn_badge', { name: s.window?.aggregation || s.name })}</span>
          ))}
        </div>
      )}

      {result.logical_query && (
        <Collapsible title={t('ai_query.collapsible_lq')} defaultOpen>
          <LogicalQueryMetaBadges lq={result.logical_query} />
          <pre className="sql-preview">{JSON.stringify(result.logical_query, null, 2)}</pre>
        </Collapsible>
      )}

      {result.sql && (
        <Collapsible title={t('ai_query.collapsible_sql')} defaultOpen>
          <pre className="sql-preview">{result.sql}</pre>
        </Collapsible>
      )}

      {result.prompt && (
        <Collapsible title={t('ai_query.collapsible_prompt')}>
          <pre className="sql-preview prompt-preview">{result.prompt}</pre>
          {result.token_usage && (
            <p className="token-info">
              {t('ai_query.token_line', {
                prompt: result.token_usage.prompt.toLocaleString(localeTag),
                completion: result.token_usage.completion.toLocaleString(localeTag),
                total: result.token_usage.total.toLocaleString(localeTag),
              })}
            </p>
          )}
          {result.token_usage && result.token_usage.prompt > 30000 && (
            <p className="prompt-warning">
              {t('ai_query.prompt_large_warning', { k: (result.token_usage.prompt / 1000).toFixed(1) })}
            </p>
          )}
        </Collapsible>
      )}

      {result.warnings && result.warnings.length > 0 && (
        <section className="warning-panel" aria-live="polite">
          <div>
            <strong>{t('ai_query.warnings_title')}</strong>
            <p>{t(warningBodyKey(result))}</p>
          </div>
          <ul>
            {result.warnings.map((w, i) => <li key={i}>{w}</li>)}
          </ul>
        </section>
      )}

      {result.retry_count !== undefined && result.retry_count >= 3 && !result.sql && (
        <div className="error-recovery">
          <p>{t('ai_query.recovery_failed', { n: result.retry_count })}</p>
        </div>
      )}

      {!result.result && result.sql && (
        <div className="btn-run-query-container">
          <button
            type="button"
            className="btn btn-primary"
            disabled={loading}
            onClick={runQuery}
          >
            {loading ? t('ai_query.loading_executing') : t('ai_query.btn_run_query')}
          </button>
        </div>
      )}
      {error && <ErrorAlert error={error} />}

      {result.result?.columns && result.result.rows && (
        <div className="results-section">
          <div className="results-header">
            <h3>{t('ai_query.results_title', { rows: result.result.stats?.row_count ?? 0 })}</h3>
            {result.visualization_hint && (
              <span className="viz-hint" title={result.visualization_hint.reason}>
                💡 {result.visualization_hint.chart_type}
              </span>
            )}
            {result.result.pivot_hint && (
              <span className="viz-hint" title={result.result.pivot_hint.reason ?? ''}>
                ↕ {result.result.pivot_hint.row_field} × {result.result.pivot_hint.column_field}
              </span>
            )}
            {(result.result.anomalies?.length ?? 0) > 0 && (
              <span className="viz-hint" title={t('ai_query.anomalies_title')}>
                {t('ai_query.anomalies_badge', { count: result.result.anomalies!.length })}
              </span>
            )}
            {pivotTable && (
              <div className="chart-toggle">
                <button
                  type="button"
                  className={tableView === 'flat' ? 'active' : ''}
                  onClick={() => setTableView('flat')}
                >
                  {t('ai_query.pivot_flat')}
                </button>
                <button
                  type="button"
                  className={tableView === 'pivot' ? 'active' : ''}
                  onClick={() => setTableView('pivot')}
                >
                  {t('ai_query.pivot_pivot')}
                </button>
              </div>
            )}
            <ChartTypeSelector
              value={chartType}
              onChange={setChartType}
              options={['bar', 'line', 'pie', 'table'] as const}
              ariaLabel={t('ai_query.chart_type_aria')}
              labels={{
                bar: t('ai_query.chart_bar'),
                line: t('ai_query.chart_line'),
                pie: t('ai_query.chart_pie'),
                table: t('ai_query.chart_table'),
              }}
            />
          </div>

          {chartType !== 'table' && chartData.length > 0 && (
            <ChartContainer data={chartData} type={chartType} />
          )}

          {chartType === 'table' && (() => {
            const flat = {
              columns: result.result.columns,
              rows: result.result.rows,
            }
            const view =
              tableView === 'pivot' && pivotTable
                ? pivotTable
                : flat
            return (
              <ResultTable
                columns={view.columns}
                rows={view.rows}
                rowCount={view.rows.length}
                durationMs={result.result.stats?.duration_ms}
                question={userQuestion}
                anomalies={tableView === 'flat' ? result.result.anomalies : undefined}
                onFilterByValue={tableView === 'flat' ? onFilterByValue : undefined}
                onCellClick={
                  tableView === 'flat'
                    ? (colName, value) => onCellDrillDown(colName, String(value))
                    : undefined
                }
              />
            )
          })()}
        </div>
      )}

      <div className="feedback-row-wrapper">
        <FeedbackSection
          onSubmitPositive={submitPositiveFeedback}
          onSubmitNegative={submitNegativeFeedback}
        />
        {result.logical_query && (
          <button
            type="button"
            className="btn btn-sm btn-ghost"
            style={{ marginLeft: 'auto', display: 'inline-flex', alignItems: 'center', gap: '0.25rem' }}
            onClick={handleSaveToLibrary}
            title={t('saved_questions.new')}
          >
            💾 {t('saved_questions.new')}
          </button>
        )}
      </div>

      <SampleDataModal open={sampleModalOpen} onClose={() => setSampleModalOpen(false)} tableName={sampleModalTable} datasourceId={datasourceId} get={get} />
    </div>
  )
}
