import type { TFunction } from '../../i18n'
import { buttonClass } from '../../lib/buttonClasses'
import { cn } from '../../lib/cn'
import { sidebarHeaderTitleClass } from './aiQueryClasses'

interface ConversationSidebarHeaderProps {
  conversationCount: number
  search: string
  onSearchChange: (value: string) => void
  isExpanded: boolean
  onToggleExpanded: () => void
  onNewConversation: () => void
  t: TFunction
}

/** The AI Query sidebar header: title + count, New button, the mobile
 * expand/collapse chevron, and the conversation search box. Extracted from
 * AIQuery to keep the page component under the complexity cap. */
export function ConversationSidebarHeader({
  conversationCount,
  search,
  onSearchChange,
  isExpanded,
  onToggleExpanded,
  onNewConversation,
  t,
}: ConversationSidebarHeaderProps) {
  return (
    <div
      className={cn(
        'border-border mb-4 flex flex-col gap-3 border-b pb-3',
        'max-[900px]:mb-0 max-[900px]:flex-row max-[900px]:items-center max-[900px]:justify-between max-[900px]:gap-2 max-[900px]:border-b-0 max-[900px]:pb-0',
        isExpanded && 'max-[900px]:mb-4 max-[900px]:border-b max-[900px]:pb-3',
      )}
    >
      <div className="flex min-w-0 items-center gap-2">
        <h3 className={sidebarHeaderTitleClass}>{t('ai_query.conv_title')}</h3>
        {conversationCount > 0 && (
          <span className="bg-canvas-subtle border-border text-foreground-muted shrink-0 rounded-full border px-2 py-0.5 text-[0.7rem] font-semibold">
            {conversationCount}
          </span>
        )}
      </div>
      <div className="flex w-full shrink-0 flex-col items-center gap-2 max-[900px]:w-auto max-[900px]:flex-row">
        <button
          className={cn(
            buttonClass('primary', { size: 'sm' }),
            'flex w-full items-center justify-center max-[900px]:w-auto',
          )}
          onClick={onNewConversation}
        >
          {t('ai_query.conv_new')}
        </button>
        <button
          type="button"
          className="border-border bg-card-raised text-foreground hidden h-8 w-8 cursor-pointer items-center justify-center rounded-lg border transition-colors hover:bg-(--control-hover-bg) max-[900px]:inline-flex"
          onClick={onToggleExpanded}
          aria-label={isExpanded ? t('common.collapse_panel') : t('common.expand_panel')}
        >
          <svg
            className={cn('h-4 w-4 transition-transform duration-200', isExpanded && 'rotate-180')}
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            strokeWidth={2}
          >
            <path strokeLinecap="round" strokeLinejoin="round" d="M19 9l-7 7-7-7" />
          </svg>
        </button>
      </div>
      {conversationCount > 0 && (
        <input
          type="search"
          value={search}
          onChange={(e) => onSearchChange(e.target.value)}
          placeholder={t('ai_query.conv_search_placeholder')}
          aria-label={t('ai_query.conv_search_placeholder')}
          className="border-border bg-card-raised text-foreground focus-visible:border-accent w-full rounded-lg border px-3 py-1.5 text-[0.82rem] outline-none max-[900px]:hidden"
        />
      )}
    </div>
  )
}
