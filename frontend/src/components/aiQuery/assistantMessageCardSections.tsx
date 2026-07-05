import { promptWarningClass, wfBadgeClass } from '../../lib/badgeClasses'
import { buttonClass } from '../../lib/buttonClasses'
import { cn } from '../../lib/cn'
import {
  sqlPreviewClass,
  warningPanelClass,
  warningPanelLiClass,
  warningPanelListClass,
  warningPanelPClass,
  warningPanelStrongClass,
} from '../../lib/feedbackClasses'
import type { AIQueryResponse, AIRuntimeSettings, SelectField } from '../../types/ai'
import { rowsToChartData } from '../../utils/chartData'
import type { PivotTableData } from '../../utils/pivotTable'
import { ResultTable } from '../ResultTable'
import { ChartContainer } from '../ui/ChartContainer'
import { ChartTypeSelector } from '../ui/ChartTypeSelector'
import {
  assistantAnswerCaptionClass,
  assistantAnswerCaretClass,
  assistantAnswerClass,
  assistantConfidenceClass,
  assistantSummaryClass,
  btnRunQueryContainerClass,
  chartToggleBtnClass,
  chartToggleClass,
  errorRecoveryClass,
  errorRecoveryPClass,
  resultsHeaderClass,
  resultsSectionClass,
  retryBadgeClass,
  vizHintClass,
} from './aiQueryClasses'
import { deriveClarificationStage, MAX_CLARIFICATION_ROUNDS } from './clarificationStage'
import { GenerationTracePanel } from './generationTrace'
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
import { RunTracePanel } from './RunTrace'
import type { AssistantMessageCardProps } from './types'
import { useTypewriter } from './useTypewriter'

type AssistantT = AssistantMessageCardProps['t']

function confidenceLevel(value: number): 'high' | 'mid' | 'low' {
  if (value >= 0.7) {
    return 'high'
  }
  return value >= 0.4 ? 'mid' : 'low'
}

// Compact one-line summary shown on every response; the rest lives behind
// the details toggle.
export function AssistantMessageSummary({ result, t }: { result: AIQueryResponse; t: AssistantT }) {
  return (
    <div className={assistantSummaryClass}>
      {result.confidence !== undefined && (
        <span className={assistantConfidenceClass(confidenceLevel(result.confidence))}>
          {t('ai_query.summary_confidence', {
            pct: Math.round(result.confidence * 100),
          })}
        </span>
      )}
      <CostBadge
        latencyMs={result.latency_ms}
        tokenUsage={result.token_usage}
        costUsd={result.cost_usd}
      />
      {result.retry_count !== undefined && result.retry_count > 0 && (
        <span className={`${retryBadgeClass} mb-0!`}>
          {t('ai_query.retry_badge', { n: result.retry_count })}
        </span>
      )}
    </div>
  )
}

// Left-aligned assistant prose shown under the ✦ avatar/header and above the
// result card. Prefers the server-synthesized answer (already in the user's
// locale) and streams it in like a typewriter; falls back to the deterministic
// insight caption (muted) and renders nothing when both are empty.
export function AssistantMessageAnswer({
  answer,
  caption,
}: {
  answer?: string
  caption?: string | null
}) {
  const trimmedAnswer = answer?.trim() ?? ''
  const trimmedCaption = caption?.trim() ?? ''
  const isCaption = trimmedAnswer === '' && trimmedCaption !== ''
  const text = trimmedAnswer || trimmedCaption
  const { shown, done } = useTypewriter(text)
  if (!text) {
    return null
  }
  return (
    <p
      className={isCaption ? assistantAnswerCaptionClass : assistantAnswerClass}
      aria-live="polite"
    >
      {/* Animated glyphs are hidden from assistive tech; the full text is
          exposed once via an sr-only node so screen readers aren't fed one
          character at a time. */}
      <span aria-hidden="true">{shown}</span>
      {!done && (
        <span aria-hidden="true" className={assistantAnswerCaretClass}>
          ▍
        </span>
      )}
      <span className="sr-only">{text}</span>
    </p>
  )
}

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
      <PromptStatsPanel stats={result.prompt_stats} tokenUsage={result.token_usage} />
      <ModelUsedBadge result={result} aiRuntime={aiRuntime} t={t} />
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
  t,
}: {
  result: AIQueryResponse
  userQuestion: string
  onSelectClarification: AssistantMessageCardProps['onSelectClarification']
  onSkipClarification: AssistantMessageCardProps['onSkipClarification']
  onUseCandidate: (index: number) => void
  t: AssistantT
}) {
  const clarificationOptions =
    result.clarification_options ?? result.clarification?.options?.map((o) => o.label) ?? []
  const showClarification =
    result.needs_clarification &&
    (clarificationOptions.length > 0 || result.clarification?.options?.length)
  const stage = deriveClarificationStage(result.clarification_round)

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
          generationTrace={result.generation_trace}
          interactiveTier={stage.interactiveTier}
          capReached={stage.capReached}
          round={stage.displayRound}
          maxRounds={MAX_CLARIFICATION_ROUNDS}
          onSelect={(choice) => onSelectClarification(choice, userQuestion)}
          onSkip={() => onSkipClarification(userQuestion)}
        />
      ) : null}
      {result.candidates && result.candidates.length > 1 && !result.needs_clarification && (
        <CandidateComparisonPanel candidates={result.candidates} onUse={onUseCandidate} />
      )}
    </>
  )
}

