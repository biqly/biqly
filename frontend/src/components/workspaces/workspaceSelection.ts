import type { Workspace } from '../../types/auth'

export function resolveActiveWorkspace(
  workspaces: Workspace[],
  preferredID: string | null,
): Workspace | null {
  if (workspaces.length === 0) {
    return null
  }
  if (preferredID) {
    const preferred = workspaces.find((workspace) => workspace.id === preferredID)
    if (preferred) {
      return preferred
    }
  }
  return workspaces.find((workspace) => workspace.is_personal) ?? workspaces[0] ?? null
}
