import { apiFetch } from './apiClient'

export interface FunctionBlocklist {
  defaults: string[]
  custom: string[]
}

function functionBlocklistURL(datasourceId: string): string {
  return `/api/datasources/${encodeURIComponent(datasourceId)}/function-blocklist`
}

export function getFunctionBlocklist(datasourceId: string): Promise<FunctionBlocklist> {
  return apiFetch<FunctionBlocklist>('GET', functionBlocklistURL(datasourceId))
}

export function updateFunctionBlocklist(
  datasourceId: string,
  custom: string[],
): Promise<FunctionBlocklist> {
  return apiFetch<FunctionBlocklist>('PUT', functionBlocklistURL(datasourceId), { custom })
}
