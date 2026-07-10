import type { TFunction } from '../../i18n'
import type { SuggestedFollowUp } from '../../types/ai'

// This file is named `FollowUpSuggestionsSection.tsx` rather than
// `FollowUpSuggestions.tsx`: this directory already has `followUpSuggestions.ts`
// (Task 3's dedup/filter helper), and the two names differ only by the
// leading letter's case. On a case-insensitive filesystem (default on
// macOS/Windows) that collision makes TypeScript's own directory scan for
// `include` silently drop one of the two files from the Program — breaking
// `tsc` and typed ESLint outright, not just import resolution. The exported
// component below is still named `FollowUpSuggestions`, matching the
// component API from the plan; only the file name differs.
interface FollowUpSuggestionsProps {
  suggestions: SuggestedFollowUp[]
  onSelect: (question: string) => void
  t: TFunction
}

export function FollowUpSuggestions({ suggestions, onSelect, t }: FollowUpSuggestionsProps) {
  if (suggestions.length === 0) {
    return null
  }

  return (
    <section
      className="border-border/70 mt-1 flex flex-wrap items-center gap-2 border-t pt-2.5"
      aria-label={t('ai_query.followups_title')}
    >
      <p className="text-foreground-muted m-0 text-[0.78rem] font-medium">
        {t('ai_query.followups_title')}
      </p>
      <div className="flex min-w-0 flex-wrap gap-1.5">
        {suggestions.map((suggestion) => (
          <button
            key={suggestion.id}
            type="button"
            className="border-border bg-surface-2 text-text hover:border-accent hover:text-accent focus-visible:ring-accent rounded-md border px-2.5 py-1 text-[0.78rem] transition focus-visible:ring-2 focus-visible:outline-none"
            aria-label={t('ai_query.followups_apply_aria', { question: suggestion.question })}
            onClick={() => onSelect(suggestion.question)}
          >
            {suggestion.label}
          </button>
        ))}
      </div>
    </section>
  )
}
