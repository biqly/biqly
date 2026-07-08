// @vitest-environment jsdom
//
// AgentTraceCard renders ClarificationCard (routingViz.tsx), which calls the
// real useT()/useLocale() hooks internally rather than accepting a `t` prop
// (unlike RunTracePanel, which this card passes an explicit `t` into). So —
// following AIQuery.test.tsx's established pattern — '../../i18n' is mocked
// to an identity stub instead of standing up a real <I18nProvider>, letting
// every rendered string (including ClarificationCard's own) resolve to its
// raw translation key for exact assertions.
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import type * as I18nModule from '../../i18n'
import type { PendingAgentClarification } from '../../types/agent'
import type { RunStep } from '../../types/ai'
import { AgentTraceCard } from './AgentTraceCard'

vi.mock('../../i18n', async (importOriginal) => ({
  ...(await importOriginal<typeof I18nModule>()),
  useLocale: () => ['en', vi.fn()],
  useT: () => (key: string, params?: Record<string, string | number>) =>
    params ? `${key}:${JSON.stringify(params)}` : key,
}))

const identityT = (key: string, params?: Record<string, string | number>) =>
  params ? `${key}:${JSON.stringify(params)}` : key

function step(overrides: Partial<RunStep>): RunStep {
  return { seq: 1, kind: 'list_models', status: 'ok', duration_ms: 5, ...overrides }
}

function clarification(
  overrides: Partial<PendingAgentClarification> = {},
): PendingAgentClarification {
  return {
    runId: 'run-1',
    question: 'Which region did you mean?',
    choices: [
      { id: 'emea', label: 'EMEA' },
      { id: 'apac', label: 'APAC' },
    ],
    allowFreeText: true,
    ...overrides,
  }
}

afterEach(() => {
  cleanup()
})

describe('AgentTraceCard', () => {
  it('renders nothing when there are no steps and no pending clarification', () => {
    const { container } = render(
      <AgentTraceCard
        steps={[]}
        clarification={null}
        onSelectClarificationChoice={vi.fn()}
        onSkipClarification={vi.fn()}
        t={identityT}
      />,
    )
    expect(container.innerHTML).toBe('')
  })

  it('renders the live trace as steps accumulate, labeling each web agent tool by name', () => {
    const { rerender } = render(
      <AgentTraceCard
        steps={[step({ seq: 1, kind: 'list_models', status: 'running' })]}
        clarification={null}
        onSelectClarificationChoice={vi.fn()}
        onSkipClarification={vi.fn()}
        t={identityT}
      />,
    )
    expect(screen.getByText('ai_query.run_trace_step_list_models')).toBeTruthy()
    expect(screen.getByText('ai_query.run_trace_running')).toBeTruthy()

    // The next step event (a distinct seq) is appended; the first step's
    // terminal status is merged in-place upstream by mergeAgentStepEvent
    // (covered by agentTraceSteps.test.ts) — this render-level test only
    // needs to prove the panel reflects a GROWING steps array live.
    rerender(
      <AgentTraceCard
        steps={[
          step({ seq: 1, kind: 'list_models', status: 'ok' }),
          step({ seq: 2, kind: 'run_question', status: 'running' }),
        ]}
        clarification={null}
        onSelectClarificationChoice={vi.fn()}
        onSkipClarification={vi.fn()}
        t={identityT}
      />,
    )
    expect(screen.getByText('ai_query.run_trace_step_list_models')).toBeTruthy()
    expect(screen.getByText('ai_query.run_trace_step_run_question')).toBeTruthy()
    expect(screen.getByText('ai_query.run_trace_running')).toBeTruthy()
  })

  it('renders the clarification card with numbered choices and calls onSelectClarificationChoice with the choice id', () => {
    const onSelect = vi.fn()
    render(
      <AgentTraceCard
        steps={[]}
        clarification={clarification()}
        onSelectClarificationChoice={onSelect}
        onSkipClarification={vi.fn()}
        t={identityT}
      />,
    )
    expect(screen.getByText('Which region did you mean?')).toBeTruthy()

    fireEvent.click(screen.getByText('EMEA'))
    expect(onSelect).toHaveBeenCalledTimes(1)
    expect(onSelect).toHaveBeenCalledWith('emea')
  })

  it('calls onSkipClarification when the skip button is clicked', () => {
    const onSkip = vi.fn()
    render(
      <AgentTraceCard
        steps={[]}
        clarification={clarification()}
        onSelectClarificationChoice={vi.fn()}
        onSkipClarification={onSkip}
        t={identityT}
      />,
    )
    fireEvent.click(screen.getByText('ai_query.clarification_skip'))
    expect(onSkip).toHaveBeenCalledTimes(1)
  })

  it('shows a free-text hint when the clarification allows it', () => {
    render(
      <AgentTraceCard
        steps={[]}
        clarification={clarification({ allowFreeText: true })}
        onSelectClarificationChoice={vi.fn()}
        onSkipClarification={vi.fn()}
        t={identityT}
      />,
    )
    expect(screen.getByText('ai_query.agent_clarification_free_text_hint')).toBeTruthy()
  })

  it('omits the free-text hint when the clarification does not allow it', () => {
    render(
      <AgentTraceCard
        steps={[]}
        clarification={clarification({ allowFreeText: false })}
        onSelectClarificationChoice={vi.fn()}
        onSkipClarification={vi.fn()}
        t={identityT}
      />,
    )
    expect(screen.queryByText('ai_query.agent_clarification_free_text_hint')).toBeNull()
  })

  it('keeps showing the trace panel alongside a clarification card in the same slot', () => {
    render(
      <AgentTraceCard
        steps={[step({ seq: 1, kind: 'list_models', status: 'ok' })]}
        clarification={clarification()}
        onSelectClarificationChoice={vi.fn()}
        onSkipClarification={vi.fn()}
        t={identityT}
      />,
    )
    expect(screen.getByText('ai_query.run_trace_step_list_models')).toBeTruthy()
    expect(screen.getByText('Which region did you mean?')).toBeTruthy()
  })
})
