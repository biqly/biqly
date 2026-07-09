import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it } from 'vitest'

import type { TranslationKey } from '../../i18n'
import type { RunStep } from '../../types/ai'
import { RunTracePanel } from './RunTrace'

// Identity stub: t(key) returns the key itself, so assertions can check for
// the exact translation key that should have been selected — this is the
// same pattern ModelingTableCard.test.tsx uses, since this repo has no
// jsdom/@testing-library/react setup to render through an <I18nProvider>
// with real hook-based useT(). Because of that, this suite covers every
// synchronous (steps-prop) rendering path but NOT the persisted-reload
// (getAgentRun useEffect) path — effects never run under
// renderToStaticMarkup. That gap is a known limitation, not a gap in this
// component's actual behavior.
const t = (key: TranslationKey, params?: Record<string, string | number>) =>
  params ? `${key}:${JSON.stringify(params)}` : key

function step(overrides: Partial<RunStep>): RunStep {
  return { seq: 1, kind: 'tool', status: 'ok', duration_ms: 10, ...overrides }
}

describe('RunTracePanel', () => {
  it('renders nothing when there are no steps and no runId', () => {
    const markup = renderToStaticMarkup(<RunTracePanel steps={[]} t={t} />)
    expect(markup).toBe('')
  })

  it('labels every new agentic runtime step kind', () => {
    const kinds = [
      'planner',
      'policy',
      'tool',
      'observation',
      'clarification',
      'final_response',
      'shadow_comparison',
    ]
    const steps = kinds.map((kind, i) => step({ seq: i + 1, kind }))
    const markup = renderToStaticMarkup(<RunTracePanel steps={steps} t={t} />)
    for (const kind of kinds) {
      expect(markup).toContain(`ai_query.run_trace_step_${kind}`)
    }
  })

  it('labels every web agent tool step kind (live-remapped kind and reloaded-persisted kind alike)', () => {
    const kinds = [
      'list_datasources',
      'list_models',
      'list_prompt_templates',
      'run_question',
      'run_logical_query',
      'list_skills',
      'run_skill',
    ]
    const steps = kinds.map((kind, i) => step({ seq: i + 1, kind }))
    const markup = renderToStaticMarkup(<RunTracePanel steps={steps} t={t} />)
    for (const kind of kinds) {
      expect(markup).toContain(`ai_query.run_trace_step_${kind}`)
    }
  })

  it('shows a running indicator instead of a duration or failed/cancelled badge for an in-progress step', () => {
    const markup = renderToStaticMarkup(
      <RunTracePanel
        steps={[step({ kind: 'run_question', status: 'running', duration_ms: 10 })]}
        t={t}
      />,
    )
    expect(markup).toContain('ai_query.run_trace_running')
    expect(markup).not.toContain('10ms')
    expect(markup).not.toContain('ai_query.run_trace_failed')
    expect(markup).not.toContain('ai_query.run_trace_cancelled')
  })

  it('still labels legacy pipeline step kinds', () => {
    const markup = renderToStaticMarkup(
      <RunTracePanel steps={[step({ kind: 'sql_dry_run' })]} t={t} />,
    )
    expect(markup).toContain('ai_query.run_trace_step_sql_dry_run')
  })

  it('renders an unknown step kind verbatim rather than failing', () => {
    const markup = renderToStaticMarkup(
      <RunTracePanel steps={[step({ kind: 'totally_unknown_kind' })]} t={t} />,
    )
    expect(markup).toContain('totally_unknown_kind')
  })

  it('maps a policy denial reason code to its label instead of the raw code', () => {
    const markup = renderToStaticMarkup(
      <RunTracePanel
        steps={[step({ kind: 'policy', status: 'failed', detail: 'hidden_column_denied' })]}
        t={t}
      />,
    )
    expect(markup).toContain('ai_query.run_trace_reason_hidden_column_denied')
    // The raw reason code itself must never appear as rendered text content.
    expect(markup).not.toMatch(/>hidden_column_denied</)
  })

  it('renders a clarification step with its own label', () => {
    const markup = renderToStaticMarkup(
      <RunTracePanel steps={[step({ kind: 'clarification', status: 'ok' })]} t={t} />,
    )
    expect(markup).toContain('ai_query.run_trace_step_clarification')
  })

  it('shows a cancelled badge, not a failed badge, for a context_canceled terminal step', () => {
    const markup = renderToStaticMarkup(
      <RunTracePanel
        steps={[step({ kind: 'final_response', status: 'failed', detail: 'context_canceled' })]}
        t={t}
      />,
    )
    expect(markup).toContain('ai_query.run_trace_cancelled')
    expect(markup).not.toContain('ai_query.run_trace_failed')
  })

  it('shows a failed badge and the mapped reason for a genuine terminal failure', () => {
    const markup = renderToStaticMarkup(
      <RunTracePanel
        steps={[step({ kind: 'final_response', status: 'failed', detail: 'tool_error' })]}
        t={t}
      />,
    )
    expect(markup).toContain('ai_query.run_trace_failed')
    expect(markup).toContain('ai_query.run_trace_reason_tool_error')
  })

  it('never marks a successful step as cancelled or failed', () => {
    const markup = renderToStaticMarkup(
      <RunTracePanel steps={[step({ kind: 'tool', status: 'ok' })]} t={t} />,
    )
    expect(markup).not.toContain('ai_query.run_trace_cancelled')
    expect(markup).not.toContain('ai_query.run_trace_failed')
  })

  it('truncates an oversized detail string instead of rendering it in full', () => {
    const longDetail = 'x'.repeat(500)
    const markup = renderToStaticMarkup(
      <RunTracePanel steps={[step({ detail: longDetail })]} t={t} />,
    )
    expect(markup).not.toContain(longDetail)
    expect(markup).toContain('x'.repeat(300) + '…')
  })

  it('leaves a short, non-reason-code detail untouched', () => {
    const markup = renderToStaticMarkup(
      <RunTracePanel steps={[step({ detail: 'resolved 3 tables' })]} t={t} />,
    )
    expect(markup).toContain('resolved 3 tables')
  })
})
