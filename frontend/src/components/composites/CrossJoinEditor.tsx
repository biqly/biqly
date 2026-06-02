import { useState } from 'react'
import { useT } from '../../i18n'
import { Select } from '../ui/Select'
import type { ComponentModelRef, CrossModelJoin } from '../../types/composite'

interface CrossJoinEditorProps {
  components: ComponentModelRef[]
  dimensionsByAlias: Record<string, string[]>
  onSubmit: (join: CrossModelJoin) => void
  onCancel: () => void
  initial?: CrossModelJoin
}

const JOIN_TYPES: CrossModelJoin['join_type'][] = ['LEFT', 'INNER', 'RIGHT']
const RELATIONSHIPS: CrossModelJoin['relationship'][] = [
  'many_to_one',
  'one_to_many',
  'one_to_one',
  'many_to_many',
]

// CrossJoinEditor edits a single cross-model join between two component aliases,
// picking the joined dimension from each side and the cardinality.
export function CrossJoinEditor({
  components,
  dimensionsByAlias,
  onSubmit,
  onCancel,
  initial,
}: CrossJoinEditorProps) {
  const t = useT()
  const [fromModel, setFromModel] = useState(initial?.from_model ?? components[0]?.alias ?? '')
  const [toModel, setToModel] = useState(initial?.to_model ?? components[1]?.alias ?? '')
  const [fromDimension, setFromDimension] = useState(initial?.from_dimension ?? '')
  const [toDimension, setToDimension] = useState(initial?.to_dimension ?? '')
  const [joinType, setJoinType] = useState<CrossModelJoin['join_type']>(initial?.join_type ?? 'LEFT')
  const [relationship, setRelationship] = useState<CrossModelJoin['relationship']>(
    initial?.relationship ?? 'many_to_one',
  )
  const [error, setError] = useState<string | null>(null)

  const aliasOptions = components.map((c) => ({ value: c.alias, label: c.alias }))
  const fromDims = dimensionsByAlias[fromModel] ?? []
  const toDims = dimensionsByAlias[toModel] ?? []

  const handleSubmit = () => {
    if (!fromModel || !toModel || fromModel === toModel) {
      setError(t('composites.editor_error_distinct'))
      return
    }
    if (!fromDimension || !toDimension) {
      setError(t('composites.editor_error_both_dims'))
      return
    }
    onSubmit({
      ...initial,
      from_model: fromModel,
      to_model: toModel,
      from_dimension: fromDimension,
      to_dimension: toDimension,
      join_type: joinType,
      relationship,
      is_active: true,
    })
  }

  return (
    <div className="cross-join-editor">
      {error && <div className="form-error">{error}</div>}
      <div className="cross-join-grid">
        <label>
          {t('composites.editor_from_model')}
          <Select value={fromModel} onChange={(v) => setFromModel(v)} options={aliasOptions} />
        </label>
        <label>
          {t('composites.editor_from_dimension')}
          <Select
            value={fromDimension}
            onChange={(v) => setFromDimension(v)}
            options={fromDims.map((d) => ({ value: d, label: d }))}
            placeholder={t('composites.editor_select_placeholder')}
          />
        </label>
        <label>
          {t('composites.editor_to_model')}
          <Select value={toModel} onChange={(v) => setToModel(v)} options={aliasOptions} />
        </label>
        <label>
          {t('composites.editor_to_dimension')}
          <Select
            value={toDimension}
            onChange={(v) => setToDimension(v)}
            options={toDims.map((d) => ({ value: d, label: d }))}
            placeholder={t('composites.editor_select_placeholder')}
          />
        </label>
        <label>
          {t('composites.editor_join_type')}
          <Select
            value={joinType}
            onChange={(v) => setJoinType(v as CrossModelJoin['join_type'])}
            options={JOIN_TYPES.map((j) => ({ value: j, label: j }))}
          />
        </label>
        <label>
          {t('composites.editor_relationship')}
          <Select
            value={relationship}
            onChange={(v) => setRelationship(v as CrossModelJoin['relationship'])}
            options={RELATIONSHIPS.map((r) => ({ value: r, label: r }))}
          />
        </label>
      </div>
      <div className="cross-join-actions">
        <button type="button" className="btn-secondary" onClick={onCancel}>
          {t('composites.editor_cancel')}
        </button>
        <button type="button" className="btn-primary" onClick={handleSubmit}>
          {t('composites.editor_save')}
        </button>
      </div>
    </div>
  )
}
