// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { I18nProvider, type TranslationKey } from '../../i18n'
import { ModelingToolLaunchers, ModelingToolsModal } from './ModelingToolsModal'

const t = (key: TranslationKey) => key

afterEach(cleanup)

describe('ModelingToolLaunchers', () => {
  it('opens the shared tools surface on the selected workflow', () => {
    const onOpen = vi.fn()
    render(<ModelingToolLaunchers tableCount={12} relationshipCount={3} onOpen={onOpen} t={t} />)

    fireEvent.click(screen.getByRole('button', { name: 'modeling.open_semantic_panel' }))
    expect(onOpen).toHaveBeenLastCalledWith('semantic')

    fireEvent.click(screen.getByRole('button', { name: 'modeling.open_join_panel' }))
    expect(onOpen).toHaveBeenLastCalledWith('relationship')
  })
})

describe('ModelingToolsModal', () => {
  it('shows one selected workflow and exposes the other as a tab', () => {
    const onTabChange = vi.fn()
    render(
      <I18nProvider>
        <ModelingToolsModal
          activeTab="semantic"
          onTabChange={onTabChange}
          onClose={vi.fn()}
          semanticContent={<div>Semantic workflow</div>}
          relationshipContent={<div>Relationship workflow</div>}
          t={t}
        />
      </I18nProvider>,
    )

    expect(screen.getByRole('dialog')).toBeTruthy()
    expect(screen.getByText('Semantic workflow')).toBeTruthy()
    expect(screen.queryByText('Relationship workflow')).toBeNull()

    const semanticTab = screen.getByRole('tab', { name: 'modeling.semantic_layer' })
    expect(semanticTab.getAttribute('aria-selected')).toBe('true')

    fireEvent.click(screen.getByRole('tab', { name: 'modeling.manual_relationship' }))
    expect(onTabChange).toHaveBeenCalledWith('relationship')
  })

  it('moves between workflows with arrow keys', () => {
    const onTabChange = vi.fn()
    render(
      <I18nProvider>
        <ModelingToolsModal
          activeTab="semantic"
          onTabChange={onTabChange}
          onClose={vi.fn()}
          semanticContent={<div>Semantic workflow</div>}
          relationshipContent={<div>Relationship workflow</div>}
          t={t}
        />
      </I18nProvider>,
    )

    fireEvent.keyDown(screen.getByRole('tab', { name: 'modeling.semantic_layer' }), {
      key: 'ArrowRight',
    })

    expect(onTabChange).toHaveBeenCalledWith('relationship')
  })

  it('places initial focus on the workflow used to open the modal', () => {
    render(
      <I18nProvider>
        <ModelingToolsModal
          activeTab="relationship"
          onTabChange={vi.fn()}
          onClose={vi.fn()}
          semanticContent={<div>Semantic workflow</div>}
          relationshipContent={<div>Relationship workflow</div>}
          t={t}
        />
      </I18nProvider>,
    )

    expect(screen.getByRole('tab', { name: 'modeling.manual_relationship' })).toBe(
      document.activeElement,
    )
  })
})