function AssistantTableRoutingSection({
  result,
  onSampleData,
  t,
}: {
  result: AIQueryResponse
  onSampleData: (tableName: string) => void
  t: AssistantT
}) {
  if (!result.table_routing) {
    return null
  }
  return (
    <Collapsible title={t('ai_query.collapsible_routing')} defaultOpen>
      <TableRoutingViz routing={result.table_routing} />
      {(result.table_routing.selected_tables?.length ?? 0) > 0 && (
        <button
          type="button"
          className={buttonClass('secondary', { className: 'btn-sample', size: 'sm' })}
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
        <pre className={cn(sqlPreviewClass, 'explain-output')}>
          {result.validation_result.explain_output}
        </pre>
      )}
      <p className={cn('plan-status', planOk ? 'plan-ok' : 'plan-warn')}>
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
        <span key={i} className={wfBadgeClass}>
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
      <pre className={cn(sqlPreviewClass, 'prompt-preview')}>{result.prompt}</pre>
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
        <p className={promptWarningClass}>
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
    <section className={warningPanelClass} aria-live="polite">
      <div>
        <strong className={warningPanelStrongClass}>{t('ai_query.warnings_title')}</strong>
        <p className={warningPanelPClass}>{t(warningBodyKey(result))}</p>
      </div>
      <ul className={warningPanelListClass}>
        {result.warnings.map((w, i) => (
          <li key={i} className={warningPanelLiClass}>
            {w}
          </li>
        ))}
      </ul>
    </section>
  )
}

export function AssistantMessageQueryDetails({
  result,
  onSampleData,
  t,
  localeTag,
}: {
  result: AIQueryResponse
  onSampleData: (tableName: string) => void
  t: AssistantT
  localeTag: string
}) {
  return (
    <>
      {result.generation_trace && !result.needs_clarification ? (
        <GenerationTracePanel trace={result.generation_trace} />
      ) : null}
      {result.run_steps?.length && !result.needs_clarification ? (
        <RunTracePanel steps={result.run_steps} />
      ) : null}
      <AssistantTableRoutingSection result={result} onSampleData={onSampleData} t={t} />
      <ValidationPlanSection result={result} t={t} />
      <WindowFieldBadges result={result} t={t} />
      {result.logical_query && (
        <Collapsible title={t('ai_query.collapsible_lq')} defaultOpen>
          <LogicalQueryMetaBadges lq={result.logical_query} />
          <pre className={sqlPreviewClass}>{JSON.stringify(result.logical_query, null, 2)}</pre>
        </Collapsible>
      )}
      {result.sql && (
        <Collapsible title={t('ai_query.collapsible_sql')} defaultOpen>
          <pre className={sqlPreviewClass}>{result.sql}</pre>
        </Collapsible>
      )}
      <PromptCollapsible result={result} t={t} localeTag={localeTag} />
      <WarningsPanel result={result} t={t} />
      {result.retry_count !== undefined && result.retry_count >= 3 && !result.sql && (
        <div className={errorRecoveryClass}>
          <p className={errorRecoveryPClass}>
            {t('ai_query.recovery_failed', { n: result.retry_count })}
          </p>
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
    <div className={btnRunQueryContainerClass}>
      <button
        type="button"
        className={buttonClass('primary')}
        disabled={loading}
        onClick={onRunQuery}
      >
        {loading ? t('ai_query.loading_thinking') : t('ai_query.btn_run_query')}
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
    <div className={resultsHeaderClass}>
      <h3>{t('ai_query.results_title', { rows: result.result.stats?.row_count ?? 0 })}</h3>
      {result.visualization_hint && (
        <span className={vizHintClass} title={result.visualization_hint.reason}>
          💡 {result.visualization_hint.chart_type}
        </span>
      )}
      {result.result.pivot_hint && (
        <span className={vizHintClass} title={result.result.pivot_hint.reason ?? ''}>
          ↕ {result.result.pivot_hint.row_field} × {result.result.pivot_hint.column_field}
        </span>
      )}
      {(result.result.anomalies?.length ?? 0) > 0 && (
        <span className={vizHintClass} title={t('ai_query.anomalies_title')}>
          {t('ai_query.anomalies_badge', { count: result.result.anomalies!.length })}
        </span>
      )}
      {pivotTable && (
        <div className={chartToggleClass}>
          <button
            type="button"
            className={chartToggleBtnClass(tableView === 'flat')}
            onClick={() => setTableView('flat')}
          >
            {t('ai_query.pivot_flat')}
          </button>
          <button
            type="button"
            className={chartToggleBtnClass(tableView === 'pivot')}
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
    <div className={resultsSectionClass}>
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
