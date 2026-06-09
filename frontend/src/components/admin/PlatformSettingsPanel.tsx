import { useCallback, useEffect, useId, useState } from 'react'

import { getPlatformSettings, updatePlatformSettings } from '../../api/admin'
import { getAIAdminConfig, updateAIAdminConfig } from '../../api/aiAdmin'
import { clearPasswordPolicyCache } from '../../api/auth'
import { useToast } from '../../hooks/useToast'
import { useT } from '../../i18n'
import { useAuth } from '../auth/AuthProvider'
import { LoadingScreen } from '../ui/LoadingScreen'
import { ReadOnlyNote } from './ReadOnlyNote'

export function PlatformSettingsPanel({ token }: { token: string }) {
  const t = useT()
  const toast = useToast()
  const { isSuperAdmin } = useAuth()
  // Platform settings are super-admin only (enforced server-side).
  const canEdit = isSuperAdmin
  const maxLLMTierInputId = useId()
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [selfSignupEnabled, setSelfSignupEnabled] = useState(true)
  const [updatedAt, setUpdatedAt] = useState<string | null>(null)

  // Ambiguity detection knobs live in the AI service runtime config
  // (/api/ai/admin/config), separate from auth-service platform settings.
  const [ambiguitySaving, setAmbiguitySaving] = useState(false)
  const [tieredEnabled, setTieredEnabled] = useState(false)
  const [maxLLMTier, setMaxLLMTier] = useState(1)
  const [ambiguityDBOverride, setAmbiguityDBOverride] = useState(false)
  const [ambiguityLoaded, setAmbiguityLoaded] = useState(false)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const data = await getPlatformSettings(token)
      setSelfSignupEnabled(data.self_signup_enabled)
      setUpdatedAt(data.updated_at ?? null)
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e))
    } finally {
      setLoading(false)
    }
  }, [token, toast])

  const loadAmbiguity = useCallback(async () => {
    try {
      const data = await getAIAdminConfig()
      setTieredEnabled(data.ambiguity.tiered_enabled)
      setMaxLLMTier(data.ambiguity.max_llm_tier_per_question)
      setAmbiguityDBOverride(data.ambiguity.db_override)
      setAmbiguityLoaded(true)
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e))
    }
  }, [toast])

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load()
    void loadAmbiguity()
  }, [load, loadAmbiguity])

  const save = async () => {
    setSaving(true)
    try {
      const data = await updatePlatformSettings(token, selfSignupEnabled)
      setSelfSignupEnabled(data.self_signup_enabled)
      setUpdatedAt(data.updated_at ?? null)
      clearPasswordPolicyCache()
      toast.success(t('admin.platform_settings.saved'))
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e))
    } finally {
      setSaving(false)
    }
  }

  const saveAmbiguity = async () => {
    setAmbiguitySaving(true)
    try {
      const data = await updateAIAdminConfig({
        tiered_enabled: tieredEnabled,
        max_llm_tier_per_question: maxLLMTier,
      })
      setTieredEnabled(data.ambiguity.tiered_enabled)
      setMaxLLMTier(data.ambiguity.max_llm_tier_per_question)
      setAmbiguityDBOverride(data.ambiguity.db_override)
      toast.success(t('admin.platform_settings.ambiguity_saved'))
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e))
    } finally {
      setAmbiguitySaving(false)
    }
  }

  if (loading) {
    return <LoadingScreen minHeight="200px" />
  }

  return (
    <div className="page-stack" style={{ maxWidth: 640 }}>
      <div>
        <h2 style={{ margin: 0 }}>{t('admin.platform_settings.title')}</h2>
        <p className="form-hint" style={{ marginTop: 8 }}>
          {t('admin.platform_settings.description')}
        </p>
      </div>

      {!canEdit && <ReadOnlyNote />}

      <label
        style={{
          display: 'flex',
          gap: 12,
          alignItems: 'flex-start',
          padding: 16,
          borderRadius: 8,
          border: '1px solid var(--border, rgba(0, 0, 0, 0.12))',
          background: 'var(--bg-card-raised, rgba(0, 0, 0, 0.02))',
          cursor: 'pointer',
        }}
      >
        <input
          type="checkbox"
          checked={selfSignupEnabled}
          onChange={(e) => setSelfSignupEnabled(e.target.checked)}
          disabled={!canEdit}
          style={{ marginTop: 3 }}
        />
        <span>
          <strong style={{ display: 'block', marginBottom: 4 }}>
            {t('admin.platform_settings.self_signup_label')}
          </strong>
          <span className="form-hint" style={{ margin: 0 }}>
            {selfSignupEnabled
              ? t('admin.platform_settings.self_signup_on_hint')
              : t('admin.platform_settings.self_signup_off_hint')}
          </span>
        </span>
      </label>

      {updatedAt && (
        <p className="form-hint" style={{ margin: 0 }}>
          {t('admin.platform_settings.last_updated', {
            date: new Date(updatedAt).toLocaleString(),
          })}
        </p>
      )}

      <div style={{ display: 'flex', gap: 8 }}>
        <button
          type="button"
          className="btn btn-primary"
          disabled={saving || !canEdit}
          onClick={() => void save()}
        >
          {saving ? t('common.saving') : t('common.save')}
        </button>
      </div>

      <div style={{ marginTop: 24 }}>
        <h2 style={{ margin: 0 }}>{t('admin.platform_settings.ambiguity_title')}</h2>
        <p className="form-hint" style={{ marginTop: 8 }}>
          {t('admin.platform_settings.ambiguity_description')}
        </p>
      </div>

      <label
        style={{
          display: 'flex',
          gap: 12,
          alignItems: 'flex-start',
          padding: 16,
          borderRadius: 8,
          border: '1px solid var(--border, rgba(0, 0, 0, 0.12))',
          background: 'var(--bg-card-raised, rgba(0, 0, 0, 0.02))',
          cursor: 'pointer',
        }}
      >
        <input
          type="checkbox"
          checked={tieredEnabled}
          onChange={(e) => setTieredEnabled(e.target.checked)}
          disabled={!canEdit || !ambiguityLoaded}
          style={{ marginTop: 3 }}
        />
        <span>
          <strong style={{ display: 'block', marginBottom: 4 }}>
            {t('admin.platform_settings.ambiguity_tiered_label')}
          </strong>
          <span className="form-hint" style={{ margin: 0 }}>
            {t('admin.platform_settings.ambiguity_tiered_hint')}
          </span>
        </span>
      </label>

      <label
        className="admin-form-label"
        style={{ gap: 4, maxWidth: 360 }}
        htmlFor={maxLLMTierInputId}
      >
        <span className="admin-label-text">
          {t('admin.platform_settings.ambiguity_max_llm_label')}
        </span>
        <input
          id={maxLLMTierInputId}
          type="number"
          min={0}
          max={10}
          value={maxLLMTier}
          onChange={(e) => setMaxLLMTier(Math.max(0, Math.min(10, Number(e.target.value) || 0)))}
          disabled={!canEdit || !ambiguityLoaded}
        />
        <span className="form-hint" style={{ margin: 0 }}>
          {t('admin.platform_settings.ambiguity_max_llm_hint')}
        </span>
      </label>

      <p className="form-hint" style={{ margin: 0 }}>
        {ambiguityDBOverride
          ? t('admin.platform_settings.ambiguity_db_override_note')
          : t('admin.platform_settings.ambiguity_env_default_note')}
      </p>

      <div style={{ display: 'flex', gap: 8 }}>
        <button
          type="button"
          className="btn btn-primary"
          disabled={ambiguitySaving || !canEdit || !ambiguityLoaded}
          onClick={() => void saveAmbiguity()}
        >
          {ambiguitySaving ? t('common.saving') : t('common.save')}
        </button>
      </div>
    </div>
  )
}
