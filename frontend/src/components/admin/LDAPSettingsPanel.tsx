import { useCallback, useEffect, useState } from 'react'

import {
  getLDAPConfig,
  type LDAPConfigInput,
  type LDAPSecurity,
  testLDAPConnection,
  updateLDAPConfig,
} from '../../api/ldap'
import { useToast } from '../../hooks/useToast'
import { useT } from '../../i18n'
import { useAuth } from '../auth/AuthProvider'
import { LoadingScreen } from '../ui/LoadingScreen'
import { Select } from '../ui/Select'
import { ReadOnlyNote } from './ReadOnlyNote'

const EMPTY: LDAPConfigInput = {
  enabled: false,
  auto_create_users: true,
  host: '',
  port: 389,
  security: 'starttls',
  skip_tls_verify: false,
  bind_dn: '',
  bind_password: '',
  base_dn: '',
  user_filter: '(uid=%s)',
  email_attr: 'mail',
  display_name_attr: 'cn',
}

export function LDAPSettingsPanel({ token }: { token: string }) {
  const t = useT()
  const toast = useToast()
  const { isSuperAdmin } = useAuth()
  const canEdit = isSuperAdmin

  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [testing, setTesting] = useState(false)
  const [form, setForm] = useState<LDAPConfigInput>(EMPTY)
  const [hasStoredPassword, setHasStoredPassword] = useState(false)

  const set = <K extends keyof LDAPConfigInput>(key: K, value: LDAPConfigInput[K]) =>
    setForm((f) => ({ ...f, [key]: value }))

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const cfg = await getLDAPConfig(token)
      setForm({
        enabled: cfg.enabled,
        auto_create_users: cfg.auto_create_users,
        host: cfg.host,
        port: cfg.port || 389,
        security: cfg.security,
        skip_tls_verify: cfg.skip_tls_verify,
        bind_dn: cfg.bind_dn,
        bind_password: '',
        base_dn: cfg.base_dn,
        user_filter: cfg.user_filter || '(uid=%s)',
        email_attr: cfg.email_attr || 'mail',
        display_name_attr: cfg.display_name_attr || 'cn',
      })
      setHasStoredPassword(cfg.has_bind_password)
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e))
    } finally {
      setLoading(false)
    }
  }, [token, toast])

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load()
  }, [load])

  const onSave = async () => {
    setSaving(true)
    try {
      const cfg = await updateLDAPConfig(token, form)
      setHasStoredPassword(cfg.has_bind_password)
      setForm((f) => ({ ...f, bind_password: '' }))
      toast.success(t('admin.ldap.saved'))
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e))
    } finally {
      setSaving(false)
    }
  }

  const onTest = async () => {
    setTesting(true)
    try {
      const res = await testLDAPConnection(token, form)
      if (res.status === 'ok') {
        toast.success(t('admin.ldap.test_ok'))
      } else {
        toast.error(t('admin.ldap.test_failed', { error: res.message ?? '' }))
      }
    } catch (e) {
      toast.error(
        t('admin.ldap.test_failed', { error: e instanceof Error ? e.message : String(e) }),
      )
    } finally {
      setTesting(false)
    }
  }

  if (loading) {
    return <LoadingScreen minHeight="200px" />
  }

  const securityOptions: { value: LDAPSecurity; label: string }[] = [
    { value: 'none', label: t('admin.ldap.security_none') },
    { value: 'starttls', label: t('admin.ldap.security_starttls') },
    { value: 'ldaps', label: t('admin.ldap.security_ldaps') },
  ]

  return (
    <div className="page-stack" style={{ maxWidth: 760 }}>
      <div>
        <h2 style={{ margin: 0 }}>{t('admin.ldap.title')}</h2>
        <p className="form-hint" style={{ marginTop: 8 }}>
          {t('admin.ldap.description')}
        </p>
      </div>

      {!canEdit && <ReadOnlyNote />}

      {/* Toggles */}
      <Toggle
        label={t('admin.ldap.enabled_label')}
        hint={t('admin.ldap.enabled_hint')}
        checked={form.enabled}
        disabled={!canEdit}
        onChange={(v) => set('enabled', v)}
      />
      <Toggle
        label={t('admin.ldap.auto_create_label')}
        hint={t('admin.ldap.auto_create_hint')}
        checked={form.auto_create_users}
        disabled={!canEdit}
        onChange={(v) => set('auto_create_users', v)}
      />

      {/* Connection */}
      <fieldset className="ldap-fieldset" disabled={!canEdit}>
        <legend>{t('admin.ldap.connection')}</legend>
        <div className="ldap-grid">
          <Field label={t('admin.ldap.host')} style={{ gridColumn: 'span 2' }}>
            <input
              className="admin-input"
              value={form.host}
              onChange={(e) => set('host', e.target.value)}
              placeholder="ldap.example.com"
            />
          </Field>
          <Field label={t('admin.ldap.port')}>
            <input
              className="admin-input"
              type="number"
              value={form.port}
              onChange={(e) => set('port', Number(e.target.value) || 0)}
            />
          </Field>
          <Field label={t('admin.ldap.security')}>
            <Select
              value={form.security}
              options={securityOptions}
              onChange={(v) => set('security', v)}
              disabled={!canEdit}
            />
          </Field>
        </div>
        <CheckRow
          label={t('admin.ldap.skip_tls_verify')}
          hint={t('admin.ldap.skip_tls_verify_hint')}
          checked={form.skip_tls_verify}
          disabled={!canEdit}
          onChange={(v) => set('skip_tls_verify', v)}
        />
        <Field label={t('admin.ldap.bind_dn')} hint={t('admin.ldap.bind_dn_hint')}>
          <input
            className="admin-input"
            value={form.bind_dn}
            onChange={(e) => set('bind_dn', e.target.value)}
            placeholder="cn=svc,dc=example,dc=com"
          />
        </Field>
        <Field
          label={t('admin.ldap.bind_password')}
          hint={hasStoredPassword ? t('admin.ldap.bind_password_set') : undefined}
        >
          <input
            className="admin-input"
            type="password"
            autoComplete="new-password"
            value={form.bind_password}
            onChange={(e) => set('bind_password', e.target.value)}
            placeholder={t('admin.ldap.bind_password_placeholder')}
          />
        </Field>
      </fieldset>

      {/* Directory & mapping */}
      <fieldset className="ldap-fieldset" disabled={!canEdit}>
        <legend>{t('admin.ldap.directory')}</legend>
        <Field label={t('admin.ldap.base_dn')} hint={t('admin.ldap.base_dn_hint')}>
          <input
            className="admin-input"
            value={form.base_dn}
            onChange={(e) => set('base_dn', e.target.value)}
            placeholder="ou=people,dc=example,dc=com"
          />
        </Field>
        <Field label={t('admin.ldap.user_filter')} hint={t('admin.ldap.user_filter_hint')}>
          <input
            className="admin-input"
            value={form.user_filter}
            onChange={(e) => set('user_filter', e.target.value)}
            placeholder="(uid=%s)"
          />
        </Field>
        <div className="ldap-grid">
          <Field label={t('admin.ldap.email_attr')}>
            <input
              className="admin-input"
              value={form.email_attr}
              onChange={(e) => set('email_attr', e.target.value)}
              placeholder="mail"
            />
          </Field>
          <Field label={t('admin.ldap.display_name_attr')}>
            <input
              className="admin-input"
              value={form.display_name_attr}
              onChange={(e) => set('display_name_attr', e.target.value)}
              placeholder="cn"
            />
          </Field>
        </div>
      </fieldset>

      <div style={{ display: 'flex', gap: 8 }}>
        <button
          type="button"
          className="btn btn-secondary"
          disabled={!canEdit || testing}
          onClick={() => void onTest()}
        >
          {testing ? '…' : t('admin.ldap.test')}
        </button>
        <button
          type="button"
          className="btn btn-primary"
          disabled={!canEdit || saving}
          onClick={() => void onSave()}
        >
          {saving ? '…' : t('admin.ldap.save')}
        </button>
      </div>
    </div>
  )
}

