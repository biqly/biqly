import { useRef, useState } from 'react'

import type { useT } from '../../i18n'
import { buttonClass } from '../../lib/buttonClasses'
import { cn } from '../../lib/cn'
import { formLabelClass, modelingFormGroupClass } from '../../lib/formClasses'
import {
  modelingMenuModelNameClass,
  modelingStatusPillClass,
  modelingToolbarActionsClass,
  modelingToolbarClass,
  modelingToolbarGroupActionsClass,
  modelingToolbarGroupDatasourceClass,
  modelingToolbarGroupModelClass,
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
  onExportModel,
  onDescribeJoins,
  describingJoins,
  onImportModel,
  importing,
  onOpenVersions,
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
  onExportModel: () => void
  onDescribeJoins: () => void
  describingJoins: boolean
  onImportModel: (file: File) => void
  importing: boolean
  onOpenVersions: () => void
}) {
  const [shareOpen, setShareOpen] = useState(false)
  const importInputRef = useRef<HTMLInputElement>(null)
  const isPublished = model?.status === 'published'

  return (
    <section className={modelingToolbarClass}>
      <div className={cn(modelingFormGroupClass, modelingToolbarGroupDatasourceClass)}>
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
      <div className={cn(modelingFormGroupClass, modelingToolbarGroupModelClass)}>
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
      <div className={cn(modelingFormGroupClass, modelingToolbarGroupActionsClass)}>
        <label className={formLabelClass}>{t('modeling.actions_label')}</label>
        <div className={modelingToolbarActionsClass}>
          <button
            className={buttonClass('primary', {
              className: 'mt-0! inline-flex w-auto! items-center gap-1.5',
            })}
            type="button"
            onClick={onCreateModel}
            disabled={!datasourceId || creatingModel}
          >
            <span aria-hidden="true">✨</span>
            {creatingModel ? t('modeling.creating') : t('modeling.create_from_metadata')}
          </button>
          <input
            ref={importInputRef}
            type="file"
            accept=".yaml,.yml,.json"
            className="hidden"
            aria-label={t('modeling.import_model')}
            onChange={(e) => {
              const file = e.target.files?.[0]
              if (file) {
                onImportModel(file)
              }
              e.target.value = ''
            }}
          />
          <button
            className={buttonClass('secondary', {
              className: 'mt-0! inline-flex w-auto! items-center gap-1.5',
            })}
            type="button"
            onClick={() => importInputRef.current?.click()}
            disabled={!datasourceId || importing}
          >
            <span aria-hidden="true">📥</span>
            {importing ? t('modeling.importing') : t('modeling.import_model')}
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
                  key: 'export',
                  icon: '📤',
                  label: t('modeling.export_model'),
                  onSelect: onExportModel,
                },
                {
                  key: 'describe-joins',
                  icon: '✨',
                  label: describingJoins
                    ? t('modeling.describing_joins')
                    : t('modeling.describe_joins_btn'),
                  disabled: describingJoins,
                  onSelect: onDescribeJoins,
                },
                {
                  key: 'versions',
                  icon: '🕘',
                  label: t('modeling.versions_title'),
                  onSelect: onOpenVersions,
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
      </div>
    </section>
  )
}
