import { useContext, useEffect, useState } from 'react'

import { getAgentRun } from '../../api/agentRuns'
import type { TFunction, TranslationKey } from '../../i18n'
import { I18nContext } from '../../i18n/context'
import type { RunStep } from '../../types/ai'
import { Collapsible } from './routingViz'

const identityT: TFunction = (key) => key

const STEP_LABEL_KEYS: Record<string, TranslationKey> = {
  table_route: 'ai_query.run_trace_step_table_route',
  context_resolve: 'ai_query.run_trace_step_context_resolve',
  ambiguity: 'ai_query.run_trace_step_ambiguity',
  prompt_build: 'ai_query.run_trace_step_prompt_build',
  multi_candidate: 'ai_query.run_trace_step_multi_candidate',
  llm_generate: 'ai_query.run_trace_step_llm_generate',
  llm_cache_hit: 'ai_query.run_trace_step_llm_cache_hit',
  parse_validate: 'ai_query.run_trace_step_parse_validate',
  sql_dry_run: 'ai_query.run_trace_step_sql_dry_run',
  // Agentic query runner step kinds (Agentic Runtime).
  planner: 'ai_query.run_trace_step_planner',
  policy: 'ai_query.run_trace_step_policy',
  tool: 'ai_query.run_trace_step_tool',
  observation: 'ai_query.run_trace_step_observation',
  clarification: 'ai_query.run_trace_step_clarification',
  final_response: 'ai_query.run_trace_step_final_response',
  shadow_comparison: 'ai_query.run_trace_step_shadow_comparison',
  // Web agent tool steps (T6/T11: the MCP-parity 6-tool set — see
  // internal/toolcontract.ToolName / internal/agent's ToolWeb* constants).
  // Both the live SSE trace (agentTraceSteps.ts remaps `kind` to the tool
  // name before it reaches this panel) and the persisted/reloaded trace
  // (webAgentRunSteps sets RunStep.Kind to the same raw tool name
  // server-side) use these exact strings, so one entry per tool covers both
  // paths.
  list_datasources: 'ai_query.run_trace_step_list_datasources',
  list_models: 'ai_query.run_trace_step_list_models',
  run_question: 'ai_query.run_trace_step_run_question',
  run_logical_query: 'ai_query.run_trace_step_run_logical_query',
  list_skills: 'ai_query.run_trace_step_list_skills',
  run_skill: 'ai_query.run_trace_step_run_skill',
}

// REASON_CODE_LABEL_KEYS maps the agent runtime's bounded reason-code
// vocabulary (policy denials, terminal failures — see internal/agent's
// Reason* constants and Failure.ReasonCode) to a human label, so the trace
// never has to render a raw policy/tool argument to explain what happened.
const REASON_CODE_LABEL_KEYS: Record<string, TranslationKey> = {
  tool_not_allowlisted: 'ai_query.run_trace_reason_tool_not_allowlisted',
  retry_budget_exhausted: 'ai_query.run_trace_reason_retry_budget_exhausted',
  airgapped_egress_denied: 'ai_query.run_trace_reason_airgapped_egress_denied',
  malformed_arguments: 'ai_query.run_trace_reason_malformed_arguments',
  identity_mismatch: 'ai_query.run_trace_reason_identity_mismatch',
  prompt_injection_suspected: 'ai_query.run_trace_reason_prompt_injection_suspected',
  multi_statement_sql_denied: 'ai_query.run_trace_reason_multi_statement_sql_denied',
  write_or_ddl_denied: 'ai_query.run_trace_reason_write_or_ddl_denied',
  hidden_column_denied: 'ai_query.run_trace_reason_hidden_column_denied',
  pii_masking_required: 'ai_query.run_trace_reason_pii_masking_required',
  invalid_join_denied: 'ai_query.run_trace_reason_invalid_join_denied',
  row_filter_required: 'ai_query.run_trace_reason_row_filter_required',
  context_canceled: 'ai_query.run_trace_reason_context_canceled',
  planner_error: 'ai_query.run_trace_reason_planner_error',
  tool_error: 'ai_query.run_trace_reason_tool_error',
  timeout: 'ai_query.run_trace_reason_timeout',
  max_steps_exceeded: 'ai_query.run_trace_reason_max_steps_exceeded',
  max_clarification_rounds_exceeded: 'ai_query.run_trace_reason_max_clarification_rounds_exceeded',
  invalid_decision_kind: 'ai_query.run_trace_reason_invalid_decision_kind',
}

