// @vitest-environment jsdom
//
// T11 review fix: the Cancel button was gated on useApi()'s `loading`, which
// is never true for an Agent Mode SSE turn (it doesn't go through
// postData/get) — so Cancel never appeared while an agent run was in
// flight. This suite drives the real ChatPanel (not a stub) to prove the
// fixed gating and the new AgentTraceCard slot, with an empty conversation
// so AssistantMessageCard's own (much larger) dependency tree never mounts.
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import type * as I18nModule from '../../i18n'
import type { PendingAgentClarification } from '../../types/agent'
import type { RunStep } from '../../types/ai'
import { ChatPanel } from './ChatPanel'
import type { ChatPanelProps } from './types'

// useSemanticCatalog calls useLocale() (needs a real <I18nProvider>) purely
// to decide whether to backfill translated labels — irrelevant to this
// suite's Cancel-gating/AgentTraceCard focus, so it's stubbed like
// AIQuery.test.tsx stubs its own hook dependencies rather than standing up
// a full provider tree.
vi.mock('../../hooks/useSemanticCatalog', () => ({
  useSemanticCatalog: () => ({
    items: [],
    canRetranslate: false,
    retranslate: vi.fn(),
    retranslating: false,
  }),
}))

// The clarification card path (AgentTraceCard -> ClarificationCard) calls
// the real useT()/useLocale() hooks rather than accepting props — mocked to
// an identity stub, same pattern as AIQuery.test.tsx/AgentTraceCard.test.tsx.
vi.mock('../../i18n', async (importOriginal) => ({
  ...(await importOriginal<typeof I18nModule>()),
  useLocale: () => ['en', vi.fn()],
  useT: () => (key: string, params?: Record<string, string | number>) =>
    params ? `${key}:${JSON.stringify(params)}` : key,
}))

// jsdom has no scrollTo implementation; ChatPanel's message-feed
// autoscroll effect calls it unconditionally on mount.
Element.prototype.scrollTo = vi.fn()
Object.defineProperty(window, 'matchMedia', {
  writable: true,
  value: vi.fn().mockReturnValue({
    matches: false,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
  }),
})

const identityT: ChatPanelProps['t'] = (key, params) =>
  params ? `${key}:${JSON.stringify(params)}` : key

function baseProps(overrides: Partial<ChatPanelProps> = {}): ChatPanelProps {
  return {
    t: identityT,
    localeNumberTag: () => 'en-US',
    localeTag: 'en-US',
    activeConversation: {
      id: 'conv-1',
      messages: [],
      created_at: '',
      updated_at: '',
      context_enabled: true,
    },
    activeConversationId: 'conv-1',
    datasourceId: '',
    semanticModelId: '',
    tables: [],
    aiRuntime: null,
    question: '',
    setQuestion: vi.fn(),
    loading: false,
    error: null,
    jobError: null,
    queryAction: null,
    aiElapsedMs: 0,
    activeJob: null,
    queueNotice: null,
    contextEnabled: true,
    onContextEnabledChange: vi.fn(),
    autoFindEnabled: true,
    onAutoFindEnabledChange: vi.fn(),
    agentModeEnabled: false,
    onAgentModeEnabledChange: vi.fn(),
    agentTraceSteps: [],
    agentClarification: null,
    onAgentClarificationChoice: vi.fn(),
    onAgentClarificationSkip: vi.fn(),
    selectedSavedQueryIds: [],
    onSelectedSavedQueryIdsChange: vi.fn(),
    onSendQuery: vi.fn(),
    onAbort: vi.fn(),
    get: vi.fn(() => Promise.resolve(null)),
    postData: vi.fn(() => Promise.resolve(null)),
    updateMessageResponse: vi.fn(),
    ...overrides,
  }
}

afterEach(() => {
  cleanup()
})

describe('ChatPanel Cancel button gating', () => {
  it('does not show Cancel when nothing is in flight', () => {
    render(<ChatPanel {...baseProps({ queryAction: null })} />)
    expect(screen.queryByText('ai_query.cancel')).toBeNull()
  })

  it('shows Cancel for the legacy (loading) in-flight path, as before', () => {
    render(<ChatPanel {...baseProps({ loading: true, queryAction: 'execute' })} />)
    expect(screen.getByText('ai_query.cancel')).toBeTruthy()
  })

  it('shows Cancel for an Agent Mode turn in flight even though useApi() loading stays false', () => {
    const onAbort = vi.fn()
    render(
      <ChatPanel
        {...baseProps({
          loading: false,
          agentModeEnabled: true,
          queryAction: 'execute',
          onAbort,
        })}
      />,
    )
    const cancelBtn = screen.getByText('ai_query.cancel')
    expect(cancelBtn).toBeTruthy()
    fireEvent.click(cancelBtn)
    expect(onAbort).toHaveBeenCalledTimes(1)
  })

  it('still hides Cancel when Agent Mode is enabled but no turn is in flight', () => {
    render(
      <ChatPanel {...baseProps({ loading: false, agentModeEnabled: true, queryAction: null })} />,
    )
    expect(screen.queryByText('ai_query.cancel')).toBeNull()
  })
})

