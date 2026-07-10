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

// conversationHarness backs the useConversation mock below with two
// conversations ('conv-1', 'conv-2') and a mutable "active" pointer, so the
// multi-conversation describe block can switch which conversation AIQuery
// sees as active (via setActiveConversationId + a re-render) without
// remounting the component — remounting would reset AIQuery's own internal
// agentTurnsByConversation state, which is exactly what these tests need to
// observe surviving a conversation switch.
const {
  mockRunAgentModeTurn,
  mockRunJob,
  mockAddMessage,
  mockPostData,
  conversationHarness,
  mockSetActiveConversationId,
} = vi.hoisted(() => {
  const conversationHarness = {
    activeConversationId: 'conv-1',
    conversations: {
      'conv-1': { id: 'conv-1', messages: [], context_enabled: true },
      'conv-2': { id: 'conv-2', messages: [], context_enabled: true },
    } as Record<string, { id: string; messages: unknown[]; context_enabled: boolean }>,
  }
  return {
    mockRunAgentModeTurn: vi.fn(),
    mockRunJob: vi.fn(() => Promise.resolve('queued')),
    mockAddMessage: vi.fn(),
    mockPostData: vi.fn(() => Promise.resolve(null)),
    conversationHarness,
    mockSetActiveConversationId: vi.fn((id: string) => {
      conversationHarness.activeConversationId = id
    }),
  }
})

vi.mock('react-router-dom', () => ({
  useLocation: () => ({ state: null }),
}))

vi.mock('../hooks/useAIJobs', () => ({
  fetchOwnAIJobs: vi.fn(() => Promise.resolve(null)),
  jobIsActive: () => false,
  useAIJobs: () => ({
    runJob: mockRunJob,
    cancelJob: vi.fn(),
    jobs: [],
    queueStatus: null,
    sessionId: 'test-session',
  }),
}))

vi.mock('../hooks/useApi', () => ({
  useApi: () => ({
    get: vi.fn(() => Promise.resolve(null)),
    postData: mockPostData,
    loading: false,
    error: null,
    abort: vi.fn(),
  }),
}))

