import { describe, expect, it } from 'vitest'

import { resolveSelectedTableKey } from './useTableBrowserPage'

const tableOptions = [
  { value: 'public.profiles', label: 'public.profiles' },
  { value: 'public.timeline_tweets', label: 'public.timeline_tweets' },
  { value: 'public.tracked_profiles', label: 'public.tracked_profiles' },
]

describe('resolveSelectedTableKey', () => {
  it('keeps a selected joined table when it exists in the model table options', () => {
    expect(resolveSelectedTableKey('public.timeline_tweets', 'public.profiles', tableOptions)).toBe(
      'public.profiles',
    )
  })

  it('falls back to the base table when the selection is stale', () => {
    expect(
      resolveSelectedTableKey('public.timeline_tweets', 'public.deleted_table', tableOptions),
    ).toBe('public.timeline_tweets')
  })
})
