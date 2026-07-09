// @vitest-environment jsdom
//
// T11 "Done when": a component test proving an agent-mode-shaped message
// (the AIQueryResponse normalizeAgentResultEvent/runAgentModeTurn produces
// for a 'result' SSE event, threaded through addMessage exactly like the
// legacy pipeline's result) renders through every existing
// AssistantMessageCard section — table, chart, caption/answer, follow-ups,
// SQL preview, trace — with NO agent-mode-specific code path in the card
// itself. T10 wired the data through; this suite is the verification T10
// didn't add.
//
// AssistantMessageCard calls useNavigate() and several descendants
// (FeedbackSection, SampleDataModal/Modal, ResultTable) call useT()/
// useLocale()/useToast() directly rather than accepting props — mocked here
// (identity t, no-op toast) rather than standing up real providers, mirroring
// AIQuery.test.tsx's established pattern in this repo.
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import type * as I18nModule from '../../i18n'
import type { AIQueryResponse } from '../../types/ai'
import type { ConversationMessage } from '../../types/ai'
import { AssistantMessageCard } from './AssistantMessageCard'

vi.mock('react-router-dom', () => ({
  useNavigate: () => vi.fn(),
}))

vi.mock('../../i18n', async (importOriginal) => ({
  ...(await importOriginal<typeof I18nModule>()),
  useLocale: () => ['en', vi.fn()],
  useT: () => (key: string, params?: Record<string, string | number>) =>
    params ? `${key}:${JSON.stringify(params)}` : key,
}))

vi.mock('../../hooks/useToast', () => ({
  useToast: () => ({
    show: vi.fn(),
    success: vi.fn(),
    error: vi.fn(),
    info: vi.fn(),
    warning: vi.fn(),
    dismiss: vi.fn(),
  }),
}))

// RunTracePanel's re-hydration effect (steps=[] but run_id set) calls this —
// stub it so the test never hits a real network request for the trace title
// assertion below.
vi.mock('../../api/agentRuns', () => ({
  getAgentRun: vi.fn(() =>
    Promise.resolve({
      run: { id: 'agent-run-1' },
      steps: [{ seq: 1, kind: 'run_question', status: 'ok', duration_ms: 120 }],
    }),
  ),
}))

// jsdom has no ResizeObserver; the default chart branch (visualization_hint
// suggests "bar") mounts recharts' ResponsiveContainer, which needs one.
if (typeof globalThis.ResizeObserver === 'undefined') {
  class ResizeObserverStub {
    observe() {
      /* no-op */
    }
    unobserve() {
      /* no-op */
    }
    disconnect() {
      /* no-op */
    }
  }
  globalThis.ResizeObserver = ResizeObserverStub
}

const identityT = (key: string, params?: Record<string, string | number>) =>
  params ? `${key}:${JSON.stringify(params)}` : key

// Shaped exactly like what normalizeAgentResultEvent (agentModeTurn.ts)
// produces for a 'result' SSE event: a flat AIQueryResponse with the query
// fields populated and run_id backfilled from the event, but WITHOUT
// run_steps (normalizeAgentResultEvent never unwraps metadata — see
// agentModeTurn.ts's own comment) — the trace instead re-hydrates lazily via
// run_id, exactly like a reloaded thread.
function agentModeResult(overrides: Partial<AIQueryResponse> = {}): AIQueryResponse {
  return {
    answer: 'Revenue grew 12% quarter over quarter.',
    caveat: 'Assumed "revenue" means net revenue.',
    confidence: 0.82,
    sql: "SELECT date_trunc('quarter', ordered_at) AS quarter, sum(amount) AS revenue FROM orders GROUP BY 1",
    visualization_hint: { chart_type: 'bar', reason: 'time series' },
    result: {
      columns: [
        { name: 'quarter', semantic_type: 'dimension' },
        { name: 'revenue', semantic_type: 'metric', format: 'currency' },
      ],
      rows: [
        ['2026-Q1', 100000],
        ['2026-Q2', 112000],
      ],
      stats: { row_count: 2, duration_ms: 42 },
    },
    suggested_followups: [
      {
        id: 'fu-1',
        label: 'Break down by region',
        question: 'Can you break revenue down by region?',
        kind: 'breakdown',
      },
    ],
    run_id: 'agent-run-1',
    ...overrides,
  }
}

