import { useEffect, useMemo, useState } from 'react'

import { fetchDescribeBatchConflict } from '../../api/describeBatchConflict'
import type { TranslationKey } from '../../i18n'
import type { TableRow } from '../../types/semantic'
import { type BulkEntry, sortBulkEntriesForDisplay } from './bulkProgress'
import { selectBulkDescribeTargets } from './metadataBulkTargetTables'

export interface BulkDescribeModalConfig {
  sample_size: number
  skip_existing: boolean
}

export function useMetadataBulkDescribeModalState({
  open,
  datasourceId,
  tables,
  typeOptions,
  bulkRunning,
  bulkEntries,
  t,
  onStartBulk,
  onRefreshTables,
}: {
  open: boolean
  datasourceId: string
  tables: TableRow[]
  typeOptions: string[]
  bulkRunning: boolean
  bulkEntries: BulkEntry[]
  t: (key: TranslationKey, params?: Record<string, string | number>) => string
  onStartBulk: (params: {
    targets: TableRow[]
    sampleSize: number
    skipExisting: boolean
    onConflict: (message: string) => void
    onFinished: () => void
  }) => void
  onRefreshTables: () => void
}) {
  const [bulkConfig, setBulkConfig] = useState<BulkDescribeModalConfig>({
    sample_size: 10,
    skip_existing: true,
  })
  const [bulkTypeEnabled, setBulkTypeEnabled] = useState<Record<string, boolean>>({})
  const [bulkSchemaRestrict, setBulkSchemaRestrict] = useState(false)
  const [bulkSchemasSelected, setBulkSchemasSelected] = useState<string[]>([])
  const [bulkScopeConflict, setBulkScopeConflict] = useState<{
    message: string
    schemas?: string
  } | null>(null)

  useEffect(() => {
    if (!open) {
      return
    }
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setBulkTypeEnabled(Object.fromEntries(typeOptions.map((ty) => [ty, true])))
    setBulkSchemaRestrict(false)
    setBulkSchemasSelected([])
  }, [open, typeOptions])

  const bulkTargetTables = useMemo(
    () =>
      selectBulkDescribeTargets(tables, bulkTypeEnabled, bulkSchemaRestrict, bulkSchemasSelected),
    [tables, bulkTypeEnabled, bulkSchemaRestrict, bulkSchemasSelected],
  )

  const bulkHasObjectType =
    typeOptions.length === 0 || typeOptions.some((ty) => bulkTypeEnabled[ty])

  const bulkScopeSchemas = useMemo(() => {
    if (bulkSchemaRestrict) {
      return [...bulkSchemasSelected].sort((a, b) => a.localeCompare(b))
    }
    return [...new Set(bulkTargetTables.map((tab) => tab.schema_name))].sort((a, b) =>
      a.localeCompare(b),
    )
  }, [bulkSchemaRestrict, bulkSchemasSelected, bulkTargetTables])

  useEffect(() => {
    if (!datasourceId || bulkScopeSchemas.length === 0) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setBulkScopeConflict(null)
      return
    }
    const controller = new AbortController()
    void fetchDescribeBatchConflict(datasourceId, bulkScopeSchemas).then((res) => {
      if (controller.signal.aborted) {
        return
      }
      if (res?.conflict) {
        setBulkScopeConflict({
          message: t('metadata.already_running'),
          schemas: res.scope_schemas?.join(', ') ?? bulkScopeSchemas.join(', '),
        })
      } else {
        setBulkScopeConflict(null)
      }
    })
    return () => {
      controller.abort()
    }
  }, [datasourceId, bulkScopeSchemas, t])

  const bulkCanStart =
    bulkTargetTables.length > 0 &&
    bulkHasObjectType &&
    (!bulkSchemaRestrict || bulkSchemasSelected.length > 0) &&
    !bulkScopeConflict &&
    !bulkRunning

  const bulkEntriesDisplay = useMemo(
    () => (bulkEntries.length > 0 ? sortBulkEntriesForDisplay(bulkEntries) : []),
    [bulkEntries],
  )

  const runBulkDescribe = () => {
    const targets = bulkTargetTables
    if (!datasourceId || targets.length === 0 || bulkScopeConflict) {
      return
    }
    setBulkScopeConflict(null)
    onStartBulk({
      targets,
      sampleSize: bulkConfig.sample_size,
      skipExisting: bulkConfig.skip_existing,
      onConflict: (message) => {
        setBulkScopeConflict({ message })
      },
      onFinished: () => {
        onRefreshTables()
      },
    })
  }

  return {
    bulkConfig,
    setBulkConfig,
    bulkTypeEnabled,
    setBulkTypeEnabled,
    bulkSchemaRestrict,
    setBulkSchemaRestrict,
    bulkSchemasSelected,
    setBulkSchemasSelected,
    bulkScopeConflict,
    bulkTargetTables,
    bulkHasObjectType,
    bulkCanStart,
    bulkEntriesDisplay,
    runBulkDescribe,
  }
}
