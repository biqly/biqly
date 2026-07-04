import { useState } from 'react'

import { cn } from '../lib/cn'
import { semanticModelSetupStatusClass } from '../lib/feedbackClasses'
import {
  modelingMobileFabClass,
  modelingMobileScrimClass,
  modelingMobileScrimVisibleClass,
  modelingPageClass,
  modelingShellClass,
} from '../lib/modelingClasses'
import { DriftPanel } from './admin/DriftPanel'
import { JoinEditor } from './modeling/JoinEditor'
import { ModelingCanvas } from './modeling/ModelingCanvas'
import { ModelingModals } from './modeling/ModelingModals'
import { ModelingPalette } from './modeling/ModelingPalette'
import { ModelingToolbar } from './modeling/ModelingToolbar'
import { ModelVersionsModal } from './modeling/ModelVersionsModal'
import { useModelingPageState } from './modeling/useModelingPageState'
import { ErrorAlert } from './ui/ErrorAlert'
import { LoadingScreen } from './ui/LoadingScreen'
import { LockedState } from './ui/LockedState'

export default function Modeling() {
  const s = useModelingPageState()
  const [versionsOpen, setVersionsOpen] = useState(false)

  if (s.pageLoading) {
    return <LoadingScreen minHeight="300px" />
  }

  return (
    <div className={modelingPageClass}>
      <ModelingToolbar
        t={s.t}
        datasourceId={s.datasourceId}
        datasources={s.datasources}
        onDatasourceChange={s.setDatasourceId}
        modelId={s.modelId}
        models={s.models}
        onModelChange={s.setModelId}
        model={s.model}
        creatingModel={s.creatingModel}
        publishing={s.publishing}
        onCreateModel={() => void s.createModel()}
        onRenameModel={s.renameModel}
        onPublishModel={() => void s.publishModel()}
        onRemoveModel={() => void s.removeModel()}
        onExportModel={() => void s.exportModel()}
        onImportModel={(file) => void s.importModel(file)}
        importing={s.importing}
        onOpenVersions={() => setVersionsOpen(true)}
      />

      {s.isLocked ? (
        <LockedState
          datasourceId={s.datasourceId}
          datasourceName={s.datasources.find((d) => d.id === s.datasourceId)?.name ?? s.dsParam}
        />
      ) : (
        <>
          {s.error && <ErrorAlert error={s.error} />}
          {s.message && <div className={semanticModelSetupStatusClass('success')}>{s.message}</div>}

          {s.model && <DriftPanel modelId={s.model.id} />}

          <section
            className={modelingShellClass({
              paletteOpen: s.paletteOpen,
              editorOpen: s.editorOpen,
            })}
          >
            <div
              className={cn(
                modelingMobileScrimClass,
                modelingMobileScrimVisibleClass(s.paletteOpen || s.editorOpen),
              )}
              hidden={!(s.paletteOpen || s.editorOpen)}
              onClick={s.closeMobilePanels}
              aria-hidden="true"
            />

            <button
              type="button"
              className={modelingMobileFabClass('left', s.paletteOpen)}
              aria-label={s.t('modeling.open_semantic_panel')}
              onClick={s.togglePalette}
            >
              <span aria-hidden="true">☰</span>
              <span>{s.t('modeling.tab_short_tables')}</span>
            </button>
            <button
              type="button"
              className={modelingMobileFabClass('right', s.editorOpen)}
              aria-label={s.t('modeling.open_join_panel')}
              onClick={s.toggleEditor}
            >
              <span aria-hidden="true">⇄</span>
              <span>{s.t('modeling.tab_short_rel')}</span>
            </button>

            <ModelingPalette
              open={s.paletteOpen}
              onToggle={s.togglePalette}
              model={s.model}
              usedTableCount={s.usedTableCount}
              joins={s.joins}
              inactiveJoins={s.inactiveJoins}
              dims={s.dims}
              inactiveDims={s.inactiveDims}
              metrics={s.metrics}
              inactiveMetrics={s.inactiveMetrics}
              activeTab={s.activeTab}
              onTabChange={s.setActiveTab}
              tables={s.tables}
              includedTables={s.includedTables}
              excludedSchemas={s.excludedSchemas}
              tableCards={s.tableCards}
              tableImpact={s.getTableImpact}
              suggestedJoins={s.suggestedJoins}
              highlightJoinId={s.highlightJoinId}
              onHighlightJoin={s.handleJoinClick}
              onSchemaToggle={(schemaName, isExcluded) => {
                void s.requestSchemaToggle(schemaName, isExcluded)
              }}
              onRenameTable={s.renameTable}
              onMakeBase={(schema, table) => {
                void s.requestMakeBase(schema, table)
              }}
              onRemoveTable={(schema, table) => {
                void s.requestTableRemoval(schema, table)
              }}
              onToggleTableVisibility={s.toggleTableVisibility}
              onOpenBaseSwap={() => s.setBaseSwapOpen(true)}
              onDeleteJoin={(joinId) => {
                void s.deleteJoin(joinId)
              }}
              onAddSuggestedJoin={(join) => {
                void s.addSuggestedJoin(join)
              }}
              onReactivateJoin={(join) => {
                void s.reactivateJoin(join)
              }}
              onEditDimension={s.setEditingDimension}
              onEditDimensionValues={s.setEnumDimension}
              onDeleteDimension={(dimensionId) => {
                void s.deleteDimension(dimensionId)
              }}
              onReactivateDimension={(dimension) => {
                void s.reactivateDimension(dimension)
              }}
              onSyncDimensions={() => {
                void s.syncDimensions()
              }}
              onOpenAddMetric={() => s.setAddMetricOpen(true)}
              onEditMetric={s.setEditingMetric}
              onDeleteMetric={(metricId) => {
                void s.deleteMetric(metricId)
              }}
              onReactivateMetric={(metric) => {
                void s.reactivateMetric(metric)
              }}
              t={s.t}
            />

            <ModelingCanvas
              canvas={s.canvas}
              tableCards={s.tableCards}
              joins={s.joins}
              baseKey={s.baseKey}
              highlightJoinId={s.highlightJoinId}
              highlightedTables={s.highlightedTables}
              highlightedColumns={s.highlightedColumns}
              highlightedJoinColumns={s.highlightedJoinColumns}
              t={s.t}
            />
            <JoinEditor
              open={s.editorOpen}
              onToggle={s.toggleEditor}
              joinForm={s.joinForm}
              onChange={s.updateJoinForm}
              tableOptions={s.tableOptions}
              fromColumns={s.fromColumns}
              toColumns={s.toColumns}
              fromColumnOptions={s.fromColumnOptions}
              toColumnOptions={s.toColumnOptions}
              fromColumnValue={s.fromColumnValue}
              toColumnValue={s.toColumnValue}
              selectedFromColumn={s.selectedFromColumn}
              canSave={s.canSaveJoin}
              saving={s.savingJoin}
              loading={s.loading}
              onSave={() => {
                void s.saveJoin()
              }}
              t={s.t}
            />
          </section>
        </>
      )}
      {s.model && (
        <ModelVersionsModal
          open={versionsOpen}
          modelId={s.model.id}
          modelName={s.model.name}
          onClose={() => setVersionsOpen(false)}
        />
      )}
      {s.model && (
        <ModelingModals
          t={s.t}
          model={s.model}
          includedTables={s.includedTables}
          columns={s.columns}
          renameTarget={s.renameTarget}
          renameValue={s.renameValue}
          savingRename={s.savingRename}
          onRenameValueChange={s.setRenameValue}
          onCloseRename={s.closeRename}
          onSubmitRename={() => {
            void s.submitRename()
          }}
          baseSwapOpen={s.baseSwapOpen}
          baseSwapCandidates={s.baseSwapCandidates}
          savingBaseSwap={s.savingBaseSwap}
          onCloseBaseSwap={() => s.setBaseSwapOpen(false)}
          onBaseSwapSubmit={s.swapBaseAndRemoveOld}
          addMetricOpen={s.addMetricOpen}
          editingMetric={s.editingMetric}
          onCloseMetricModal={() => {
            s.setAddMetricOpen(false)
            s.setEditingMetric(null)
          }}
          onMetricCreated={async () => {
            const isEdit = !!s.editingMetric
            s.setAddMetricOpen(false)
            s.setEditingMetric(null)
            await s.refreshModels(s.model!.id)
            s.setMessage(isEdit ? s.t('modeling.metric_updated') : s.t('modeling.metric_added'))
          }}
          postData={s.postData}
          putData={s.putData}
          editingDimension={s.editingDimension}
          onCloseEditDimension={() => s.setEditingDimension(null)}
          onDimensionSaved={async () => {
            s.setEditingDimension(null)
            await s.refreshModels(s.model!.id)
            s.setMessage(s.t('modeling.dimension_updated'))
          }}
          enumDimension={s.enumDimension}
          onCloseEnumDimension={() => s.setEnumDimension(null)}
          onEnumSaved={async () => {
            s.setEnumDimension(null)
            await s.refreshModels(s.model!.id)
            s.setMessage(s.t('modeling.dimension_label_updated'))
          }}
        />
      )}
    </div>
  )
}
