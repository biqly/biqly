import type { TranslationKey } from '../../i18n'
import { useT } from '../../i18n'
import type { RunStep } from '../../types/ai'
import { Collapsible } from './routingViz'

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
}

export function RunTracePanel({
  steps,
  defaultOpen = false,
}: {
  steps: RunStep[]
  defaultOpen?: boolean
}) {
  const t = useT()
  if (steps.length === 0) {
    return null
  }
  return (
    <Collapsible title={t('ai_query.run_trace_title')} defaultOpen={defaultOpen}>
      <ol className="text-foreground-muted m-0 mt-3 flex list-none flex-col gap-0 p-0 text-[0.88rem]">
        {steps.map((step) => {
          const labelKey = STEP_LABEL_KEYS[step.kind]
          const failed = step.status === 'failed'
          return (
            <li
              key={step.seq}
              className="border-border relative flex flex-wrap items-baseline gap-x-2 gap-y-1 border-l-2 pb-2.5 pl-4 last:pb-0"
            >
              <span
                aria-hidden="true"
                className={`absolute top-[0.3rem] left-[-0.32rem] h-2.5 w-2.5 rounded-full ${
                  failed ? 'bg-error' : 'bg-success'
                }`}
              />
              <span className="text-foreground font-semibold">
                {labelKey ? t(labelKey) : step.kind}
              </span>
              {step.attempt != null && step.attempt > 0 ? (
                <span className="text-foreground-faint text-[0.78rem]">
                  {t('ai_query.run_trace_attempt', { n: step.attempt })}
                </span>
              ) : null}
              <span className="text-foreground-faint text-[0.78rem]">{step.duration_ms}ms</span>
              {failed ? (
                <span className="text-error text-[0.78rem] font-semibold">
                  {t('ai_query.run_trace_failed')}
                </span>
              ) : null}
              {step.detail ? (
                <span className="text-foreground-faint basis-full text-[0.78rem] wrap-break-word">
                  {step.detail}
                </span>
              ) : null}
            </li>
          )
        })}
      </ol>
    </Collapsible>
  )
}
