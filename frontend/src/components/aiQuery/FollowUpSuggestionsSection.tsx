import type { LooseTFunction } from '../../i18n'
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
//
// `t` is typed as `LooseTFunction` (not `TFunction`) because the i18n keys
// this component references (`ai_query.followups_title`,
// `ai_query.followups_apply_aria`) don't exist yet — a later task adds them
// to the strict `TranslationKey` union. `LooseTFunction` is the same
// escape hatch `ExpressionBuilder.tsx` uses for this exact situation. Switch
// back to `TFunction` once those keys land.
interface FollowUpSuggestionsProps {
  suggestions: SuggestedFollowUp[]
  onSelect: (question: string) => void
  t: LooseTFunction
}

export function FollowUpSuggestions({ suggestions, onSelect, t }: FollowUpSuggestionsProps) {
  if (suggestions.length === 0) {
    return null
  }

  return (
    <section
      className="border-border/70 mt-4 border-t pt-3"
      aria-label={t('ai_query.followups_title')}
    >
      <p className="text-text mb-2 text-sm font-medium">{t('ai_query.followups_title')}</p>
      <div className="flex flex-wrap gap-2">
        {suggestions.map((suggestion) => (
          <button
            key={suggestion.id}
            type="button"
            className="border-border bg-surface-2 text-text hover:border-accent hover:text-accent focus-visible:ring-accent rounded-md border px-3 py-1.5 text-sm transition focus-visible:ring-2 focus-visible:outline-none"
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
