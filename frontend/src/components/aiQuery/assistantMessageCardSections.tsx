import type { AIQueryResponse, AIRuntimeSettings, SelectField } from '../../types/ai'
import { rowsToChartData } from '../../utils/chartData'
import type { PivotTableData } from '../../utils/pivotTable'
import { ResultTable } from '../ResultTable'
import { ChartContainer } from '../ui/ChartContainer'
import { ChartTypeSelector } from '../ui/ChartTypeSelector'
import { ErrorAlert } from '../ui/ErrorAlert'
import {
  CandidateComparisonPanel,
  ClarificationCard,
  Collapsible,
  ConfidenceBar,
  CostBadge,
  LogicalQueryMetaBadges,
  PromptStatsPanel,
  TableRoutingViz,
} from './routingViz'
import { warningBodyKey } from './routingVizUtils'
import type { AssistantMessageCardProps } from './types'

type AssistantT = AssistantMessageCardProps['t']

export function AssistantMessageHeader({
  result,
  aiRuntime,
  t,
}: {
  result: AIQueryResponse
  aiRuntime: AIRuntimeSettings | null | undefined
  t: AssistantT
}) {
  return (
    <>
      {result.confidence !== undefined && (
        <ConfidenceBar value={result.confidence} breakdown={result.confidence_breakdown} />
      )}
      <CostBadge
        latencyMs={result.latency_ms}
        tokenUsage={result.token_usage}
        costUsd={result.cost_usd}
      />
      <PromptStatsPanel stats={result.prompt_stats} tokenUsage={result.token_usage} />
      <ModelUsedBadge result={result} aiRuntime={aiRuntime} t={t} />
      {result.retry_count !== undefined && result.retry_count > 0 && (
        <div className="retry-badge">{t('ai_query.retry_badge', { n: result.retry_count })}</div>
      )}
    </>
  )
}

function ModelUsedBadge({
  result,
  aiRuntime,
  t,
}: {
  result: AIQueryResponse
  aiRuntime: AIRuntimeSettings | null | undefined
  t: AssistantT
}) {
  if (!result.model_used) {
    return null
  }
  const configuredQuery =
    aiRuntime?.query_model_override &&
    aiRuntime.query_model &&
    result.model_used !== aiRuntime.query_model
      ? aiRuntime.query_model
      : null
  const configuredLlm =
    !aiRuntime?.query_model_override &&
    aiRuntime?.llm_model &&
    result.model_used !== aiRuntime.llm_model
      ? aiRuntime.llm_model
      : null
  return (
    <div
      className="model-used-badge"
      style={{ fontSize: '0.8rem', color: 'var(--text-secondary)', marginBottom: '0.5rem' }}
    >
      {t('ai_query.model_used')} <code translate="no">{result.model_used}</code>
      {configuredQuery && (
        <span>
          {' '}
          ({t('ai_query.configured')} <code translate="no">{configuredQuery}</code>)
        </span>
      )}
      {configuredLlm && (
        <span>
          {' '}
          ({t('ai_query.configured')} <code translate="no">{configuredLlm}</code>)
        </span>
      )}
    </div>
  )
}

export function AssistantMessageClarificationSections({
  result,
  userQuestion,
  onSelectClarification,
  onSkipClarification,
  onUseCandidate,
  onSampleData,
  t,
}: {
  result: AIQueryResponse
  userQuestion: string
  onSelectClarification: AssistantMessageCardProps['onSelectClarification']
  onSkipClarification: AssistantMessageCardProps['onSkipClarification']
  onUseCandidate: (index: number) => void
  onSampleData: (tableName: string) => void
  t: AssistantT
}) {
  const clarificationOptions =
    result.clarification_options ?? result.clarification?.options?.map((o) => o.label) ?? []
  const showClarification =
    result.needs_clarification &&
    (clarificationOptions.length > 0 || result.clarification?.options?.length)

  return (
    <>
      {showClarification ? (
        <ClarificationCard
          question={
            result.clarification?.question ??
            result.clarification_question ??
            t('ai_query.clarify_default')
          }
          options={clarificationOptions}
          clarification={result.clarification}
          onSelect={(choice) => onSelectClarification(choice, userQuestion)}
          onSkip={() => onSkipClarification(userQuestion)}
        />
      ) : null}
      {result.candidates && result.candidates.length > 1 && !result.needs_clarification && (
        <CandidateComparisonPanel candidates={result.candidates} onUse={onUseCandidate} />
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
                if (firstSel) {
                  onSampleData(firstSel)
                }
              }}
            >
              {t('ai_query.sample_preview_btn')}
            </button>
          )}
        </Collapsible>
      )}
    </>
  )
}

function ValidationPlanSection({ result, t }: { result: AIQueryResponse; t: AssistantT }) {
  if (!result.validation_result) {
    return null
  }
  const planOk = result.validation_result.plan_ok
  return (
    <Collapsible
      title={planOk ? t('ai_query.plan_ok_title') : t('ai_query.plan_warn_title')}
      defaultOpen={!planOk}
    >
      {result.validation_result.explain_output && (
        <pre className="sql-preview explain-output">{result.validation_result.explain_output}</pre>
      )}
      <p className={`plan-status ${planOk ? 'plan-ok' : 'plan-warn'}`}>
        {planOk ? t('ai_query.plan_ok_body') : t('ai_query.plan_warn_body')}
      </p>
    </Collapsible>
  )
}

