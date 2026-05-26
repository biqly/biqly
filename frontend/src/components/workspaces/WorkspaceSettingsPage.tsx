import { useEffect, useState, useCallback } from 'react'
import {
  getWorkspace,
  updateWorkspace,
  listWorkspaceMembers,
  addWorkspaceMember,
  removeWorkspaceMember,
  updateWorkspaceMemberRole,
  listWorkspaceDatasources,
  attachWorkspaceDatasource,
  detachWorkspaceDatasource,
  listRoles,
} from '../../api/admin'
import { useT } from '../../i18n'
import type { Workspace, WorkspaceMember, WorkspaceDatasource, Role } from '../../types/auth'

interface Props {
  token: string
  workspaceID: string
  onBack: () => void
}

export function WorkspaceSettingsPage({ token, workspaceID, onBack }: Props) {
  const t = useT()
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
      setEditDesc(ws.description || '')
      setEditMFARequired(ws.mfa_required)
      setError(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      setLoading(false)
    }
  }, [token, workspaceID])

  useEffect(() => {
    load()
  }, [load])

  const isPersonal = workspace?.is_personal ?? false

  async function onSave(e: React.FormEvent) {
    e.preventDefault()
    if (isPersonal) return
    try {
      await updateWorkspace(token, workspaceID, editName, editDesc || undefined, editMFARequired)
      setSuccess(t('admin.workspaces.save_success'))
      setTimeout(() => setSuccess(null), 3000)
      load()
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }

  async function onInviteMember(e: React.FormEvent) {
    e.preventDefault()
    if (!inviteUserID.trim() || !inviteRoleID) return
    try {
      await addWorkspaceMember(token, workspaceID, inviteUserID.trim(), inviteRoleID)
      setInviteUserID('')
      setInviteRoleID('')
      load()
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }

  async function onRemoveMember(userID: string) {
    if (!confirm(t('admin.workspaces.confirm_remove_member'))) return
    try {
      await removeWorkspaceMember(token, workspaceID, userID)
      load()
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }

  async function onChangeRole(userID: string, roleID: string) {
    try {
      await updateWorkspaceMemberRole(token, workspaceID, userID, roleID)
      load()
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }

  async function onAttachDS(e: React.FormEvent) {
    e.preventDefault()
    if (!attachDsID.trim()) return
    try {
      await attachWorkspaceDatasource(token, workspaceID, attachDsID.trim())
      setAttachDsID('')
      load()
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }

  async function onDetachDS(dsID: string) {
    if (!confirm(t('admin.workspaces.confirm_detach'))) return
    try {
      await detachWorkspaceDatasource(token, workspaceID, dsID)
      load()
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }

  if (loading) return <div style={{ padding: 24 }}>{t('common.loading')}</div>
  if (!workspace) return <div style={{ padding: 24 }}>{t('common.error')}</div>

  return (
    <div className="ws-settings">
      <button onClick={onBack} className="ws-settings__back">{t('admin.workspaces.back')}</button>

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
          <form onSubmit={onSave} className="ws-settings__form">
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
            <button type="submit" className="ws-settings__btn-primary">{t('common.save')}</button>
          </form>
        )}
      </section>

      {/* ── Members ── */}
      <section className="ws-settings__section">
        <h3>{t('admin.workspaces.members')}</h3>
        {members.length === 0 ? (
          <p className="ws-settings__empty">{t('admin.workspaces.members_empty')}</p>
        ) : (
          <table className="ws-settings__table">
            <thead>
              <tr>
                <th>{t('admin.workspaces.invite_user')}</th>
                <th>{t('admin.workspaces.role')}</th>
                <th>{t('admin.workspaces.joined_at')}</th>
                <th>{t('common.actions')}</th>
              </tr>
            </thead>
            <tbody>
              {members.map((m) => (
                <tr key={m.user_id}>
                  <td className="ws-settings__mono">{m.user_id.slice(0, 8)}…</td>
                  <td>
                    <select
                      value={m.role_id}
                      onChange={(e) => onChangeRole(m.user_id, e.target.value)}
                    >
                      {roles.map((r) => (
                        <option key={r.id} value={r.id}>{r.name}</option>
                      ))}
                    </select>
                  </td>
                  <td>{new Date(m.joined_at).toLocaleDateString()}</td>
                  <td>
                    <button onClick={() => onRemoveMember(m.user_id)} className="ws-settings__btn-danger">
                      {t('common.delete')}
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}

        <form onSubmit={onInviteMember} className="ws-settings__inline-form">
          <input
            placeholder={t('admin.workspaces.invite_user')}
            value={inviteUserID}
            onChange={(e) => setInviteUserID(e.target.value)}
            required
          />
          <select value={inviteRoleID} onChange={(e) => setInviteRoleID(e.target.value)} required>
            <option value="">{t('admin.workspaces.invite_role')}</option>
            {roles.map((r) => (
              <option key={r.id} value={r.id}>{r.name}</option>
            ))}
          </select>
          <button type="submit" className="ws-settings__btn-primary">
            {t('admin.workspaces.invite_member')}
          </button>
        </form>
      </section>

      {/* ── Datasources ── */}
      <section className="ws-settings__section">
        <h3>{t('admin.workspaces.datasources')}</h3>
        {datasources.length === 0 ? (
          <p className="ws-settings__empty">{t('admin.workspaces.datasources_empty')}</p>
        ) : (
          <table className="ws-settings__table">
            <thead>
              <tr>
                <th>{t('admin.workspaces.datasource_id')}</th>
                <th>{t('admin.datasource_access.level')}</th>
                <th>{t('common.actions')}</th>
              </tr>
            </thead>
            <tbody>
              {datasources.map((d) => (
                <tr key={d.datasource_id}>
                  <td className="ws-settings__mono">{d.datasource_id.slice(0, 8)}…</td>
                  <td><span className="ws-settings__level-badge">{d.access_level}</span></td>
                  <td>
                    <button onClick={() => onDetachDS(d.datasource_id)} className="ws-settings__btn-danger">
                      {t('common.delete')}
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}

        <form onSubmit={onAttachDS} className="ws-settings__inline-form">
          <input
            placeholder={t('admin.workspaces.datasource_id')}
            value={attachDsID}
            onChange={(e) => setAttachDsID(e.target.value)}
            required
          />
          <button type="submit" className="ws-settings__btn-primary">
            {t('admin.workspaces.attach_datasource')}
          </button>
        </form>
      </section>
    </div>
  )
}
