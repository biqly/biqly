import { useState } from 'react'

import { semanticModelSetupStatusClass } from '../lib/feedbackClasses'
import { modelingPageClass, modelingShellClass } from '../lib/modelingClasses'
import { DriftPanel } from './admin/DriftPanel'
import { JoinEditor } from './modeling/JoinEditor'
import { ModelingCanvas } from './modeling/ModelingCanvas'
import { ModelingModals } from './modeling/ModelingModals'
import { ModelingPalette } from './modeling/ModelingPalette'
import { ModelingToolbar } from './modeling/ModelingToolbar'
import {
  ModelingToolLaunchers,
  ModelingToolsModal,
  type ModelingToolsTab,
} from './modeling/ModelingToolsModal'
import { ModelVersionsModal } from './modeling/ModelVersionsModal'
import { TableDetailModal } from './modeling/TableDetailModal'
import { useModelingPageState } from './modeling/useModelingPageState'
import { ErrorAlert } from './ui/ErrorAlert'
import { LoadingScreen } from './ui/LoadingScreen'
import { LockedState } from './ui/LockedState'

export default function Modeling() {
  const s = useModelingPageState()
  const [versionsOpen, setVersionsOpen] = useState(false)
  const [toolsTab, setToolsTab] = useState<ModelingToolsTab | null>(null)

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
        onDescribeJoins={() => void s.describeJoins()}
        describingJoins={s.describingJoins}
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

          <section className={modelingShellClass}>
            <ModelingToolLaunchers
              tableCount={s.usedTableCount}
              relationshipCount={s.joins.length}
              onOpen={setToolsTab}
              t={s.t}
            />
            <ModelingCanvas
              canvas={s.canvas}
              tableCards={s.tableCards}
              columns={s.columns}
              joins={s.joins}
              baseKey={s.baseKey}
              highlightJoinId={s.highlightJoinId}
              highlightedTables={s.highlightedTables}
              highlightedColumns={s.highlightedColumns}
              highlightedJoinColumns={s.highlightedJoinColumns}
              modelColumnsByTable={s.modelColumnsByTable}
              pendingColumnKeys={s.pendingColumnKeys}
              onToggleColumnDimension={(table, columnName) => {
                void s.toggleColumnDimension(table, columnName)
              }}
              onDeleteJoin={(joinId) => {
                void s.deleteJoin(joinId)
              }}
              onOpenTableDetail={s.setDetailTable}
              onAddCalcField={() => s.setAddMetricOpen(true)}
              onAddRelationship={() => setToolsTab('relationship')}
              t={s.t}
            />
          </section>

          <ModelingToolsModal
            activeTab={toolsTab}
            onTabChange={setToolsTab}
            onClose={() => setToolsTab(null)}
            t={s.t}
            semanticContent={
              <ModelingPalette
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
                onOpenAddDimension={() => s.setAddDimensionOpen(true)}
                onEditMetric={s.setEditingMetric}
                onDeleteMetric={(metricId) => {
                  void s.deleteMetric(metricId)
                }}
                onReactivateMetric={(metric) => {
                  void s.reactivateMetric(metric)
                }}
                t={s.t}
              />
            }
            relationshipContent={
              <JoinEditor
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
            }
          />
        </>
      )}
      {s.model && (
        <ModelVersionsModal
          open={versionsOpen}
          modelId={s.model.id}
          modelName={s.model.name}
          onClose={() => setVersionsOpen(false)}
          onRolledBack={() => {
            void s.refreshModels(s.model!.id)
            s.setMessage(s.t('modeling.versions_restored'))
          }}
        />
      )}
      {s.model && (
        <TableDetailModal
          open={s.detailTable !== null}
          table={s.detailTable}
          model={s.model}
          columns={s.columns}
          datasourceId={s.datasourceId}
          postData={s.postData}
          onClose={() => s.setDetailTable(null)}
          onEdit={(table) => {
            s.setDetailTable(null)
            s.renameTable(table)
          }}
          t={s.t}
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
          addDimensionOpen={s.addDimensionOpen}
          onCloseAddDimension={() => s.setAddDimensionOpen(false)}
          onDimensionCreated={async () => {
            s.setAddDimensionOpen(false)
            await s.refreshModels(s.model!.id)
            s.setMessage(s.t('modeling.dimension_added'))
          }}
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
