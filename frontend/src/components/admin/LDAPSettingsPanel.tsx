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
import { buttonClass } from '../../lib/buttonClasses'
import { cn } from '../../lib/cn'
import { formHintClass } from '../../lib/formClasses'
import { errorMessage } from '../../utils/error'
import { useAuth } from '../auth/AuthProvider'
import { LoadingScreen } from '../ui/LoadingScreen'
import { Select } from '../ui/Select'
import {
  adminFormLabelClass,
  adminInputClass,
  adminLabelTextClass,
  ldapGridClass,
  ldapToggleClass,
} from './adminClasses'
import { AdminFormSection } from './AdminFormSection'
import { AdminPanelShell } from './AdminPanelShell'

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
      toast.error(errorMessage(e))
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
      toast.error(errorMessage(e))
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
      toast.error(t('admin.ldap.test_failed', { error: errorMessage(e) }))
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
    <AdminPanelShell
      title={t('admin.ldap.title')}
      description={t('admin.ldap.description')}
      readOnly={!canEdit}
      maxWidth={760}
    >
      {/* Toggles */}
      <div className="flex flex-col gap-4">
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
      </div>

      {/* Connection */}
      <AdminFormSection title={t('admin.ldap.connection')} disabled={!canEdit}>
        <div className={ldapGridClass}>
          <Field label={t('admin.ldap.host')} className="col-span-2">
            <input
              className={adminInputClass}
              value={form.host}
              onChange={(e) => set('host', e.target.value)}
              placeholder="ldap.example.com"
            />
          </Field>
          <Field label={t('admin.ldap.port')}>
            <input
              className={adminInputClass}
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
        <div className="grid grid-cols-2 gap-3 max-[640px]:grid-cols-1">
          <Field label={t('admin.ldap.bind_dn')} hint={t('admin.ldap.bind_dn_hint')}>
            <input
              className={adminInputClass}
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
              className={adminInputClass}
              type="password"
              autoComplete="new-password"
              value={form.bind_password}
              onChange={(e) => set('bind_password', e.target.value)}
              placeholder={t('admin.ldap.bind_password_placeholder')}
            />
          </Field>
        </div>
      </AdminFormSection>

      {/* Directory & mapping */}
      <AdminFormSection title={t('admin.ldap.directory')} disabled={!canEdit}>
        <div className="grid grid-cols-2 gap-3 max-[640px]:grid-cols-1">
          <Field label={t('admin.ldap.base_dn')} hint={t('admin.ldap.base_dn_hint')}>
            <input
              className={adminInputClass}
              value={form.base_dn}
              onChange={(e) => set('base_dn', e.target.value)}
              placeholder="ou=people,dc=example,dc=com"
            />
          </Field>
          <Field label={t('admin.ldap.user_filter')} hint={t('admin.ldap.user_filter_hint')}>
            <input
              className={adminInputClass}
              value={form.user_filter}
              onChange={(e) => set('user_filter', e.target.value)}
              placeholder="(uid=%s)"
            />
          </Field>
        </div>
        <div className={ldapGridClass}>
          <Field label={t('admin.ldap.email_attr')}>
            <input
              className={adminInputClass}
              value={form.email_attr}
              onChange={(e) => set('email_attr', e.target.value)}
              placeholder="mail"
            />
          </Field>
          <Field label={t('admin.ldap.display_name_attr')}>
            <input
              className={adminInputClass}
              value={form.display_name_attr}
              onChange={(e) => set('display_name_attr', e.target.value)}
              placeholder="cn"
            />
          </Field>
        </div>
      </AdminFormSection>

      <div className="flex gap-2">
        <button
          type="button"
          className={buttonClass('secondary')}
          disabled={!canEdit || testing}
          onClick={() => void onTest()}
        >
          {testing ? '…' : t('admin.ldap.test')}
        </button>
        <button
          type="button"
          className={buttonClass('primary')}
          disabled={!canEdit || saving}
          onClick={() => void onSave()}
        >
          {saving ? '…' : t('admin.ldap.save')}
        </button>
      </div>
    </AdminPanelShell>
  )
}

function Field({
  label,
  hint,
  children,
  className,
}: {
  label: string
  hint?: string
  children: React.ReactNode
  className?: string
}) {
  return (
    <label className={cn(adminFormLabelClass, className)}>
      <span className={adminLabelTextClass}>{label}</span>
      {children}
      {hint && <span className={cn(formHintClass, 'm-0')}>{hint}</span>}
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
    <label className={ldapToggleClass}>
      <input
        type="checkbox"
        checked={checked}
        disabled={disabled}
        onChange={(e) => onChange(e.target.checked)}
      />
      <span>
        <strong className="block font-bold">{label}</strong>
        {hint && <span className={cn(formHintClass, 'm-0')}>{hint}</span>}
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
    <label className="mt-2 flex items-start gap-2">
      <input
        type="checkbox"
        checked={checked}
        disabled={disabled}
        onChange={(e) => onChange(e.target.checked)}
        className="mt-0.5"
      />
      <span>
        {label}
        {hint && <span className={cn(formHintClass, 'm-0')}>{hint}</span>}
      </span>
    </label>
  )
}
