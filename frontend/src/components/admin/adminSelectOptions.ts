import type { AuthUser, Workspace } from '../../types/auth'
import type { Datasource } from '../../types/metadata'
import type { SemanticModelSummary } from '../../types/semantic'
import type { SelectOption } from '../ui/Select'
import { SECURITY_POLICY_ROLES } from './securityPolicyConstants'

export const DATASOURCE_ACCESS_LEVELS = ['read', 'write', 'admin'] as const

export type DatasourceAccessLevel = (typeof DATASOURCE_ACCESS_LEVELS)[number]

export function securityRoleOptions(): SelectOption[] {
  return SECURITY_POLICY_ROLES.map((r) => ({ value: r, label: r }))
}

export function datasourceAccessLevelOptions(): SelectOption[] {
  return DATASOURCE_ACCESS_LEVELS.map((l) => ({ value: l, label: l }))
}

export function datasourceSelectOptions(
  datasources: Datasource[],
  loading?: boolean,
): SelectOption[] {
  if (loading) {
    return [{ value: '', label: 'Loading…', disabled: true }]
  }
  if (datasources.length === 0) {
    return [{ value: '', label: 'No datasources', disabled: true }]
  }
  return datasources.map((ds) => ({
    value: ds.id,
    label: `${ds.name} (${ds.type})`,
  }))
}

export function semanticModelSelectOptions(
  models: SemanticModelSummary[],
  loading?: boolean,
  emptyLabel = 'No models available',
): SelectOption[] {
  if (loading) {
    return [{ value: '', label: 'Loading…', disabled: true }]
  }
  if (models.length === 0) {
    return [{ value: '', label: emptyLabel, disabled: true }]
  }
  return models.map((m) => ({ value: m.id, label: m.name }))
}

export function userSelectOptions(users: AuthUser[], emptyLabel: string): SelectOption[] {
  return [
    { value: '', label: emptyLabel },
    ...users.map((u) => ({
      value: u.id,
      label: u.displayName ? `${u.email} (${u.displayName})` : u.email,
    })),
  ]
}

export function shareUserSelectOptions(users: AuthUser[], loading?: boolean): SelectOption[] {
  if (loading) {
    return [{ value: '', label: 'Loading…', disabled: true }]
  }
  if (users.length === 0) {
    return [{ value: '', label: 'No users', disabled: true }]
  }
  return users.map((u) => ({
    value: u.id,
    label: u.displayName ? `${u.displayName} · ${u.email}` : u.email,
    hint: u.username || undefined,
  }))
}

export function workspaceSelectOptions(workspaces: Workspace[], loading?: boolean): SelectOption[] {
  if (loading) {
    return [{ value: '', label: 'Loading…', disabled: true }]
  }
  if (workspaces.length === 0) {
    return [{ value: '', label: 'No workspaces', disabled: true }]
  }
  return workspaces.map((w) => ({
    value: w.id,
    label: w.name,
    hint: w.slug,
  }))
}

export function datasourcePickerOptions(
  datasources: Datasource[],
  emptyLabel: string,
): SelectOption[] {
  return [
    { value: '', label: emptyLabel },
    ...datasources.map((d) => ({ value: d.id, label: d.name })),
  ]
}

export function stringSelectOptions(values: string[], emptyLabel?: string): SelectOption[] {
  const opts = values.map((v) => ({ value: v, label: v }))
  if (emptyLabel !== undefined) {
    return [{ value: '', label: emptyLabel }, ...opts]
  }
  return opts
}

export function numberSelectOptions(values: number[]): SelectOption[] {
  return values.map((n) => ({ value: String(n), label: String(n) }))
}

export function roleSelectOptions(
  roles: { id: string; name: string; description?: string | null }[],
): SelectOption[] {
  return roles.map((role) => ({
    value: role.id,
    label: role.description ? `${role.name} (${role.description})` : role.name,
  }))
}

export function userDisplayLabel(
  userID: string,
  users: AuthUser[],
  fallback?: { email?: string; display_name?: string | null },
): string {
  const u = users.find((x) => x.id === userID)
  if (u) {
    return u.displayName ? `${u.displayName} · ${u.email}` : u.email
  }
  if (fallback?.email) {
    return fallback.display_name ? `${fallback.display_name} · ${fallback.email}` : fallback.email
  }
  return userID
}

export function datasourceDisplayLabel(
  datasourceID: string,
  datasources: Datasource[],
  fallbackName?: string,
): string {
  const ds = datasources.find((d) => d.id === datasourceID)
  if (ds) {
    return ds.name
  }
  if (fallbackName) {
    return fallbackName
  }
  return datasourceID
}
