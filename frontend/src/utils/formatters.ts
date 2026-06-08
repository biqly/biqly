import type { Locale } from '../i18n'
import { localeLanguageTag } from '../i18n'

export function getRateColor(rate: number): string {
  if (rate >= 80) {
    return 'var(--success)'
  }
  if (rate >= 50) {
    return 'var(--warning)'
  }
  return 'var(--error)'
}

export function localeNumberTag(locale: Locale): string {
  return localeLanguageTag(locale)
}

/** Safe display string for unknown cell/query values (avoids implicit object toString). */
export function unknownToDisplayString(value: unknown): string {
  if (value === null || value === undefined) {
    return ''
  }
  if (typeof value === 'string') {
    return value
  }
  if (typeof value === 'number' || typeof value === 'boolean' || typeof value === 'bigint') {
    return String(value)
  }
  if (typeof value === 'object') {
    return JSON.stringify(value)
  }
  if (typeof value === 'symbol') {
    return value.description ?? ''
  }
  return ''
}
