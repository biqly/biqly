import { type Dispatch, type SetStateAction, useState } from 'react'

import type { useConfirm } from '../../hooks/useConfirm'
import type { TranslationKey } from '../../i18n'
import type {
  SemanticDimension,
  SemanticJoin,
  SemanticMetric,
  SemanticModelDetail,
  TableRow,
} from '../../types/semantic'
import {
  reactivateDimensionPayload,
  reactivateJoinPayload,
  reactivateMetricPayload,
  renameDimensionPayload,
  renameMetricPayload,
} from './entityActions'
import type { RenameTarget } from './types'

type Translate = (key: TranslationKey, vars?: Record<string, string | number>) => string
type DeleteData = <T = unknown>(url: string) => Promise<T | null>
type PutData = <T = unknown>(url: string, body: unknown) => Promise<T | null>
type PatchData = <T = unknown>(url: string, body: unknown) => Promise<T | null>

interface UseEntityActionsOptions {
  model: SemanticModelDetail | null
  joins: SemanticJoin[]
  dims: SemanticDimension[]
  metrics: SemanticMetric[]
  confirm: ReturnType<typeof useConfirm>
  deleteData: DeleteData
  putData: PutData
  patchData: PatchData
  refreshModels: (selectedId?: string) => Promise<void>
  loadSuggestedJoins: () => Promise<void>
  setTables: Dispatch<SetStateAction<TableRow[]>>
  setMessage: Dispatch<SetStateAction<string | null>>
  t: Translate
}

