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

/** Safe display string for unknown cell/query values (avoids implicit object toString). */
/** Human-readable duration from milliseconds (ms, s, or min as appropriate). */
export function formatDurationMs(ms: number): string {
  if (!Number.isFinite(ms) || ms <= 0) {
    return '—'
  }
  const rounded = Math.round(ms)
  if (rounded < 1000) {
    return `${rounded} ms`
  }
  const totalSeconds = rounded / 1000
  if (totalSeconds < 60) {
    return totalSeconds < 10 ? `${totalSeconds.toFixed(1)} s` : `${Math.round(totalSeconds)} s`
  }
  const minutes = Math.floor(totalSeconds / 60)
  const seconds = Math.round(totalSeconds % 60)
  if (minutes < 60) {
    if (seconds === 0) {
      return `${minutes} min`
    }
    return `${minutes} min ${seconds} s`
  }
  const hours = Math.floor(minutes / 60)
  const remMinutes = minutes % 60
  if (remMinutes === 0) {
    return `${hours} h`
  }
  return `${hours} h ${remMinutes} min`
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
