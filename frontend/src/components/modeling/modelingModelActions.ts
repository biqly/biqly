import type { TranslationKey } from '../../i18n'
import type {
  GenerateSemanticModelResponse,
  SemanticJoin,
  SemanticModelDetail,
  SemanticModelSummary,
} from '../../types/semantic'
import { schemaImpact, tableImpact } from './modelingImpact'
import type { JoinForm, SuggestedJoin } from './types'
import { publishModelRequest, suggestedJoinToPayload } from './types'
import { buildJoinPayload, canSaveJoinForm } from './utils'

type ConfirmFn = (opts: {
  title: string
  message?: string
  variant?: 'default' | 'danger' | 'warning'
  confirmLabel?: string
}) => Promise<boolean>

type TFn = (key: TranslationKey, vars?: Record<string, string | number>) => string

export async function runCreateModel(deps: {
  datasourceId: string
  creatingModel: boolean
  postData: <T>(url: string, body: unknown, opts?: { timeout?: number }) => Promise<T | null>
  setCreatingModel: (v: boolean) => void
  setMessage: (m: string | null) => void
  setModelId: (id: string) => void
  setModel: (m: SemanticModelDetail | null) => void
  refreshModels: (id: string) => Promise<void>
  t: TFn
}) {
  const {
    datasourceId,
    creatingModel,
    postData,
    setCreatingModel,
    setMessage,
    setModelId,
    setModel,
    refreshModels,
    t,
  } = deps
  if (!datasourceId || creatingModel) {
    return
  }
  setCreatingModel(true)
  setMessage(null)
  try {
    const res = await postData<GenerateSemanticModelResponse>(
      '/api/semantic/models/generate',
      { datasource_id: datasourceId, publish: true },
      { timeout: 180_000 },
    )
    if (!res?.model) {
      return
    }
    setModelId(res.model.id)
    setModel(res.model)
    await refreshModels(res.model.id)
    setMessage(res.published ? t('modeling.created_published') : t('modeling.created_draft'))
  } finally {
    setCreatingModel(false)
  }
}

export async function runRemoveModel(deps: {
  model: SemanticModelDetail | null
  confirm: ConfirmFn
  deleteData: (url: string) => Promise<unknown>
  get: <T>(url: string) => Promise<T | null>
  datasourceId: string
  setModels: (models: SemanticModelSummary[]) => void
  setModelId: (id: string) => void
  setModel: (m: SemanticModelDetail | null) => void
  setMessage: (m: string | null) => void
  t: TFn
}) {
  const {
    model,
    confirm,
    deleteData,
    get,
    datasourceId,
    setModels,
    setModelId,
    setModel,
    setMessage,
    t,
  } = deps
  if (!model) {
    return
  }
  const name = model.label ?? model.name
  const ok = await confirm({
    title: t('modeling.confirm_delete_model_title'),
    message: t('modeling.confirm_delete_model_body', { name }),
    variant: 'danger',
    confirmLabel: t('common.delete'),
  })
  if (!ok) {
    return
  }
  setMessage(null)
  await deleteData(`/api/semantic/models/${model.id}`)
  const list = await get<SemanticModelSummary[]>(
    `/api/semantic/models?datasource_id=${encodeURIComponent(datasourceId)}`,
  )
  const next = list ?? []
  setModels(next)
  const nextId = next[0]?.id ?? ''
  setModelId(nextId)
  if (!nextId) {
    setModel(null)
  }
  setMessage(t('modeling.model_deleted'))
}

export async function runRequestTableRemoval(
  deps: {
    model: SemanticModelDetail | null
    confirm: ConfirmFn
    postData: <T = unknown>(url: string, body: unknown) => Promise<T | null>
    refreshModels: (id: string) => Promise<void>
    loadSuggestedJoins: () => Promise<void>
    setMessage: (m: string | null) => void
    setBaseSwapOpen: (v: boolean) => void
    toggleTableVisibility: (schema: string, table: string, visible: boolean) => void
    t: TFn
  },
  schema: string,
  table: string,
) {
  const {
    model,
    confirm,
    postData,
    refreshModels,
    loadSuggestedJoins,
    setMessage,
    setBaseSwapOpen,
    toggleTableVisibility,
    t,
  } = deps
  if (!model) {
    return
  }
  const isBase = schema === model.base_schema && table === model.base_table
  if (isBase) {
    setBaseSwapOpen(true)
    return
  }
  const impact = tableImpact(model, schema, table)
  if (impact.joins === 0 && impact.dims === 0 && impact.metrics === 0) {
    toggleTableVisibility(schema, table, false)
    return
  }
  const ok = await confirm({
    title: t('modeling.remove_table_title'),
    message: t('modeling.remove_table_body', {
      table,
      joins: impact.joins,
      dims: impact.dims,
      metrics: impact.metrics,
    }),
    variant: 'warning',
    confirmLabel: t('modeling.remove_table_action'),
  })
  if (!ok) {
    return
  }
  setMessage(null)
  await postData(`/api/semantic/models/${model.id}/tables/remove`, { schema, table })
  await refreshModels(model.id)
  await loadSuggestedJoins()
  setMessage(t('modeling.table_removed'))
}

