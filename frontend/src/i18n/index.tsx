import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'
import { en, type Dictionary } from './locales/en'
import { tr } from './locales/tr'

const STORAGE_KEY = 'biqly_locale'

const dictionaries = {
  en,
  tr,
} satisfies Record<string, Dictionary>

export type Locale = keyof typeof dictionaries
export const SUPPORTED_LOCALES = Object.keys(dictionaries) as Locale[]
export const DEFAULT_LOCALE: Locale = 'tr'
export const FALLBACK_LOCALE: Locale = 'en'
export const LOCALE_OPTIONS: Record<Locale, { label: string; short: string; languageTag: string }> = {
  en: { label: 'English', short: 'EN', languageTag: 'en-US' },
  tr: { label: 'Türkçe', short: 'TR', languageTag: 'tr-TR' },
}

export function localeLanguageTag(locale: Locale): string {
  return LOCALE_OPTIONS[locale]?.languageTag ?? LOCALE_OPTIONS[FALLBACK_LOCALE].languageTag
}

type LeafKeys<T, Prefix extends string = ''> = {
  [K in keyof T & string]: T[K] extends string
    ? `${Prefix}${K}`
    : T[K] extends Record<string, unknown>
      ? LeafKeys<T[K], `${Prefix}${K}.`>
      : never
}[keyof T & string]

export type TranslationKey = LeafKeys<Dictionary>

function readLocaleFromStorage(): Locale | null {
  if (typeof window === 'undefined') return null
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY)
    if (raw && (SUPPORTED_LOCALES as string[]).includes(raw)) {
      return raw as Locale
    }
  } catch {
    /* ignore */
  }
  return null
}

function detectBrowserLocale(): Locale {
  if (typeof navigator === 'undefined') return DEFAULT_LOCALE
  const candidates = [
    navigator.language,
    ...(Array.isArray(navigator.languages) ? navigator.languages : []),
  ]
  for (const raw of candidates) {
    if (!raw) continue
    const base = raw.toLowerCase().split(/[-_]/)[0] as Locale
    if ((SUPPORTED_LOCALES as string[]).includes(base)) return base
  }
  return DEFAULT_LOCALE
}

let currentLocale: Locale = readLocaleFromStorage() ?? detectBrowserLocale()

function applyLocaleSideEffects(locale: Locale) {
  currentLocale = locale
  if (typeof document !== 'undefined') {
    document.documentElement.lang = locale
  }
  try {
    if (typeof window !== 'undefined') {
      window.localStorage.setItem(STORAGE_KEY, locale)
    }
  } catch {
    /* ignore */
  }
}

applyLocaleSideEffects(currentLocale)

/** getLocale lets non-React layers (useApi, etc.) read the active locale. */
export function getLocale(): Locale {
  return currentLocale
}

function lookup(dict: Dictionary, key: string): string | undefined {
  const parts = key.split('.')
  let cur: unknown = dict
  for (const p of parts) {
    if (cur && typeof cur === 'object' && p in (cur as Record<string, unknown>)) {
      cur = (cur as Record<string, unknown>)[p]
    } else {
      return undefined
    }
  }
  return typeof cur === 'string' ? cur : undefined
}

function interpolate(template: string, params?: Record<string, string | number>): string {
  if (!params || !template.includes('{{')) return template
  return template.replace(/\{\{\s*([\w]+)\s*\}\}/g, (_, k) => {
    const v = params[k]
    return v === undefined || v === null ? `{{${k}}}` : String(v)
  })
}

function translate(locale: Locale, key: TranslationKey, params?: Record<string, string | number>): string {
  const primary = lookup(dictionaries[locale], key)
  if (primary !== undefined) return interpolate(primary, params)
  if (locale !== FALLBACK_LOCALE) {
    const fallback = lookup(dictionaries[FALLBACK_LOCALE], key)
    if (fallback !== undefined) return interpolate(fallback, params)
  }
  return key
}

interface I18nContextValue {
  locale: Locale
  setLocale: (loc: Locale) => void
  t: (key: TranslationKey, params?: Record<string, string | number>) => string
  supported: readonly Locale[]
}

const I18nContext = createContext<I18nContextValue | null>(null)

export function I18nProvider({ children }: { children: ReactNode }) {
  const [locale, setLocaleState] = useState<Locale>(currentLocale)

  useEffect(() => {
    applyLocaleSideEffects(locale)
  }, [locale])

  const setLocale = useCallback((loc: Locale) => {
    setLocaleState(loc)
  }, [])

  const t = useCallback(
    (key: TranslationKey, params?: Record<string, string | number>) => translate(locale, key, params),
    [locale],
  )

  const value = useMemo<I18nContextValue>(() => ({
    locale,
    setLocale,
    t,
    supported: SUPPORTED_LOCALES,
  }), [locale, setLocale, t])

  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>
}

export function useI18n(): I18nContextValue {
  const ctx = useContext(I18nContext)
  if (!ctx) {
    throw new Error('useI18n must be used within an <I18nProvider>')
  }
  return ctx
}

export function useT() {
  return useI18n().t
}

export function useLocale() {
  const { locale, setLocale } = useI18n()
  return [locale, setLocale] as const
}