/** Caps how much of a step's free-text detail is ever rendered — a defensive
 * bound, independent of the server already keeping this field short, so the
 * trace can never grow into a place to dump large or sensitive payloads. */
const MAX_DETAIL_CHARS = 300

function truncateDetail(detail: string): string {
  return detail.length > MAX_DETAIL_CHARS ? detail.slice(0, MAX_DETAIL_CHARS) + '…' : detail
}

/** Renders a step's detail: a known reason code becomes its human label;
 * anything else is shown verbatim (truncated) rather than reinterpreted. */
function renderDetail(t: TFunction, detail: string): string {
  const reasonKey = REASON_CODE_LABEL_KEYS[detail]
  return reasonKey ? t(reasonKey) : truncateDetail(detail)
}

/** A step is a terminal outcome (success or failure) of the run itself,
 * styled distinctly from an intermediate planner/tool/policy step. */
function isTerminalStepKind(kind: string): boolean {
  return kind === 'final_response' || kind === 'fail'
}

export function RunTracePanel({
  steps,
  runId,
  defaultOpen = false,
  t: tProp,
}: {
  steps: RunStep[]
  /** When set and no in-request steps are provided (e.g. a reloaded thread),
   * the panel re-hydrates the timeline from the persisted run. */
  runId?: string
  defaultOpen?: boolean
  /** Overrides the useT() hook — for tests that render outside an
   * <I18nProvider>. Production callers should never pass this. */
  t?: TFunction
}) {
  // useContext (not useT()) so this component renders outside an
  // <I18nProvider> when a caller supplies t explicitly (e.g. tests).
  // useT() throws in that case; useContext never does.
  const ctx = useContext(I18nContext)
  const t = tProp ?? ctx?.t ?? identityT
  const [hydrated, setHydrated] = useState<RunStep[] | null>(null)

  useEffect(() => {
    if (steps.length > 0 || !runId) {
      return
    }
    let cancelled = false
    void getAgentRun(runId)
      .then((detail) => {
        if (!cancelled) {
          setHydrated(detail.steps)
        }
      })
      .catch(() => {
        // Best-effort: the persisted trace is a convenience, not required.
      })
    return () => {
      cancelled = true
    }
  }, [steps.length, runId])

  const effectiveSteps = steps.length > 0 ? steps : (hydrated ?? [])
  if (effectiveSteps.length === 0) {
    return null
  }
  return (
    <Collapsible title={t('ai_query.run_trace_title')} defaultOpen={defaultOpen}>
      <ol className="text-foreground-muted m-0 mt-3 flex list-none flex-col gap-0 p-0 text-[0.88rem]">
        {effectiveSteps.map((step) => {
          const labelKey = STEP_LABEL_KEYS[step.kind]
          const failed = step.status === 'failed'
          const running = step.status === 'running'
          const cancelled = failed && step.detail === 'context_canceled'
          const terminal = isTerminalStepKind(step.kind)
          return (
            <li
              key={step.seq}
              className="border-border relative flex flex-wrap items-baseline gap-x-2 gap-y-1 border-l-2 pb-2.5 pl-4 last:pb-0"
            >
              <span
                aria-hidden="true"
                className={`absolute top-[0.3rem] left-[-0.32rem] h-2.5 w-2.5 rounded-full ${
                  failed ? 'bg-error' : running ? 'bg-accent animate-pulse' : 'bg-success'
                }`}
              />
              <span className={`text-foreground font-semibold ${terminal ? 'underline' : ''}`}>
                {labelKey ? t(labelKey) : step.kind}
              </span>
              {step.attempt != null && step.attempt > 0 ? (
                <span className="text-foreground-faint text-[0.78rem]">
                  {t('ai_query.run_trace_attempt', { n: step.attempt })}
                </span>
              ) : null}
              {running ? (
                <span className="text-foreground-faint text-[0.78rem]">
                  {t('ai_query.run_trace_running')}
                </span>
              ) : (
                <span className="text-foreground-faint text-[0.78rem]">{step.duration_ms}ms</span>
              )}
              {cancelled ? (
                <span className="text-foreground-faint text-[0.78rem] font-semibold">
                  {t('ai_query.run_trace_cancelled')}
                </span>
              ) : failed ? (
                <span className="text-error text-[0.78rem] font-semibold">
                  {t('ai_query.run_trace_failed')}
                </span>
              ) : null}
              {step.detail ? (
                <span className="text-foreground-faint basis-full text-[0.78rem] wrap-break-word">
                  {renderDetail(t, step.detail)}
                </span>
              ) : null}
            </li>
          )
        })}
      </ol>
    </Collapsible>
  )
}
