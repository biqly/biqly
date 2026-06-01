import { useCallback, useEffect, useState } from 'react'
import { useT } from '../../i18n'
import { useToast } from '../../hooks/useToast'
import { getPlatformSettings, updatePlatformSettings } from '../../api/admin'
import { clearPasswordPolicyCache } from '../../api/auth'
import { LoadingScreen } from '../ui/LoadingScreen'

export function PlatformSettingsPanel({ token }: { token: string }) {
  const t = useT()
  const toast = useToast()
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [selfSignupEnabled, setSelfSignupEnabled] = useState(true)
  const [updatedAt, setUpdatedAt] = useState<string | null>(null)

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

  useEffect(() => {
    void load()
  }, [load])

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

      <label
        style={{
          display: 'flex',
          gap: 12,
          alignItems: 'flex-start',
          padding: 16,
          borderRadius: 8,
          border: '1px solid var(--border, #27272a)',
          background: 'var(--surface-elevated, #18181b)',
          cursor: 'pointer',
        }}
      >
        <input
          type="checkbox"
          checked={selfSignupEnabled}
          onChange={(e) => setSelfSignupEnabled(e.target.checked)}
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
        <button type="button" className="btn btn-primary" disabled={saving} onClick={() => void save()}>
          {saving ? t('common.saving') : t('common.save')}
        </button>
      </div>
    </div>
  )
}
