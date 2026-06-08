import type { TableRow } from '../../types/semantic'

export function selectBulkDescribeTargets(
  tables: TableRow[],
  bulkTypeEnabled: Record<string, boolean>,
  bulkSchemaRestrict: boolean,
  bulkSchemasSelected: string[],
): TableRow[] {
  const restrictTypes = Object.keys(bulkTypeEnabled).length > 0
  return tables.filter((tab) => {
    if (restrictTypes && !bulkTypeEnabled[tab.table_type]) {
      return false
    }
    if (!bulkSchemaRestrict) {
      return true
    }
    if (bulkSchemasSelected.length === 0) {
      return false
    }
    return bulkSchemasSelected.includes(tab.schema_name)
  })
}
