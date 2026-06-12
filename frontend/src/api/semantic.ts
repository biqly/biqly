import { request } from '../hooks/useApi'
import type { SemanticModelFieldsPage } from '../types/semantic'

/**
 * GET /api/semantic/models/:id/fields?page=&page_size=
 * Same URL and params previously built inline in FieldPermissionPanel.
 */
export function listSemanticModelFields(
  modelId: string,
  page: number,
  pageSize: number,
): Promise<{ data: SemanticModelFieldsPage | null; error: string | null }> {
  const params = new URLSearchParams({
    page: String(page),
    page_size: String(pageSize),
  })
  return request<SemanticModelFieldsPage>(
    'GET',
    `/api/semantic/models/${encodeURIComponent(modelId)}/fields?${params}`,
  )
}
