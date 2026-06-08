import type { useT } from '../../i18n'
import type { Datasource } from '../../types/metadata'
import type { SemanticModelDetail } from '../../types/semantic'
import { ShareButton } from '../sharing/ShareButton'
import { Select } from '../ui/Select'

export function ModelingToolbar({
  t,
  datasourceId,
  datasources,
  onDatasourceChange,
  modelId,
  models,
  onModelChange,
  model,
  creatingModel,
  publishing,
  onCreateModel,
  onRenameModel,
  onPublishModel,
  onRemoveModel,
}: {
  t: ReturnType<typeof useT>
  datasourceId: string
  datasources: Datasource[]
  onDatasourceChange: (id: string) => void
  modelId: string
  models: { id: string; label?: string | null; name: string; status: string }[]
  onModelChange: (id: string) => void
  model: SemanticModelDetail | null
  creatingModel: boolean
  publishing: boolean
  onCreateModel: () => void
  onRenameModel: () => void
  onPublishModel: () => void
  onRemoveModel: () => void
}) {
  return (
    <section className="modeling-toolbar">
      <div className="form-group">
        <label htmlFor="modeling-datasource">{t('modeling.datasource_label')}</label>
        <Select
          id="modeling-datasource"
          name="datasource"
          value={datasourceId}
          onChange={onDatasourceChange}
          placeholder={t('modeling.datasource_placeholder')}
          header={t('modeling.datasource_header')}
          options={datasources.map((d) => ({ value: d.id, label: d.name, hint: d.type }))}
        />
      </div>
      <div className="form-group">
        <label htmlFor="modeling-model">{t('modeling.model_label')}</label>
        <Select
          id="modeling-model"
          name="model"
          value={modelId}
          onChange={onModelChange}
          placeholder={
            models.length === 0 ? t('modeling.no_models') : t('modeling.model_placeholder')
          }
          header={t('modeling.model_header')}
          options={models.map((m) => ({ value: m.id, label: m.label ?? m.name, hint: m.status }))}
        />
      </div>
      <div className="modeling-toolbar-actions">
        <button
          className="btn btn-primary"
          type="button"
          onClick={onCreateModel}
          disabled={!datasourceId || creatingModel}
        >
          {creatingModel ? t('modeling.creating') : t('modeling.create_from_metadata')}
        </button>
        {model && (
          <button
            className="btn btn-secondary"
            type="button"
            onClick={onRenameModel}
            title={t('modeling.rename_model_button_title')}
          >
            {t('modeling.rename_model_button')}
          </button>
        )}
        {model && (
          <button
            className="btn btn-secondary"
            type="button"
            onClick={onPublishModel}
            disabled={publishing || model.status === 'published'}
          >
            {publishing
              ? t('modeling.publishing')
              : model.status === 'published'
                ? t('modeling.published')
                : t('modeling.publish')}
          </button>
        )}
        {model && (
          <button
            className="btn btn-danger-outline"
            type="button"
            onClick={onRemoveModel}
            title={t('modeling.delete_model_title')}
          >
            {t('common.delete')}
          </button>
        )}
        {model && <ShareButton resourceType="model" resourceID={model.id} />}
      </div>
    </section>
  )
}
