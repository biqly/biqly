import clsx from 'clsx'
import { useCallback, useEffect, useMemo, useState } from 'react'

import {
  addWorkspaceMember,
  attachWorkspaceDatasource,
  detachWorkspaceDatasource,
  getWorkspace,
  listRoles,
  listWorkspaceDatasources,
  listWorkspaceMembers,
  removeWorkspaceMember,
  updateWorkspace,
  updateWorkspaceMemberRole,
} from '../../api/admin'
import { useAdminLookups } from '../../hooks/useAdminLookups'
import { useConfirm } from '../../hooks/useConfirm'
import { localeLanguageTag, useLocale, useT } from '../../i18n'
import type { Role, Workspace, WorkspaceDatasource, WorkspaceMember } from '../../types/auth'
import { formatDateOnly } from '../../utils/formatters'
import {
  adminBtnAutoWidthClass,
  adminFormLabelClass,
  adminLabelTextClass,
} from '../admin/adminClasses'
import {
  datasourceDisplayLabel,
  datasourcePickerOptions,
  roleSelectOptions,
  userDisplayLabel,
  userSelectOptions,
} from '../admin/adminSelectOptions'
import { useAuth } from '../auth/AuthProvider'
import { LoadingOverlay } from '../ui/LoadingOverlay'
import { Select } from '../ui/Select'

interface Props {
  token: string
  workspaceID: string
}

