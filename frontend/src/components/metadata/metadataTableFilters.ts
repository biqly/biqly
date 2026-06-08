import type { TableRow } from '../../types/semantic'

export function filterMetadataTables(
  tables: TableRow[],
  schemaFilter: string,
  typeFilter: string,
): TableRow[] {
  return tables.filter((tab) => {
    if (schemaFilter && tab.schema_name !== schemaFilter) {
      return false
    }
    if (typeFilter && tab.table_type !== typeFilter) {
      return false
    }
    return true
  })
}
