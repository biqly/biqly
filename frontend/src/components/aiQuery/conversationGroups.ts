// Groups the conversation sidebar by recency (Today / Yesterday / This week /
// This month / Older), newest first inside each group.

export type ConversationGroupKey = 'today' | 'yesterday' | 'week' | 'month' | 'older'

export const CONVERSATION_GROUP_ORDER: ConversationGroupKey[] = [
  'today',
  'yesterday',
  'week',
  'month',
  'older',
]

function startOfDay(d: Date): number {
  const copy = new Date(d)
  copy.setHours(0, 0, 0, 0)
  return copy.getTime()
}

/** conversationGroupKey buckets a timestamp relative to `now`. Invalid or
 * missing timestamps land in "older" (they cannot be recent work). */
export function conversationGroupKey(
  updatedAt: string | undefined,
  now: Date,
): ConversationGroupKey {
  const ts = updatedAt ? new Date(updatedAt).getTime() : NaN
  if (!Number.isFinite(ts)) {
    return 'older'
  }
  const today = startOfDay(now)
  const dayMs = 24 * 60 * 60 * 1000
  if (ts >= today) {
    return 'today'
  }
  if (ts >= today - dayMs) {
    return 'yesterday'
  }
  if (ts >= today - 6 * dayMs) {
    return 'week'
  }
  if (ts >= today - 29 * dayMs) {
    return 'month'
  }
  return 'older'
}

export interface ConversationGroup<T> {
  key: ConversationGroupKey
  items: T[]
}

/** groupConversationsByRecency buckets and orders conversations for the
 * sidebar: fixed group order, newest first within each group. */
export function groupConversationsByRecency<T extends { updated_at?: string }>(
  items: T[],
  now: Date,
): ConversationGroup<T>[] {
  const buckets = new Map<ConversationGroupKey, T[]>()
  for (const item of items) {
    const key = conversationGroupKey(item.updated_at, now)
    const bucket = buckets.get(key)
    if (bucket) {
      bucket.push(item)
    } else {
      buckets.set(key, [item])
    }
  }
  return CONVERSATION_GROUP_ORDER.filter((key) => buckets.has(key)).map((key) => ({
    key,
    items: [...buckets.get(key)!].sort(
      (a, b) => new Date(b.updated_at ?? 0).getTime() - new Date(a.updated_at ?? 0).getTime(),
    ),
  }))
}
