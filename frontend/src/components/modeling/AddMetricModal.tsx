import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useT } from '../../i18n'
import type { ColumnRow, SemanticModelDetail, TableRow } from '../../types/semantic'
import { Modal } from '../ui/Modal'
import { Select } from '../ui/Select'

const METRIC_AGGREGATION_OPTIONS = [
  { value: 'count', label: 'count' },
  { value: 'count_distinct', label: 'count_distinct' },
  { value: 'sum', label: 'sum' },
  { value: 'avg', label: 'avg' },
  { value: 'min', label: 'min' },
  { value: 'max', label: 'max' },
] as const

export interface AddMetricModalProps {
  model: SemanticModelDetail
  includedTables: TableRow[]
  columns: ColumnRow[]
  onClose: () => void
  onCreated: () => void | Promise<void>
  postData: (url: string, body: unknown) => Promise<unknown>
  t: ReturnType<typeof useT>
}

export function AddMetricModal({ model, includedTables, columns, onClose, onCreated, postData, t }: AddMetricModalProps) {
  const [name, setName] = useState('')
  const [label, setLabel] = useState('')
  const [mode, setMode] = useState<'simple' | 'custom'>('simple')
  const [saving, setSaving] = useState(false)

  // Simple Mode state
  const [selectedSchema, setSelectedSchema] = useState(model.base_schema)
  const [selectedTable, setSelectedTable] = useState(model.base_table)
  const [selectedColumn, setSelectedColumn] = useState('')
  const [selectedAggregation, setSelectedAggregation] = useState<'count' | 'sum' | 'avg' | 'min' | 'max' | 'count_distinct'>('sum')
  const [format, setFormat] = useState('')

  // Custom Mode state
  const [expression, setExpression] = useState('')

  // Get active tables in model
  const modelTableKeys = useMemo(() => {
    const keys = new Set<string>()
    if (model) {
      keys.add(`${model.base_schema}.${model.base_table}`)
      ;(model.joins ?? []).forEach((j) => {
        if (j.is_active !== false) {
          keys.add(`${j.from_schema || model.base_schema}.${j.from_table}`)
          keys.add(`${j.to_schema || model.base_schema}.${j.to_table}`)
        }
      })
    }
    return keys
  }, [model])

  const textareaRef = useRef<HTMLTextAreaElement | null>(null)
  const containerRef = useRef<HTMLDivElement | null>(null)

  const [showSuggestions, setShowSuggestions] = useState(false)
  const [suggestions, setSuggestions] = useState<string[]>([])
  const [suggestionIndex, setSuggestionIndex] = useState(0)
  const [queryStartIdx, setQueryStartIdx] = useState(-1)

  const funcCandidates = useMemo(() => [
    'sum(',
    'avg(',
    'count(',
    'count_distinct(',
    'min(',
    'max(',
    'case',
    'when',
    'then',
    'else',
    'end'
  ], [])

  const fieldCandidates = useMemo(() => {
    const list: string[] = []
    if (model.dimensions) {
      model.dimensions.filter(d => d.is_active).forEach(d => {
        list.push(`[${d.name}]`)
      })
    }
    if (model.metrics) {
      model.metrics.filter(m => m.is_active).forEach(m => {
        list.push(`[${m.name}]`)
      })
    }
    columns.forEach(c => {
      if (modelTableKeys.has(`${c.schema_name}.${c.table_name}`)) {
        list.push(`[${c.table_name}.${c.column_name}]`)
      }
    })
    return Array.from(new Set(list)).sort()
  }, [model, columns, modelTableKeys])

  const checkSuggestions = useCallback((val: string, cursorIndex: number) => {
    const lastOpen = val.lastIndexOf('[', cursorIndex - 1)
    let isInside = false
    if (lastOpen !== -1) {
      const textBetween = val.substring(lastOpen, cursorIndex)
      if (!textBetween.includes(']')) {
        isInside = true
      }
    }

    if (isInside) {
      const query = val.substring(lastOpen, cursorIndex).toLowerCase()
      setQueryStartIdx(lastOpen)
      const filtered = fieldCandidates.filter((cand) => cand.toLowerCase().includes(query))
      if (filtered.length > 0) {
        setSuggestions(filtered)
        setSuggestionIndex(0)
        setShowSuggestions(true)
      } else {
        setShowSuggestions(false)
      }
    } else {
      const wordMatch = val.substring(0, cursorIndex).match(/[a-zA-Z_]+$/)
      if (wordMatch) {
        const word = wordMatch[0].toLowerCase()
        const wordStart = cursorIndex - wordMatch[0].length
        setQueryStartIdx(wordStart)
        const filtered = funcCandidates.filter((f) => f.toLowerCase().startsWith(word))
        if (filtered.length > 0) {
          setSuggestions(filtered)
          setSuggestionIndex(0)
          setShowSuggestions(true)
        } else {
          setShowSuggestions(false)
        }
      } else {
        setShowSuggestions(false)
      }
    }
  }, [fieldCandidates, funcCandidates])

  const insertSuggestion = useCallback((suggestion: string) => {
    const textarea = textareaRef.current
    if (!textarea) return
    const start = queryStartIdx
    const end = textarea.selectionEnd
    const text = textarea.value
    const newValue = text.substring(0, start) + suggestion + text.substring(end)
    setExpression(newValue)
    setShowSuggestions(false)
    setTimeout(() => {
      textarea.focus()
      const newCursorPos = start + suggestion.length
      textarea.setSelectionRange(newCursorPos, newCursorPos)
    }, 0)
  }, [queryStartIdx])

  const highlightExpression = (expr: string) => {
    if (!expr) return null
    const regex = /(\[[^\]]*\]|\bcase\b|\bwhen\b|\bthen\b|\belse\b|\bend\b|\bsum\b|\bavg\b|\bcount\b|\bcount_distinct\b|\bmin\b|\bmax\b)/gi
    const parts = expr.split(regex)
    return parts.map((part, idx) => {
      const partLower = part.toLowerCase()
      if (part.startsWith('[') && part.endsWith(']')) {
        const inner = part.slice(1, -1)
        return (
          <span key={idx} style={{ fontWeight: 'bold' }}>
            <span style={{ color: 'rgba(255, 255, 255, 0.35)' }}>[</span>
            <span style={{ color: 'var(--accent)' }}>{inner}</span>
            <span style={{ color: 'rgba(255, 255, 255, 0.35)' }}>]</span>
          </span>
        )
      }
      if (['case', 'when', 'then', 'else', 'end'].includes(partLower)) {
        return (
          <span key={idx} style={{ color: '#fbbf24', fontWeight: 'bold' }}>
            {part}
          </span>
        )
      }
      if (['sum', 'avg', 'count', 'count_distinct', 'min', 'max'].includes(partLower)) {
        return (
          <span key={idx} style={{ color: '#60a5fa', fontWeight: 'bold' }}>
            {part}
          </span>
        )
      }
      return <span key={idx}>{part}</span>
    })
  }

  const handleTextareaChange = (e: React.ChangeEvent<HTMLTextAreaElement>) => {
    const val = e.target.value
    setExpression(val)
    checkSuggestions(val, e.target.selectionStart)
  }

  const handleTextareaSelect = (e: React.SyntheticEvent<HTMLTextAreaElement>) => {
    const target = e.target as HTMLTextAreaElement
    checkSuggestions(target.value, target.selectionStart)
  }

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (!showSuggestions) return
    if (e.key === 'ArrowDown') {
      e.preventDefault()
      setSuggestionIndex((prev) => (prev + 1) % suggestions.length)
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      setSuggestionIndex((prev) => (prev - 1 + suggestions.length) % suggestions.length)
    } else if (e.key === 'Enter' || e.key === 'Tab') {
      e.preventDefault()
      const selected = suggestions[suggestionIndex]
      if (selected && queryStartIdx !== -1) {
        insertSuggestion(selected)
      }
    } else if (e.key === 'Escape') {
      e.preventDefault()
      setShowSuggestions(false)
    }
  }

  const handleTextareaScroll = (e: React.UIEvent<HTMLTextAreaElement>) => {
    const textarea = e.currentTarget
    const underlay = textarea.parentElement?.querySelector('.expression-editor-underlay') as HTMLElement
    if (underlay) {
      underlay.scrollTop = textarea.scrollTop
      underlay.scrollLeft = textarea.scrollLeft
    }
  }

  useEffect(() => {
    const textarea = textareaRef.current
    if (textarea) {
      const underlay = textarea.parentElement?.querySelector('.expression-editor-underlay') as HTMLElement
      if (underlay) {
        underlay.scrollTop = textarea.scrollTop
        underlay.scrollLeft = textarea.scrollLeft
      }
    }
  }, [expression])

  useEffect(() => {
    const handleOutsideClick = (e: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setShowSuggestions(false)
      }
    }
    document.addEventListener('mousedown', handleOutsideClick)
    return () => {
      document.removeEventListener('mousedown', handleOutsideClick)
    }
  }, [])


  // Simple Mode lists
  const availableSchemas = useMemo(() => {
    const schemas = new Set<string>()
    modelTableKeys.forEach((key) => {
      const parts = key.split('.')
      if (parts.length >= 2 && parts[0]) {
        schemas.add(parts[0])
      }
    })
    return Array.from(schemas).sort()
  }, [modelTableKeys])

  const availableTables = useMemo(() => {
    return includedTables.filter((t) => {
      return t.schema_name === selectedSchema && modelTableKeys.has(`${t.schema_name}.${t.table_name}`)
    })
  }, [includedTables, selectedSchema, modelTableKeys])

  const availableColumns = useMemo(() => {
    return columns.filter((c) => {
      return c.schema_name === selectedSchema && c.table_name === selectedTable
    })
  }, [columns, selectedSchema, selectedTable])

  // Select first table/column when schema/table changes
  useEffect(() => {
    if (availableTables.length > 0) {
      const found = availableTables.find((t) => t.table_name === selectedTable)
      if (!found && availableTables[0]) {
        setSelectedTable(availableTables[0].table_name)
      }
    } else {
      setSelectedTable('')
    }
  }, [selectedSchema, availableTables, selectedTable])

  useEffect(() => {
    if (availableColumns.length > 0) {
      const found = availableColumns.find((c) => c.column_name === selectedColumn)
      if (!found && availableColumns[0]) {
        setSelectedColumn(availableColumns[0].column_name)
      }
    } else {
      setSelectedColumn('')
    }
  }, [selectedTable, availableColumns, selectedColumn])

  // Sync Simple selection to Custom mode if they toggle tabs
  const handleModeChange = (newMode: 'simple' | 'custom') => {
    if (newMode === 'custom' && mode === 'simple' && selectedColumn) {
      const ref = selectedSchema === model.base_schema
        ? `${selectedTable}.${selectedColumn}`
        : `${selectedSchema}.${selectedTable}.${selectedColumn}`
      setExpression(`${selectedAggregation}([${ref}])`)
    }
    setMode(newMode)
  }

  // Custom Mode Table Expand/Collapse state
  const [expandedTable, setExpandedTable] = useState<string | null>(model.base_table)

  // Insertion logic
  const insertTextAtCursor = (text: string) => {
    const textarea = document.getElementById('metric-expression') as HTMLTextAreaElement
    if (!textarea) {
      setExpression((prev) => prev + text)
      return
    }
    const start = textarea.selectionStart
    const end = textarea.selectionEnd
    const prevValue = textarea.value
    const newValue = prevValue.substring(0, start) + text + prevValue.substring(end)
    setExpression(newValue)
    
    let caretPos = start + text.length
    if (text.endsWith('()')) {
      caretPos = start + text.length - 1
    } else if (text.includes('[field]')) {
      caretPos = start + text.indexOf('[field]')
    } else if (text.includes('[amount]')) {
      caretPos = start + text.indexOf('[amount]')
    }
    
    setTimeout(() => {
      textarea.focus()
      textarea.selectionStart = textarea.selectionEnd = caretPos
    }, 0)
  }

  const submit = async () => {
    if (!name.trim()) return

    let finalExpr = ''
    let finalAgg = ''

    if (mode === 'simple') {
      if (!selectedColumn) return
      const ref = selectedSchema === model.base_schema
        ? `${selectedTable}.${selectedColumn}`
        : `${selectedSchema}.${selectedTable}.${selectedColumn}`
      finalExpr = ref
      finalAgg = selectedAggregation
    } else {
      if (!expression.trim()) return
      finalExpr = expression.trim()
      finalAgg = 'custom'
    }

    setSaving(true)
    try {
      await postData(`/api/semantic/models/${model.id}/metrics`, {
        name: name.trim(),
        label: label.trim() || undefined,
        expression: finalExpr,
        aggregation: finalAgg,
        format: format.trim() || undefined,
      })
      await onCreated()
    } finally {
      setSaving(false)
    }
  }

  const customFuncs = [
    { label: 'sum(...)', value: 'sum()' },
    { label: 'avg(...)', value: 'avg()' },
    { label: 'count(...)', value: 'count()' },
    { label: 'count_distinct(...)', value: 'count_distinct()' },
    { label: 'min(...)', value: 'min()' },
    { label: 'max(...)', value: 'max()' },
    { label: 'SumIf (Koşullu Toplam)', value: 'sum(case when [field] = \'değer\' then [amount] else 0 end)' },
    { label: 'CountIf (Koşullu Sayım)', value: 'count(case when [field] = \'değer\' then 1 else null end)' },
  ]

  const customOps = ['+', '-', '*', '/', '(', ')']

  return (
    <Modal
      open
      onClose={onClose}
      closeOnBackdrop={!saving}
      className={mode === 'custom' ? 'modal-card--metric' : 'modal-card--modeling'}
      labelledBy="modeling-add-metric-title"
      title={t('modeling.add_metric_title')}
      >
        <form
          onSubmit={(event) => { event.preventDefault(); void submit() }}
        >
          <div className="modal-form-row">
            <div className="form-group">
              <label htmlFor="metric-name">{t('modeling.metric_name_label')}</label>
              <input
                id="metric-name"
                autoFocus
                value={name}
                onChange={(e) => setName(e.target.value)}
                disabled={saving}
                autoComplete="off"
              />
            </div>
            <div className="form-group">
              <label htmlFor="metric-label">{t('modeling.metric_label_label')}</label>
              <input
                id="metric-label"
                value={label}
                onChange={(e) => setLabel(e.target.value)}
                disabled={saving}
                autoComplete="off"
              />
            </div>
          </div>

          <div className="toggle-group metric-mode-toggle" role="tablist" aria-label={t('modeling.add_metric_title')}>
            <button
              type="button"
              className={`toggle-btn ${mode === 'simple' ? 'active' : ''}`}
              onClick={() => handleModeChange('simple')}
              disabled={saving}
              role="tab"
              aria-selected={mode === 'simple'}
            >
              {t('modeling.simple_metric')}
            </button>
            <button
              type="button"
              className={`toggle-btn ${mode === 'custom' ? 'active' : ''}`}
              onClick={() => handleModeChange('custom')}
              disabled={saving}
              role="tab"
              aria-selected={mode === 'custom'}
            >
              {t('modeling.custom_expression')}
            </button>
          </div>

          {mode === 'simple' ? (
            <>
              <div className="modal-form-row">
                <div className="form-group">
                  <label htmlFor="metric-schema">{t('modeling.pick_schema')}</label>
                  <Select
                    id="metric-schema"
                    name="schema"
                    value={selectedSchema}
                    onChange={(val) => setSelectedSchema(val)}
                    disabled={saving}
                    options={availableSchemas.map((s) => ({ value: s, label: s }))}
                  />
                </div>
                <div className="form-group">
                  <label htmlFor="metric-table">{t('modeling.pick_table')}</label>
                  <Select
                    id="metric-table"
                    name="table"
                    value={selectedTable}
                    onChange={(val) => setSelectedTable(val)}
                    disabled={saving}
                    options={availableTables.map((tbl) => ({ value: tbl.table_name, label: tbl.label || tbl.table_name }))}
                  />
                </div>
              </div>
              <div className="modal-form-row">
                <div className="form-group">
                  <label htmlFor="metric-column">{t('modeling.pick_column')}</label>
                  <Select
                    id="metric-column"
                    name="column"
                    value={selectedColumn}
                    onChange={(val) => setSelectedColumn(val)}
                    disabled={saving}
                    options={availableColumns.map((col) => ({ value: col.column_name, label: `${col.column_name} (${col.data_type})` }))}
                  />
                </div>
                <div className="form-group">
                  <label htmlFor="metric-aggregation">{t('modeling.metric_aggregation_label')}</label>
                  <Select
                    id="metric-aggregation"
                    name="aggregation"
                    value={selectedAggregation}
                    onChange={(value) => setSelectedAggregation(value as typeof selectedAggregation)}
                    disabled={saving}
                    options={[...METRIC_AGGREGATION_OPTIONS]}
                  />
                </div>
              </div>
            </>
          ) : (
            <>
              <div className="form-group">
                <label htmlFor="metric-expression">{t('modeling.metric_expression_label')}</label>
                <div ref={containerRef} className="expression-editor-container">
                  <pre className="expression-editor-underlay">{highlightExpression(expression)}</pre>
                  <textarea
                    ref={textareaRef}
                    id="metric-expression"
                    className="expression-editor-textarea"
                    value={expression}
                    onChange={handleTextareaChange}
                    onSelect={handleTextareaSelect}
                    onKeyDown={handleKeyDown}
                    onScroll={handleTextareaScroll}
                    placeholder="sum([orders.total_amount]) / sum([orders.quantity])"
                    autoComplete="off"
                    rows={3}
                    spellCheck={false}
                    disabled={saving}
                  />
                  {showSuggestions && suggestions.length > 0 && (
                    <div
                      className="autocomplete-dropdown"
                      style={{
                        position: 'absolute',
                        bottom: 'auto',
                        top: '100%',
                        left: '0',
                        background: 'var(--bg-card-raised)',
                        border: '1px solid var(--border-strong)',
                        borderRadius: '0.5rem',
                        boxShadow: 'var(--shadow-lg, 0 10px 25px -5px rgba(0, 0, 0, 0.3), 0 8px 10px -6px rgba(0, 0, 0, 0.3))',
                        zIndex: 10,
                        maxHeight: '150px',
                        overflowY: 'auto',
                        width: '320px',
                        padding: '0.25rem',
                        backdropFilter: 'blur(12px)',
                        WebkitBackdropFilter: 'blur(12px)',
                        marginTop: '0.25rem',
                      }}
                    >
                      <div style={{ fontSize: '0.68rem', color: 'var(--text-secondary)', padding: '0.25rem 0.4rem', borderBottom: '1px solid var(--border)', marginBottom: '0.25rem' }}>
                        {t('modeling.metric_intellisense_hint')}
                      </div>
                      {suggestions.map((s, idx) => {
                        const isSelected = idx === suggestionIndex
                        return (
                          <div
                            key={s}
                            style={{
                              padding: '0.35rem 0.5rem',
                              borderRadius: '0.3rem',
                              cursor: 'pointer',
                              fontSize: '0.78rem',
                              fontFamily: 'var(--font-mono, monospace)',
                              color: isSelected ? '#ffffff' : 'var(--text-primary)',
                              background: isSelected ? 'var(--accent)' : 'transparent',
                            }}
                            onClick={() => insertSuggestion(s)}
                          >
                            {s}
                          </div>
                        )
                      })}
                    </div>
                  )}
                </div>
              </div>

              <div className="metric-helper-grid">
                <div className="metric-helper-pane">
                  <h4>{t('modeling.helper_fields')}</h4>
                  <div style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem' }}>
                    {(model.dimensions ?? []).length > 0 && (
                      <div>
                        <div style={{ fontSize: '0.72rem', fontWeight: 600, color: 'var(--text-secondary)', marginBottom: '0.2rem' }}>
                          {t('modeling.tab_short_dim')}
                        </div>
                        <div className="metric-helper-list">
                          {(model.dimensions ?? []).filter(d => d.is_active).map((d) => (
                            <button
                              key={d.id}
                              type="button"
                              className="metric-helper-badge"
                              onClick={() => insertTextAtCursor(`[${d.name}]`)}
                              title={d.column_ref}
                            >
                              {d.label || d.name}
                            </button>
                          ))}
                        </div>
                      </div>
                    )}

                    {(model.metrics ?? []).length > 0 && (
                      <div>
                        <div style={{ fontSize: '0.72rem', fontWeight: 600, color: 'var(--text-secondary)', marginBottom: '0.2rem' }}>
                          {t('modeling.tab_short_metric')}
                        </div>
                        <div className="metric-helper-list">
                          {(model.metrics ?? []).filter(m => m.is_active).map((m) => (
                            <button
                              key={m.id}
                              type="button"
                              className="metric-helper-badge"
                              onClick={() => insertTextAtCursor(`[${m.name}]`)}
                              title={`${m.aggregation}(${m.expression})`}
                            >
                              {m.label || m.name}
                            </button>
                          ))}
                        </div>
                      </div>
                    )}

                    <div>
                      <div style={{ fontSize: '0.72rem', fontWeight: 600, color: 'var(--text-secondary)', marginBottom: '0.2rem' }}>
                        {t('modeling.tab_short_tables')}
                      </div>
                      {Array.from(modelTableKeys).map((tableKey) => {
                        const parts = tableKey.split('.')
                        const schemaName = parts[0]
                        const tableName = parts[1]
                        const tbl = includedTables.find((t) => t.schema_name === schemaName && t.table_name === tableName)
                        const tableLabel = tbl?.label || tableName
                        const isExpanded = expandedTable === tableKey
                        
                        const tableCols = columns.filter((c) => c.schema_name === schemaName && c.table_name === tableName)

                        return (
                          <div key={tableKey} style={{ marginBottom: '0.2rem' }}>
                            <div
                              className="metric-helper-table-header"
                              onClick={() => setExpandedTable(isExpanded ? null : tableKey)}
                            >
                              <span>{tableLabel}</span>
                              <span style={{ fontSize: '0.7rem', color: 'var(--text-secondary)' }}>
                                {isExpanded ? '▼' : '▶'}
                              </span>
                            </div>
                            {isExpanded && (
                              <div className="metric-helper-table-columns">
                                {tableCols.map((c) => (
                                  <button
                                    key={c.id}
                                    type="button"
                                    className="metric-helper-badge"
                                    onClick={() => {
                                      insertTextAtCursor(`[${tableName}.${c.column_name}]`)
                                    }}
                                    title={`${c.column_name} (${c.data_type})`}
                                  >
                                    {c.column_name}
                                  </button>
                                ))}
                              </div>
                            )}
                          </div>
                        )
                      })}
                    </div>
                  </div>
                </div>

                <div className="metric-helper-pane">
                  <h4>{t('modeling.helper_funcs')}</h4>
                  <div className="metric-helper-list" style={{ flexDirection: 'column', gap: '0.35rem', alignItems: 'stretch' }}>
                    {customFuncs.map((f, idx) => (
                      <button
                        key={idx}
                        type="button"
                        className="metric-helper-badge"
                        style={{ justifyContent: 'flex-start', textAlign: 'left' }}
                        onClick={() => insertTextAtCursor(f.value)}
                      >
                        {f.label}
                      </button>
                    ))}
                  </div>

                  <h4 style={{ marginTop: '0.5rem' }}>{t('modeling.helper_ops')}</h4>
                  <div className="metric-helper-list">
                    {customOps.map((op, idx) => (
                      <button
                        key={idx}
                        type="button"
                        className="metric-helper-badge"
                        onClick={() => insertTextAtCursor(op)}
                      >
                        {op}
                      </button>
                    ))}
                  </div>
                </div>
              </div>
            </>
          )}

          <div className="form-group">
            <label htmlFor="metric-format">{t('modeling.metric_format_label')}</label>
            <input
              id="metric-format"
              value={format}
              onChange={(e) => setFormat(e.target.value)}
              disabled={saving}
              placeholder="$#,##0.00"
              autoComplete="off"
            />
          </div>
          <div className="modal-actions">
          <button className="btn btn-secondary" type="button" onClick={onClose} disabled={saving}>
            {t('common.cancel')}
          </button>
          <button
            className="btn btn-primary"
            type="submit"
            disabled={saving || !name.trim() || (mode === 'simple' ? !selectedColumn : !expression.trim())}
          >
            {saving ? t('common.saving') : t('common.create')}
          </button>
        </div>
      </form>
    </Modal>
  )
}