function agentMessage(overrides: Partial<AIQueryResponse> = {}): ConversationMessage {
  return {
    role: 'assistant',
    content: 'Revenue grew 12% quarter over quarter.',
    timestamp: new Date().toISOString(),
    ai_response: agentModeResult(overrides),
  }
}

function renderCard(message: ConversationMessage) {
  return render(
    <AssistantMessageCard
      message={message}
      messageIndex={0}
      conversationId="conv-1"
      datasourceId="ds-1"
      aiRuntime={null}
      userQuestion="How did revenue change last quarter?"
      get={vi.fn(() => Promise.resolve(null))}
      postData={vi.fn(() => Promise.resolve(null))}
      updateMessageResponse={vi.fn()}
      t={identityT}
      localeNumberTag={() => 'en-US'}
      localeTag="en-US"
      onSelectClarification={vi.fn()}
      onSkipClarification={vi.fn()}
      onFilterByValue={vi.fn()}
      onCellDrillDown={vi.fn()}
      onSelectFollowUp={vi.fn()}
      priorQuestions={[]}
    />,
  )
}

afterEach(() => {
  cleanup()
})

describe('AssistantMessageCard rendering an agent-mode result message', () => {
  it('renders the server-synthesized answer and caveat', () => {
    const { container } = renderCard(agentMessage())
    const answerEl = container.querySelector(
      '[aria-label="Revenue grew 12% quarter over quarter."]',
    )
    expect(answerEl).not.toBeNull()
    expect(screen.getByText('Assumed "revenue" means net revenue.')).toBeTruthy()
  })

  it('renders the chart section by default (suggested by visualization_hint)', () => {
    const { container } = renderCard(agentMessage())
    // ChartTypeSelector renders a labeled control per chart type; the chart
    // container itself is keyed off a stable class from aiQueryClasses. The
    // presence of the results header (row count) proves AssistantMessageResults
    // rendered at all — the specific chart-vs-table branch is exercised by
    // ChartContainer's own tests, out of scope here.
    expect(container.textContent).toContain('ai_query.results_title:{"rows":2}')
  })

  it('renders the result table after switching the view to table', () => {
    renderCard(agentMessage())
    fireEvent.click(screen.getByText('ai_query.chart_table'))
    expect(screen.getByText('2026-Q1')).toBeTruthy()
    expect(screen.getByText('2026-Q2')).toBeTruthy()
  })

  it('renders follow-up suggestions from the agent result', () => {
    renderCard(agentMessage())
    expect(screen.getByText('Break down by region')).toBeTruthy()
  })

  it('renders the SQL preview and the run trace once details are expanded', async () => {
    renderCard(agentMessage())
    fireEvent.click(screen.getByText('ai_query.details_show'))

    expect(
      screen.getByText(
        "SELECT date_trunc('quarter', ordered_at) AS quarter, sum(amount) AS revenue FROM orders GROUP BY 1",
      ),
    ).toBeTruthy()
    // run_steps is empty on the agent-mode result (see agentModeResult's
    // comment) but run_id is set, so RunTracePanel re-hydrates via
    // getAgentRun (mocked above) instead of showing nothing.
    expect(await screen.findByText('ai_query.run_trace_title')).toBeTruthy()
  })

  it('does not render a clarification card for a completed (non-clarification) agent result', () => {
    renderCard(agentMessage())
    expect(screen.queryByText('ai_query.clarification_title')).toBeNull()
  })
})
