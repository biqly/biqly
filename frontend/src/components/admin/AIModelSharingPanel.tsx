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

type GrantListItem = {
  key: string
  targetBadge: 'workspace' | 'role'
  resourceBadge: 'provider' | 'model'
  label: string
  onRevoke: () => Promise<void>
}

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

  const targetOptions = useMemo(() => {
    const list = targetKind === 'workspace' ? workspaces : roles
    return list.map((x) => ({ value: x.id, label: x.name }))
  }, [targetKind, workspaces, roles])

  const resourceOptions = useMemo(() => {
    if (grantKind === 'provider') {
      return providers.map((p) => ({ value: p.id, label: p.name }))
    }
    return models.map((m) => ({ value: m.id, label: `${m.display_name} (${m.provider_name})` }))
  }, [grantKind, providers, models])

  const grantItems = useMemo((): GrantListItem[] => {
    if (!grants || !accessToken) return []
    const items: GrantListItem[] = []
    for (const g of grants.provider_workspaces ?? []) {
      items.push({
        key: `pw-${g.workspace_id}-${g.provider_id}`,
        targetBadge: 'workspace',
        resourceBadge: 'provider',
        label: `${workspaceName(g.workspace_id)} → ${providerName(g.provider_id)}`,
        onRevoke: () => revokeProviderWorkspace(accessToken, g.workspace_id, g.provider_id),
      })
    }
    for (const g of grants.model_workspaces ?? []) {
      items.push({
        key: `mw-${g.workspace_id}-${g.model_id}`,
        targetBadge: 'workspace',
        resourceBadge: 'model',
        label: `${workspaceName(g.workspace_id)} → ${modelLabel(g.model_id)}`,
        onRevoke: () => revokeModelWorkspace(accessToken, g.workspace_id, g.model_id),
      })
    }
    for (const g of grants.provider_roles ?? []) {
      items.push({
        key: `pr-${g.role_id}-${g.provider_id}`,
        targetBadge: 'role',
        resourceBadge: 'provider',
        label: `${roleName(g.role_id)} → ${providerName(g.provider_id)}`,
        onRevoke: () => revokeProviderRole(accessToken, g.role_id, g.provider_id),
      })
    }
    for (const g of grants.model_roles ?? []) {
      items.push({
        key: `mr-${g.role_id}-${g.model_id}`,
        targetBadge: 'role',
        resourceBadge: 'model',
        label: `${roleName(g.role_id)} → ${modelLabel(g.model_id)}`,
        onRevoke: () => revokeModelRole(accessToken, g.role_id, g.model_id),
      })
    }
    items.sort((a, b) => a.label.localeCompare(b.label, undefined, { sensitivity: 'base' }))
    return items
  }, [grants, accessToken, workspaceName, providerName, modelLabel, roleName])

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

  const targetPlaceholder =
    targetKind === 'workspace'
      ? t('admin.ai_model_access.select_workspace')
      : t('admin.ai_model_access.select_role')

  const resourcePlaceholder =
    grantKind === 'provider'
      ? t('admin.ai_model_access.select_provider')
      : t('admin.ai_model_access.select_model')

  return (
    <section className="admin-ai-card">
      <div className="admin-ai-card__header">
        <strong>{t('admin.ai_model_access.title')}</strong>
        <p>{t('admin.ai_model_access.description')}</p>
      </div>

      <LoadingOverlay loading={loading}>
        <div className="admin-ai-card__body">
          <div className="admin-ai-grant-toolbar">
            <label className="admin-form-label" style={{ gap: 4, margin: 0 }}>
              <span className="admin-label-text">{t('admin.ai_model_access.target_kind')}</span>
              <Select
                value={targetKind}
                onChange={(v) => {
                  setTargetKind(v as TargetKind)
                  setTargetID('')
                }}
                options={[
                  { value: 'workspace', label: t('admin.ai_model_access.workspace') },
                  { value: 'role', label: t('admin.ai_model_access.role') },
                ]}
              />
            </label>
            <label className="admin-form-label" style={{ gap: 4, margin: 0 }}>
              <span className="admin-label-text">
                {targetKind === 'workspace' ? t('admin.ai_model_access.workspace') : t('admin.ai_model_access.role')}
              </span>
              <Select
                value={targetID}
                onChange={setTargetID}
                options={[{ value: '', label: targetPlaceholder }, ...targetOptions]}
                searchable={targetOptions.length > 6}
                placeholder={targetPlaceholder}
              />
            </label>
            <label className="admin-form-label" style={{ gap: 4, margin: 0 }}>
              <span className="admin-label-text">{t('admin.ai_model_access.grant_kind')}</span>
              <Select
                value={grantKind}
                onChange={(v) => {
                  setGrantKind(v as GrantKind)
                  setResourceID('')
                }}
                options={[
                  { value: 'provider', label: t('admin.ai_model_access.provider') },
                  { value: 'model', label: t('admin.ai_model_access.model') },
                ]}
              />
            </label>
            <label className="admin-form-label" style={{ gap: 4, margin: 0 }}>
              <span className="admin-label-text">
                {grantKind === 'provider' ? t('admin.ai_model_access.provider') : t('admin.ai_model_access.model')}
              </span>
              <Select
                value={resourceID}
                onChange={setResourceID}
                options={[{ value: '', label: resourcePlaceholder }, ...resourceOptions]}
                searchable={resourceOptions.length > 6}
                placeholder={resourcePlaceholder}
              />
            </label>
            <div className="admin-ai-grant-toolbar__actions">
              <button
                type="button"
                className="btn btn-primary btn-sm"
                disabled={submitting || !targetID || !resourceID}
                onClick={handleGrant}
              >
                {t('admin.ai_model_access.grant_btn')}
              </button>
            </div>
          </div>

          <p className="admin-ai-grants-meta">
            {targetKind === 'workspace'
              ? t('admin.ai_model_access.workspaces_available', { count: workspaces.length })
              : t('admin.ai_model_access.roles_available', { count: roles.length })}
            {grantItems.length > 0
              ? ` · ${t('admin.ai_model_access.grants_count', { count: grantItems.length })}`
              : ''}
          </p>

          {grantItems.length === 0 ? (
            <p style={{ margin: 0, fontSize: 13, color: 'var(--text-secondary, #a1a1aa)' }}>
              {t('admin.ai_model_access.empty')}
            </p>
          ) : (
            <ul className="admin-ai-grant-list" aria-label={t('admin.ai_model_access.grants_list')}>
              {grantItems.map((item) => (
                <li key={item.key} className="admin-ai-grant-row">
                  <div className="admin-ai-grant-row__main">
                    <div className="admin-ai-grant-row__badges">
                      <span className={`admin-ai-badge admin-ai-badge--${item.targetBadge}`}>
                        {t(`admin.ai_model_access.${item.targetBadge}`)}
                      </span>
                      <span className={`admin-ai-badge admin-ai-badge--${item.resourceBadge}`}>
                        {t(`admin.ai_model_access.${item.resourceBadge}`)}
                      </span>
                    </div>
                    <div className="admin-ai-grant-row__label">{item.label}</div>
                  </div>
                  <button
                    type="button"
                    className="btn btn-secondary btn-sm"
                    onClick={() => revoke(item.onRevoke)}
                  >
                    {t('admin.ai_model_access.revoke_btn')}
                  </button>
                </li>
              ))}
            </ul>
          )}
        </div>
      </LoadingOverlay>
    </section>
  )
}
