import type { ReactNode } from 'react'

import type { TFunction, TranslationKey } from '../../i18n'
import type { Conversation } from '../../types/ai'
import type { ConversationGroup, ConversationGroupKey } from './conversationGroups'

const CONVERSATION_GROUP_LABEL_KEYS: Record<ConversationGroupKey, TranslationKey> = {
  today: 'ai_query.group_today',
  yesterday: 'ai_query.group_yesterday',
  week: 'ai_query.group_this_week',
  month: 'ai_query.group_this_month',
  older: 'ai_query.group_older',
}

interface ConversationListProps {
  /** True when a non-empty search is active and produced zero matches. */
  emptySearch: boolean
  pinnedConversations: Conversation[]
  unpinnedGroups: ConversationGroup<Conversation>[]
  renderItem: (conv: Conversation) => ReactNode
  t: TFunction
}

function ConversationGroupBlock({
  heading,
  conversations,
  renderItem,
}: {
  heading: string
  conversations: Conversation[]
  renderItem: (conv: Conversation) => ReactNode
}) {
  return (
    <div className="flex flex-col gap-2">
      <h4 className="text-foreground-faint m-0 mt-1 px-1 text-[0.68rem] font-bold tracking-widest uppercase">
        {heading}
      </h4>
      {conversations.map((c) => renderItem(c))}
    </div>
  )
}

/** The grouped conversation list: a Pinned group above the recency groups,
 * or a "no matches" note when a search filtered everything out. Split from
 * AIQuery so the page component stays under the cyclomatic-complexity cap. */
export function ConversationList({
  emptySearch,
  pinnedConversations,
  unpinnedGroups,
  renderItem,
  t,
}: ConversationListProps) {
  if (emptySearch) {
    return (
      <p className="text-foreground-muted m-0 px-1 py-4 text-center text-[0.82rem]">
        {t('ai_query.conv_search_empty')}
      </p>
    )
  }
  return (
    <>
      {pinnedConversations.length > 0 && (
        <ConversationGroupBlock
          heading={t('ai_query.conv_group_pinned')}
          conversations={pinnedConversations}
          renderItem={renderItem}
        />
      )}
      {unpinnedGroups.map((group) => (
        <ConversationGroupBlock
          key={group.key}
          heading={t(CONVERSATION_GROUP_LABEL_KEYS[group.key])}
          conversations={group.items}
          renderItem={renderItem}
        />
      ))}
    </>
  )
}
