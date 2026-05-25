import type { Datasource } from '../../types/metadata'

export type DatasourceAccessState = 'allowed' | 'unknown'

export interface DatasourceAccessView {
  datasource: Datasource
  access: DatasourceAccessState
}

export function buildDatasourceAccessView(
  datasources: Datasource[],
  accessibleDatasourceIDs: string[] | null,
): DatasourceAccessView[] {
  if (accessibleDatasourceIDs === null) {
    return datasources.map((datasource) => ({ datasource, access: 'unknown' }))
  }

  const allowed = new Set(accessibleDatasourceIDs)
  return datasources
    .filter((datasource) => allowed.has(datasource.id))
    .map((datasource) => ({ datasource, access: 'allowed' }))
}
