import { useCallback, useEffect, useState } from 'react'
import { Bar } from 'recharts/es6/cartesian/Bar'
import { CartesianGrid } from 'recharts/es6/cartesian/CartesianGrid'
import { Line } from 'recharts/es6/cartesian/Line'
import { XAxis } from 'recharts/es6/cartesian/XAxis'
import { YAxis } from 'recharts/es6/cartesian/YAxis'
import { BarChart } from 'recharts/es6/chart/BarChart'
import { LineChart } from 'recharts/es6/chart/LineChart'
import { PieChart } from 'recharts/es6/chart/PieChart'

// @ts-ignore
import { Legend } from 'recharts/es6/component/Legend'
import { ResponsiveContainer } from 'recharts/es6/component/ResponsiveContainer'
import { Tooltip } from 'recharts/es6/component/Tooltip'
import { Pie } from 'recharts/es6/polar/Pie'

import { useApi } from '../hooks/useApi'
import { useDatasources } from '../hooks/useDatasources'
import { useSemanticModels } from '../hooks/useSemanticModels'
import { useT } from '../i18n'
import type { LogicalQuery, QueryResultPayload, SelectField } from '../types/ai'
import { chartAxisStroke, chartGridStroke, chartTooltipStyle } from '../utils/chartConfig'
import { chartColor } from '../utils/constants'
import { ErrorAlert } from './ui/ErrorAlert'
import { LoadingOverlay } from './ui/LoadingOverlay'
import { LoadingScreen } from './ui/LoadingScreen'
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
      setWidgets(data.widgets || [])
      setIsDirty(false)
    }
  }, [dashboardId, get])

  useEffect(() => {
    fetchDashboard()
  }, [fetchDashboard])

  // Load Saved Questions when selected DS/Model changes in modal
  useEffect(() => {
    if (selDatasourceId && isConfigModalOpen) {
      const url = `/api/ai/examples?datasource_id=${encodeURIComponent(selDatasourceId)}${selModelId ? `&model_id=${encodeURIComponent(selModelId)}` : ''}`
      get<SavedQuestion[]>(url).then((res) => {
        setSavedQuestions(res ?? [])
      })
    } else {
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
      fetchDashboard()
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
    setConfigHeight(w.h || 'medium')

    // Prefill linking query states if present
    if (w.saved_query_id) {
      const q = savedQuestions.find((item) => item.id === w.saved_query_id)
      if (w.logical_query?.datasource_id) {
        setSelDatasourceId(w.logical_query.datasource_id)
        setSelModelId(w.logical_query.model_id ?? '')
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
    <div className="page-stack" style={{ position: 'relative' }}>
      <LoadingOverlay loading={loading}>
        {error && <ErrorAlert error={error} />}

        {/* Header Panel */}
        <div className="card">
          <div
            style={{
              display: 'flex',
              justifyContent: 'space-between',
              alignItems: 'center',
              flexWrap: 'wrap',
              gap: '1rem',
            }}
          >
            <div>
              <button
                type="button"
                className="btn btn-secondary btn-sm"
                onClick={onBack}
                style={{ marginBottom: '0.5rem' }}
              >
                ← Back to Dashboards
              </button>
              <h2 style={{ fontSize: '1.8rem', fontWeight: 700 }}>{dashboard?.name}</h2>
              {dashboard?.description && (
                <p style={{ color: 'var(--text-muted)', fontSize: '0.9rem', marginTop: '0.2rem' }}>
                  {dashboard.description}
                </p>
              )}
            </div>
            <div style={{ display: 'flex', gap: '0.75rem' }}>
              {isEditMode ? (
                <>
                  <button
                    type="button"
                    className="btn btn-secondary"
                    onClick={() => {
                      setIsEditMode(false)
                      fetchDashboard()
                    }}
                  >
                    Cancel
                  </button>
                  <button
                    type="button"
                    className="btn btn-primary"
                    onClick={handleSaveLayout}
                    disabled={!isDirty}
                  >
                    💾 Save Dashboard
                  </button>
                </>
              ) : (
                <button
                  type="button"
                  className="btn btn-primary"
                  onClick={() => setIsEditMode(true)}
                >
                  ✏️ Edit Layout
                </button>
              )}
            </div>
          </div>

          {/* Toolbox */}
          {isEditMode && (
            <div
              style={{
                display: 'flex',
                gap: '0.75rem',
                marginTop: '1.5rem',
                padding: '1rem 0 0 0',
                borderTop: '1px solid var(--border)',
              }}
            >
              <span
                style={{
                  alignSelf: 'center',
                  fontWeight: 600,
                  color: 'var(--text-muted)',
                  fontSize: '0.9rem',
                }}
              >
                Add Widget:
              </span>
              <button
                type="button"
                className="btn btn-secondary btn-sm"
                onClick={() => handleAddWidget('kpi')}
              >
                ➕ KPI
              </button>
              <button
                type="button"
                className="btn btn-secondary btn-sm"
                onClick={() => handleAddWidget('chart')}
              >
                ➕ Chart
              </button>
              <button
                type="button"
                className="btn btn-secondary btn-sm"
                onClick={() => handleAddWidget('table')}
              >
                ➕ Table
              </button>
              <button
                type="button"
                className="btn btn-secondary btn-sm"
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
            className="card"
            style={{
              padding: '6rem 2rem',
              textAlign: 'center',
              border: '2px dashed var(--border)',
            }}
          >
            <p style={{ color: 'var(--text-muted)' }}>This dashboard is empty.</p>
            {isEditMode && (
              <div
                style={{
                  display: 'flex',
                  justifyContent: 'center',
                  gap: '1rem',
                  marginTop: '1rem',
                }}
              >
                <button
                  type="button"
                  className="btn btn-secondary"
                  onClick={() => handleAddWidget('chart')}
                >
                  Add Chart
                </button>
                <button
                  type="button"
                  className="btn btn-secondary"
                  onClick={() => handleAddWidget('kpi')}
                >
                  Add KPI
                </button>
              </div>
            )}
          </div>
        ) : (
          <div
            style={{
              display: 'grid',
              gridTemplateColumns: 'repeat(12, 1fr)',
              gap: '1.5rem',
            }}
          >
            {widgets.map((widget, idx) => (
              <div
                key={widget.id}
                draggable={isEditMode}
                onDragStart={(e) => handleDragStart(e, idx)}
                onDragOver={(e) => e.preventDefault()}
                onDrop={(e) => handleDrop(e, idx)}
                className="card card--elevated"
                style={{
                  gridColumn: `span ${widget.w || 6}`,
                  minHeight: heightPixels(widget.h),
                  display: 'flex',
                  flexDirection: 'column',
                  position: 'relative',
                  padding: '1.25rem',
                  border: isEditMode ? '2px dashed var(--accent-light, var(--border))' : undefined,
                  cursor: isEditMode ? 'grab' : 'default',
                }}
              >
                {/* Widget Header Controls */}
                <div
                  style={{
                    display: 'flex',
                    justifyContent: 'space-between',
                    alignItems: 'center',
                    marginBottom: '0.8rem',
                    borderBottom: '1px solid var(--border-light, #f1f5f9)',
                    paddingBottom: '0.4rem',
                  }}
                >
                  <h3
                    style={{ margin: 0, fontSize: '1rem', fontWeight: 600, color: 'var(--text)' }}
                  >
                    {widget.title}
                  </h3>
                  {isEditMode && (
                    <div style={{ display: 'flex', gap: '0.4rem', alignItems: 'center' }}>
                      {/* Width Resize */}
                      <button
                        type="button"
                        className="btn btn-secondary btn-sm"
                        style={{ padding: '0.1rem 0.3rem', fontSize: '0.75rem' }}
                        onClick={() => {
                          const newW = Math.max(2, (widget.w || 6) - 1)
                          setWidgets(
                            widgets.map((w) => (w.id === widget.id ? { ...w, w: newW } : w)),
                          )
                          setIsDirty(true)
                        }}
                        title="Shrink Width"
                      >
                        ◀
                      </button>
                      <span style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>
                        {widget.w || 6}
                      </span>
                      <button
                        type="button"
                        className="btn btn-secondary btn-sm"
                        style={{ padding: '0.1rem 0.3rem', fontSize: '0.75rem' }}
                        onClick={() => {
                          const newW = Math.min(12, (widget.w || 6) + 1)
                          setWidgets(
                            widgets.map((w) => (w.id === widget.id ? { ...w, w: newW } : w)),
                          )
                          setIsDirty(true)
                        }}
                        title="Expand Width"
                      >
                        ▶
                      </button>

                      {/* Config / Delete */}
                      <button
                        type="button"
                        className="btn btn-secondary btn-sm"
                        style={{ padding: '0.1rem 0.3rem' }}
                        onClick={() => handleOpenConfig(widget)}
                        title="Configure Widget"
                      >
                        ⚙️
                      </button>
                      <button
                        type="button"
                        className="btn btn-danger btn-sm"
                        style={{ padding: '0.1rem 0.3rem' }}
                        onClick={() => handleDeleteWidget(widget.id)}
                        title="Delete Widget"
                      >
                        ✕
                      </button>
                    </div>
                  )}
                </div>

                {/* Mapped visual display */}
                <div style={{ flexGrow: 1, overflow: 'hidden' }}>
                  <WidgetRenderer widget={widget} />
                </div>
              </div>
            ))}
          </div>
        )}

        {/* Widget Editor Configuration Modal */}
        {isConfigModalOpen && (
          <div className="modal-backdrop">
            <div className="modal-content" style={{ maxWidth: '600px', width: '90%' }}>
              <div className="modal-header">
                <h3>Configure Widget</h3>
                <button
                  type="button"
                  className="modal-close"
                  onClick={() => setIsConfigModalOpen(false)}
                >
                  ✕
                </button>
              </div>
              <div
                className="modal-body"
                style={{
                  display: 'flex',
                  flexDirection: 'column',
                  gap: '1.25rem',
                  maxHeight: '70vh',
                  overflowY: 'auto',
                }}
              >
                <div className="form-field">
                  <label className="form-label" htmlFor="w-title">
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

                <div className="form-row">
                  <div className="form-field" style={{ flex: 1 }}>
                    <label className="form-label" htmlFor="w-width">
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
                  <div className="form-field" style={{ flex: 1 }}>
                    <label className="form-label" htmlFor="w-height">
                      Height Class
                    </label>
                    <Select
                      id="w-height"
                      value={configHeight}
                      onChange={(val: any) => setConfigHeight(val)}
                      options={[
                        { value: 'small', label: 'Small (180px)' },
                        { value: 'medium', label: 'Medium (300px)' },
                        { value: 'large', label: 'Large (440px)' },
                      ]}
                    />
                  </div>
                </div>

                <div className="form-field">
                  <label className="form-label" htmlFor="w-type">
                    Widget Type
                  </label>
                  <Select
                    id="w-type"
                    value={configType}
                    onChange={(val: any) => setConfigType(val)}
                    options={[
                      { value: 'text', label: 'Text Block' },
                      { value: 'kpi', label: 'KPI Summary Card' },
                      { value: 'chart', label: 'Chart Visualization' },
                      { value: 'table', label: 'Data Table' },
                    ]}
                  />
                </div>

                {configType === 'text' ? (
                  <div className="form-field">
                    <label className="form-label" htmlFor="w-content">
                      Text Content
                    </label>
                    <textarea
                      id="w-content"
                      value={configContent}
                      onChange={(e) => setConfigContent(e.target.value)}
                      placeholder="Write text/markdown here..."
                      rows={6}
                    />
                  </div>
                ) : (
                  <>
                    <div
                      style={{
                        padding: '0.8rem',
                        border: '1px solid var(--border)',
                        borderRadius: '0.5rem',
                        background: 'var(--bg-card-raised)',
                      }}
                    >
                      <h4 style={{ margin: '0 0 0.8rem 0', fontSize: '0.9rem', fontWeight: 600 }}>
                        Link Saved Query Data Source
                      </h4>

                      <div className="form-row" style={{ gap: '0.75rem', marginBottom: '0.8rem' }}>
                        <div className="form-field" style={{ flex: 1 }}>
                          <label className="form-label" htmlFor="w-ds">
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
                        <div className="form-field" style={{ flex: 1 }}>
                          <label className="form-label" htmlFor="w-model">
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

                      <div className="form-field">
                        <label className="form-label" htmlFor="w-question">
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
                      <div
                        style={{
                          padding: '0.8rem',
                          border: '1px solid var(--border)',
                          borderRadius: '0.5rem',
                          marginTop: '0.5rem',
                        }}
                      >
                        <h4 style={{ margin: '0 0 0.8rem 0', fontSize: '0.9rem', fontWeight: 600 }}>
                          Map Fields / Columns
                        </h4>

                        {configType === 'kpi' && (
                          <div className="form-field">
                            <label className="form-label" htmlFor="w-kpi-col">
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
                          <div style={{ display: 'grid', gap: '1rem' }}>
                            <div className="form-field">
                              <label className="form-label" htmlFor="w-chart-type">
                                Chart Visualization Type
                              </label>
                              <Select
                                id="w-chart-type"
                                value={configChartType}
                                onChange={(val: any) => setConfigChartType(val)}
                                options={[
                                  { value: 'line', label: '📈 Line Chart' },
                                  { value: 'bar', label: '📊 Bar Chart' },
                                  { value: 'pie', label: '🍕 Pie Chart' },
                                ]}
                              />
                            </div>

                            <div className="form-row" style={{ gap: '0.75rem' }}>
                              <div className="form-field" style={{ flex: 1 }}>
                                <label className="form-label" htmlFor="w-xaxis">
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
                              <div className="form-field" style={{ flex: 1 }}>
                                <label className="form-label">
                                  Y-Axis Metric Series (Select multiple)
                                </label>
                                <div
                                  style={{
                                    maxHeight: '100px',
                                    overflowY: 'auto',
                                    border: '1px solid var(--border)',
                                    padding: '0.4rem',
                                    borderRadius: '0.4rem',
                                  }}
                                >
                                  {availableColumns.map((col) => {
                                    const checked = yAxisColumns.includes(col)
                                    return (
                                      <label
                                        key={col}
                                        style={{
                                          display: 'flex',
                                          gap: '0.4rem',
                                          alignItems: 'center',
                                          fontSize: '0.8rem',
                                          padding: '0.1rem 0',
                                          cursor: 'pointer',
                                        }}
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
                          <div className="form-field">
                            <label className="form-label">Visible Table Columns</label>
                            <div
                              style={{
                                display: 'flex',
                                flexWrap: 'wrap',
                                gap: '0.8rem',
                                border: '1px solid var(--border)',
                                padding: '0.6rem',
                                borderRadius: '0.4rem',
                              }}
                            >
                              {availableColumns.map((col) => {
                                const checked = visibleColumns.includes(col)
                                return (
                                  <label
                                    key={col}
                                    style={{
                                      display: 'flex',
                                      gap: '0.4rem',
                                      alignItems: 'center',
                                      fontSize: '0.8rem',
                                      cursor: 'pointer',
                                    }}
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
              </div>
              <div className="modal-footer">
                <button
                  type="button"
                  className="btn btn-secondary"
                  onClick={() => setIsConfigModalOpen(false)}
                >
                  Cancel
                </button>
                <button
                  type="button"
                  className="btn btn-primary"
                  onClick={handleSaveConfig}
                  disabled={configType !== 'text' && !selQuestionId}
                >
                  Apply Configuration
                </button>
              </div>
            </div>
          </div>
        )}
      </LoadingOverlay>
    </div>
  )
}

type ChartRow = Record<string, unknown>

function WidgetRenderer({ widget }: { widget: Widget }) {
  const { postData, loading, error } = useApi()
  const [data, setData] = useState<ChartRow[] | null>(null)
  const [columns, setColumns] = useState<QueryResultPayload['columns']>([])

  useEffect(() => {
    if (widget.type === 'text') {
      return
    }

    if (!widget.logical_query) {
      setData(null)
      return
    }

    // Execute Saved logical_query
    postData<QueryResultPayload>('/api/query/run', widget.logical_query).then((res) => {
      if (res?.rows && res.columns) {
        setColumns(res.columns)
        const mapped = res.rows.map((row) => {
          const obj: ChartRow = {}
          res.columns.forEach((col, idx) => {
            obj[col.name] = row[idx]
          })
          return obj
        })
        setData(mapped)
      } else {
        setData([])
      }
    })
  }, [widget.logical_query, widget.type, postData])

  if (widget.type === 'text') {
    return (
      <div
        style={{
          padding: '0.5rem',
          whiteSpace: 'pre-wrap',
          overflow: 'auto',
          height: '100%',
          fontSize: '0.95rem',
          lineHeight: '1.5',
        }}
      >
        {widget.content ?? <span style={{ color: 'var(--text-muted)' }}>Empty text widget.</span>}
      </div>
    )
  }

  if (!widget.saved_query_id) {
    return (
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          height: '100%',
          minHeight: '120px',
          color: 'var(--text-muted)',
          fontSize: '0.85rem',
        }}
      >
        ⚠️ Not configured yet. Edit settings to link data.
      </div>
    )
  }

  if (loading) {
    return (
      <div
        style={{
          display: 'flex',
          flexDirection: 'column',
          gap: '0.5rem',
          alignItems: 'center',
          justifyContent: 'center',
          height: '100%',
          minHeight: '120px',
        }}
      >
        <span
          className="status-dot"
          style={{ background: 'var(--accent)', animation: 'pulse 1.5s infinite' }}
        />
        <span style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>Executing query...</span>
      </div>
    )
  }

  if (error) {
    return (
      <div
        style={{
          color: 'var(--error)',
          padding: '1rem',
          height: '100%',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          fontSize: '0.85rem',
        }}
      >
        ⚠️ Error: {error}
      </div>
    )
  }

  if (!data || data.length === 0) {
    return (
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          height: '100%',
          minHeight: '120px',
          color: 'var(--text-muted)',
          fontSize: '0.85rem',
        }}
      >
        No data returned.
      </div>
    )
  }

  if (widget.type === 'kpi') {
    const valCol = widget.config?.valueColumn ?? columns[0]?.name ?? ''
    const val = valCol && data[0]?.[valCol] !== undefined ? data[0][valCol] : 'N/A'
    let formattedVal: string | number = 'N/A'
    if (typeof val === 'number') {
      formattedVal =
        val % 1 !== 0
          ? val.toLocaleString(undefined, {
              minimumFractionDigits: 2,
              maximumFractionDigits: 2,
            })
          : val.toLocaleString()
    } else if (val !== null && val !== undefined && typeof val !== 'object') {
      formattedVal = String(val)
    } else if (typeof val === 'object') {
      formattedVal = JSON.stringify(val)
    }
    return (
      <div
        style={{
          display: 'flex',
          flexDirection: 'column',
          justifyContent: 'center',
          height: '100%',
          minHeight: '100px',
          padding: '0.5rem',
        }}
      >
        <span
          style={{
            fontSize: '2.2rem',
            fontWeight: 800,
            color: 'var(--accent)',
            wordBreak: 'break-all',
          }}
        >
          {formattedVal}
        </span>
        <span style={{ fontSize: '0.8rem', color: 'var(--text-muted)', marginTop: '0.2rem' }}>
          Latest value of {valCol}
        </span>
      </div>
    )
  }

  if (widget.type === 'chart') {
    const xKey = widget.config?.xAxisColumn ?? columns[0]?.name
    const yKeys = widget.config?.yAxisColumns ?? []

    if (!xKey || yKeys.length === 0) {
      return (
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            height: '100%',
            minHeight: '120px',
            color: 'var(--text-muted)',
            fontSize: '0.85rem',
          }}
        >
          ⚠️ Please map chart dimensions and metrics.
        </div>
      )
    }

    if (widget.chart_type === 'pie') {
      const pieKey = yKeys[0]
      const pieData = data.map((row, idx) => ({
        name: String(row[xKey] ?? 'Other'),
        value: Number(row[pieKey!]) || 0,
        fill: chartColor(idx),
      }))

      return (
        <div style={{ width: '100%', height: '100%', minHeight: '200px' }}>
          <ResponsiveContainer width="100%" height="100%">
            <PieChart>
              <Pie
                data={pieData}
                dataKey="value"
                nameKey="name"
                cx="50%"
                cy="50%"
                outerRadius="80%"
                label
              />
              <Tooltip contentStyle={chartTooltipStyle} />
              <Legend />
            </PieChart>
          </ResponsiveContainer>
        </div>
      )
    }

    return (
      <div style={{ width: '100%', height: '100%', minHeight: '200px' }}>
        <ResponsiveContainer width="100%" height="100%">
          {widget.chart_type === 'bar' ? (
            <BarChart data={data} margin={{ top: 5, right: 5, left: 0, bottom: 5 }}>
              <CartesianGrid strokeDasharray="3 3" stroke={chartGridStroke} />
              <XAxis dataKey={xKey} stroke={chartAxisStroke} tick={{ fontSize: 10 }} />
              <YAxis stroke={chartAxisStroke} tick={{ fontSize: 10 }} />
              <Tooltip contentStyle={chartTooltipStyle} />
              <Legend />
              {yKeys.map((key, idx) => (
                <Bar key={key} dataKey={key} fill={chartColor(idx)} radius={[4, 4, 0, 0]} />
              ))}
            </BarChart>
          ) : (
            <LineChart data={data} margin={{ top: 5, right: 5, left: 0, bottom: 5 }}>
              <CartesianGrid strokeDasharray="3 3" stroke={chartGridStroke} />
              <XAxis dataKey={xKey} stroke={chartAxisStroke} tick={{ fontSize: 10 }} />
              <YAxis stroke={chartAxisStroke} tick={{ fontSize: 10 }} />
              <Tooltip contentStyle={chartTooltipStyle} />
              <Legend />
              {yKeys.map((key, idx) => (
                <Line
                  key={key}
                  type="monotone"
                  dataKey={key}
                  stroke={chartColor(idx)}
                  strokeWidth={2}
                  dot={{ r: 3 }}
                />
              ))}
            </LineChart>
          )}
        </ResponsiveContainer>
      </div>
    )
  }

  if (widget.type === 'table') {
    const showCols = widget.config?.visibleColumns ?? columns.map((col) => col.name)
    if (showCols.length === 0) {
      return (
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            height: '100%',
            minHeight: '120px',
            color: 'var(--text-muted)',
            fontSize: '0.85rem',
          }}
        >
          ⚠️ Please select columns to display.
        </div>
      )
    }

    return (
      <div className="results-table-scroll" style={{ maxHeight: '100%', overflowY: 'auto' }}>
        <table className="results-table" style={{ fontSize: '0.85rem', width: '100%' }}>
          <thead>
            <tr>
              {showCols.map((col) => (
                <th key={col} style={{ padding: '0.5rem' }}>
                  {col}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {data.slice(0, 100).map((row, idx) => (
              <tr key={idx}>
                {showCols.map((col) => {
                  const val = row[col]
                  let displayVal: React.ReactNode
                  if (typeof val === 'object' && val !== null) {
                    displayVal = JSON.stringify(val)
                  } else if (val === null || val === undefined) {
                    displayVal = <em style={{ color: 'var(--text-muted)' }}>null</em>
                  } else {
                    displayVal = String(val)
                  }
                  return (
                    <td key={col} style={{ padding: '0.4rem 0.5rem' }}>
                      {displayVal}
                    </td>
                  )
                })}
              </tr>
            ))}
          </tbody>
        </table>
        {data.length > 100 && (
          <p
            style={{
              fontSize: '0.75rem',
              color: 'var(--text-muted)',
              textAlign: 'center',
              margin: '0.5rem 0 0 0',
            }}
          >
            Showing top 100 rows
          </p>
        )}
      </div>
    )
  }

  return null
}
