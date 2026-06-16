import { useState } from 'react'

import type { useT } from '../../i18n'
import { legacyButtonClass } from '../../lib/buttonClasses'
import { modelingFormGroupClass } from '../../lib/formClasses'
import {
  modelingMenuModelNameClass,
  modelingStatusPillClass,
  modelingToolbarActionsClass,
  modelingToolbarClass,
  modelingToolbarModelRowClass,
} from '../../lib/modelingClasses'
import type { Datasource } from '../../types/metadata'
import type { SemanticModelDetail } from '../../types/semantic'
import { ShareButton } from '../sharing/ShareButton'
import { ActionMenu } from '../ui/ActionMenu'
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
  const [shareOpen, setShareOpen] = useState(false)
  const isPublished = model?.status === 'published'

  return (
    <section className={modelingToolbarClass}>
      <div className={modelingFormGroupClass}>
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
      <div className={modelingFormGroupClass}>
        <label htmlFor="modeling-model">{t('modeling.model_label')}</label>
        <div className={modelingToolbarModelRowClass}>
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
          {model && (
            <span className={modelingStatusPillClass(isPublished)}>
              {isPublished ? t('modeling.published') : t('modeling.status_draft')}
            </span>
          )}
        </div>
      </div>
      <div className={modelingToolbarActionsClass}>
        <button
          className={legacyButtonClass('btn btn-primary mt-0! w-auto!')}
          type="button"
          onClick={onCreateModel}
          disabled={!datasourceId || creatingModel}
        >
          <span aria-hidden="true">✨</span>{' '}
          {creatingModel ? t('modeling.creating') : t('modeling.create_from_metadata')}
        </button>
        {model && (
          <ActionMenu
            label={
              <>
                {t('modeling.model_menu')} <span aria-hidden="true">▾</span>
              </>
            }
            header={<strong className={modelingMenuModelNameClass}>{model.name}</strong>}
            items={[
              {
                key: 'rename',
                icon: '✏️',
                label: t('modeling.rename_model_button'),
                onSelect: onRenameModel,
              },
              {
                key: 'publish',
                icon: '🚀',
                label: publishing
                  ? t('modeling.publishing')
                  : isPublished
                    ? t('modeling.published')
                    : t('modeling.publish'),
                disabled: publishing || isPublished,
                onSelect: onPublishModel,
              },
              {
                key: 'share',
                icon: '🔗',
                label: t('admin.sharing.share'),
                onSelect: () => setShareOpen(true),
              },
              {
                key: 'delete',
                icon: '🗑️',
                label: t('common.delete'),
                danger: true,
                onSelect: onRemoveModel,
              },
            ]}
          />
        )}
        {model && (
          <ShareButton
            resourceType="model"
            resourceID={model.id}
            open={shareOpen}
            onOpenChange={setShareOpen}
            showTrigger={false}
          />
        )}
      </div>
    </section>
  )
}
