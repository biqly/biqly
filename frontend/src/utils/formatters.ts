import { localeLanguageTag } from '../i18n'
import type { Locale } from '../i18n'

export function getRateColor(rate: number): string {
  if (rate >= 80) return 'var(--success)'
  if (rate >= 50) return 'var(--warning)'
  return 'var(--error)'
}

export function localeNumberTag(locale: Locale): string {
  return localeLanguageTag(locale)
}
