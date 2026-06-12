export type PageToken = number | 'gap' | 'pad'

/**
 * Stable 7-slot page window: the token count never changes while navigating,
 * so the control keeps a constant width (slots are min-width reserved).
 */
export function buildStablePageTokens(current: number, total: number): PageToken[] {
  const SLOTS = 7
  if (total <= SLOTS) {
    const tokens: PageToken[] = Array.from({ length: total }, (_, i) => i + 1)
    while (tokens.length < SLOTS) {
      tokens.push('pad')
    }
    return tokens
  }
  if (current <= 4) {
    return [1, 2, 3, 4, 5, 'gap', total]
  }
  if (current >= total - 3) {
    return [1, 'gap', total - 4, total - 3, total - 2, total - 1, total]
  }
  return [1, 'gap', current - 1, current, current + 1, 'gap', total]
}