export function WorkspaceSettingsPage({ token, workspaceID }: Props) {
  const t = useT()
  const [locale] = useLocale()
  const confirm = useConfirm()
  const { hasPermission } = useAuth()
  const { users, datasources: allDatasources } = useAdminLookups(token)
  const [workspace, setWorkspace] = useState<Workspace | null>(null)
  const [members, setMembers] = useState<WorkspaceMember[]>([])
  const [datasources, setDatasources] = useState<WorkspaceDatasource[]>([])
  const [roles, setRoles] = useState<Role[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [success, setSuccess] = useState<string | null>(null)

  const [editName, setEditName] = useState('')
  const [editDesc, setEditDesc] = useState('')
  const [editMFARequired, setEditMFARequired] = useState(false)

  const [inviteUserID, setInviteUserID] = useState('')
  const [inviteRoleID, setInviteRoleID] = useState('')

  const [attachDsID, setAttachDsID] = useState('')

  const memberUserOptions = useMemo(
    () => userSelectOptions(users, t('admin.workspaces.select_user')),
    [users, t],
  )
  const attachDsOptions = useMemo(
    () => datasourcePickerOptions(allDatasources, t('admin.workspaces.select_datasource')),
    [allDatasources, t],
  )

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const [ws, m, ds, rRes] = await Promise.all([
        getWorkspace(token, workspaceID),
        listWorkspaceMembers(token, workspaceID),
        listWorkspaceDatasources(token, workspaceID),
        listRoles(token),
      ])
      setWorkspace(ws)
      setMembers(m)
      setDatasources(ds)
      setRoles(rRes.roles)
      setEditName(ws.name)
      setEditDesc(ws.description ?? '')
      setEditMFARequired(ws.mfa_required)
      setError(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      setLoading(false)
    }
  }, [token, workspaceID])

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- async data fetch on scope change.
    void load()
  }, [load])

  const isPersonal = workspace?.is_personal ?? false
  // A workspace is manageable only when it is not a personal workspace AND the
  // caller actually holds the relevant permission (super admins always do).
  // The backend enforces the same checks; this only drives the UI affordances.
  const canManageMembers = !isPersonal && hasPermission('admin:workspaces', 'workspace:invite')
  const canManageDatasources =
    !isPersonal && hasPermission('admin:workspaces', 'workspace:manage_datasources')
  const restrictedNote = isPersonal
    ? t('admin.workspaces.personal_readonly')
    : t('admin.workspaces.no_manage_permission')

  async function onSave(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault()
    if (isPersonal) {
      return
    }
    try {
      await updateWorkspace(token, workspaceID, editName, editDesc || undefined, editMFARequired)
      setSuccess(t('admin.workspaces.save_success'))
      setTimeout(() => setSuccess(null), 3000)
      void load()
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }

  async function onInviteMember(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault()
    if (!inviteUserID.trim() || !inviteRoleID) {
      return
    }
    try {
      await addWorkspaceMember(token, workspaceID, inviteUserID.trim(), inviteRoleID)
      setInviteUserID('')
      setInviteRoleID('')
      void load()
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }

  async function onRemoveMember(userID: string) {
    const ok = await confirm({
      title: t('admin.workspaces.confirm_remove_member'),
      variant: 'danger',
    })
    if (!ok) {
      return
    }
    try {
      await removeWorkspaceMember(token, workspaceID, userID)
      void load()
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }

  async function onChangeRole(userID: string, roleID: string) {
    try {
      await updateWorkspaceMemberRole(token, workspaceID, userID, roleID)
      void load()
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }

  async function onAttachDS(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault()
    if (!attachDsID.trim()) {
      return
    }
    try {
      await attachWorkspaceDatasource(token, workspaceID, attachDsID.trim())
      setAttachDsID('')
      void load()
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }

  async function onDetachDS(dsID: string) {
    const ok = await confirm({
      title: t('admin.workspaces.confirm_detach'),
      variant: 'danger',
    })
    if (!ok) {
      return
    }
    try {
      await detachWorkspaceDatasource(token, workspaceID, dsID)
      void load()
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }

  if (!workspace && loading) {
    return (
      <div
        className="flex flex-col gap-5"
        style={{
          minHeight: 300,
          alignItems: 'center',
          justifyContent: 'center',
          position: 'relative',
        }}
      >
        <LoadingOverlay loading={true} />
      </div>
    )
  }

  if (!workspace) {
    return (
      <div className="flex flex-col gap-5">
        <div className="py-6 px-0 text-center text-error">{t('common.error')}</div>
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-5" style={{ position: 'relative' }}>
      <LoadingOverlay loading={loading}>
        <h2 className="m-0 text-[20px]">{t('admin.workspaces.settings_title')}</h2>

        {error && (
          <div className="py-2.5 px-3.5 bg-error/10 border border-error/25 rounded-[6px] text-error text-[13px]">
            {error}
          </div>
        )}
        {success && (
          <div className="py-2.5 px-3.5 bg-success/10 border border-success/25 rounded-[6px] text-success text-[13px]">
            {success}
          </div>
        )}

        {/* ── Info / Edit Form ── */}
        <section className={`p-4 border border-border rounded-[8px] bg-card`}>
          <div className="flex items-center gap-2.5 mb-3">
            <span
              className={clsx(
                'inline-block py-[2px] px-2.5 rounded-[12px] text-[11px] font-semibold uppercase tracking-[0.5px]',
                isPersonal ? 'bg-accent/10 text-accent' : 'bg-success/10 text-success',
              )}
            >
              {isPersonal ? t('admin.workspaces.type_personal') : t('admin.workspaces.type_team')}
            </span>
            <span className="font-mono text-[12px] text-foreground-muted">{workspace.slug}</span>
          </div>

          {isPersonal ? (
            <p className="text-foreground-muted text-[13px] italic m-0">
              {t('admin.workspaces.personal_readonly')}
            </p>
          ) : (
            <form
              onSubmit={(e) => {
                void onSave(e)
              }}
              className="flex gap-2.5 items-end flex-wrap"
            >
              <label className="flex flex-col gap-1 text-[12px] text-foreground-muted">
                <span>{t('admin.workspaces.name')}</span>
                <input
                  className="py-[7px] px-2.5 min-w-[200px]"
                  value={editName}
                  onChange={(e) => setEditName(e.target.value)}
                  required
                />
              </label>
              <label className="flex flex-col gap-1 text-[12px] text-foreground-muted">
                <span>{t('admin.workspaces.description')}</span>
                <input
                  className="py-[7px] px-2.5 min-w-[200px]"
                  value={editDesc}
                  onChange={(e) => setEditDesc(e.target.value)}
                />
              </label>
              <label className="inline-flex items-center gap-2 min-h-[32px] text-[13px] text-foreground whitespace-nowrap">
                <input
                  type="checkbox"
                  className="w-4 h-4 m-0"
                  checked={editMFARequired}
                  onChange={(e) => setEditMFARequired(e.target.checked)}
                />
                <span>{t('admin.workspaces.mfa_required')}</span>
              </label>
              <button type="submit" className={`btn btn-primary ${adminBtnAutoWidthClass}`}>
                {t('common.save')}
              </button>
            </form>
          )}
        </section>

        {/* ── Members ── */}
        <section className={`p-4 border border-border rounded-[8px] bg-card`}>
          <h3 className="m-0 mb-3 text-[15px] font-semibold">{t('admin.workspaces.members')}</h3>
          {members.length === 0 ? (
            <p className="text-foreground-muted text-[13px] my-2 mx-0">
              {t('admin.workspaces.members_empty')}
            </p>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full border-collapse text-[13px] mb-3 [&_th]:py-2.5 [&_th]:px-3 [&_th]:text-left [&_th]:align-middle [&_th]:border-b [&_th]:border-border [&_td]:py-2.5 [&_td]:px-3 [&_td]:text-left [&_td]:align-middle [&_td]:border-b [&_td]:border-border [&_th]:font-semibold [&_th]:text-[11px] [&_th]:uppercase [&_th]:tracking-[0.4px] [&_th]:text-foreground-muted [&_th]:whitespace-nowrap">
                <thead>
                  <tr>
                    <th>{t('admin.fields.user')}</th>
                    <th>{t('admin.workspaces.role')}</th>
                    <th>{t('admin.workspaces.joined_at')}</th>
                    <th>{t('common.actions')}</th>
                  </tr>
                </thead>
                <tbody>
                  {members.map((m) => (
                    <tr key={m.user_id}>
                      <td>
                        {userDisplayLabel(m.user_id, users, {
                          email: m.email,
                          display_name: m.display_name,
                        })}
                      </td>
                      <td className="min-w-[240px] w-[28%]">
                        <Select
                          size="sm"
                          value={m.role_id}
                          options={roleSelectOptions(roles)}
                          onChange={(roleID) => void onChangeRole(m.user_id, roleID)}
                          disabled={!canManageMembers}
                        />
                      </td>
                      <td className="text-foreground-muted whitespace-nowrap">
                        {formatDateOnly(m.joined_at, localeLanguageTag(locale))}
                      </td>
                      <td className="w-[1%] whitespace-nowrap text-right">
                        <button
                          type="button"
                          onClick={() => {
                            void onRemoveMember(m.user_id)
                          }}
                          className="inline-flex items-center justify-center min-h-[1.85rem] px-2.5 bg-transparent border border-error/30 text-error rounded-[6px] cursor-pointer text-[12px] leading-[1.2] hover:bg-error/6 transition-colors"
                          disabled={!canManageMembers}
                        >
                          {t('common.delete')}
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}

          {canManageMembers ? (
            <form
              onSubmit={(e) => {
                void onInviteMember(e)
              }}
              className={`flex gap-3 items-end flex-wrap mt-3 pt-3 border-t border-border`}
            >
              <label
                className={`${adminFormLabelClass} flex-1 shrink-0 basis-[240px] min-w-[240px] max-w-[320px]`}
              >
                <span className={adminLabelTextClass}>{t('admin.fields.user')}</span>
                <Select
                  value={inviteUserID}
                  onChange={setInviteUserID}
                  placeholder={t('admin.workspaces.select_user')}
                  options={memberUserOptions}
                />
              </label>
              <label
                className={`${adminFormLabelClass} flex-1 shrink-0 basis-[240px] min-w-[240px] max-w-[320px]`}
              >
                <span className={adminLabelTextClass}>{t('admin.workspaces.role')}</span>
                <Select
                  value={inviteRoleID}
                  onChange={setInviteRoleID}
                  placeholder={t('admin.workspaces.invite_role')}
                  options={roleSelectOptions(roles)}
                />
              </label>
              <button
                type="submit"
                className={`btn btn-primary ${adminBtnAutoWidthClass} self-end min-h-[2.1rem] mb-0`}
              >
                {t('admin.workspaces.invite_member')}
              </button>
            </form>
          ) : (
            <p className="text-foreground-muted text-[13px] italic m-0">{restrictedNote}</p>
          )}
        </section>

        {/* ── Datasources ── */}
        <section className={`p-4 border border-border rounded-[8px] bg-card`}>
          <h3 className="m-0 mb-3 text-[15px] font-semibold">
            {t('admin.workspaces.datasources')}
          </h3>
          {datasources.length === 0 ? (
            <p className="text-foreground-muted text-[13px] my-2 mx-0">
              {t('admin.workspaces.datasources_empty')}
            </p>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full border-collapse text-[13px] mb-3 [&_th]:py-2.5 [&_th]:px-3 [&_th]:text-left [&_th]:align-middle [&_th]:border-b [&_th]:border-border [&_td]:py-2.5 [&_td]:px-3 [&_td]:text-left [&_td]:align-middle [&_td]:border-b [&_td]:border-border [&_th]:font-semibold [&_th]:text-[11px] [&_th]:uppercase [&_th]:tracking-[0.4px] [&_th]:text-foreground-muted [&_th]:whitespace-nowrap">
                <thead>
                  <tr>
                    <th>{t('admin.workspaces.datasource_name')}</th>
                    <th>{t('admin.datasource_access.level')}</th>
                    <th>{t('common.actions')}</th>
                  </tr>
                </thead>
                <tbody>
                  {datasources.map((d) => (
                    <tr key={d.datasource_id}>
                      <td>
                        {datasourceDisplayLabel(d.datasource_id, allDatasources, d.datasource_name)}
                      </td>
                      <td className="text-foreground-muted whitespace-nowrap">
                        <span className="inline-block py-[2px] px-2 rounded-[10px] text-[11px] font-medium bg-accent/10 text-accent">
                          {d.access_level}
                        </span>
                      </td>
                      <td className="w-[1%] whitespace-nowrap text-right">
                        <button
                          type="button"
                          onClick={() => {
                            void onDetachDS(d.datasource_id)
                          }}
                          className="inline-flex items-center justify-center min-h-[1.85rem] px-2.5 bg-transparent border border-error/30 text-error rounded-[6px] cursor-pointer text-[12px] leading-[1.2] hover:bg-error/6 transition-colors"
                          disabled={!canManageDatasources}
                        >
                          {t('common.delete')}
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}

          {canManageDatasources ? (
            <form
              onSubmit={(e) => {
                void onAttachDS(e)
              }}
              className={`flex gap-3 items-end flex-wrap mt-3 pt-3 border-t border-border`}
            >
              <label
                className={`${adminFormLabelClass} flex-1 shrink-0 basis-[240px] min-w-[240px] max-w-[320px]`}
              >
                <span className={adminLabelTextClass}>{t('admin.workspaces.datasource_name')}</span>
                <Select
                  value={attachDsID}
                  onChange={setAttachDsID}
                  placeholder={t('admin.workspaces.select_datasource')}
                  options={attachDsOptions}
                />
              </label>
              <button
                type="submit"
                className={`btn btn-primary ${adminBtnAutoWidthClass} self-end min-h-[2.1rem] mb-0`}
              >
                {t('admin.workspaces.attach_datasource')}
              </button>
            </form>
          ) : (
            <p className="text-foreground-muted text-[13px] italic m-0">{restrictedNote}</p>
          )}
        </section>
      </LoadingOverlay>
    </div>
  )
}
