import type { GenerateSemanticModelResponse } from '../../types/semantic'
import type { MetadataTFunction } from '../metadata/utils'

export function QueryBuilderDraftModelWarning({
  show,
  t,
}: {
  show: boolean
  t: MetadataTFunction
}) {
  if (!show) {
    return null
  }
  return (
    <p
      className="hint-text"
      style={{ marginBottom: '1rem', fontSize: '0.82rem', color: 'var(--text-muted)' }}
    >
      {t('query_builder.draft_model_warning')}
    </p>
  )
}

export function QueryBuilderEmptyModelSetup({
  show,
  generatingModel,
  onCreate,
  t,
}: {
  show: boolean
  generatingModel: boolean
  onCreate: () => void
  t: MetadataTFunction
}) {
  if (!show) {
    return null
  }
  return (
    <div className="semantic-model-setup" style={{ marginBottom: '1rem' }}>
      <div>
        <strong>{t('query_builder.model_setup_title')}</strong>
        <p>{t('query_builder.model_setup_body')}</p>
      </div>
      <button type="button" className="btn btn-sm" onClick={onCreate} disabled={generatingModel}>
        {generatingModel
          ? t('query_builder.model_setup_generating')
          : t('query_builder.model_setup_create')}
      </button>
    </div>
  )
}

export function QueryBuilderGeneratedModelBanner({
  generatedModel,
  t,
}: {
  generatedModel: GenerateSemanticModelResponse | null
  t: MetadataTFunction
}) {
  if (!generatedModel) {
    return null
  }
  const isError = generatedModel.validation?.valid === false
  return (
    <div
      className={
        isError
          ? 'semantic-model-setup semantic-model-setup--error'
          : 'semantic-model-setup semantic-model-setup--success'
      }
      style={{ marginBottom: '1rem' }}
    >
      <div>
        <strong>
          {generatedModel.published
            ? t('query_builder.model_setup_created_published')
            : t('query_builder.model_setup_created_draft')}
        </strong>
        <p>
          {t('query_builder.model_setup_summary', {
            dimensions: generatedModel.model.dimensions?.length ?? 0,
            metrics: generatedModel.model.metrics?.length ?? 0,
            joins: generatedModel.model.joins?.length ?? 0,
          })}
        </p>
        {generatedModel.validation?.errors?.length ? (
          <ul>
            {generatedModel.validation.errors.map((msg) => (
              <li key={msg}>{msg}</li>
            ))}
          </ul>
        ) : null}
      </div>
    </div>
  )
}