vi.mock('../hooks/useConversation', () => ({
  useConversation: () => ({
    // Deliberately empty (not Object.values(conversations)): a non-empty
    // list would mount the real SidebarConversationItem, which needs a
    // ConfirmProvider this minimal harness doesn't set up. This suite only
    // needs activeConversation/activeConversationId to be switchable.
    conversations: [],
    activeConversation: conversationHarness.conversations[conversationHarness.activeConversationId],
    activeConversationId: conversationHarness.activeConversationId,
    setActiveConversationId: mockSetActiveConversationId,
    createConversation: vi.fn(() => conversationHarness.conversations['conv-1']),
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
      <button onClick={() => props.onSendQuery('How many orders?', false)}>preview</button>
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
  mockRunJob.mockClear()
  mockAddMessage.mockReset()
  mockPostData.mockClear()
  mockSetActiveConversationId.mockClear()
  conversationHarness.activeConversationId = 'conv-1'
})

describe('AIQuery Agent Mode stream cleanup', () => {
  it('uses the governed preview path instead of starting an agent run', async () => {
    render(<AIQuery />)
    fireEvent.click(screen.getByText('preview'))
    await act(async () => {
      await Promise.resolve()
    })
    expect(mockRunAgentModeTurn).not.toHaveBeenCalled()
    expect(mockRunJob).toHaveBeenCalledWith('preview', expect.anything(), expect.anything())
    expect(mockPostData).not.toHaveBeenCalled()
  })

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

// Regression coverage for the review fix: agentSteps/agentClarification/the
// abort ref must be scoped per conversation id, not shared as a single
// global value. Before the fix, a fresh question sent in a DIFFERENT
// conversation than the one holding a pending clarification (a) wiped that
// other conversation's clarification/trace unconditionally, and (b) — via
// the single global AbortController ref — aborted (and killed server-side) a
// genuinely still-streaming run in that other conversation, contradicting
// the documented "the run keeps going in the background" behavior.
describe('AIQuery Agent Mode multi-conversation scoping', () => {
  it("switching to a different conversation and sending a fresh question does not clear the other conversation's pending clarification", async () => {
    let resolveConv1: (outcome: AgentTurnOutcome) => void = () => undefined
    mockRunAgentModeTurn.mockImplementationOnce(
      () =>
        new Promise<AgentTurnOutcome>((resolve) => {
          resolveConv1 = resolve
        }),
    )

    const { rerender } = render(<AIQuery />)

    // conv-1 (the active conversation) asks a question and pauses on a
    // clarification.
    fireEvent.click(screen.getByText('send'))
    expect(mockRunAgentModeTurn).toHaveBeenCalledTimes(1)
    await act(async () => {
      resolveConv1({
        kind: 'clarification',
        runId: 'run-conv1',
        question: 'Which region did you mean?',
        choices: [{ id: 'emea', label: 'EMEA' }],
        allowFreeText: true,
      })
      await Promise.resolve()
    })
    expect(screen.getByTestId('agent-clarification-question').textContent).toBe(
      'Which region did you mean?',
    )

    // Switch the active conversation to conv-2 — its own trace is empty, so
    // the card must not show conv-1's clarification while conv-2 is active.
    conversationHarness.activeConversationId = 'conv-2'
    rerender(<AIQuery />)
    expect(screen.getByTestId('agent-clarification-question').textContent).toBe('')

    // Send a brand new question in conv-2. Per the bug report, the old code
    // unconditionally cleared agentSteps/agentClarification here even though
    // they belonged to conv-1, not conv-2.
    const conv2Requests: unknown[] = []
    mockRunAgentModeTurn.mockImplementationOnce((request: unknown) => {
      conv2Requests.push(request)
      return new Promise<AgentTurnOutcome>(() => undefined) // never resolves
    })
    fireEvent.click(screen.getByText('send'))
    expect(mockRunAgentModeTurn).toHaveBeenCalledTimes(2)
    expect(conv2Requests[0]).toMatchObject({ conversation_id: 'conv-2' })
    // conv-2's own send must not be misrouted as an answer to conv-1's
    // clarification (no resume_run_id/clarification_answer).
    expect(conv2Requests[0]).not.toHaveProperty('resume_run_id')

    // Switch back to conv-1: its paused clarification must still be there.
    conversationHarness.activeConversationId = 'conv-1'
    rerender(<AIQuery />)
    expect(screen.getByTestId('agent-clarification-question').textContent).toBe(
      'Which region did you mean?',
    )
  })

  it("starting a fresh turn in a different conversation does not abort another conversation's genuinely in-flight stream", () => {
    const signals: AbortSignal[] = []
    mockRunAgentModeTurn.mockImplementation(
      (_request: unknown, options: RunAgentModeTurnOptions) => {
        signals.push(options.signal!)
        return new Promise<AgentTurnOutcome>(() => undefined) // never resolves
      },
    )

    const { rerender } = render(<AIQuery />)

    // conv-1 starts a turn that never settles (a genuinely in-flight stream).
    fireEvent.click(screen.getByText('send'))
    expect(signals).toHaveLength(1)
    expect(signals[0]!.aborted).toBe(false)

    // Switch to conv-2 and start its own turn.
    conversationHarness.activeConversationId = 'conv-2'
    rerender(<AIQuery />)
    fireEvent.click(screen.getByText('send'))
    expect(signals).toHaveLength(2)

    // conv-1's stream must still be running — a different conversation's
    // fresh turn must not have superseded/aborted it.
    expect(signals[0]!.aborted).toBe(false)
    expect(signals[1]!.aborted).toBe(false)

    // Cancel while conv-2 is active must only abort conv-2's own stream.
    fireEvent.click(screen.getByText('abort'))
    expect(signals[0]!.aborted).toBe(false)
    expect(signals[1]!.aborted).toBe(true)
  })
})
