// @vitest-environment jsdom
//
// Covers the T10 review fix: Agent Mode's in-flight SSE stream must be
// aborted on unmount (no leaked fetch / wasted backend call), and an
// intentional abort must not surface as a user-facing error message. See
// FollowUpSuggestionsSection.test.tsx for why this file opts into jsdom +
// @testing-library/react rather than the renderToStaticMarkup pattern used
// elsewhere: exercising an effect's unmount cleanup requires a real DOM
// mount/unmount lifecycle, which a static string render cannot do.
//
// Every hook AIQuery.tsx depends on is mocked directly (rather than routed
// through real providers) so this suite stays focused on sendQuery's Agent
// Mode branch and the unmount-abort wiring, without dragging in
// ChatPanel/RoutingPanel's own dependency trees (semantic catalog, saved
// queries, job polling, etc.) — those are exercised by their own component
// tests. ChatPanel and RoutingPanel are stubbed to a minimal control surface
// that exposes exactly the props this suite asserts on.
import { act, cleanup, render, screen } from '@testing-library/react'
import { fireEvent } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import type * as I18nModule from '../i18n'
import type { AgentTurnOutcome, RunAgentModeTurnOptions } from './aiQuery/agentModeTurn'

const { mockRunAgentModeTurn, mockAddMessage } = vi.hoisted(() => ({
  mockRunAgentModeTurn: vi.fn(),
  mockAddMessage: vi.fn(),
}))

vi.mock('react-router-dom', () => ({
  useLocation: () => ({ state: null }),
}))

vi.mock('../hooks/useAIJobs', () => ({
  fetchOwnAIJobs: vi.fn(() => Promise.resolve(null)),
  jobIsActive: () => false,
  useAIJobs: () => ({
    runJob: vi.fn(),
    cancelJob: vi.fn(),
    jobs: [],
    queueStatus: null,
    sessionId: 'test-session',
  }),
}))

vi.mock('../hooks/useApi', () => ({
  useApi: () => ({
    get: vi.fn(() => Promise.resolve(null)),
    postData: vi.fn(() => Promise.resolve(null)),
    loading: false,
    error: null,
    abort: vi.fn(),
  }),
}))

vi.mock('../hooks/useConversation', () => ({
  useConversation: () => ({
    conversations: [],
    activeConversation: {
      id: 'conv-1',
      messages: [],
      context_enabled: true,
    },
    activeConversationId: 'conv-1',
    setActiveConversationId: vi.fn(),
    createConversation: vi.fn(() => ({ id: 'conv-1' })),
    addMessage: mockAddMessage,
    appendAssistantForJob: vi.fn(),
    deleteConversation: vi.fn(),
    renameConversation: vi.fn(),
    updateConversationContext: vi.fn(),
    updateMessageResponse: vi.fn(),
  }),
}))

vi.mock('../hooks/useDatasources', () => ({
  useDatasources: () => ({ datasources: [] }),
}))

vi.mock('../hooks/useQueryParam', () => ({
  useQueryParam: () => ['', vi.fn()],
}))

vi.mock('../hooks/useSemanticModels', () => ({
  useSemanticModels: () => ({ models: [] }),
}))

vi.mock('../i18n', async (importOriginal) => ({
  ...(await importOriginal<typeof I18nModule>()),
  useLocale: () => ['en'],
  useT: () => (key: string) => key,
}))

vi.mock('./auth/AuthProvider', () => ({
  useAuth: () => ({ accessToken: 'test-token' }),
}))

// Agent Mode is opt-in (defaults off); force it on so sendQuery routes
// through runAgentModeTurn without an extra toggle-click step.
vi.mock('./aiQuery/agentModeStorage', () => ({
  loadAgentModeEnabled: () => true,
  saveAgentModeEnabled: vi.fn(),
}))

vi.mock('./aiQuery/agentModeTurn', () => ({
  runAgentModeTurn: mockRunAgentModeTurn,
}))

vi.mock('./aiQuery/RoutingPanel', () => ({
  RoutingPanel: () => null,
}))

