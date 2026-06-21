import { useCallback, useEffect, useState } from 'react'

import { useApi } from '../hooks/useApi'
import { useDatasources } from '../hooks/useDatasources'
import { useSemanticModels } from '../hooks/useSemanticModels'
import { useT } from '../i18n'
import { legacyButtonClass } from '../lib/buttonClasses'
import { legacyCardClass } from '../lib/cardClasses'
import { cn } from '../lib/cn'
import { formRowClass, legacyFormClass } from '../lib/formClasses'
import { legacyLayoutClass } from '../lib/layoutClasses'
import { modalActionsBorderedClass } from '../lib/modalClasses'
import type { LogicalQuery, SelectField } from '../types/ai'
import { DashboardWidgetRenderer } from './dashboard/DashboardWidgetRenderer'
import { ErrorAlert } from './ui/ErrorAlert'
import { LoadingOverlay } from './ui/LoadingOverlay'
import { LoadingScreen } from './ui/LoadingScreen'
import { Modal } from './ui/Modal'
import { Select } from './ui/Select'

interface Widget {
  id: string
  type: 'chart' | 'table' | 'kpi' | 'text'
  title: string
  w: number // column span (1-12)
  h: 'small' | 'medium' | 'large'
  saved_query_id?: string
  logical_query?: LogicalQuery
  chart_type?: 'line' | 'bar' | 'pie'
  config?: {
    xAxisColumn?: string
    yAxisColumns?: string[]
    valueColumn?: string
    visibleColumns?: string[]
  }
  content?: string
}

interface Dashboard {
  id: string
  name: string
  description?: string
  widgets: Widget[]
}

interface DashboardBuilderProps {
  dashboardId: string
  onBack: () => void
}

interface SavedQuestion {
  id: string
  name: string
  question: string
  logical_query: LogicalQuery
  datasource_id: string
  model_id?: string
}

