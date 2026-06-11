import type { TranslationKey } from '../../i18n'

export type JoinTypeKey = 'left' | 'inner' | 'right' | 'full'

export function normalizeJoinType(raw: string): JoinTypeKey | null {
  const v = raw.trim().toLowerCase()
  if (v === 'left' || v === 'inner' || v === 'right' || v === 'full') {
    return v
  }
  return null
}

export function joinTypeHintKey(raw: string): TranslationKey | null {
  const type = normalizeJoinType(raw)
  if (!type) {
    return null
  }
  return `modeling.join_hint_${type}`
}