export function useEntityActions({
  model,
  joins,
  dims,
  metrics,
  confirm,
  deleteData,
  putData,
  patchData,
  refreshModels,
  loadSuggestedJoins,
  setTables,
  setMessage,
  t,
}: UseEntityActionsOptions) {
  const [renameTarget, setRenameTarget] = useState<RenameTarget | null>(null)
  const [renameValue, setRenameValue] = useState('')
  const [savingRename, setSavingRename] = useState(false)

  const openRename = (target: RenameTarget) => {
    setRenameTarget(target)
    setRenameValue(target.current)
    setMessage(null)
  }

  const closeRename = () => {
    if (savingRename) {
      return
    }
    setRenameTarget(null)
    setRenameValue('')
  }

  const renameModel = () => {
    if (!model) {
      return
    }
    openRename({
      kind: 'model',
      current: model.label || model.name,
      title: t('modeling.rename_model_title'),
      subtitle: model.name,
    })
  }

  const renameTable = (table: TableRow) => {
    openRename({
      kind: 'table',
      current: table.label || table.table_name,
      table,
      title: t('modeling.rename_table_title'),
      subtitle: `${table.schema_name}.${table.table_name}`,
    })
  }

  const renameDimension = (dimension: SemanticDimension) => {
    if (!model) {
      return
    }
    openRename({
      kind: 'dimension',
      current: dimension.label || dimension.name,
      dimension,
      title: t('modeling.rename_dimension_title'),
      subtitle: dimension.column_ref,
    })
  }

  const renameMetric = (metric: SemanticMetric) => {
    if (!model) {
      return
    }
    openRename({
      kind: 'metric',
      current: metric.label || metric.name,
      metric,
      title: t('modeling.rename_metric_title'),
      subtitle: `${metric.aggregation}(${metric.expression})`,
    })
  }

  const deleteJoin = async (joinId: string) => {
    if (!model) {
      return
    }
    const join = joins.find((item) => item.id === joinId)
    const ok = await confirm({
      title: t('modeling.delete_join_title'),
      message: join
        ? t('modeling.delete_join_body_named', { name: join.name })
        : t('modeling.delete_join_body_generic'),
      variant: 'danger',
      confirmLabel: t('common.delete'),
    })
    if (!ok) {
      return
    }
    setMessage(null)
    await deleteData(`/api/semantic/models/${model.id}/joins/${joinId}`)
    await refreshModels(model.id)
    await loadSuggestedJoins()
    setMessage(t('modeling.relationship_deleted'))
  }

  const deleteDimension = async (dimensionId: string) => {
    if (!model) {
      return
    }
    const dimension = dims.find((item) => item.id === dimensionId)
    const ok = await confirm({
      title: t('modeling.confirm_delete_dimension_title'),
      message: dimension
        ? t('modeling.confirm_delete_dimension_body_named', {
            name: dimension.label || dimension.name,
          })
        : t('modeling.confirm_delete_dimension_body_generic'),
      variant: 'danger',
      confirmLabel: t('common.delete'),
    })
    if (!ok) {
      return
    }
    setMessage(null)
    await deleteData(`/api/semantic/models/${model.id}/dimensions/${dimensionId}`)
    await refreshModels(model.id)
    setMessage(t('modeling.dimension_deleted'))
  }

  const deleteMetric = async (metricId: string) => {
    if (!model) {
      return
    }
    const metric = metrics.find((item) => item.id === metricId)
    const ok = await confirm({
      title: t('modeling.confirm_delete_metric_title'),
      message: metric
        ? t('modeling.confirm_delete_metric_body_named', { name: metric.label || metric.name })
        : t('modeling.confirm_delete_metric_body_generic'),
      variant: 'danger',
      confirmLabel: t('common.delete'),
    })
    if (!ok) {
      return
    }
    setMessage(null)
    await deleteData(`/api/semantic/models/${model.id}/metrics/${metricId}`)
    await refreshModels(model.id)
    setMessage(t('modeling.metric_deleted'))
  }

  const reactivateJoin = async (join: SemanticJoin) => {
    if (!model) {
      return
    }
    await putData(`/api/semantic/models/${model.id}/joins/${join.id}`, reactivateJoinPayload(join))
    await refreshModels(model.id)
    setMessage(t('modeling.reactivate_relationship'))
  }

  const reactivateDimension = async (dimension: SemanticDimension) => {
    if (!model) {
      return
    }
    await putData(
      `/api/semantic/models/${model.id}/dimensions/${dimension.id}`,
      reactivateDimensionPayload(dimension),
    )
    await refreshModels(model.id)
    setMessage(t('modeling.reactivate_dimension'))
  }

  const reactivateMetric = async (metric: SemanticMetric) => {
    if (!model) {
      return
    }
    await putData(
      `/api/semantic/models/${model.id}/metrics/${metric.id}`,
      reactivateMetricPayload(metric),
    )
    await refreshModels(model.id)
    setMessage(t('modeling.reactivate_metric'))
  }

  const submitRename = async () => {
    if (!renameTarget || savingRename) {
      return
    }
    const trimmed = renameValue.trim()
    if (!trimmed || trimmed === renameTarget.current) {
      closeRename()
      return
    }
    setSavingRename(true)
    try {
      if (renameTarget.kind === 'table') {
        const updated = await patchData<TableRow>(`/api/metadata/tables/${renameTarget.table.id}`, {
          label: trimmed,
        })
        if (updated) {
          setTables((current) =>
            current.map((table) => (table.id === renameTarget.table.id ? updated : table)),
          )
          setMessage(t('modeling.table_label_updated'))
        }
      } else if (renameTarget.kind === 'model' && model) {
        await putData(`/api/semantic/models/${model.id}`, {
          label: trimmed,
          base_schema: model.base_schema,
          base_table: model.base_table,
        })
        await refreshModels(model.id)
        setMessage(t('modeling.model_renamed'))
      } else if (renameTarget.kind === 'dimension' && model) {
        const dimension = renameTarget.dimension
        await putData(
          `/api/semantic/models/${model.id}/dimensions/${dimension.id}`,
          renameDimensionPayload(dimension, trimmed),
        )
        await refreshModels(model.id)
        setMessage(t('modeling.dimension_label_updated'))
      } else if (renameTarget.kind === 'metric' && model) {
        const metric = renameTarget.metric
        await putData(
          `/api/semantic/models/${model.id}/metrics/${metric.id}`,
          renameMetricPayload(metric, trimmed),
        )
        await refreshModels(model.id)
        setMessage(t('modeling.metric_label_updated'))
      }
      setRenameTarget(null)
      setRenameValue('')
    } finally {
      setSavingRename(false)
    }
  }

  return {
    renameTarget,
    renameValue,
    savingRename,
    setRenameValue,
    closeRename,
    renameModel,
    renameTable,
    renameDimension,
    renameMetric,
    deleteJoin,
    deleteDimension,
    deleteMetric,
    reactivateJoin,
    reactivateDimension,
    reactivateMetric,
    submitRename,
  }
}