// Minimal stand-in exposing only what this suite drives/asserts: onSendQuery
// (to trigger a send or a free-text clarification answer), jobError/
// queryAction (to prove a clean abort never surfaces as a user-facing
// error), onAbort (Cancel wiring), and the T11 clarification/trace props
// (to drive and observe the resume path without rendering the real
// ChatPanel/AgentTraceCard tree).
vi.mock('./aiQuery/ChatPanel', () => ({
  ChatPanel: (props: {
    onSendQuery: (q: string, execute: boolean) => void
    onAbort: () => void
    jobError: string | null
    queryAction: 'preview' | 'execute' | null
    agentTraceSteps: unknown[]
    agentClarification: { runId: string; question: string } | null
    onAgentClarificationChoice: (choiceId: string) => void
    onAgentClarificationSkip: () => void
  }) => (
    <div>
      <button onClick={() => props.onSendQuery('How many orders?', true)}>send</button>
      <button onClick={props.onAbort}>abort</button>
      <button onClick={() => props.onAgentClarificationChoice('emea')}>select-choice</button>
      <button onClick={props.onAgentClarificationSkip}>skip-clarification</button>
      <div data-testid="job-error">{props.jobError ?? ''}</div>
      <div data-testid="query-action">{props.queryAction ?? ''}</div>
      <div data-testid="agent-clarification-question">
        {props.agentClarification?.question ?? ''}
      </div>
      <div data-testid="agent-trace-step-count">{props.agentTraceSteps.length}</div>
    </div>
  ),
}))

import AIQuery from './AIQuery'

afterEach(() => {
  cleanup()
  mockRunAgentModeTurn.mockReset()
  mockAddMessage.mockReset()
})

