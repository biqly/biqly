// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import type { TranslationKey } from '../../i18n'
import { AgentConfigurationPopover } from './AgentConfigurationPopover'

const t = (key: TranslationKey, params?: Record<string, string | number>) =>
  params ? `${key}:${JSON.stringify(params)}` : key

afterEach(cleanup)

describe('AgentConfigurationPopover', () => {
  it('summarizes active settings and exposes the configuration as an accessible dialog', () => {
    render(
      <AgentConfigurationPopover
        t={t}
        contextAvailable
        contextEnabled
        onContextEnabledChange={vi.fn()}
        autoFindEnabled
        onAutoFindEnabledChange={vi.fn()}
        agentModeEnabled={false}
        onAgentModeEnabledChange={vi.fn()}
      />,
    )

    const trigger = screen.getByRole('button', { name: /ai_query.agent_config/i })
    expect(trigger.getAttribute('aria-expanded')).toBe('false')
    fireEvent.click(trigger)
    expect(trigger.getAttribute('aria-expanded')).toBe('true')
    expect(screen.getByRole('dialog')).toBeTruthy()
    expect(screen.getByText(/"count":2/)).toBeTruthy()
  })

  it('wires each setting and returns focus after Escape', () => {
    const onContext = vi.fn()
    const onAutoFind = vi.fn()
    const onAgentMode = vi.fn()
    render(
      <AgentConfigurationPopover
        t={t}
        contextAvailable
        contextEnabled
        onContextEnabledChange={onContext}
        autoFindEnabled
        onAutoFindEnabledChange={onAutoFind}
        agentModeEnabled={false}
        onAgentModeEnabledChange={onAgentMode}
      />,
    )

    const trigger = screen.getByRole('button', { name: /ai_query.agent_config/i })
    fireEvent.click(trigger)
    fireEvent.click(screen.getByRole('checkbox', { name: /^ai_query.context_toggle/ }))
    fireEvent.click(screen.getByRole('checkbox', { name: /^ai_query.auto_find_toggle/ }))
    fireEvent.click(screen.getByRole('checkbox', { name: /^ai_query.agent_mode_toggle/ }))
    expect(onContext).toHaveBeenCalledWith(false)
    expect(onAutoFind).toHaveBeenCalledWith(false)
    expect(onAgentMode).toHaveBeenCalledWith(true)

    fireEvent.keyDown(document, { key: 'Escape' })
    expect(screen.queryByRole('dialog')).toBeNull()
    expect(document.activeElement).toBe(trigger)
  })

  it('omits conversation context when no conversation history exists', () => {
    render(
      <AgentConfigurationPopover
        t={t}
        contextAvailable={false}
        contextEnabled={false}
        onContextEnabledChange={vi.fn()}
        autoFindEnabled={false}
        onAutoFindEnabledChange={vi.fn()}
        agentModeEnabled={false}
        onAgentModeEnabledChange={vi.fn()}
      />,
    )
    fireEvent.click(screen.getByRole('button', { name: /ai_query.agent_config/i }))
    expect(screen.queryByRole('checkbox', { name: /^ai_query.context_toggle/ })).toBeNull()
    expect(screen.getByText('ai_query.agent_config_caps_managed')).toBeTruthy()
  })
})