function WindowFieldBadges({ result, t }: { result: AIQueryResponse; t: AssistantT }) {
  const windowFields =
    result.logical_query?.select?.filter(
      (s): s is SelectField & { type: 'window' } => s.type === 'window',
    ) ?? []
  if (windowFields.length === 0) {
    return null
  }
  return (
    <div style={{ marginBottom: '0.5rem' }}>
      {windowFields.map((s, i) => (
        <span key={i} className="wf-badge">
          {t('ai_query.window_fn_badge', { name: s.window?.aggregation ?? s.name })}
        </span>
      ))}
    </div>
  )
}

function PromptCollapsible({
  result,
  t,
  localeTag,
}: {
  result: AIQueryResponse
  t: AssistantT
  localeTag: string
}) {
  if (!result.prompt) {
    return null
  }
  return (
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
          {t('ai_query.prompt_large_warning', {
            k: (result.token_usage.prompt / 1000).toFixed(1),
          })}
        </p>
      )}
    </Collapsible>
  )
}

function WarningsPanel({ result, t }: { result: AIQueryResponse; t: AssistantT }) {
  if (!result.warnings?.length) {
    return null
  }
  return (
    <section className="warning-panel" aria-live="polite">
      <div>
        <strong>{t('ai_query.warnings_title')}</strong>
        <p>{t(warningBodyKey(result))}</p>
      </div>
      <ul>
        {result.warnings.map((w, i) => (
          <li key={i}>{w}</li>
        ))}
      </ul>
    </section>
  )
}

export function AssistantMessageQueryDetails({
  result,
  t,
  localeTag,
}: {
  result: AIQueryResponse
  t: AssistantT
  localeTag: string
}) {
  return (
    <>
      <ValidationPlanSection result={result} t={t} />
      <WindowFieldBadges result={result} t={t} />
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
      <PromptCollapsible result={result} t={t} localeTag={localeTag} />
      <WarningsPanel result={result} t={t} />
      {result.retry_count !== undefined && result.retry_count >= 3 && !result.sql && (
        <div className="error-recovery">
          <p>{t('ai_query.recovery_failed', { n: result.retry_count })}</p>
        </div>
      )}
    </>
  )
}

export function AssistantMessageRunQuery({
  loading,
  onRunQuery,
  t,
}: {
  loading: boolean
  onRunQuery: () => void
  t: AssistantT
}) {
  return (
    <div className="btn-run-query-container">
      <button type="button" className="btn btn-primary" disabled={loading} onClick={onRunQuery}>
        {loading ? t('ai_query.loading_executing') : t('ai_query.btn_run_query')}
      </button>
    </div>
  )
}

function ResultsHeaderHints({
  result,
  pivotTable,
  tableView,
  setTableView,
  chartType,
  setChartType,
  t,
}: {
  result: AIQueryResponse & { result: NonNullable<AIQueryResponse['result']> }
  pivotTable: PivotTableData | null
  tableView: 'flat' | 'pivot'
  setTableView: (value: 'flat' | 'pivot') => void
  chartType: 'bar' | 'line' | 'pie' | 'table'
  setChartType: (value: 'bar' | 'line' | 'pie' | 'table') => void
  t: AssistantT
}) {
  return (
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
  )
}

export function AssistantMessageResults({
  result,
  chartType,
  setChartType,
  tableView,
  setTableView,
  pivotTable,
  userQuestion,
  onFilterByValue,
  onCellDrillDown,
  t,
}: {
  result: AIQueryResponse & { result: NonNullable<AIQueryResponse['result']> }
  chartType: 'bar' | 'line' | 'pie' | 'table'
  setChartType: (value: 'bar' | 'line' | 'pie' | 'table') => void
  tableView: 'flat' | 'pivot'
  setTableView: (value: 'flat' | 'pivot') => void
  pivotTable: PivotTableData | null
  userQuestion: string
  onFilterByValue: AssistantMessageCardProps['onFilterByValue']
  onCellDrillDown: AssistantMessageCardProps['onCellDrillDown']
  t: AssistantT
}) {
  const chartData = rowsToChartData(result.result.rows)
  const tableData = tableView === 'pivot' && pivotTable ? pivotTable : result.result

  return (
    <div className="results-section">
      <ResultsHeaderHints
        result={result}
        pivotTable={pivotTable}
        tableView={tableView}
        setTableView={setTableView}
        chartType={chartType}
        setChartType={setChartType}
        t={t}
      />
      {chartType !== 'table' && chartData.length > 0 && (
        <ChartContainer data={chartData} type={chartType} />
      )}
      {chartType === 'table' && (
        <ResultTable
          columns={tableData.columns}
          rows={tableData.rows}
          rowCount={tableData.rows.length}
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
      )}
    </div>
  )
}
