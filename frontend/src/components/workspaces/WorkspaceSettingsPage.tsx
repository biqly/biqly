import '../../styles/workspace.css'

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
import { useT } from '../../i18n'
import type { Role, Workspace, WorkspaceDatasource, WorkspaceMember } from '../../types/auth'
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

  async function onSave(e: React.SubmitEvent<HTMLFormElement>) {
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

  async function onInviteMember(e: React.SubmitEvent<HTMLFormElement>) {
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

  async function onAttachDS(e: React.SubmitEvent<HTMLFormElement>) {
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
        className="ws-settings"
        style={{
          minHeight: 300,
          display: 'flex',
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
      <div className="ws-settings">
        <div style={{ padding: 24, textAlign: 'center', color: 'var(--red-text, #f87171)' }}>
          {t('common.error')}
        </div>
      </div>
    )
  }

  return (
    <div className="ws-settings" style={{ position: 'relative' }}>
      <LoadingOverlay loading={loading}>
        <h2 className="ws-settings__title">{t('admin.workspaces.settings_title')}</h2>

        {error && <div className="ws-settings__error">{error}</div>}
        {success && <div className="ws-settings__success">{success}</div>}

        {/* ── Info / Edit Form ── */}
        <section className="ws-settings__section">
          <div className="ws-settings__info-row">
            <span className="ws-settings__badge" data-type={isPersonal ? 'personal' : 'team'}>
              {isPersonal ? t('admin.workspaces.type_personal') : t('admin.workspaces.type_team')}
            </span>
            <span className="ws-settings__slug">{workspace.slug}</span>
          </div>

          {isPersonal ? (
            <p className="ws-settings__readonly-note">{t('admin.workspaces.personal_readonly')}</p>
          ) : (
            <form
              onSubmit={(e) => {
                void onSave(e)
              }}
              className="ws-settings__form"
            >
              <label className="ws-settings__field">
                <span>{t('admin.workspaces.name')}</span>
                <input value={editName} onChange={(e) => setEditName(e.target.value)} required />
              </label>
              <label className="ws-settings__field">
                <span>{t('admin.workspaces.description')}</span>
                <input value={editDesc} onChange={(e) => setEditDesc(e.target.value)} />
              </label>
              <label className="ws-settings__check">
                <input
                  type="checkbox"
                  checked={editMFARequired}
                  onChange={(e) => setEditMFARequired(e.target.checked)}
                />
                <span>{t('admin.workspaces.mfa_required')}</span>
              </label>
              <button type="submit" className="ws-settings__btn-primary">
                {t('common.save')}
              </button>
            </form>
          )}
        </section>

        {/* ── Members ── */}
        <section className="ws-settings__section">
          <h3>{t('admin.workspaces.members')}</h3>
          {members.length === 0 ? (
            <p className="ws-settings__empty">{t('admin.workspaces.members_empty')}</p>
          ) : (
            <div className="ws-settings__table-container">
              <table className="ws-settings__table">
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
                      <td className="ws-settings__cell-control">
                        <Select
                          size="sm"
                          value={m.role_id}
                          options={roleSelectOptions(roles)}
                          onChange={(roleID) => void onChangeRole(m.user_id, roleID)}
                          disabled={!canManageMembers}
                        />
                      </td>
                      <td className="ws-settings__cell-muted">
                        {new Date(m.joined_at).toLocaleDateString()}
                      </td>
                      <td className="ws-settings__cell-actions">
                        <button
                          type="button"
                          onClick={() => {
                            void onRemoveMember(m.user_id)
                          }}
                          className="ws-settings__btn-danger"
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
              className="ws-settings__toolbar"
            >
              <label className="admin-form-label ws-settings__field-240">
                <span className="admin-label-text">{t('admin.fields.user')}</span>
                <Select
                  value={inviteUserID}
                  onChange={setInviteUserID}
                  placeholder={t('admin.workspaces.select_user')}
                  options={memberUserOptions}
                />
              </label>
              <label className="admin-form-label ws-settings__field-240">
                <span className="admin-label-text">{t('admin.workspaces.role')}</span>
                <Select
                  value={inviteRoleID}
                  onChange={setInviteRoleID}
                  placeholder={t('admin.workspaces.invite_role')}
                  options={roleSelectOptions(roles)}
                />
              </label>
              <button
                type="submit"
                className="ws-settings__btn-primary ws-settings__toolbar-submit"
              >
                {t('admin.workspaces.invite_member')}
              </button>
            </form>
          ) : (
            <p className="ws-settings__readonly-note">{restrictedNote}</p>
          )}
        </section>

        {/* ── Datasources ── */}
        <section className="ws-settings__section">
          <h3>{t('admin.workspaces.datasources')}</h3>
          {datasources.length === 0 ? (
            <p className="ws-settings__empty">{t('admin.workspaces.datasources_empty')}</p>
          ) : (
            <div className="ws-settings__table-container">
              <table className="ws-settings__table">
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
                      <td className="ws-settings__cell-muted">
                        <span className="ws-settings__level-badge">{d.access_level}</span>
                      </td>
                      <td className="ws-settings__cell-actions">
                        <button
                          type="button"
                          onClick={() => {
                            void onDetachDS(d.datasource_id)
                          }}
                          className="ws-settings__btn-danger"
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
              className="ws-settings__toolbar"
            >
              <label className="admin-form-label ws-settings__field-240">
                <span className="admin-label-text">{t('admin.workspaces.datasource_name')}</span>
                <Select
                  value={attachDsID}
                  onChange={setAttachDsID}
                  placeholder={t('admin.workspaces.select_datasource')}
                  options={attachDsOptions}
                />
              </label>
              <button
                type="submit"
                className="ws-settings__btn-primary ws-settings__toolbar-submit"
              >
                {t('admin.workspaces.attach_datasource')}
              </button>
            </form>
          ) : (
            <p className="ws-settings__readonly-note">{restrictedNote}</p>
          )}
        </section>
      </LoadingOverlay>
    </div>
  )
}
