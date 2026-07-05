import { useCallback, useEffect, useMemo, useState } from 'react'

import {
  LEXICON_DOMAINS,
  type LexiconDomain,
  type LexiconEntry,
  listLexicon,
  resetLexiconDomain,
  TEMPORAL_LEXICON_DOMAIN,
  upsertLexicon,
} from '../../api/aiLexicon'
import { useConfirmedMutation } from '../../hooks/useConfirmedMutation'
import { useToast } from '../../hooks/useToast'
import { type TranslationKey, useT } from '../../i18n'
import { errorMessage } from '../../utils/error'
import { Button } from '../ui/Button'
import { LoadingScreen } from '../ui/LoadingScreen'
import { Select } from '../ui/Select'
import {
  adminInputClass,
  adminTableClass,
  adminTableContainerClass,
  adminTdClass,
  adminTextMutedClass,
  adminThClass,
  adminTheadRowClass,
  adminTrClass,
} from './adminClasses'
import { AdminPanelShell } from './AdminPanelShell'

const DOMAIN_LABEL_KEYS: Record<LexiconDomain, TranslationKey> = {
  temporal_phrase: 'admin.nl_lexicon.domain.temporal_phrase',
  grain_synonym: 'admin.nl_lexicon.domain.grain_synonym',
  soft_delete: 'admin.nl_lexicon.domain.soft_delete',
  intent_token: 'admin.nl_lexicon.domain.intent_token',
  row_count: 'admin.nl_lexicon.domain.row_count',
  token_synonym: 'admin.nl_lexicon.domain.token_synonym',
  metric_synonym: 'admin.nl_lexicon.domain.metric_synonym',
}

// DraftRow is an editable table row; values holds the comma-separated terms (or
// interpretation keys for the temporal_phrase domain).
interface DraftRow {
  uid: number
  locale: string
  key: string
  values: string
  isActive: boolean
}

let uidCounter = 0
const nextUid = () => (uidCounter += 1)

function toDraft(e: LexiconEntry): DraftRow {
  const values = (e.domain === TEMPORAL_LEXICON_DOMAIN ? e.interpretation_keys : e.terms) ?? []
  return {
    uid: nextUid(),
    locale: e.locale,
    key: e.key,
    values: values.join(', '),
    isActive: e.is_active ?? true,
  }
}

function toEntry(domain: LexiconDomain, d: DraftRow): LexiconEntry | null {
  const locale = d.locale.trim().toLowerCase()
  const key = d.key.trim()
  const values = d.values
    .split(',')
    .map((v) => v.trim())
    .filter((v) => v.length > 0)
  if (!locale || !key || values.length === 0) {
    return null
  }
  const entry: LexiconEntry = { locale, domain, key, is_active: d.isActive }
  if (domain === TEMPORAL_LEXICON_DOMAIN) {
    entry.interpretation_keys = values
  } else {
    entry.terms = values
  }
  return entry
}

