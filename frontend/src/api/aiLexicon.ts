import { buildQueryString } from '../utils/query'
import { apiFetch } from './apiClient'
import { ADMIN_OPTS, AI_API_BASE } from './constants'

// Lexicon domains, mirroring internal/ai/lexicon.Domains. The NL lexicon is the
// admin-managed vocabulary the AI uses for time-grain / count / soft-delete /
// intent detection; it is seeded from embedded defaults and editable here.
export const LEXICON_DOMAINS = [
  'temporal_phrase',
  'grain_synonym',
  'soft_delete',
  'intent_token',
  'row_count',
  'token_synonym',
  'metric_synonym',
] as const

export type LexiconDomain = (typeof LEXICON_DOMAINS)[number]

// TEMPORAL_LEXICON_DOMAIN stores interpretation_keys instead of terms.
export const TEMPORAL_LEXICON_DOMAIN: LexiconDomain = 'temporal_phrase'

export interface LexiconEntry {
  locale: string
  domain: LexiconDomain
  key: string
  terms?: string[]
  interpretation_keys?: string[]
  is_active?: boolean
}

export const listLexicon = (opts: { locale?: string; domain?: string } = {}) =>
  apiFetch<{ entries: LexiconEntry[] }>(
    'GET',
    `${AI_API_BASE}/admin/lexicon${buildQueryString({ locale: opts.locale, domain: opts.domain })}`,
    undefined,
    ADMIN_OPTS,
  )

export const upsertLexicon = (entries: LexiconEntry[]) =>
  apiFetch<{ updated: number }>('PUT', `${AI_API_BASE}/admin/lexicon`, { entries }, ADMIN_OPTS)

export const resetLexiconDomain = (domain: LexiconDomain) =>
  apiFetch<{ domain: string; restored: number }>(
    'POST',
    `${AI_API_BASE}/admin/lexicon/reset`,
    { domain },
    ADMIN_OPTS,
  )
