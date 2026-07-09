import { ApiError, fetchJSON } from './apiClient'

export interface DbtImportResult {
  imported_models: {
    model: { id: string; name: string }
    validation: { valid: boolean; errors?: string[] }
  }[]
  skipped: string[]
  warnings: string[]
}

interface ImportDbtProjectInput {
  datasourceId: string
  manifest: File
  catalog?: File | null
}

export async function importDbtProject({
  datasourceId,
  manifest,
  catalog,
}: ImportDbtProjectInput): Promise<DbtImportResult> {
  const form = new FormData()
  form.append('manifest', manifest)
  if (catalog) {
    form.append('catalog', catalog)
  }

  const params = new URLSearchParams({ datasource_id: datasourceId })
  const { data, status, error } = await fetchJSON<DbtImportResult>(
    `/api/catalog/dbt/import?${params.toString()}`,
    { method: 'POST', body: form },
  )
  if (error) {
    throw new ApiError(error, status)
  }
  if (!data) {
    throw new ApiError('Expected import response data', status)
  }
  return data
}