// NLLexiconPanel manages the natural-language lexicon (ai_nl_lexicon): the
// admin-editable, DB-backed vocabulary the AI uses to detect time grains,
// counts, soft-delete wording and other intents. Defaults are seeded
// automatically; edits here override them and converge across replicas within
// the store TTL. Each domain can be reset to the embedded defaults.
export function NLLexiconPanel() {
  const t = useT()
  const toast = useToast()
  const confirmMutation = useConfirmedMutation()
  const [domain, setDomain] = useState<LexiconDomain>('grain_synonym')
  const [drafts, setDrafts] = useState<DraftRow[]>([])
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)

  const load = useCallback(
    async (d: LexiconDomain) => {
      setLoading(true)
      try {
        const res = await listLexicon({ domain: d })
        setDrafts(res.entries.map(toDraft))
      } catch (err) {
        toast.error(errorMessage(err))
        setDrafts([])
      } finally {
        setLoading(false)
      }
    },
    [toast],
  )

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load(domain)
  }, [domain, load])

  const domainOptions = useMemo(
    () => LEXICON_DOMAINS.map((d) => ({ value: d, label: t(DOMAIN_LABEL_KEYS[d]) })),
    [t],
  )

  const isTemporal = domain === TEMPORAL_LEXICON_DOMAIN
  const valuesHeader = isTemporal
    ? t('admin.nl_lexicon.col_interpretation_keys')
    : t('admin.nl_lexicon.col_terms')

  const updateDraft = (uid: number, patch: Partial<DraftRow>) =>
    setDrafts((ds) => ds.map((d) => (d.uid === uid ? { ...d, ...patch } : d)))
  const removeDraft = (uid: number) => setDrafts((ds) => ds.filter((d) => d.uid !== uid))
  const addDraft = () =>
    setDrafts((ds) => [
      ...ds,
      { uid: nextUid(), locale: 'en', key: '', values: '', isActive: true },
    ])

  const handleSave = async () => {
    const entries = drafts
      .map((d) => toEntry(domain, d))
      .filter((e): e is LexiconEntry => e !== null)
    if (entries.length === 0) {
      toast.error(t('admin.nl_lexicon.nothing_to_save'))
      return
    }
    setSaving(true)
    try {
      const res = await upsertLexicon(entries)
      toast.success(t('admin.nl_lexicon.saved', { count: res.updated }))
      await load(domain)
    } catch (err) {
      toast.error(errorMessage(err))
    } finally {
      setSaving(false)
    }
  }

  const handleReset = async () => {
    const ok = await confirmMutation(() => resetLexiconDomain(domain), {
      title: t('admin.nl_lexicon.reset_confirm_title'),
      message: t('admin.nl_lexicon.reset_confirm_message'),
      successMessage: t('admin.nl_lexicon.reset_done'),
      variant: 'warning',
    })
    if (ok) {
      await load(domain)
    }
  }

  return (
    <AdminPanelShell
      title={t('admin.nl_lexicon.title')}
      description={t('admin.nl_lexicon.description')}
      action={
        <div className="flex gap-2">
          <Button
            variant="ghost"
            size="sm"
            onClick={() => void handleReset()}
            disabled={saving || loading}
          >
            {t('admin.nl_lexicon.reset')}
          </Button>
          <Button
            variant="primary"
            size="sm"
            onClick={() => void handleSave()}
            disabled={saving || loading}
          >
            {saving ? t('admin.nl_lexicon.saving') : t('admin.nl_lexicon.save')}
          </Button>
        </div>
      }
    >
      <label className="flex max-w-sm flex-col gap-1">
        <span className="text-foreground-muted text-xs">{t('admin.nl_lexicon.domain_label')}</span>
        <Select<LexiconDomain>
          value={domain}
          onChange={setDomain}
          options={domainOptions}
          ariaLabel={t('admin.nl_lexicon.domain_label')}
        />
      </label>

      {loading ? (
        <LoadingScreen minHeight="160px" />
      ) : (
        <div className={adminTableContainerClass}>
          <table className={adminTableClass} style={{ fontSize: 13, minWidth: 720 }}>
            <thead>
              <tr className={adminTheadRowClass}>
                <th className={adminThClass} style={{ width: 96 }}>
                  {t('admin.nl_lexicon.col_locale')}
                </th>
                <th className={adminThClass}>{t('admin.nl_lexicon.col_key')}</th>
                <th className={adminThClass}>{valuesHeader}</th>
                <th className={adminThClass} style={{ width: 72 }}>
                  {t('admin.nl_lexicon.col_active')}
                </th>
                <th className={adminThClass} style={{ width: 96 }}>
                  {t('admin.nl_lexicon.col_actions')}
                </th>
              </tr>
            </thead>
            <tbody>
              {drafts.map((d) => (
                <tr key={d.uid} className={adminTrClass}>
                  <td className={adminTdClass}>
                    <input
                      className={adminInputClass}
                      value={d.locale}
                      onChange={(e) => updateDraft(d.uid, { locale: e.target.value })}
                      aria-label={t('admin.nl_lexicon.col_locale')}
                    />
                  </td>
                  <td className={adminTdClass}>
                    <input
                      className={adminInputClass}
                      value={d.key}
                      onChange={(e) => updateDraft(d.uid, { key: e.target.value })}
                      aria-label={t('admin.nl_lexicon.col_key')}
                    />
                  </td>
                  <td className={adminTdClass}>
                    <input
                      className={adminInputClass}
                      value={d.values}
                      placeholder={t('admin.nl_lexicon.values_hint')}
                      onChange={(e) => updateDraft(d.uid, { values: e.target.value })}
                      aria-label={valuesHeader}
                    />
                  </td>
                  <td className={adminTdClass}>
                    <input
                      type="checkbox"
                      checked={d.isActive}
                      onChange={(e) => updateDraft(d.uid, { isActive: e.target.checked })}
                      aria-label={t('admin.nl_lexicon.col_active')}
                    />
                  </td>
                  <td className={adminTdClass}>
                    <Button variant="ghost" size="sm" onClick={() => removeDraft(d.uid)}>
                      {t('admin.nl_lexicon.remove')}
                    </Button>
                  </td>
                </tr>
              ))}
              {drafts.length === 0 && (
                <tr>
                  <td className={adminTextMutedClass} colSpan={5}>
                    {t('admin.nl_lexicon.empty')}
                  </td>
                </tr>
              )}
            </tbody>
          </table>
          <div className="mt-3">
            <Button variant="secondary" size="sm" onClick={addDraft}>
              {t('admin.nl_lexicon.add')}
            </Button>
          </div>
        </div>
      )}
    </AdminPanelShell>
  )
}
