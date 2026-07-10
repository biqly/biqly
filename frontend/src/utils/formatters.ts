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

/**
 * Locale-aware date+time display (the repeated
 * `new Date(x).toLocaleString(localeLanguageTag(locale))` screen pattern).
 * Unparseable input falls through as-is (AuditLog convention).
 */
export function formatDateTime(value: string | Date, languageTag: string): string {
  const date = value instanceof Date ? value : new Date(value)
  if (Number.isNaN(date.getTime())) {
    return typeof value === 'string' ? value : ''
  }
  return date.toLocaleString(languageTag)
}

/** Locale-aware date-only display; unparseable input falls through as-is. */
export function formatDateOnly(value: string | Date, languageTag: string): string {
  const date = value instanceof Date ? value : new Date(value)
  if (Number.isNaN(date.getTime())) {
    return typeof value === 'string' ? value : ''
  }
  return date.toLocaleDateString(languageTag)
}

/** Locale-aware time-only display for compact chat/message timestamps. */
export function formatTimeOnly(value: string | Date, languageTag: string): string {
  const date = value instanceof Date ? value : new Date(value)
  if (Number.isNaN(date.getTime())) {
    return typeof value === 'string' ? value : ''
  }
  return date.toLocaleTimeString(languageTag, { hour: '2-digit', minute: '2-digit' })
}

/** Safe display string for unknown cell/query values (avoids implicit object toString). */
/** Human-readable duration from milliseconds — ms, seconds, or minutes as
 * appropriate, localized when a language tag is given ("34,4 sn", "2,6 dk."). */
export function formatDurationMs(ms: number, languageTag = 'en'): string {
  if (!Number.isFinite(ms) || ms <= 0) {
    return '—'
  }
  const unitFormat = (value: number, unit: string, maxFraction: number) =>
    new Intl.NumberFormat(languageTag, {
      style: 'unit',
      unit,
      unitDisplay: 'short',
      maximumFractionDigits: maxFraction,
    }).format(value)
  if (ms < 1000) {
    return unitFormat(Math.round(ms), 'millisecond', 0)
  }
  if (ms < 60_000) {
    return unitFormat(ms / 1000, 'second', 1)
  }
  if (ms < 3_600_000) {
    return unitFormat(ms / 60_000, 'minute', 1)
  }
  return unitFormat(ms / 3_600_000, 'hour', 1)
}

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