describe('AIQuery Agent Mode stream cleanup', () => {
  it('aborts the in-flight Agent Mode stream when the component unmounts mid-stream', () => {
    let capturedSignal: AbortSignal | undefined
    mockRunAgentModeTurn.mockImplementation(
      (_request: unknown, options: RunAgentModeTurnOptions) => {
        capturedSignal = options.signal
        // Never resolves during the test — mirrors a stream still in flight.
        return new Promise<AgentTurnOutcome>(() => undefined)
      },
    )

    const { unmount } = render(<AIQuery />)
    fireEvent.click(screen.getByText('send'))

    expect(mockRunAgentModeTurn).toHaveBeenCalledTimes(1)
    expect(capturedSignal).toBeDefined()
    expect(capturedSignal?.aborted).toBe(false)

    unmount()

    expect(capturedSignal?.aborted).toBe(true)
  })

  it('does not surface a spurious error when the stream resolves as a clean abort ("none")', async () => {
    let resolveTurn: (outcome: AgentTurnOutcome) => void = () => undefined
    mockRunAgentModeTurn.mockImplementation(
      () =>
        new Promise<AgentTurnOutcome>((resolve) => {
          resolveTurn = resolve
        }),
    )

    render(<AIQuery />)
    fireEvent.click(screen.getByText('send'))
    expect(mockRunAgentModeTurn).toHaveBeenCalledTimes(1)

    // Simulate runAgentModeTurn's real behavior when the abort raced the
    // stream: it resolves to 'none' rather than throwing/erroring (see
    // agentModeTurn.test.ts for the unit-level proof of that mapping).
    await act(async () => {
      resolveTurn({ kind: 'none' })
      await Promise.resolve()
    })

    expect(screen.getByTestId('job-error').textContent).toBe('')
    // A clean abort must not add an error message to the conversation either.
    expect(mockAddMessage).toHaveBeenCalledTimes(1) // only the initial user message
  })

  it("does not let a superseded turn clobber the superseding turn's loading state", async () => {
    // Reproduces the double-send race: Turn 1 is still in flight when Turn 2
    // starts. Turn 2 aborts Turn 1 (agentStreamAbortRef's "abort the previous
    // turn" safety net), which makes Turn 1's promise settle to {kind:'none'}
    // almost immediately — but Turn 1's own `finally` must not clear
    // queryAction, because that belongs to Turn 2 now. Only Turn 2 settling
    // should clear it.
    const resolvers: ((outcome: AgentTurnOutcome) => void)[] = []
    const signals: AbortSignal[] = []
    mockRunAgentModeTurn.mockImplementation(
      (_request: unknown, options: RunAgentModeTurnOptions) => {
        signals.push(options.signal!)
        return new Promise<AgentTurnOutcome>((resolve) => {
          resolvers.push(resolve)
        })
      },
    )

    render(<AIQuery />)

    fireEvent.click(screen.getByText('send'))
    expect(mockRunAgentModeTurn).toHaveBeenCalledTimes(1)
    expect(screen.getByTestId('query-action').textContent).toBe('execute')

    // Turn 2 starts: this aborts Turn 1's signal via the existing
    // "abort the previous turn" safety net in sendQuery.
    fireEvent.click(screen.getByText('send'))
    expect(mockRunAgentModeTurn).toHaveBeenCalledTimes(2)
    expect(signals[0]!.aborted).toBe(true)
    expect(signals[1]!.aborted).toBe(false)
    expect(screen.getByTestId('query-action').textContent).toBe('execute')

    // Turn 1's aborted stream settles to 'none' (per agentModeTurn.ts's
    // abort handling) while Turn 2 is still genuinely in flight.
    await act(async () => {
      resolvers[0]!({ kind: 'none' })
      await Promise.resolve()
    })

    // Turn 2 is still running — its loading state must survive Turn 1's
    // settlement rather than being clobbered back to null.
    expect(screen.getByTestId('query-action').textContent).toBe('execute')

    // Now Turn 2 itself settles; only now should the UI go idle.
    await act(async () => {
      resolvers[1]!({ kind: 'none' })
      await Promise.resolve()
    })

    expect(screen.getByTestId('query-action').textContent).toBe('')
  })

  it('aborts the in-flight Agent Mode stream when Cancel (onAbort) is clicked', () => {
    let capturedSignal: AbortSignal | undefined
    mockRunAgentModeTurn.mockImplementation(
      (_request: unknown, options: RunAgentModeTurnOptions) => {
        capturedSignal = options.signal
        return new Promise<AgentTurnOutcome>(() => undefined)
      },
    )

    render(<AIQuery />)
    fireEvent.click(screen.getByText('send'))
    expect(capturedSignal?.aborted).toBe(false)

    fireEvent.click(screen.getByText('abort'))

    expect(capturedSignal?.aborted).toBe(true)
  })

  it('surfaces a clarification_required outcome as a pending clarification, then resumes with resume_run_id/clarification_answer when a choice is selected', async () => {
    let resolveFirst: (outcome: AgentTurnOutcome) => void = () => undefined
    const secondCallRequests: unknown[] = []
    mockRunAgentModeTurn
      .mockImplementationOnce(
        () =>
          new Promise<AgentTurnOutcome>((resolve) => {
            resolveFirst = resolve
          }),
      )
      .mockImplementationOnce((request: unknown) => {
        secondCallRequests.push(request)
        return Promise.resolve<AgentTurnOutcome>({ kind: 'none' })
      })

    render(<AIQuery />)
    fireEvent.click(screen.getByText('send'))
    expect(mockRunAgentModeTurn).toHaveBeenCalledTimes(1)

    await act(async () => {
      resolveFirst({
        kind: 'clarification',
        runId: 'run-9',
        question: 'Which region did you mean?',
        choices: [{ id: 'emea', label: 'EMEA' }],
        allowFreeText: true,
      })
      await Promise.resolve()
    })

    expect(screen.getByTestId('agent-clarification-question').textContent).toBe(
      'Which region did you mean?',
    )
    // The turn is paused, not running: the composer/Cancel gating must not
    // stay stuck "busy" while waiting on the user.
    expect(screen.getByTestId('query-action').textContent).toBe('')

    fireEvent.click(screen.getByText('select-choice'))
    await act(async () => {
      await Promise.resolve()
    })

    expect(mockRunAgentModeTurn).toHaveBeenCalledTimes(2)
    expect(secondCallRequests[0]).toMatchObject({
      resume_run_id: 'run-9',
      clarification_answer: 'emea',
    })
    // The resume is itself a real user-visible turn: a bubble for the chosen
    // answer, in addition to the original question.
    expect(mockAddMessage).toHaveBeenCalledTimes(2)
  })
})
