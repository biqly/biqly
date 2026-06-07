import {
  createContext,
  type ReactNode,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
} from 'react'

import type { Dictionary, LocaleSectionName } from './locales/dictionary'
import { core as enCore } from './locales/en/core'
import { core as trCore } from './locales/tr/core'

export type { Dictionary } from './locales/dictionary'

const STORAGE_KEY = 'biqly_locale'

export type Locale = 'en' | 'tr'
export const SUPPORTED_LOCALES: Locale[] = ['en', 'tr']
export const DEFAULT_LOCALE: Locale = 'tr'
export const FALLBACK_LOCALE: Locale = 'en'
export const LOCALE_OPTIONS: Record<Locale, { label: string; short: string; languageTag: string }> =
  {
    en: { label: 'English', short: 'EN', languageTag: 'en-US' },
    tr: { label: 'Türkçe', short: 'TR', languageTag: 'tr-TR' },
  }

export function localeLanguageTag(locale: Locale): string {
  return LOCALE_OPTIONS[locale].languageTag
}

type LeafKeys<T, Prefix extends string = ''> = {
  [K in keyof T & string]: T[K] extends string
    ? `${Prefix}${K}`
    : T[K] extends Record<string, unknown>
      ? LeafKeys<T[K], `${Prefix}${K}.`>
      : never
}[keyof T & string]

export type TranslationKey = LeafKeys<Dictionary>

// --- Runtime dictionary registry ---------------------------------------------
// Core text ships in the main bundle; `admin`/`auth` sections are loaded from
// their own chunks on demand and merged in here. Lookups fall back gracefully
// (to the fallback locale, then to the key) until a section has loaded.

type PartialDict = Record<string, unknown>

const dictionaries: Record<Locale, PartialDict> = {
  en: { ...enCore },
  tr: { ...trCore },
}

const sectionLoaders: Record<Locale, Record<LocaleSectionName, () => Promise<PartialDict>>> = {
  en: {
    admin: () => import('./locales/en/admin').then((m) => ({ admin: m.admin })),
    auth: () => import('./locales/en/auth').then((m) => ({ auth: m.auth })),
  },
  tr: {
    admin: () => import('./locales/tr/admin').then((m) => ({ admin: m.admin })),
    auth: () => import('./locales/tr/auth').then((m) => ({ auth: m.auth })),
  },
}

const loadedSections = new Set<string>()
const inFlight = new Map<string, Promise<void>>()
const listeners = new Set<() => void>()

function notify() {
  listeners.forEach((l) => l())
}

/** Load (once) a lazy locale section and merge it into the active registry. */
export function loadLocaleSection(locale: Locale, section: LocaleSectionName): Promise<void> {
  const key = `${locale}:${section}`
  if (loadedSections.has(key)) {
    return Promise.resolve()
  }
  const existing = inFlight.get(key)
  if (existing) {
    return existing
  }
  const p = sectionLoaders[locale][section]()
    .then((mod) => {
      dictionaries[locale] = { ...dictionaries[locale], ...mod }
      loadedSections.add(key)
      inFlight.delete(key)
      notify()
    })
    .catch((err) => {
      inFlight.delete(key)
      throw err
    })
  inFlight.set(key, p)
  return p
}

function sectionReady(locale: Locale, section: LocaleSectionName): boolean {
  return loadedSections.has(`${locale}:${section}`)
}

function readLocaleFromStorage(): Locale | null {
  if (typeof window === 'undefined') {
    return null
  }
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
  if (typeof navigator === 'undefined') {
    return DEFAULT_LOCALE
  }
  const language = typeof navigator.language === 'string' ? navigator.language : ''
  const languages = Array.isArray(navigator.languages)
    ? navigator.languages.filter((lang): lang is string => typeof lang === 'string')
    : []
  const candidates: string[] = [language, ...languages]
  for (const raw of candidates) {
    if (!raw) {
      continue
    }
    const base = raw.toLowerCase().split(/[-_]/)[0] as Locale
    if ((SUPPORTED_LOCALES as string[]).includes(base)) {
      return base
    }
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

function lookup(dict: PartialDict, key: string): string | undefined {
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
  if (!params || !template.includes('{{')) {
    return template
  }
  return template.replace(/\{\{\s*([\w]+)\s*\}\}/g, (_, key: string) => {
    const v = params[key]
    return v === undefined ? `{{${key}}}` : String(v)
  })
}

function translate(
  locale: Locale,
  key: TranslationKey,
  params?: Record<string, string | number>,
): string {
  const primary = lookup(dictionaries[locale], key)
  if (primary !== undefined) {
    return interpolate(primary, params)
  }
  if (locale !== FALLBACK_LOCALE) {
    const fallback = lookup(dictionaries[FALLBACK_LOCALE], key)
    if (fallback !== undefined) {
      return interpolate(fallback, params)
    }
  }
  return key
}

interface I18nContextValue {
  locale: Locale
  setLocale: (loc: Locale) => void
  t: (key: TranslationKey, params?: Record<string, string | number>) => string
  supported: readonly Locale[]
}

export type TFunction = I18nContextValue['t']

/** For call sites with dynamic or not-yet-indexed translation keys. */
export type LooseTFunction = (key: string, params?: Record<string, string | number>) => string

const I18nContext = createContext<I18nContextValue | null>(null)

export function I18nProvider({ children }: { children: ReactNode }) {
  const [locale, setLocaleState] = useState<Locale>(currentLocale)
  // Bumped whenever a lazy locale section finishes loading, so `t` consumers re-render.
  const [version, setVersion] = useState(0)

  useEffect(() => {
    applyLocaleSideEffects(locale)
  }, [locale])

  useEffect(() => {
    const listener = () => setVersion((v) => v + 1)
    listeners.add(listener)
    return () => {
      listeners.delete(listener)
    }
  }, [])

  const setLocale = useCallback((loc: Locale) => {
    setLocaleState(loc)
  }, [])

  const t = useCallback(
    (key: TranslationKey, params?: Record<string, string | number>) =>
      translate(locale, key, params),
    // version participates so consumers update when admin/auth chunks land.
    [locale, version],
  )

  const value = useMemo<I18nContextValue>(
    () => ({
      locale,
      setLocale,
      t,
      supported: SUPPORTED_LOCALES,
    }),
    [locale, setLocale, t],
  )

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

/**
 * Ensure a lazy locale section is loaded for the active locale (and the
 * fallback locale, so missing keys still resolve). Returns true once ready.
 */
export function useLocaleSection(section: LocaleSectionName): boolean {
  const { locale } = useI18n()
  const [ready, setReady] = useState(
    () => sectionReady(locale, section) && sectionReady(FALLBACK_LOCALE, section),
  )

  useEffect(() => {
    let cancelled = false
    setReady(sectionReady(locale, section) && sectionReady(FALLBACK_LOCALE, section))
    void Promise.all([
      loadLocaleSection(locale, section),
      loadLocaleSection(FALLBACK_LOCALE, section),
    ]).then(() => {
      if (!cancelled) {
        setReady(true)
      }
    })
    return () => {
      cancelled = true
    }
  }, [locale, section])

  return ready
}

/** Gates children until the named lazy locale section has loaded. */
export function LocaleSection({
  name,
  children,
  fallback = null,
}: {
  name: LocaleSectionName
  children: ReactNode
  fallback?: ReactNode
}) {
  const ready = useLocaleSection(name)
  return <>{ready ? children : fallback}</>
}
