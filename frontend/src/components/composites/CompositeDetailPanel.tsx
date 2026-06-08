import type { useT } from '../../i18n'
import type {
  CompositeModelDetail,
  CompositeValidationResult,
  CrossModelJoin,
  DimensionConflictResolution,
  SuggestedCrossJoin,
} from '../../types/composite'
import { Select } from '../ui/Select'
import { CompositeCanvas } from './CompositeCanvas'

export function CompositeDetailPanel({
  t,
  detail,
  loading,
  validation,
  modelNames,
  dimensionsByAlias,
  suggestions,
  addModelId,
  addAlias,
  addRole,
  availableModels,
  usedAliases,
  onValidate,
  onPublish,
  onRollback,
  onAddModelIdChange,
  onAddAliasChange,
  onAddRoleChange,
  onAddComponent,
  onRemoveComponent,
  onLoadSuggestions,
  onAddJoin,
  onEditJoin,
  onRemoveCrossJoin,
  onApplySuggestion,
  onSetCanonicalDate,
  onResolutionChange,
}: {
  t: ReturnType<typeof useT>
  detail: CompositeModelDetail
  loading: boolean
  validation: CompositeValidationResult | null
  modelNames: Record<string, string>
  dimensionsByAlias: Record<string, string[]>
  suggestions: SuggestedCrossJoin[]
  addModelId: string
  addAlias: string
  addRole: 'primary' | 'secondary'
  availableModels: { id: string; label?: string | null; name: string }[]
  usedAliases: Set<string>
  onValidate: () => void
  onPublish: () => void
  onRollback: () => void
  onAddModelIdChange: (id: string) => void
  onAddAliasChange: (alias: string) => void
  onAddRoleChange: (role: 'primary' | 'secondary') => void
  onAddComponent: () => void
  onRemoveComponent: (modelId: string) => void
  onLoadSuggestions: () => void
  onAddJoin: () => void
  onEditJoin: (join: CrossModelJoin) => void
  onRemoveCrossJoin: (joinId?: string) => void
  onApplySuggestion: (s: SuggestedCrossJoin) => void
  onSetCanonicalDate: (alias: string, dimension: string) => void
  onResolutionChange: (res: DimensionConflictResolution) => void
}) {
  return (
    <>
      <div className="composite-detail-head">
        <div>
          <h2>{detail.label ?? detail.name}</h2>
          {detail.description && <p>{detail.description}</p>}
          <span className={`composite-status status-${detail.status}`}>
            {detail.status} · v{detail.version}
          </span>
        </div>
        <div className="composite-actions">
          <button type="button" className="btn-secondary" onClick={onValidate} disabled={loading}>
            {t('composites.validate')}
          </button>
          <button type="button" className="btn-primary" onClick={onPublish} disabled={loading}>
            {t('composites.publish')}
          </button>
          <button type="button" className="btn-secondary" onClick={onRollback} disabled={loading}>
            {t('composites.rollback')}
          </button>
        </div>
      </div>

      {validation && (
        <div className={`composite-validation ${validation.valid ? 'valid' : 'invalid'}`}>
          <strong>
            {validation.valid
              ? t('composites.validation_success')
              : t('composites.validation_errors')}
          </strong>
          {(validation.errors ?? []).map((e, i) => (
            <div key={`err-${i}`} className="validation-error">
              {e.field ? `${e.field}: ` : ''}
              {e.message}
            </div>
          ))}
          {(validation.warnings ?? []).map((wn, i) => (
            <div key={`warn-${i}`} className="validation-warning">
              {wn.message}
            </div>
          ))}
        </div>
      )}

      <div className="composite-canvas-wrap">
        <CompositeCanvas
          components={detail.components ?? []}
          crossJoins={detail.cross_model_joins ?? []}
          modelNames={modelNames}
        />
      </div>

      <div className="composite-section">
        <h3>{t('composites.components_title')}</h3>
        <ul className="component-list">
          {(detail.components ?? []).map((c) => (
            <li key={c.model_id}>
              <span className="component-alias">{c.alias}</span>
              <span className="component-model">{modelNames[c.model_id] ?? c.model_id}</span>
              <span className={`component-role role-${c.role}`}>{c.role}</span>
              <button
                type="button"
                className="btn-icon-danger"
                onClick={() => onRemoveComponent(c.model_id)}
                aria-label={t('composites.aria_remove')}
              >
                ×
              </button>
            </li>
          ))}
        </ul>
        <div className="component-add-row">
          <Select
            value={addModelId}
            onChange={(v) => {
              onAddModelIdChange(v)
              const m = availableModels.find((x) => x.id === v)
              if (m && !addAlias) {
                onAddAliasChange(m.name)
              }
            }}
            options={availableModels.map((m) => ({
              value: m.id,
              label: m.label ?? m.name,
            }))}
            placeholder={t('composites.model_select')}
          />
          <input
            type="text"
            placeholder={t('composites.alias_placeholder')}
            value={addAlias}
            onChange={(e) => onAddAliasChange(e.target.value)}
          />
          <Select
            value={addRole}
            onChange={onAddRoleChange}
            options={[
              { value: 'primary', label: 'primary' },
              { value: 'secondary', label: 'secondary' },
            ]}
          />
          <button
            type="button"
            className="btn-secondary"
            onClick={onAddComponent}
            disabled={!addModelId || !addAlias.trim() || usedAliases.has(addAlias.trim())}
          >
            {t('composites.add')}
          </button>
        </div>
      </div>

      <div className="composite-section">
        <div className="section-head-row">
          <h3>{t('composites.cross_joins_title')}</h3>
          <div>
            <button type="button" className="btn-secondary" onClick={onLoadSuggestions}>
              {t('composites.suggest')}
            </button>
            <button
              type="button"
              className="btn-secondary"
              onClick={onAddJoin}
              disabled={(detail.components ?? []).length < 2}
            >
              {t('composites.add_join')}
            </button>
          </div>
        </div>
        <ul className="cross-join-list">
          {(detail.cross_model_joins ?? []).map((j) => (
            <li key={j.id}>
              <span>
                {j.from_model}.{j.from_dimension} → {j.to_model}.{j.to_dimension}
              </span>
              <span className="join-meta">
                {j.join_type} · {j.relationship}
              </span>
              <button type="button" className="btn-link" onClick={() => onEditJoin(j)}>
                {t('composites.edit')}
              </button>
              <button
                type="button"
                className="btn-icon-danger"
                onClick={() => onRemoveCrossJoin(j.id)}
                aria-label={t('composites.aria_delete')}
              >
                ×
              </button>
            </li>
          ))}
        </ul>
        {suggestions.length > 0 && (
          <div className="cross-join-suggestions">
            <h4>{t('composites.suggested_joins')}</h4>
            {suggestions.map((s, i) => (
              <div key={`sug-${i}`} className="suggestion-row">
                <span>
                  {s.from_model}.{s.from_dimension} → {s.to_model}.{s.to_dimension}
                </span>
                <span className="suggestion-reason">{s.reason}</span>
                <button type="button" className="btn-link" onClick={() => onApplySuggestion(s)}>
                  {t('composites.apply')}
                </button>
              </div>
            ))}
          </div>
        )}
      </div>

      <div className="composite-section">
        <h3>{t('composites.canonical_date_title')}</h3>
        <p className="section-hint">{t('composites.canonical_date_hint')}</p>
        <div className="canonical-date-grid">
          {(detail.components ?? []).map((c) => (
            <div key={c.alias} className="canonical-date-model">
              <strong>{c.alias}</strong>
              <div className="canonical-date-dims">
                {(dimensionsByAlias[c.alias] ?? []).map((dim) => {
                  const active =
                    detail.canonical_date?.model_alias === c.alias &&
                    detail.canonical_date.dimension_name === dim
                  return (
                    <button
                      key={dim}
                      type="button"
                      className={`date-dim-chip ${active ? 'active' : ''}`}
                      onClick={() => onSetCanonicalDate(c.alias, dim)}
                    >
                      {dim}
                    </button>
                  )
                })}
              </div>
            </div>
          ))}
        </div>
      </div>

      <div className="composite-section">
        <h3>{t('composites.conflicts_title')}</h3>
        <p className="section-hint">{t('composites.conflicts_hint')}</p>
        <ul className="resolution-list">
          {(detail.conflict_resolutions ?? []).map((res) => (
            <li key={res.dimension_name}>
              <span className="resolution-name">{res.dimension_name}</span>
              <Select
                value={res.resolution}
                onChange={(v) =>
                  onResolutionChange({
                    ...res,
                    resolution: v,
                  })
                }
                options={[
                  { value: 'use_primary', label: t('composites.resolution_use_primary') },
                  { value: 'rename', label: t('composites.resolution_rename') },
                  { value: 'merge', label: t('composites.resolution_merge') },
                ]}
              />
            </li>
          ))}
          {(detail.conflict_resolutions ?? []).length === 0 && (
            <li className="resolution-empty">{t('composites.no_conflicts')}</li>
          )}
        </ul>
      </div>
    </>
  )
}
