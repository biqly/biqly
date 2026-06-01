import { useCallback, useEffect, useMemo, useState } from 'react'
import { useT } from '../../i18n'
import { useAuth } from '../auth/AuthProvider'
import { useToast } from '../../hooks/useToast'
import { Select } from '../ui/Select'
import { LoadingOverlay } from '../ui/LoadingOverlay'
import { listRoles, listWorkspaces } from '../../api/admin'
import { listModels, listProviders, type AIModel, type AIProvider } from '../../api/aiProviders'
import {
  grantModelRole,
  grantModelWorkspace,
  grantProviderRole,
  grantProviderWorkspace,
  listAIModelAccess,
  revokeModelRole,
  revokeModelWorkspace,
  revokeProviderRole,
  revokeProviderWorkspace,
  type AIModelAccessGrants,
} from '../../api/aiModelAccess'

type TargetKind = 'workspace' | 'role'
type GrantKind = 'provider' | 'model'

export function AIModelSharingPanel() {
  const t = useT()
  const toast = useToast()
  const { accessToken } = useAuth()

  const [loading, setLoading] = useState(true)
  const [grants, setGrants] = useState<AIModelAccessGrants | null>(null)
  const [workspaces, setWorkspaces] = useState<{ id: string; name: string }[]>([])
  const [roles, setRoles] = useState<{ id: string; name: string }[]>([])
  const [providers, setProviders] = useState<AIProvider[]>([])
  const [models, setModels] = useState<AIModel[]>([])

  const [targetKind, setTargetKind] = useState<TargetKind>('workspace')
  const [grantKind, setGrantKind] = useState<GrantKind>('provider')
  const [targetID, setTargetID] = useState('')
  const [resourceID, setResourceID] = useState('')
  const [submitting, setSubmitting] = useState(false)

  const reload = useCallback(async () => {
    if (!accessToken) return
    setLoading(true)
    try {
      const [g, ws, rs, provs, allModels] = await Promise.all([
        listAIModelAccess(accessToken),
        listWorkspaces(accessToken, 1, 500),
        listRoles(accessToken, 1, 500),
        listProviders(),
        listModels(),
      ])
      setGrants(g)
      setWorkspaces((ws.workspaces ?? []).map((w) => ({ id: w.id, name: w.name })))
      setRoles((rs.roles ?? []).map((r) => ({ id: r.id, name: r.name })))
      setProviders(provs ?? [])
      setModels(allModels ?? [])
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e))
    } finally {
      setLoading(false)
    }
  }, [accessToken, toast])

  useEffect(() => {
    reload()
  }, [reload])

  const providerName = useMemo(() => {
    const m = new Map(providers.map((p) => [p.id, p.name]))
    return (id: string) => m.get(id) ?? id.slice(0, 8)
  }, [providers])

  const modelLabel = useMemo(() => {
    const m = new Map(models.map((x) => [x.id, x.display_name]))
    return (id: string) => m.get(id) ?? id.slice(0, 8)
  }, [models])

  const workspaceName = useMemo(() => {
    const m = new Map(workspaces.map((w) => [w.id, w.name]))
    return (id: string) => m.get(id) ?? id.slice(0, 8)
  }, [workspaces])

  const roleName = useMemo(() => {
    const m = new Map(roles.map((r) => [r.id, r.name]))
    return (id: string) => m.get(id) ?? id.slice(0, 8)
  }, [roles])

  const targetOptions = targetKind === 'workspace'
    ? workspaces.map((w) => ({ value: w.id, label: w.name }))
    : roles.map((r) => ({ value: r.id, label: r.name }))

  const resourceOptions = grantKind === 'provider'
    ? providers.map((p) => ({ value: p.id, label: p.name }))
    : models.map((m) => ({ value: m.id, label: `${m.display_name} (${m.provider_name})` }))

  const handleGrant = async () => {
    if (!accessToken || !targetID || !resourceID) return
    setSubmitting(true)
    try {
      if (targetKind === 'workspace' && grantKind === 'provider') {
        await grantProviderWorkspace(accessToken, targetID, resourceID)
      } else if (targetKind === 'workspace' && grantKind === 'model') {
        await grantModelWorkspace(accessToken, targetID, resourceID)
      } else if (targetKind === 'role' && grantKind === 'provider') {
        await grantProviderRole(accessToken, targetID, resourceID)
      } else {
        await grantModelRole(accessToken, targetID, resourceID)
      }
      toast.success(t('admin.ai_model_access.granted'))
      setResourceID('')
      await reload()
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e))
    } finally {
      setSubmitting(false)
    }
  }

  const revoke = async (fn: () => Promise<void>) => {
    if (!accessToken) return
    try {
      await fn()
      toast.success(t('admin.ai_model_access.revoked'))
      await reload()
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e))
    }
  }

  return (
    <section style={cardStyle}>
      <div style={cardHeaderStyle}>
        <div>
          <strong>{t('admin.ai_model_access.title')}</strong>
          <p style={{ margin: '6px 0 0', fontSize: 13, color: 'var(--text-secondary, #a1a1aa)' }}>
            {t('admin.ai_model_access.description')}
          </p>
        </div>
      </div>

      <LoadingOverlay loading={loading}>
        <div style={{ padding: 16, display: 'flex', flexDirection: 'column', gap: 16 }}>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(160px, 1fr))', gap: 12 }}>
            <Field label={t('admin.ai_model_access.target_kind')}>
              <Select
                value={targetKind}
                onChange={(v) => { setTargetKind(v as TargetKind); setTargetID('') }}
                options={[
                  { value: 'workspace', label: t('admin.ai_model_access.workspace') },
                  { value: 'role', label: t('admin.ai_model_access.role') },
                ]}
              />
            </Field>
            <Field label={t('admin.ai_model_access.grant_kind')}>
              <Select
                value={grantKind}
                onChange={(v) => { setGrantKind(v as GrantKind); setResourceID('') }}
                options={[
                  { value: 'provider', label: t('admin.ai_model_access.provider') },
                  { value: 'model', label: t('admin.ai_model_access.model') },
                ]}
              />
            </Field>
            <Field label={targetKind === 'workspace' ? t('admin.ai_model_access.workspace') : t('admin.ai_model_access.role')}>
              <Select
                value={targetID}
                onChange={setTargetID}
                options={[{ value: '', label: '—' }, ...targetOptions]}
              />
            </Field>
            <Field label={grantKind === 'provider' ? t('admin.ai_model_access.provider') : t('admin.ai_model_access.model')}>
              <Select
                value={resourceID}
                onChange={setResourceID}
                options={[{ value: '', label: '—' }, ...resourceOptions]}
              />
            </Field>
          </div>
          <div>
            <button
              type="button"
              className="btn btn-primary btn-sm"
              disabled={submitting || !targetID || !resourceID}
              onClick={handleGrant}
            >
              {t('admin.ai_model_access.grant_btn')}
            </button>
          </div>

          {grants && (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 10, fontSize: 13 }}>
              {(grants.provider_workspaces ?? []).map((g) => (
                <GrantRow
                  key={`pw-${g.workspace_id}-${g.provider_id}`}
                  label={`${workspaceName(g.workspace_id)} → ${providerName(g.provider_id)} (${t('admin.ai_model_access.provider')})`}
                  onRevoke={() => revoke(() => revokeProviderWorkspace(accessToken!, g.workspace_id, g.provider_id))}
                />
              ))}
              {(grants.model_workspaces ?? []).map((g) => (
                <GrantRow
                  key={`mw-${g.workspace_id}-${g.model_id}`}
                  label={`${workspaceName(g.workspace_id)} → ${modelLabel(g.model_id)}`}
                  onRevoke={() => revoke(() => revokeModelWorkspace(accessToken!, g.workspace_id, g.model_id))}
                />
              ))}
              {(grants.provider_roles ?? []).map((g) => (
                <GrantRow
                  key={`pr-${g.role_id}-${g.provider_id}`}
                  label={`${roleName(g.role_id)} → ${providerName(g.provider_id)} (${t('admin.ai_model_access.provider')})`}
                  onRevoke={() => revoke(() => revokeProviderRole(accessToken!, g.role_id, g.provider_id))}
                />
              ))}
              {(grants.model_roles ?? []).map((g) => (
                <GrantRow
                  key={`mr-${g.role_id}-${g.model_id}`}
                  label={`${roleName(g.role_id)} → ${modelLabel(g.model_id)}`}
                  onRevoke={() => revoke(() => revokeModelRole(accessToken!, g.role_id, g.model_id))}
                />
              ))}
              {!grants.provider_workspaces?.length &&
                !grants.model_workspaces?.length &&
                !grants.provider_roles?.length &&
                !grants.model_roles?.length && (
                  <p style={{ color: 'var(--text-secondary)', margin: 0 }}>{t('admin.ai_model_access.empty')}</p>
                )}
            </div>
          )}
        </div>
      </LoadingOverlay>
    </section>
  )
}

function GrantRow({ label, onRevoke }: { label: string; onRevoke: () => void }) {
  const t = useT()
  return (
    <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 12 }}>
      <span>{label}</span>
      <button type="button" className="btn btn-secondary btn-sm" onClick={onRevoke}>
        {t('admin.ai_model_access.revoke_btn')}
      </button>
    </div>
  )
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div>
      <label style={{ display: 'block', fontSize: 12, fontWeight: 600, marginBottom: 4 }}>{label}</label>
      {children}
    </div>
  )
}

const cardStyle: React.CSSProperties = {
  border: '1px solid var(--border, rgba(255,255,255,0.06))',
  borderRadius: 8,
  background: 'var(--surface, rgba(255,255,255,0.02))',
}

const cardHeaderStyle: React.CSSProperties = {
  padding: '12px 16px',
  borderBottom: '1px solid var(--border, rgba(255,255,255,0.06))',
}