export async function runRequestSchemaToggle(
  deps: {
    model: SemanticModelDetail | null
    confirm: ConfirmFn
    postData: <T = unknown>(url: string, body: unknown) => Promise<T | null>
    refreshModels: (id: string) => Promise<void>
    loadSuggestedJoins: () => Promise<void>
    setMessage: (m: string | null) => void
    toggleSchemaExcluded: (schemaName: string) => Promise<void>
    t: TFn
  },
  schemaName: string,
  isExcluded: boolean,
) {
  const {
    model,
    confirm,
    postData,
    refreshModels,
    loadSuggestedJoins,
    setMessage,
    toggleSchemaExcluded,
    t,
  } = deps
  if (isExcluded) {
    await toggleSchemaExcluded(schemaName)
    return
  }
  if (!model) {
    return
  }
  const impact = schemaImpact(model, schemaName)
  const ok = await confirm({
    title: t('modeling.exclude_schema_title'),
    message: t('modeling.exclude_schema_body', {
      schema: schemaName,
      joins: impact.joins,
      dims: impact.dims,
      metrics: impact.metrics,
    }),
    variant: 'warning',
    confirmLabel: t('modeling.exclude_schema_action'),
  })
  if (!ok) {
    return
  }
  setMessage(null)
  await postData(`/api/semantic/models/${model.id}/schemas/remove`, { schema: schemaName })
  await refreshModels(model.id)
  await loadSuggestedJoins()
  setMessage(t('modeling.schema_excluded'))
}

export async function runSaveJoin(deps: {
  model: SemanticModelDetail | null
  joinForm: JoinForm
  columns: Parameters<typeof canSaveJoinForm>[2]
  postData: <T = unknown>(url: string, body: unknown) => Promise<T | null>
  refreshModels: (id: string) => Promise<void>
  loadSuggestedJoins: () => Promise<void>
  setSavingJoin: (v: boolean) => void
  setMessage: (m: string | null) => void
  t: TFn
}) {
  const {
    model,
    joinForm,
    columns,
    postData,
    refreshModels,
    loadSuggestedJoins,
    setSavingJoin,
    setMessage,
    t,
  } = deps
  if (!model || !canSaveJoinForm(model, joinForm, columns)) {
    return
  }
  setSavingJoin(true)
  setMessage(null)
  try {
    await postData<SemanticJoin>(
      `/api/semantic/models/${model.id}/joins`,
      buildJoinPayload(joinForm),
    )
    await refreshModels(model.id)
    await loadSuggestedJoins()
    setMessage(t('modeling.relationship_added'))
  } finally {
    setSavingJoin(false)
  }
}

export async function runAddSuggestedJoin(
  deps: {
    model: SemanticModelDetail | null
    postData: <T = unknown>(url: string, body: unknown) => Promise<T | null>
    refreshModels: (id: string) => Promise<void>
    loadSuggestedJoins: () => Promise<void>
    setMessage: (m: string | null) => void
    t: TFn
  },
  suggestion: SuggestedJoin,
) {
  const { model, postData, refreshModels, loadSuggestedJoins, setMessage, t } = deps
  if (!model) {
    return
  }
  setMessage(null)
  try {
    await postData<SemanticJoin>(
      `/api/semantic/models/${model.id}/joins`,
      suggestedJoinToPayload(suggestion),
    )
    await refreshModels(model.id)
    await loadSuggestedJoins()
    setMessage(t('modeling.fk_relationship_added'))
  } catch {
    setMessage(t('modeling.relationship_add_failed'))
  }
}