describe('ChatPanel AgentTraceCard slot', () => {
  const steps: RunStep[] = [{ seq: 1, kind: 'list_models', status: 'ok', duration_ms: 5 }]
  const clarification: PendingAgentClarification = {
    runId: 'run-1',
    question: 'Which region did you mean?',
    choices: [{ id: 'emea', label: 'EMEA' }],
    allowFreeText: true,
  }

  it('renders nothing extra when Agent Mode is off, even with steps present', () => {
    render(
      <ChatPanel
        {...baseProps({ agentModeEnabled: false, agentTraceSteps: steps, queryAction: 'execute' })}
      />,
    )
    expect(screen.queryByTestId('agent-trace-card')).toBeNull()
  })

  it('renders the live trace while an Agent Mode turn is in flight', () => {
    render(
      <ChatPanel
        {...baseProps({ agentModeEnabled: true, agentTraceSteps: steps, queryAction: 'execute' })}
      />,
    )
    expect(screen.getByTestId('agent-trace-card')).toBeTruthy()
    expect(screen.getByText('ai_query.run_trace_step_list_models')).toBeTruthy()
  })

  it('renders the clarification card and wires its choice button to onAgentClarificationChoice', () => {
    const onChoice = vi.fn()
    render(
      <ChatPanel
        {...baseProps({
          agentModeEnabled: true,
          agentTraceSteps: steps,
          agentClarification: clarification,
          queryAction: null,
          onAgentClarificationChoice: onChoice,
        })}
      />,
    )
    expect(screen.getByText('Which region did you mean?')).toBeTruthy()
    fireEvent.click(screen.getByText('EMEA'))
    expect(onChoice).toHaveBeenCalledWith('emea')
  })
})

describe('ChatPanel Phase 3 composer', () => {
  it('uses one configuration trigger instead of loose toggles', () => {
    render(<ChatPanel {...baseProps()} />)
    expect(screen.getByRole('button', { name: /ai_query.agent_config/ })).toBeTruthy()
    expect(screen.queryByRole('checkbox')).toBeNull()
  })

  it('previews without execution and runs analysis as the primary action', () => {
    const onSendQuery = vi.fn()
    render(
      <ChatPanel
        {...baseProps({ datasourceId: 'ds-1', question: 'Revenue by month', onSendQuery })}
      />,
    )
    fireEvent.click(screen.getByRole('button', { name: 'ai_query.preview_plan_query' }))
    fireEvent.click(screen.getByRole('button', { name: /ai_query.run_analysis/ }))
    expect(onSendQuery).toHaveBeenNthCalledWith(1, 'Revenue by month', false)
    expect(onSendQuery).toHaveBeenNthCalledWith(2, 'Revenue by month', true)
  })

  it('keeps both actions disabled for whitespace-only input', () => {
    render(<ChatPanel {...baseProps({ datasourceId: 'ds-1', question: '   ' })} />)
    expect(
      screen.getByRole('button', { name: 'ai_query.preview_plan_query' }).hasAttribute('disabled'),
    ).toBe(true)
    expect(
      screen.getByRole('button', { name: /ai_query.run_analysis/ }).hasAttribute('disabled'),
    ).toBe(true)
  })

  it('presents AI Data Analyst onboarding and capability cards', () => {
    render(<ChatPanel {...baseProps()} />)
    expect(screen.getByText('ai_query.empty_analyst_title')).toBeTruthy()
    expect(screen.getByText('ai_query.empty_capability_explore_title')).toBeTruthy()
    expect(screen.getByText('ai_query.empty_capability_explain_title')).toBeTruthy()
    expect(screen.getByText('ai_query.empty_capability_visualize_title')).toBeTruthy()
    expect(screen.getAllByRole('button', { name: /ai_query.suggestion_/ })).toHaveLength(4)
  })

  it('rotates prompt guidance while the composer is idle', () => {
    vi.useFakeTimers()
    render(<ChatPanel {...baseProps()} />)
    expect(screen.getByRole('textbox').getAttribute('placeholder')).toBe('ai_query.placeholder')
    void act(() => vi.advanceTimersByTime(4_000))
    expect(screen.getByRole('textbox').getAttribute('placeholder')).toBe(
      'ai_query.placeholder_compare',
    )
    vi.useRealTimers()
  })
})
