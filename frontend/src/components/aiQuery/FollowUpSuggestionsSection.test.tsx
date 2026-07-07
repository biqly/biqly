// @vitest-environment jsdom
//
// This suite renders through real jsdom + @testing-library/react (rather
// than the renderToStaticMarkup + markup-string pattern used elsewhere in
// this directory, e.g. RunTrace.test.tsx) because it must exercise actual
// click/focus behavior, which a static string render cannot do. Neither
// jsdom nor @testing-library/react were previously installed in this repo;
// both were added as devDependencies for this component test. The
// `@vitest-environment jsdom` docblock above scopes the jsdom environment to
// this file only, leaving the suite's default `node` environment untouched
// for every other test.
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import type { SuggestedFollowUp } from '../../types/ai'
// Named `FollowUpSuggestionsSection` (not `FollowUpSuggestions`) on disk:
// this directory already has `followUpSuggestions.ts` (Task 3's helper), and
// `FollowUpSuggestions.tsx`/`followUpSuggestions.ts` differ only by the
// leading letter's case. On a case-insensitive filesystem (default on
// macOS/Windows) that collision made TypeScript's own directory scan for
// `include` silently drop one of the two files from the Program — breaking
// `tsc`/`eslint`'s typed linting outright, not just import resolution. The
// exported component is still named `FollowUpSuggestions`, matching the
// component API in the plan; only the file name differs.
import { FollowUpSuggestions } from './FollowUpSuggestionsSection'

// Identity stub: t(key, params) returns the key plus its serialized params,
// so assertions can check for the exact key and/or param values without a
// real dictionary. Same pattern as RunTrace.test.tsx and
// suggestedQuestions.test.ts in this directory.
const t = (key: string, params?: Record<string, string | number>) =>
  params ? `${key}:${JSON.stringify(params)}` : key

function suggestion(overrides: Partial<SuggestedFollowUp> = {}): SuggestedFollowUp {
  return {
    id: 'sf-1',
    label: 'Break down by region',
    question: 'Can you break this down by region?',
    kind: 'breakdown',
    ...overrides,
  }
}

afterEach(() => {
  cleanup()
})

describe('FollowUpSuggestions', () => {
  it('renders nothing when there are no suggestions', () => {
    const { container } = render(<FollowUpSuggestions suggestions={[]} onSelect={vi.fn()} t={t} />)
    expect(container.innerHTML).toBe('')
  })

  it('renders one button per suggestion with the label as visible text', () => {
    const suggestions = [
      suggestion({ id: 'sf-1', label: 'Break down by region' }),
      suggestion({
        id: 'sf-2',
        label: 'Compare to last quarter',
        question: 'Compare to last quarter?',
      }),
    ]
    render(<FollowUpSuggestions suggestions={suggestions} onSelect={vi.fn()} t={t} />)

    const [first, second] = screen.getAllByRole('button')
    expect(first?.textContent).toBe('Break down by region')
    expect(second?.textContent).toBe('Compare to last quarter')
  })

  it('sets type="button" on every suggestion button', () => {
    render(<FollowUpSuggestions suggestions={[suggestion()]} onSelect={vi.fn()} t={t} />)
    expect(screen.getByRole('button').getAttribute('type')).toBe('button')
  })

  it('includes the suggestion question in the button aria-label', () => {
    const s = suggestion({ question: 'Can you break this down by region?' })
    render(<FollowUpSuggestions suggestions={[s]} onSelect={vi.fn()} t={t} />)

    const button = screen.getByRole('button')
    expect(button.getAttribute('aria-label')).toContain(s.question)
  })

  it('calls onSelect with the suggestion question when clicked', () => {
    const onSelect = vi.fn()
    const s = suggestion({ question: 'Can you break this down by region?' })
    render(<FollowUpSuggestions suggestions={[s]} onSelect={onSelect} t={t} />)

    fireEvent.click(screen.getByRole('button'))

    expect(onSelect).toHaveBeenCalledTimes(1)
    expect(onSelect).toHaveBeenCalledWith(s.question)
  })

  it('renders keyboard-focusable native buttons', () => {
    render(<FollowUpSuggestions suggestions={[suggestion()]} onSelect={vi.fn()} t={t} />)
    const button = screen.getByRole('button')
    expect(button.tagName).toBe('BUTTON')
    expect(button.tabIndex).not.toBe(-1)

    button.focus()
    expect(document.activeElement).toBe(button)
  })
})