export default function DashboardBuilder({ dashboardId, onBack }: DashboardBuilderProps) {
  const t = useT()
  const { get, putData, loading, error } = useApi()
  const [dashboard, setDashboard] = useState<Dashboard | null>(null)
  const [widgets, setWidgets] = useState<Widget[]>([])
  const [isEditMode, setIsEditMode] = useState(false)
  const [isDirty, setIsDirty] = useState(false)

  // Configure widget modal
  const [activeConfigWidget, setActiveConfigWidget] = useState<Widget | null>(null)
  const [isConfigModalOpen, setIsConfigModalOpen] = useState(false)
  const [configTitle, setConfigTitle] = useState('')
  const [configType, setConfigType] = useState<'chart' | 'table' | 'kpi' | 'text'>('text')
  const [configChartType, setConfigChartType] = useState<'line' | 'bar' | 'pie'>('line')
  const [configContent, setConfigContent] = useState('')
  const [configWidth, setConfigWidth] = useState(6)
  const [configHeight, setConfigHeight] = useState<'small' | 'medium' | 'large'>('medium')

  // Saved query linking
  const { datasources } = useDatasources()
  const [selDatasourceId, setSelDatasourceId] = useState('')
  const { models } = useSemanticModels(selDatasourceId)
  const [selModelId, setSelModelId] = useState('')
  const [savedQuestions, setSavedQuestions] = useState<SavedQuestion[]>([])
  const [selQuestionId, setSelQuestionId] = useState('')
  const [availableColumns, setAvailableColumns] = useState<string[]>([])

  // Column mapping states
  const [xAxisColumn, setXAxisColumn] = useState('')
  const [yAxisColumns, setYAxisColumns] = useState<string[]>([])
  const [valueColumn, setValueColumn] = useState('')
  const [visibleColumns, setVisibleColumns] = useState<string[]>([])

  const fetchDashboard = useCallback(async () => {
    const data = await get<Dashboard>(`/api/dashboards/${dashboardId}`)
    if (data) {
      setDashboard(data)
      setWidgets(data.widgets)
      setIsDirty(false)
    }
  }, [dashboardId, get])

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void fetchDashboard()
  }, [fetchDashboard])

  // Load Saved Questions when selected DS/Model changes in modal
  useEffect(() => {
    if (selDatasourceId && isConfigModalOpen) {
      const url = `/api/ai/examples?datasource_id=${encodeURIComponent(selDatasourceId)}${selModelId ? `&model_id=${encodeURIComponent(selModelId)}` : ''}`
      void get<SavedQuestion[]>(url).then((res) => {
        setSavedQuestions(res ?? [])
      })
    } else {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setSavedQuestions([])
    }
  }, [selDatasourceId, selModelId, isConfigModalOpen, get])

  // Reset columns mapping when selected Saved Question changes
  const handleQuestionChange = (qId: string) => {
    setSelQuestionId(qId)
    const q = savedQuestions.find((item) => item.id === qId)
    if (q?.logical_query) {
      // Find columns referenced in select block
      const selectItems = q.logical_query.select ?? []
      const cols = selectItems.map((item: SelectField) => item.alias ?? item.name)
      setAvailableColumns(cols)
      if (cols.length > 0) {
        const firstCol = cols[0] ?? ''
        setXAxisColumn(firstCol)
        setValueColumn(firstCol)
        setYAxisColumns([firstCol])
        setVisibleColumns(cols)
      } else {
        setXAxisColumn('')
        setValueColumn('')
        setYAxisColumns([])
        setVisibleColumns([])
      }
    } else {
      setAvailableColumns([])
    }
  }

  const handleSaveLayout = async () => {
    if (!dashboard) {
      return
    }
    const payload = {
      name: dashboard.name,
      description: dashboard.description,
      widgets,
    }
    const res = await putData(`/api/dashboards/${dashboardId}`, payload)
    if (res) {
      setIsDirty(false)
      void fetchDashboard()
    }
  }

  const handleAddWidget = (type: 'chart' | 'table' | 'kpi' | 'text') => {
    const newWidget: Widget = {
      id: `w-${Date.now()}`,
      type,
      title: `New ${type.toUpperCase()} Widget`,
      w: type === 'kpi' ? 3 : 6,
      h: 'medium',
      content: type === 'text' ? 'Double click or edit to write text...' : undefined,
    }
    setWidgets([...widgets, newWidget])
    setIsDirty(true)
  }

  const handleDeleteWidget = (id: string) => {
    setWidgets(widgets.filter((w) => w.id !== id))
    setIsDirty(true)
  }

  // Open modal to configure widget
  const handleOpenConfig = (w: Widget) => {
    setActiveConfigWidget(w)
    setConfigTitle(w.title)
    setConfigType(w.type)
    setConfigChartType(w.chart_type ?? 'line')
    setConfigContent(w.content ?? '')
    setConfigWidth(w.w || 6)
    setConfigHeight(w.h)

    // Prefill linking query states if present
    if (w.saved_query_id) {
      if (w.logical_query?.datasource_id) {
        setSelDatasourceId(w.logical_query.datasource_id)
        setSelModelId(w.logical_query.model_id)
      }
      setSelQuestionId(w.saved_query_id)

      const selectItems = w.logical_query?.select ?? []
      const cols = selectItems.map((item: SelectField) => item.alias ?? item.name)
      setAvailableColumns(cols)

      setXAxisColumn(w.config?.xAxisColumn ?? '')
      setYAxisColumns(w.config?.yAxisColumns ?? [])
      setValueColumn(w.config?.valueColumn ?? '')
      setVisibleColumns(w.config?.visibleColumns ?? [])
    } else {
      if (datasources.length > 0) {
        setSelDatasourceId(datasources[0]!.id)
      }
      setSelModelId('')
      setSelQuestionId('')
      setAvailableColumns([])
      setXAxisColumn('')
      setYAxisColumns([])
      setValueColumn('')
      setVisibleColumns([])
    }

    setIsConfigModalOpen(true)
  }

  const handleSaveConfig = () => {
    if (!activeConfigWidget) {
      return
    }

    const selectedQ = savedQuestions.find((q) => q.id === selQuestionId)

    const updated: Widget = {
      ...activeConfigWidget,
      title: configTitle,
      type: configType,
      w: configWidth,
      h: configHeight,
      content: configType === 'text' ? configContent : undefined,
      chart_type: configType === 'chart' ? configChartType : undefined,
      saved_query_id: configType !== 'text' ? selQuestionId : undefined,
      logical_query: configType !== 'text' && selectedQ ? selectedQ.logical_query : undefined,
      config:
        configType !== 'text'
          ? {
              xAxisColumn: configType === 'chart' ? xAxisColumn : undefined,
              yAxisColumns: configType === 'chart' ? yAxisColumns : undefined,
              valueColumn: configType === 'kpi' ? valueColumn : undefined,
              visibleColumns: configType === 'table' ? visibleColumns : undefined,
            }
          : undefined,
    }

    setWidgets(widgets.map((w) => (w.id === activeConfigWidget.id ? updated : w)))
    setIsConfigModalOpen(false)
    setIsDirty(true)
  }

  // HTML5 Drag and drop reordering
  const handleDragStart = (e: React.DragEvent, idx: number) => {
    if (!isEditMode) {
      return
    }
    e.dataTransfer.setData('text/plain', idx.toString())
  }

  const handleDrop = (e: React.DragEvent, toIdx: number) => {
    e.preventDefault()
    if (!isEditMode) {
      return
    }
    const fromIdxStr = e.dataTransfer.getData('text/plain')
    const fromIdx = parseInt(fromIdxStr, 10)
    if (isNaN(fromIdx) || fromIdx === toIdx) {
      return
    }

    const nextWidgets = [...widgets]
    const [moved] = nextWidgets.splice(fromIdx, 1)
    nextWidgets.splice(toIdx, 0, moved!)
    setWidgets(nextWidgets)
    setIsDirty(true)
  }

  const heightPixels = (h: 'small' | 'medium' | 'large') => {
    switch (h) {
      case 'small':
        return '180px'
      case 'large':
        return '440px'
      default:
        return '300px'
    }
  }

  if (loading && !dashboard) {
    return <LoadingScreen minHeight="300px" />
  }

  return (
    <div className={cn(legacyLayoutClass('page-stack'), 'relative')}>
      <LoadingOverlay loading={loading}>
        {error && <ErrorAlert error={error} />}

        {/* Header Panel */}
        <div className={legacyCardClass('card')}>
          <div className="flex flex-wrap items-center justify-between gap-4">
            <div>
              <button
                type="button"
                className={cn(legacyButtonClass('btn btn-secondary btn-sm'), 'mb-2')}
                onClick={onBack}
              >
                ← Back to Dashboards
              </button>
              <h2 className="text-[1.8rem] font-bold">{dashboard?.name}</h2>
              {dashboard?.description && (
                <p className="text-foreground-faint mt-0.5 text-[0.9rem]">
                  {dashboard.description}
                </p>
              )}
            </div>
            <div className="flex gap-3">
              {isEditMode ? (
                <>
                  <button
                    type="button"
                    className={legacyButtonClass('btn btn-secondary')}
                    onClick={() => {
                      setIsEditMode(false)
                      void fetchDashboard()
                    }}
                  >
                    Cancel
                  </button>
                  <button
                    type="button"
                    className={legacyButtonClass('btn btn-primary')}
                    onClick={() => {
                      void handleSaveLayout()
                    }}
                    disabled={!isDirty}
                  >
                    💾 Save Dashboard
                  </button>
                </>
              ) : (
                <button
                  type="button"
                  className={legacyButtonClass('btn btn-primary')}
                  onClick={() => setIsEditMode(true)}
                >
                  ✏️ Edit Layout
                </button>
              )}
            </div>
          </div>

          {/* Toolbox */}
          {isEditMode && (
            <div className="border-border mt-6 flex gap-3 border-t pt-4">
              <span className="text-foreground-faint self-center text-[0.9rem] font-semibold">
                Add Widget:
              </span>
              <button
                type="button"
                className={legacyButtonClass('btn btn-secondary btn-sm')}
                onClick={() => handleAddWidget('kpi')}
              >
                ➕ KPI
              </button>
              <button
                type="button"
                className={legacyButtonClass('btn btn-secondary btn-sm')}
                onClick={() => handleAddWidget('chart')}
              >
                ➕ Chart
              </button>
              <button
                type="button"
                className={legacyButtonClass('btn btn-secondary btn-sm')}
                onClick={() => handleAddWidget('table')}
              >
                ➕ Table
              </button>
              <button
                type="button"
                className={legacyButtonClass('btn btn-secondary btn-sm')}
                onClick={() => handleAddWidget('text')}
              >
                ➕ Text
              </button>
            </div>
          )}
        </div>

        {/* Widgets Grid */}
        {widgets.length === 0 ? (
          <div
            className={cn(
              legacyCardClass('card'),
              'border-border border-2 border-dashed px-8 py-24 text-center',
            )}
          >
            <p className="text-foreground-faint">This dashboard is empty.</p>
            {isEditMode && (
              <div className="mt-4 flex justify-center gap-4">
                <button
                  type="button"
                  className={legacyButtonClass('btn btn-secondary')}
                  onClick={() => handleAddWidget('chart')}
                >
                  Add Chart
                </button>
                <button
                  type="button"
                  className={legacyButtonClass('btn btn-secondary')}
                  onClick={() => handleAddWidget('kpi')}
                >
                  Add KPI
                </button>
              </div>
            )}
          </div>
        ) : (
          <div className="grid grid-cols-12 gap-6">
            {widgets.map((widget, idx) => (
              <div
                key={widget.id}
                draggable={isEditMode}
                onDragStart={(e) => handleDragStart(e, idx)}
                onDragOver={(e) => e.preventDefault()}
                onDrop={(e) => handleDrop(e, idx)}
                className={cn(legacyCardClass('card card--elevated'), 'relative flex flex-col p-5')}
                style={{
                  gridColumn: `span ${widget.w || 6}`,
                  minHeight: heightPixels(widget.h),
                  border: isEditMode ? '2px dashed var(--accent-light, var(--border))' : undefined,
                  cursor: isEditMode ? 'grab' : 'default',
                }}
              >
                {/* Widget Header Controls */}
                <div className="mb-3 flex items-center justify-between border-b border-(--border-light,#f1f5f9) pb-1">
                  <h3 className="text-foreground m-0 text-base font-semibold">{widget.title}</h3>
                  {isEditMode && (
                    <div className="flex items-center gap-1">
                      {/* Width Resize */}
                      <button
                        type="button"
                        className={cn(
                          legacyButtonClass('btn btn-secondary btn-sm'),
                          'px-[0.3rem] py-[0.1rem] text-xs',
                        )}
                        onClick={() => {
                          const newW = Math.max(2, (widget.w || 6) - 1)
                          setWidgets(
                            widgets.map((w) => (w.id === widget.id ? { ...w, w: newW } : w)),
                          )
                          setIsDirty(true)
                        }}
                        title={t('customDashboards.builder_shrink_width_aria')}
                        aria-label={t('customDashboards.builder_shrink_width_aria')}
                      >
                        ◀
                      </button>
                      <span className="text-foreground-faint text-xs">{widget.w || 6}</span>
                      <button
                        type="button"
                        className={cn(
                          legacyButtonClass('btn btn-secondary btn-sm'),
                          'px-[0.3rem] py-[0.1rem] text-xs',
                        )}
                        onClick={() => {
                          const newW = Math.min(12, (widget.w || 6) + 1)
                          setWidgets(
                            widgets.map((w) => (w.id === widget.id ? { ...w, w: newW } : w)),
                          )
                          setIsDirty(true)
                        }}
                        title={t('customDashboards.builder_expand_width_aria')}
                        aria-label={t('customDashboards.builder_expand_width_aria')}
                      >
                        ▶
                      </button>

                      {/* Config / Delete */}
                      <button
                        type="button"
                        className={cn(
                          legacyButtonClass('btn btn-secondary btn-sm'),
                          'px-[0.3rem] py-[0.1rem]',
                        )}
                        onClick={() => handleOpenConfig(widget)}
                        title={t('customDashboards.builder_configure_widget_aria')}
                        aria-label={t('customDashboards.builder_configure_widget_aria')}
                      >
                        ⚙️
                      </button>
                      <button
                        type="button"
                        className={cn(
                          legacyButtonClass('btn btn-danger btn-sm'),
                          'px-[0.3rem] py-[0.1rem]',
                        )}
                        onClick={() => handleDeleteWidget(widget.id)}
                        title={t('customDashboards.builder_delete_widget_aria')}
                        aria-label={t('customDashboards.builder_delete_widget_aria')}
                      >
                        ✕
                      </button>
                    </div>
                  )}
                </div>

                {/* Mapped visual display */}
                <div className="flex-1 overflow-hidden">
                  <DashboardWidgetRenderer widget={widget} />
                </div>
              </div>
            ))}
          </div>
        )}

        {/* Widget Editor Configuration Modal */}
        <Modal
          open={isConfigModalOpen}
          onClose={() => setIsConfigModalOpen(false)}
          title={t('customDashboards.builder_configure_widget_title')}
          className="w-[90%] max-w-150"
          bodyClassName="flex max-h-[70vh] flex-col gap-5 overflow-y-auto"
        >
          <div className={legacyFormClass('form-field')}>
            <label className={legacyFormClass('form-label')} htmlFor="w-title">
              Widget Title
            </label>
            <input
              id="w-title"
              type="text"
              value={configTitle}
              onChange={(e) => setConfigTitle(e.target.value)}
              required
            />
          </div>

          <div className={formRowClass}>
            <div className={cn(legacyFormClass('form-field'), 'flex-1')}>
              <label className={legacyFormClass('form-label')} htmlFor="w-width">
                Width Grid Span (2-12 Columns)
              </label>
              <Select
                id="w-width"
                value={configWidth.toString()}
                onChange={(val) => setConfigWidth(parseInt(val, 10))}
                options={Array.from({ length: 11 }, (_, i) => ({
                  value: (i + 2).toString(),
                  label: `${i + 2} Columns`,
                }))}
              />
            </div>
            <div className={cn(legacyFormClass('form-field'), 'flex-1')}>
              <label className={legacyFormClass('form-label')} htmlFor="w-height">
                Height Class
              </label>
              <Select
                id="w-height"
                value={configHeight}
                onChange={setConfigHeight}
                options={[
                  { value: 'small', label: 'Small (180px)' },
                  { value: 'medium', label: 'Medium (300px)' },
                  { value: 'large', label: 'Large (440px)' },
                ]}
              />
            </div>
          </div>

          <div className={legacyFormClass('form-field')}>
            <label className={legacyFormClass('form-label')} htmlFor="w-type">
              Widget Type
            </label>
            <Select
              id="w-type"
              value={configType}
              onChange={setConfigType}
              options={[
                { value: 'text', label: 'Text Block' },
                { value: 'kpi', label: 'KPI Summary Card' },
                { value: 'chart', label: 'Chart Visualization' },
                { value: 'table', label: 'Data Table' },
              ]}
            />
          </div>

          {configType === 'text' ? (
            <div className={legacyFormClass('form-field')}>
              <label className={legacyFormClass('form-label')} htmlFor="w-content">
                Text Content
              </label>
              <textarea
                id="w-content"
                value={configContent}
                onChange={(e) => setConfigContent(e.target.value)}
                placeholder={t('customDashboards.builder_text_content_placeholder')}
                rows={6}
              />
            </div>
          ) : (
            <>
              <div className="border-border bg-card-raised rounded-lg border p-3">
                <h4 className="m-0 mb-3 text-[0.9rem] font-semibold">
                  Link Saved Query Data Source
                </h4>

                <div className={cn(formRowClass, 'mb-[0.8rem] gap-3')}>
                  <div className={cn(legacyFormClass('form-field'), 'flex-1')}>
                    <label className={legacyFormClass('form-label')} htmlFor="w-ds">
                      Datasource
                    </label>
                    <Select
                      id="w-ds"
                      value={selDatasourceId}
                      onChange={(val) => {
                        setSelDatasourceId(val)
                        setSelModelId('')
                        setSelQuestionId('')
                      }}
                      options={datasources.map((d) => ({ value: d.id, label: d.name }))}
                    />
                  </div>
                  <div className={cn(legacyFormClass('form-field'), 'flex-1')}>
                    <label className={legacyFormClass('form-label')} htmlFor="w-model">
                      Model
                    </label>
                    <Select
                      id="w-model"
                      value={selModelId}
                      onChange={(val) => {
                        setSelModelId(val)
                        setSelQuestionId('')
                      }}
                      options={[
                        { value: '', label: 'All Models' },
                        ...models.map((m) => ({ value: m.id, label: m.label ?? m.name })),
                      ]}
                    />
                  </div>
                </div>

                <div className={legacyFormClass('form-field')}>
                  <label className={legacyFormClass('form-label')} htmlFor="w-question">
                    Saved Question (Query)
                  </label>
                  <Select
                    id="w-question"
                    value={selQuestionId}
                    onChange={handleQuestionChange}
                    options={[
                      { value: '', label: 'Select Saved Question...' },
                      ...savedQuestions.map((q) => ({
                        value: q.id,
                        label: q.name || q.question,
                      })),
                    ]}
                  />
                </div>
              </div>

              {selQuestionId && availableColumns.length > 0 && (
                <div className="border-border mt-2 rounded-lg border p-3">
                  <h4 className="m-0 mb-3 text-[0.9rem] font-semibold">Map Fields / Columns</h4>

                  {configType === 'kpi' && (
                    <div className={legacyFormClass('form-field')}>
                      <label className={legacyFormClass('form-label')} htmlFor="w-kpi-col">
                        KPI Metric Column
                      </label>
                      <Select
                        id="w-kpi-col"
                        value={valueColumn}
                        onChange={setValueColumn}
                        options={availableColumns.map((col) => ({ value: col, label: col }))}
                      />
                    </div>
                  )}

                  {configType === 'chart' && (
                    <div className="grid gap-4">
                      <div className={legacyFormClass('form-field')}>
                        <label className={legacyFormClass('form-label')} htmlFor="w-chart-type">
                          Chart Visualization Type
                        </label>
                        <Select
                          id="w-chart-type"
                          value={configChartType}
                          onChange={setConfigChartType}
                          options={[
                            { value: 'line', label: '📈 Line Chart' },
                            { value: 'bar', label: '📊 Bar Chart' },
                            { value: 'pie', label: '🍕 Pie Chart' },
                          ]}
                        />
                      </div>

                      <div className={cn(formRowClass, 'gap-3')}>
                        <div className={cn(legacyFormClass('form-field'), 'flex-1')}>
                          <label className={legacyFormClass('form-label')} htmlFor="w-xaxis">
                            X-Axis Category Column
                          </label>
                          <Select
                            id="w-xaxis"
                            value={xAxisColumn}
                            onChange={setXAxisColumn}
                            options={availableColumns.map((col) => ({
                              value: col,
                              label: col,
                            }))}
                          />
                        </div>
                        <div className={cn(legacyFormClass('form-field'), 'flex-1')}>
                          <label className={legacyFormClass('form-label')}>
                            Y-Axis Metric Series (Select multiple)
                          </label>
                          <div className="border-border max-h-25 overflow-y-auto rounded border p-1">
                            {availableColumns.map((col) => {
                              const checked = yAxisColumns.includes(col)
                              return (
                                <label
                                  key={col}
                                  className="flex cursor-pointer items-center gap-1 py-[0.1rem] text-xs"
                                >
                                  <input
                                    type="checkbox"
                                    checked={checked}
                                    onChange={() => {
                                      if (checked) {
                                        setYAxisColumns(yAxisColumns.filter((y) => y !== col))
                                      } else {
                                        setYAxisColumns([...yAxisColumns, col])
                                      }
                                    }}
                                  />
                                  {col}
                                </label>
                              )
                            })}
                          </div>
                        </div>
                      </div>
                    </div>
                  )}

                  {configType === 'table' && (
                    <div className={legacyFormClass('form-field')}>
                      <label className={legacyFormClass('form-label')}>Visible Table Columns</label>
                      <div className="border-border flex flex-wrap gap-3 rounded border p-1.5">
                        {availableColumns.map((col) => {
                          const checked = visibleColumns.includes(col)
                          return (
                            <label
                              key={col}
                              className="flex cursor-pointer items-center gap-1 text-xs"
                            >
                              <input
                                type="checkbox"
                                checked={checked}
                                onChange={() => {
                                  if (checked) {
                                    setVisibleColumns(visibleColumns.filter((c) => c !== col))
                                  } else {
                                    setVisibleColumns([...visibleColumns, col])
                                  }
                                }}
                              />
                              {col}
                            </label>
                          )
                        })}
                      </div>
                    </div>
                  )}
                </div>
              )}
            </>
          )}
          <div className={modalActionsBorderedClass()}>
            <button
              type="button"
              className={legacyButtonClass('btn btn-secondary')}
              onClick={() => setIsConfigModalOpen(false)}
            >
              Cancel
            </button>
            <button
              type="button"
              className={legacyButtonClass('btn btn-primary')}
              onClick={handleSaveConfig}
              disabled={configType !== 'text' && !selQuestionId}
            >
              Apply Configuration
            </button>
          </div>
        </Modal>
      </LoadingOverlay>
    </div>
  )
}
