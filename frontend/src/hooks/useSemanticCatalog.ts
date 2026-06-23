import { useEffect, useMemo, useState } from 'react'

import type { TableOption } from '../components/aiQuery/types'
import type { CompositeModelDetail } from '../types/composite'
import type { SemanticModelDetail } from '../types/semantic'
import { useApi } from './useApi'

export type CatalogEntryType = 'dimension' | 'metric' | 'table'

export interface CatalogEntry {
  type: CatalogEntryType
  /** Canonical name the LLM matches against the schema in the prompt. */
  name: string
  /** Human-readable label shown in the popup. */
  label: string
  /** Short hint rendered under the label. */
  hint?: string
  /** Section label for grouping. */
  group: string
}

const COMPOSITE_PREFIX = 'composite:'

function tableLabel(schema: string, table: string): string {
  return `${schema}.${table}`
}

function pushUniqueTables(out: CatalogEntry[], seen: Set<string>, schema: string, table: string) {
  if (!schema || !table) {
    return
  }
  const label = tableLabel(schema, table)
  if (seen.has(label)) {
    return
  }
  seen.add(label)
  out.push({ type: 'table', name: label, label, group: 'Tables' })
}

function catalogFromModel(model: SemanticModelDetail): CatalogEntry[] {
  const entries: CatalogEntry[] = []
  const seenTables = new Set<string>()

  for (const d of model.dimensions ?? []) {
    if (d.is_active === false) {
      continue
    }
    const name = d.name
    entries.push({
      type: 'dimension',
      name,
      label: d.label?.trim() ?? name,
      hint: d.description?.trim() ?? d.column_ref,
      group: 'Dimensions',
    })
  }
  for (const m of model.metrics ?? []) {
    if (m.is_active === false) {
      continue
    }
    entries.push({
      type: 'metric',
      name: m.name,
      label: m.label?.trim() ?? m.name,
      hint: m.description?.trim() ?? m.aggregation,
      group: 'Metrics',
    })
  }
  pushUniqueTables(entries, seenTables, model.base_schema, model.base_table)
  for (const j of model.joins ?? []) {
    if (j.is_active === false) {
      continue
    }
    pushUniqueTables(entries, seenTables, j.from_schema ?? model.base_schema, j.from_table)
    pushUniqueTables(entries, seenTables, j.to_schema ?? model.base_schema, j.to_table)
  }
  return entries
}

function catalogFromTables(tables: TableOption[]): CatalogEntry[] {
  return tables.slice(0, 200).map((t) => {
    const label = tableLabel(t.schema_name, t.table_name)
    return { type: 'table' as const, name: label, label, group: 'Tables' }
  })
}

/** Merge several models' catalogs, de-duplicating tables by label. */
function mergeCatalogs(models: (SemanticModelDetail | null)[]): CatalogEntry[] {
  const merged: CatalogEntry[] = []
  const seenTables = new Set<string>()
  for (const m of models) {
    if (!m) {
      continue
    }
    for (const e of catalogFromModel(m)) {
      if (e.type === 'table' && seenTables.has(e.name)) {
        continue
      }
      if (e.type === 'table') {
        seenTables.add(e.name)
      }
      merged.push(e)
    }
  }
  return merged
}

/**
 * Builds a flat, suggestible catalog of semantic-model objects (dimensions,
 * metrics, tables) scoped to the currently selected semantic model.
 *
 * - Regular model id → `GET /api/semantic/models/{id}`.
 * - Composite id (`composite:...`) → composite detail + each component model merged.
 * - Auto-detect (empty) → datasource tables only.
 */
export function useSemanticCatalog(
  semanticModelId: string,
  datasourceTables: TableOption[],
): { items: CatalogEntry[]; loading: boolean } {
  const { get } = useApi()
  const [items, setItems] = useState<CatalogEntry[]>([])
  const [loading, setLoading] = useState(false)

  const isComposite = semanticModelId.startsWith(COMPOSITE_PREFIX)
  const compositeId = isComposite ? semanticModelId.slice(COMPOSITE_PREFIX.length) : ''
  const modelId = !isComposite && semanticModelId ? semanticModelId : ''

  useEffect(() => {
    const cancelledRef: { current: boolean } = { current: false }

    void (async () => {
      setLoading(true)
      setItems([])
      try {
        if (isComposite && compositeId) {
          const composite = await get<CompositeModelDetail>(
            `/api/semantic/composites/${encodeURIComponent(compositeId)}`,
          )
          const components = composite?.components
          if (components?.length) {
            const models = await Promise.all(
              components.map((c) =>
                get<SemanticModelDetail>(`/api/semantic/models/${encodeURIComponent(c.model_id)}`),
              ),
            )
            if (!cancelledRef.current) {
              setItems(mergeCatalogs(models))
            }
          }
        } else if (modelId) {
          const model = await get<SemanticModelDetail>(
            `/api/semantic/models/${encodeURIComponent(modelId)}`,
          )
          if (!cancelledRef.current) {
            setItems(model ? catalogFromModel(model) : [])
          }
        } else {
          setItems(catalogFromTables(datasourceTables))
        }
      } finally {
        if (!cancelledRef.current) {
          setLoading(false)
        }
      }
    })()

    return () => {
      cancelledRef.current = true
    }
    // datasourceTables is intentionally not a dep: in auto-detect mode we only
    // want a snapshot when the selection (re)enters auto-detect, refreshed by
    // the modelId/compositeId deps.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [modelId, compositeId, isComposite, get])

  return useMemo(() => ({ items, loading }), [items, loading])
}
