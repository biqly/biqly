import { useCallback, useEffect, useMemo, useState } from 'react'

import type { TableOption } from '../components/aiQuery/types'
import { useLocale } from '../i18n'
import type { CompositeModelDetail } from '../types/composite'
import type { SemanticDimension, SemanticMetric, SemanticModelDetail } from '../types/semantic'
import { request, useApi } from './useApi'

export type CatalogEntryType = 'dimension' | 'metric' | 'table'

export interface CatalogEntry {
  type: CatalogEntryType
  /** Canonical name the LLM matches against the schema in the prompt. */
  name: string
  /** Human-readable label shown in the popup. */
  label: string
  /**
   * Schema-qualified reference shown under the label (e.g.
   * `schema.table.column` for a dimension, the expression for a metric, or
   * `schema.table` for a table). Hidden when identical to the label.
   */
  ref?: string
  /** Longer description rendered under the reference. */
  description?: string
  /** Section label for grouping. */
  group: string
}

const COMPOSITE_PREFIX = 'composite:'

function tableLabel(schema: string, table: string): string {
  return `${schema}.${table}`
}

/**
 * Expands a dimension's `column_ref` to a fully `schema.table.column`
 * reference for display. Already-qualified refs pass through; a bare
 * `column` is prefixed with the model's base schema+table, and a
 * `table.column` is prefixed with the base schema.
 */
function qualifyColumnRef(columnRef: string, schema: string, table: string): string {
  const ref = columnRef.trim()
  if (!ref) {
    return ''
  }
  const dots = (ref.match(/\./g) ?? []).length
  if (dots >= 2) {
    return ref
  }
  if (dots === 1) {
    return schema ? `${schema}.${ref}` : ref
  }
  return [schema, table, ref].filter(Boolean).join('.')
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
  out.push({ type: 'table', name: label, label, ref: label, group: 'Tables' })
}

/** Returns the trimmed string, or undefined when null/empty/whitespace. */
function nonEmpty(s: string | null | undefined): string | undefined {
  const t = s?.trim()
  if (!t) {
    return undefined
  }
  return t
}

function dimensionEntry(d: SemanticDimension, schema: string, table: string): CatalogEntry {
  return {
    type: 'dimension',
    name: d.name,
    label: nonEmpty(d.label) ?? d.name,
    ref: nonEmpty(qualifyColumnRef(d.column_ref, schema, table)),
    description: nonEmpty(d.description),
    group: 'Dimensions',
  }
}

function metricEntry(m: SemanticMetric): CatalogEntry {
  return {
    type: 'metric',
    name: m.name,
    label: nonEmpty(m.label) ?? m.name,
    ref: nonEmpty(m.expression) ?? nonEmpty(m.aggregation),
    description: nonEmpty(m.description),
    group: 'Metrics',
  }
}

function catalogFromModel(model: SemanticModelDetail): CatalogEntry[] {
  const entries: CatalogEntry[] = []
  const seenTables = new Set<string>()

  for (const d of model.dimensions ?? []) {
    if (d.is_active !== false) {
      entries.push(dimensionEntry(d, model.base_schema, model.base_table))
    }
  }
  for (const m of model.metrics ?? []) {
    if (m.is_active !== false) {
      entries.push(metricEntry(m))
    }
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
    return { type: 'table' as const, name: label, label, ref: label, group: 'Tables' }
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
 * Best-effort backfill of locale translations for a semantic model. The AI
 * service translates the model's (and its dimensions'/metrics') label and
 * description into the request locale — sent via the `X-Locale` header — and
 * caches them; the subsequent model read overlays them. The LLM is only hit
 * when something is missing, so repeat calls are cheap. Errors are swallowed:
 * a translation hiccup must never block the @-mention catalog.
 */
async function ensureModelTranslated(modelId: string, force = false): Promise<void> {
  const qs = force ? '?force=true' : ''
  await request('POST', `/api/ai/semantic/models/${encodeURIComponent(modelId)}/translate${qs}`, {})
}

export interface SemanticCatalog {
  items: CatalogEntry[]
  loading: boolean
  /** Whether a re-translate action applies (non-English locale + a model selected). */
  canRetranslate: boolean
  /** Forces a fresh translation of the selected model, overwriting stale cache, then reloads. */
  retranslate: () => Promise<void>
  /** True while a forced re-translation is in flight. */
  retranslating: boolean
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
): SemanticCatalog {
  const { get } = useApi()
  const [locale] = useLocale()
  const [items, setItems] = useState<CatalogEntry[]>([])
  const [loading, setLoading] = useState(false)
  const [retranslating, setRetranslating] = useState(false)
  const [reloadTick, setReloadTick] = useState(0)

  const isComposite = semanticModelId.startsWith(COMPOSITE_PREFIX)
  const compositeId = isComposite ? semanticModelId.slice(COMPOSITE_PREFIX.length) : ''
  const modelId = !isComposite && semanticModelId ? semanticModelId : ''
  // English is the stored base language — only non-English needs a backfill.
  const needsTranslation = locale !== 'en'
  const canRetranslate = needsTranslation && (modelId !== '' || compositeId !== '')

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
            if (needsTranslation) {
              await Promise.all(components.map((c) => ensureModelTranslated(c.model_id)))
            }
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
          if (needsTranslation) {
            await ensureModelTranslated(modelId)
          }
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
    // the modelId/compositeId deps. reloadTick re-runs after a forced re-translate.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [modelId, compositeId, isComposite, needsTranslation, reloadTick, get])

  const retranslate = useCallback(async () => {
    if (!canRetranslate || retranslating) {
      return
    }
    setRetranslating(true)
    try {
      if (compositeId) {
        const composite = await get<CompositeModelDetail>(
          `/api/semantic/composites/${encodeURIComponent(compositeId)}`,
        )
        await Promise.all(
          (composite?.components ?? []).map((c) => ensureModelTranslated(c.model_id, true)),
        )
      } else if (modelId) {
        await ensureModelTranslated(modelId, true)
      }
      // Re-run the load effect so the freshly cached translations are read back.
      setReloadTick((t) => t + 1)
    } finally {
      setRetranslating(false)
    }
  }, [canRetranslate, retranslating, compositeId, modelId, get])

  return useMemo(
    () => ({ items, loading, canRetranslate, retranslate, retranslating }),
    [items, loading, canRetranslate, retranslate, retranslating],
  )
}