function Field({
  label,
  hint,
  children,
  style,
}: {
  label: string
  hint?: string
  children: React.ReactNode
  style?: React.CSSProperties
}) {
  return (
    <label
      className="admin-form-label"
      style={{ display: 'flex', flexDirection: 'column', gap: 4, ...style }}
    >
      <span className="admin-label-text">{label}</span>
      {children}
      {hint && (
        <span className="form-hint" style={{ margin: 0 }}>
          {hint}
        </span>
      )}
    </label>
  )
}

function Toggle({
  label,
  hint,
  checked,
  disabled,
  onChange,
}: {
  label: string
  hint?: string
  checked: boolean
  disabled?: boolean
  onChange: (v: boolean) => void
}) {
  return (
    <label className="ldap-toggle">
      <input
        type="checkbox"
        checked={checked}
        disabled={disabled}
        onChange={(e) => onChange(e.target.checked)}
      />
      <span>
        <strong style={{ display: 'block' }}>{label}</strong>
        {hint && (
          <span className="form-hint" style={{ margin: 0 }}>
            {hint}
          </span>
        )}
      </span>
    </label>
  )
}

function CheckRow({
  label,
  hint,
  checked,
  disabled,
  onChange,
}: {
  label: string
  hint?: string
  checked: boolean
  disabled?: boolean
  onChange: (v: boolean) => void
}) {
  return (
    <label style={{ display: 'flex', gap: 8, alignItems: 'flex-start', marginTop: 8 }}>
      <input
        type="checkbox"
        checked={checked}
        disabled={disabled}
        onChange={(e) => onChange(e.target.checked)}
        style={{ marginTop: 3 }}
      />
      <span>
        {label}
        {hint && (
          <span className="form-hint" style={{ margin: 0, display: 'block' }}>
            {hint}
          </span>
        )}
      </span>
    </label>
  )
}
