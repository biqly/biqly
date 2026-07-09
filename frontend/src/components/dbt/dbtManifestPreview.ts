interface DbtManifestNode {
  resource_type?: unknown
  name?: unknown
  alias?: unknown
  config?: { enabled?: unknown } | null
}

interface DbtManifest {
  nodes?: unknown
}

export function dbtManifestModelNames(value: unknown): string[] {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    return []
  }
  const manifest = value as DbtManifest
  if (!manifest.nodes || typeof manifest.nodes !== 'object' || Array.isArray(manifest.nodes)) {
    return []
  }

  return Object.values(manifest.nodes)
    .filter(isEnabledModel)
    .map((node) => preferredModelName(node))
    .filter((name): name is string => name !== '')
    .sort((left, right) => left.localeCompare(right))
}

function isEnabledModel(value: unknown): value is DbtManifestNode {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    return false
  }
  const node = value as DbtManifestNode
  return node.resource_type === 'model' && node.config?.enabled !== false
}

function preferredModelName(node: DbtManifestNode): string {
  const alias = typeof node.alias === 'string' ? node.alias.trim() : ''
  if (alias) {
    return alias
  }
  return typeof node.name === 'string' ? node.name.trim() : ''
}
