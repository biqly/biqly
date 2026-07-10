// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import type { TranslationKey } from '../../i18n'
import { PromptTextarea } from './PromptTextarea'

const t = (key: TranslationKey, params?: Record<string, string | number>) =>
  params ? `${key}:${JSON.stringify(params)}` : key

const baseProps = {
  value: '',
  onChange: vi.fn(),
  onSubmit: vi.fn(),
  onAbort: vi.fn(),
  disabled: false,
  inFlight: false,
  placeholder: 'Ask a question',
  items: [],
  savedQueries: [],
  selectedSavedQueryIds: [],
  onSelectedSavedQueryIdsChange: vi.fn(),
  t,
}

afterEach(() => {
  cleanup()
  vi.clearAllMocks()
})

describe('PromptTextarea command tokens', () => {
  it('labels the composer and inserts a selected # business term', () => {
    const onChange = vi.fn()
    render(
      <PromptTextarea
        {...baseProps}
        value="#rev"
        onChange={onChange}
        items={[
          {
            type: 'term',
            name: 'revenue',
            label: 'Revenue',
            description: 'Recognized revenue after refunds',
            group: 'Business terms',
          },
        ]}
      />,
    )

    const textarea = screen.getByRole<HTMLTextAreaElement>('textbox', {
      name: 'ai_query.question_label',
    })
    textarea.setSelectionRange(4, 4)
    fireEvent.keyUp(textarea, { key: 'v' })
    fireEvent.mouseDown(screen.getByRole('option', { name: /Revenue/ }))
    expect(onChange).toHaveBeenCalledWith('#revenue ')
  })

  it('aborts with Escape whenever a run is in flight', () => {
    const onAbort = vi.fn()
    render(<PromptTextarea {...baseProps} inFlight onAbort={onAbort} />)
    fireEvent.keyDown(screen.getByRole('textbox'), { key: 'Escape' })
    expect(onAbort).toHaveBeenCalledTimes(1)
  })
})
